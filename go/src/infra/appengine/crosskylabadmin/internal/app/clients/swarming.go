// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//
// package clients exports wrappers for client side bindings for API used by
// crosskylabadmin app. These interfaces provide a way to fake/stub out the API
// calls for tests.
//
// The package is named clients instead of swarming etc because callers often
// need to also reference names from the underlying generated bindings.

package clients

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
)

const (
	// MaxConcurrentSwarmingCalls is the maximum number of concurrent swarming
	// calls made within the context of a single RPC call to this app.
	//
	// There is no per-instance limit (yet).
	MaxConcurrentSwarmingCalls = 20

	// BotIDDimensionKey identifies the swarming dimension containing
	// the ID of BOT
	BotIDDimensionKey = "id"
	// DutIDDimensionKey identifies the swarming dimension containing the ID for
	// the DUT corresponding to a bot.
	DutIDDimensionKey = "dut_id"
	// DutModelDimensionKey identifies the swarming dimension containing the
	// Autotest model label for the DUT.
	DutModelDimensionKey = "label-model"
	// DutPoolDimensionKey identifies the swarming dimension containing the
	// Autotest pool label for the DUT.
	DutPoolDimensionKey = "label-pool"
	// DutOSDimensionKey identifies the swarming dimension containing the
	// OS label for the DUT.
	DutOSDimensionKey = "label-os_type"
	// DutNameDimensionKey identifies the swarming dimension
	// containing the DUT name.
	DutNameDimensionKey = "dut_name"
	// DutStateDimensionKey identifies the swarming dimension containing the
	// autotest DUT state for a bot.
	DutStateDimensionKey = "dut_state"

	// PoolDimensionKey identifies the swarming pool dimension.
	PoolDimensionKey = "pool"
	// SwarmingTimeLayout is the layout used by swarming RPCs to specify timestamps.
	SwarmingTimeLayout = "2006-01-02T15:04:05.999999999"

	// maxSwarmingIterations is a sensible maximum number of iterations for functions that call swarming (possibly with pagination) in a loop.
	maxSwarmingIterations = 3000

	// swarmingQueryLimit is a sensible maximum number of entities to query at a time.
	swarmingQueryLimit = 500
)

// paginationChunkSize is the number of items requested in a single page in
// various Swarming RPC calls.
const paginationChunkSize = 100

// SwarmingClient exposes Swarming client API used by this package.
//
// In prod, a SwarmingClient for interacting with the Swarming service will be
// used. Tests should use a fake.
type SwarmingClient interface {
	ListAliveIdleBotsInPool(c context.Context, pool string, dims strpair.Map) ([]*swarmingv2.BotInfo, error)
	ListAliveBotsInPool(context.Context, string, strpair.Map) ([]*swarmingv2.BotInfo, error)
	ListBotTasks(id string) BotTasksCursor
	ListRecentTasks(c context.Context, tags []string, state swarmingv2.StateQuery, limit int32) ([]*swarmingv2.TaskResultResponse, error)
	ListSortedRecentTasksForBot(c context.Context, botID string, limit int32) ([]*swarmingv2.TaskResultResponse, error)
	CreateTask(c context.Context, name string, args *SwarmingCreateTaskArgs) (string, error)
	GetTaskResult(ctx context.Context, tid string) (*swarmingv2.TaskResultResponse, error)
}

// SwarmingCreateTaskArgs contains the arguments to SwarmingClient.CreateTask.
//
// This struct contains only a small subset of the Swarming task arguments that
// is needed by this app.
type SwarmingCreateTaskArgs struct {
	Cmd []string
	// The task targets a dut with the given bot id.
	BotID string
	// The task targets a dut with the given dut id.
	DutID string
	// If non-empty, the task targets a dut in the given state.
	DutState             string
	DutName              string
	ExecutionTimeoutSecs int32
	ExpirationSecs       int32
	Pool                 string
	Priority             int32
	Tags                 []string
	User                 string
	Realm                string
	ServiceAccount       string
}

type swarmingClientImpl struct {
	botsClient     swarmingv2.BotsClient
	swarmingClient swarmingv2.SwarmingClient
	tasksClient    swarmingv2.TasksClient
}

// NewSwarmingClient returns a SwarmingClient for interaction with the Swarming
// service.
func NewSwarmingClient(c context.Context, host string) (SwarmingClient, error) {
	// The Swarming call to list bots requires special previliges (beyond task
	// trigger privilege) This app is authorized to make those API calls.
	t, err := auth.GetRPCTransport(c, auth.AsSelf)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get RPC transport for host %s", host).Err()
	}
	prpcClient := &prpc.Client{
		C:    &http.Client{Transport: t},
		Host: host,
		Options: &prpc.Options{
			UserAgent: "crosskylabadmin",
		},
	}
	swarmingClient := swarmingv2.NewSwarmingClient(prpcClient)
	botsClient := swarmingv2.NewBotsClient(prpcClient)
	tasksClient := swarmingv2.NewTasksClient(prpcClient)
	return &swarmingClientImpl{
		botsClient:     botsClient,
		swarmingClient: swarmingClient,
		tasksClient:    tasksClient,
	}, nil
}

// ListAliveIdleBotsInPool lists the Swarming bots in the given pool.
//
// Use dims to restrict to dimensions beyond pool.
func (sc *swarmingClientImpl) ListAliveIdleBotsInPool(ctx context.Context, pool string, dims strpair.Map) ([]*swarmingv2.BotInfo, error) {
	dims.Set(PoolDimensionKey, pool)
	dimsPairs := asPairs(dims)

	getRequest := func(cursor string) *swarmingv2.BotsRequest {
		return &swarmingv2.BotsRequest{
			Cursor:     cursor,
			Dimensions: dimsPairs,
			IsDead:     swarmingv2.NullableBool_FALSE,
			IsBusy:     swarmingv2.NullableBool_FALSE,
			Limit:      swarmingQueryLimit,
		}
	}

	cursor := ""
	var out []*swarmingv2.BotInfo

	for i := 0; i < maxSwarmingIterations; i++ {
		resp, err := sc.botsClient.ListBots(ctx, getRequest(cursor))
		if err != nil {
			logging.Errorf(ctx, "botsClient.ListBots failed: %s", err)
			return nil, errors.Reason("failed to list alive and idle bots in pool %s", pool).Err()
		}
		out = append(out, resp.GetItems()...)
		cursor = resp.GetCursor()
		if cursor == "" {
			return out, nil
		}
	}

	return nil, errors.New("internal error in app/clients/swarming.go: we iterated too much over the alive idle bots without encountering an error. Consider raising the limits.")
}

// ListAliveBotsInPool lists the Swarming bots in the given pool.
//
// Use dims to restrict to dimensions beyond pool.
func (sc *swarmingClientImpl) ListAliveBotsInPool(ctx context.Context, pool string, dims strpair.Map) ([]*swarmingv2.BotInfo, error) {
	dims.Set(PoolDimensionKey, pool)
	dimsPairs := asPairs(dims)

	getRequest := func(cursor string) *swarmingv2.BotsRequest {
		return &swarmingv2.BotsRequest{
			Cursor:     cursor,
			Dimensions: dimsPairs,
			IsDead:     swarmingv2.NullableBool_FALSE,
			Limit:      swarmingQueryLimit,
		}
	}

	cursor := ""
	var out []*swarmingv2.BotInfo

	for i := 0; i < maxSwarmingIterations; i++ {
		resp, err := sc.botsClient.ListBots(ctx, getRequest(cursor))
		if err != nil {
			logging.Errorf(ctx, "botsClient.ListBots failed: %s", err)
			return nil, errors.Reason("failed to list alive and idle bots in pool %s", pool).Err()
		}
		out = append(out, resp.GetItems()...)
		cursor = resp.GetCursor()
		if cursor == "" {
			return out, nil
		}
	}

	return nil, errors.New("internal error in app/clients/swarming.go: we iterated too much over the alive bots without encountering an error. Consider raising the limits.")
}

// CreateTask creates a Swarming task.
//
// On success, CreateTask returns the opaque task ID returned by Swarming.
func (sc *swarmingClientImpl) CreateTask(ctx context.Context, name string, args *SwarmingCreateTaskArgs) (string, error) {

	dims, err := convertToDimensions(args)
	if err != nil {
		logging.Errorf(ctx, "convertToDimensions failed: %s", err)
		return "", errors.Reason("Failed to create dimentions").Err()
	}

	req := &swarmingv2.NewTaskRequest{
		EvaluateOnly: false,
		Name:         name,
		Priority:     args.Priority,
		Tags:         args.Tags,
		TaskSlices: []*swarmingv2.TaskSlice{
			{
				ExpirationSecs: args.ExpirationSecs,
				Properties: &swarmingv2.TaskProperties{
					Command:              args.Cmd,
					Dimensions:           dims,
					ExecutionTimeoutSecs: args.ExecutionTimeoutSecs,
					// We never want tasks deduplicated with earlier tasks.
					Idempotent: false,
				},
				// There are no fallback task slices.
				// Wait around until the first slice can run.
				WaitForCapacity: true,
			},
		},
		User:           args.User,
		Realm:          args.Realm,
		ServiceAccount: args.ServiceAccount,
	}
	resp, err := sc.tasksClient.NewTask(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "tasksClient.NewTask failed: ", err)
		return "", errors.Reason("Failed to create task").Err()
	}
	return resp.TaskId, nil
}

// Converts task args to the key-value dimension set.
//
// Minimum required one of BotID, DutID or DutName
func convertToDimensions(args *SwarmingCreateTaskArgs) ([]*swarmingv2.StringPair, error) {
	if args.DutID == "" && args.DutName == "" && args.BotID == "" {
		return nil, errors.Reason("invalid argument: one of (DutID, DutName, BotID) need to be specified").Err()
	}
	dims := []*swarmingv2.StringPair{
		{
			Key:   PoolDimensionKey,
			Value: args.Pool,
		},
	}
	if args.BotID != "" {
		dims = append(dims, &swarmingv2.StringPair{
			Key:   BotIDDimensionKey,
			Value: args.BotID,
		})
	} else if args.DutID != "" {
		dims = append(dims, &swarmingv2.StringPair{
			Key:   DutIDDimensionKey,
			Value: args.DutID,
		})
	} else if args.DutName != "" {
		dims = append(dims, &swarmingv2.StringPair{
			Key:   DutNameDimensionKey,
			Value: args.DutName,
		})
	}
	if args.DutState != "" {
		dims = append(dims, &swarmingv2.StringPair{
			Key:   DutStateDimensionKey,
			Value: args.DutState,
		})
	}
	return dims, nil
}

// GetTaskResult gets the task result for a given task ID.
func (sc *swarmingClientImpl) GetTaskResult(ctx context.Context, tid string) (*swarmingv2.TaskResultResponse, error) {
	resp, err := sc.tasksClient.GetResult(ctx, &swarmingv2.TaskIdWithPerfRequest{
		TaskId: tid,
	})
	if err != nil {
		return nil, errors.Annotate(err, "failed to get result for task %s", tid).Err()
	}
	return resp, nil
}

// ListRecentTasks lists tasks with the given tags and in the given state.
//
// The most recent |limit| tasks are returned.
// state may be left "" to skip filtering by state.
func (sc *swarmingClientImpl) ListRecentTasks(ctx context.Context, tags []string, state swarmingv2.StateQuery, limit int32) ([]*swarmingv2.TaskResultResponse, error) {
	if limit < 0 {
		panic(fmt.Sprintf("limit set to %d which is < 0", limit))
	}

	req := &swarmingv2.TasksWithPerfRequest{
		Tags:  tags,
		Limit: limit,
		State: state,
	}

	resp, err := sc.tasksClient.ListTasks(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "tasksClient.ListTasks failed: %s", err)
		return nil, errors.Reason("failed to list tasks with tags %s", strings.Join(tags, " ")).Err()
	}

	return resp.GetItems(), nil
}

// BotTasksCursor tracks a paginated query for Swarming bot tasks.
type BotTasksCursor interface {
	Next(context.Context, int32) ([]*swarmingv2.TaskResultResponse, error)
}

// botTasksCursorImpl tracks a paginated query for Swarming bot tasks.
type botTasksCursorImpl struct {
	description    string
	botID          string
	cursor         string
	swarmingClient *swarmingClientImpl
	done           bool
}

// Next returns at most the next N tasks from the task cursor.
func (c *botTasksCursorImpl) Next(ctx context.Context, n int32) ([]*swarmingv2.TaskResultResponse, error) {
	if c.done || n < 1 {
		return nil, nil
	}
	resp, err := c.swarmingClient.botsClient.ListBotTasks(ctx, &swarmingv2.BotTasksRequest{
		BotId:  c.botID,
		Limit:  n,
		Cursor: c.cursor,
	})
	if err != nil {
		return nil, err
	}
	c.cursor = resp.GetCursor()
	return resp.GetItems(), nil
}

// ListBotTasks lists the bot's tasks.  Since the query is paginated,
// this function returns a TaskCursor that the caller can iterate on.
func (sc *swarmingClientImpl) ListBotTasks(id string) BotTasksCursor {
	// TODO(pprabhu): These should really be sorted by STARTED_TS.
	// See crbug.com/857595 and crbug.com/857598
	return &botTasksCursorImpl{
		description:    fmt.Sprintf("tasks for bot %s", id),
		botID:          id,
		cursor:         id,
		swarmingClient: sc,
		done:           false,
	}
}

// ListSortedRecentTasksForBot lists the most recent tasks for the bot with
// given dutID.
//
// duration specifies how far in the back are the tasks allowed to have
// started. limit limits the number of tasks returned.
func (sc *swarmingClientImpl) ListSortedRecentTasksForBot(ctx context.Context, botID string, limit int32) ([]*swarmingv2.TaskResultResponse, error) {
	var trs []*swarmingv2.TaskResultResponse
	c := sc.ListBotTasks(botID)
	p := Pager{Remaining: limit}
	for {
		chunk := p.Next()
		if chunk == 0 {
			break
		}
		trs2, err := c.Next(ctx, chunk)
		if err != nil {
			return nil, err
		}
		if len(trs2) == 0 {
			break
		}
		p.Record(int32(len(trs2)))
		trs = append(trs, trs2...)
	}
	return trs, nil
}

// TimeSinceBotTask calls TimeSinceBotTaskN with time.Now().
func TimeSinceBotTask(tr *swarmingv2.TaskResultResponse) (*duration.Duration, error) {
	return TimeSinceBotTaskN(tr, time.Now())
}

// TimeSinceBotTaskN returns the duration.Duration elapsed since the given task
// completed on a bot.
//
// This function only considers tasks that were executed by Swarming to a
// specific bot. For tasks that were never executed on a bot, this function
// returns nil duration.
func TimeSinceBotTaskN(tr *swarmingv2.TaskResultResponse, now time.Time) (*duration.Duration, error) {
	if tr.State == swarmingv2.TaskState_RUNNING {
		return &duration.Duration{}, nil
	}
	t, err := TaskDoneTime(tr)
	if err != nil {
		return nil, errors.Annotate(err, "get time since bot task").Err()
	}
	if t.IsZero() {
		return nil, nil
	}
	return durationpb.New(now.Sub(t)), nil
}

// TaskDoneTime returns the time when the given task completed on a
// bot.  If the task was never run or is still running, this function
// returns a zero time.
func TaskDoneTime(tr *swarmingv2.TaskResultResponse) (time.Time, error) {
	switch tr.State {
	case swarmingv2.TaskState_RUNNING:
		return time.Time{}, nil
	case swarmingv2.TaskState_COMPLETED, swarmingv2.TaskState_TIMED_OUT:
		// TIMED_OUT tasks are considered to have completed as opposed to EXPIRED
		// tasks, which set tr.AbandonedTs
		return tr.CompletedTs.AsTime(), nil
	case swarmingv2.TaskState_KILLED:
		return tr.AbandonedTs.AsTime(), nil
	// TODO(b/317136548): Replace this with a default.
	case swarmingv2.TaskState_BOT_DIED, swarmingv2.TaskState_CANCELED, swarmingv2.TaskState_EXPIRED, swarmingv2.TaskState_NO_RESOURCE, swarmingv2.TaskState_PENDING:
		// These states do not indicate any actual run of a task on the dut.
		return time.Time{}, nil
	default:
		return time.Time{}, errors.Reason("get task done time: unknown task state %s", tr.State).Err()
	}
}

// Pager manages pagination of API calls.
type Pager struct {
	// Remaining is set to the number of items to retrieve.  This
	// can be modified after Pager has been used, but not
	// concurrently.
	Remaining int32
}

// Next returns the number of items to request.  If there are no more
// items to request, returns 0.
func (p *Pager) Next() int32 {
	switch {
	case p.Remaining < 0:
		return 0
	case p.Remaining < paginationChunkSize:
		return p.Remaining
	default:
		return paginationChunkSize
	}
}

// Record records that items have been received (since a request may
// not return the exact number of items requested).
func (p *Pager) Record(n int32) {
	p.Remaining -= n
}

// GetStateDimension gets the dut_state value from a dimension slice.
func GetStateDimension(dims []*swarmingv2.StringListPair) fleet.DutState {
	for _, p := range dims {
		if p.Key != DutStateDimensionKey {
			continue
		}
		if len(p.Value) != 1 {
			return fleet.DutState_DutStateInvalid
		}
		return dutStateMap[p.Value[0]]
	}
	return fleet.DutState_DutStateInvalid
}

// GetStateDimensionV2 gets the dut_state value from a dimension slice.
func GetStateDimensionV2(dims []*swarmingv2.StringListPair) fleet.DutState {
	for _, p := range dims {
		if p.GetKey() != DutStateDimensionKey {
			continue
		}
		if len(p.GetValue()) != 1 {
			return fleet.DutState_DutStateInvalid
		}
		return dutStateMap[p.GetValue()[0]]
	}
	return fleet.DutState_DutStateInvalid
}

// HealthyDutStates is the set of healthy DUT states.
var HealthyDutStates = map[fleet.DutState]bool{
	fleet.DutState_Ready:        true,
	fleet.DutState_NeedsCleanup: true,
	fleet.DutState_NeedsRepair:  true,
	fleet.DutState_NeedsReset:   true,
}

// dutStateMap maps string values to DutState values.  The zero value
// for unknown keys is DutState_StateInvalid.
var dutStateMap = map[string]fleet.DutState{
	"ready":               fleet.DutState_Ready,
	"needs_cleanup":       fleet.DutState_NeedsCleanup,
	"needs_repair":        fleet.DutState_NeedsRepair,
	"needs_reset":         fleet.DutState_NeedsReset,
	"repair_failed":       fleet.DutState_RepairFailed,
	"needs_manual_repair": fleet.DutState_NeedsManualRepair,
	"needs_replacement":   fleet.DutState_NeedsReplacement,
	"needs_deploy":        fleet.DutState_NeedsDeploy,
}

// DutStateRevMap mapping DutState to swarming value representation
var DutStateRevMap = map[fleet.DutState]string{
	fleet.DutState_Ready:             "ready",
	fleet.DutState_NeedsCleanup:      "needs_cleanup",
	fleet.DutState_NeedsRepair:       "needs_repair",
	fleet.DutState_NeedsReset:        "needs_reset",
	fleet.DutState_RepairFailed:      "repair_failed",
	fleet.DutState_NeedsManualRepair: "needs_manual_repair",
	fleet.DutState_NeedsReplacement:  "needs_replacement",
	fleet.DutState_NeedsDeploy:       "needs_deploy",
}

// asPairs converts a map into a sorted slice of pairs.
func asPairs(m strpair.Map) []*swarmingv2.StringPair {
	if len(m) == 0 {
		return nil
	}
	res := make([]*swarmingv2.StringPair, 0, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		values := m[k]
		sortedValues := values[:]
		sort.Strings(sortedValues)
		for _, v := range sortedValues {
			res = append(res, &swarmingv2.StringPair{
				Key:   k,
				Value: v,
			})
		}
	}
	return res
}
