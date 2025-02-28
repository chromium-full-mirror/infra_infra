// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	schedukeapi "go.chromium.org/chromiumos/config/go/test/scheduling"
	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	luciauth "go.chromium.org/luci/server/auth"
)

const (
	dmExperiment = "dm"
	// SchedukeDevPool is the pool that the Scheduke dev instance schedules on.
	SchedukeDevPool = "schedukeTest"
)

var (
	schedukeDevURL                  = "https://front-door-2q7tjgq5za-wl.a.run.app"
	schedukeProdURL                 = "https://front-door-4vl5zcgwzq-wl.a.run.app"
	schedukeExecutionEndpoint       = "tasks/add"
	schedukeGetExecutionEndpoint    = "tasks"
	schedukeCancelExecutionEndpoint = "tasks/cancel"
	maxHTTPRetries                  = 5
)

type SchedukeClient struct {
	baseURL                          string
	gerritClient, schedukeHTTPClient *http.Client
	ctx                              context.Context
	dmPools                          []string
	blockedPools                     []string
}

// NewSchedukeClientForCLI returns a Scheduke client that can be called from a
// CLI that talks to the given Scheduke environment (dev/prod), and uses the
// given auth info to determine whether the CLI is being used by a human or not.
func NewSchedukeClientForCLI(ctx context.Context, dev bool, authOpts auth.Options) (*SchedukeClient, error) {
	// Set up Gerrit client.
	gc, err := SilentLoginHTTPClient(ctx, authOpts)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: setting up Gerrit client").Err()
	}
	dmPools, err := GetPoolsFromURL(ctx, gc, DmPoolsURL)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: failed to fetch dm pools").Err()
	}
	blockedPools, err := GetPoolsFromURL(ctx, gc, BlockedPoolsURL)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: failed to fetch blocked pools").Err()
	}

	// Determine Scheduke instance to send requests to.
	baseURL := schedukeProdURL
	if dev {
		baseURL = schedukeDevURL
	}

	// Set up Scheduke client.
	var sc *http.Client
	schedukeAuthOpts := withIDTokenAudience(baseURL, authOpts)
	sc, err = SilentLoginHTTPClient(ctx, schedukeAuthOpts)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: setting up Scheduke HTTP client").Err()
	}

	s := SchedukeClient{
		ctx:                ctx,
		baseURL:            baseURL,
		gerritClient:       gc,
		schedukeHTTPClient: sc,
		dmPools:            dmPools,
		blockedPools:       blockedPools,
	}

	// Ping Scheduke base URL to confirm IAM works; don't use exponential backoff
	// here so that we return errors quickly to the end-user.
	if _, err = s.makeRequest(http.MethodGet, s.baseURL, nil, false); err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: confirming Scheduke auth").Err()
	}

	return &s, err
}

// NewSchedukeClientForLUCIExe returns a Scheduke client that can be called from
// luciexe code running on a Buildbucket build.
func NewSchedukeClientForLUCIExe(ctx context.Context, pool string) (*SchedukeClient, error) {
	// Set up Gerrit client.
	gerritAuthOpts := chromeinfra.SetDefaultAuthOptions(auth.Options{
		Scopes: GerritAuthScopes,
	})
	gc, err := SilentLoginHTTPClient(ctx, gerritAuthOpts)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForLUCIExe: setting up Gerrit client").Err()
	}
	dmPools, err := GetPoolsFromURL(ctx, gc, DmPoolsURL)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: failed to fetch dm pools").Err()
	}
	blockedPools, err := GetPoolsFromURL(ctx, gc, BlockedPoolsURL)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: failed to fetch blocked pools").Err()
	}

	// Determine Scheduke instance to send requests to.
	baseURL := schedukeProdURL
	if pool == SchedukeDevPool {
		baseURL = schedukeDevURL
	}

	// Set up Scheduke client.
	schedukeAuthOpts := chromeinfra.SetDefaultAuthOptions(auth.Options{
		UseIDTokens: true,
		Audience:    baseURL,
	})
	sc, err := SilentLoginHTTPClient(ctx, schedukeAuthOpts)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForLUCIExe: setting up Scheduke HTTP client").Err()
	}

	return &SchedukeClient{
		ctx:                ctx,
		baseURL:            baseURL,
		gerritClient:       gc,
		schedukeHTTPClient: sc,
		dmPools:            dmPools,
		blockedPools:       blockedPools,
	}, nil
}

// NewSchedukeClientForGCP returns a Scheduke client that can be called from a
// GCP environment.
func NewSchedukeClientForGCP(ctx context.Context, pool string) (*SchedukeClient, error) {
	// Set Up Gerrit client.
	gerritRPCOpts := luciauth.WithScopes(GerritAuthScopes...)
	gc, err := GCPHTTPClient(ctx, gerritRPCOpts)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForGCP: seeting up Gerrit client").Err()
	}
	dmPools, err := GetPoolsFromURL(ctx, gc, DmPoolsURL)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: failed to fetch dm pools").Err()
	}
	blockedPools, err := GetPoolsFromURL(ctx, gc, BlockedPoolsURL)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForCLI: failed to fetch blocked pools").Err()
	}

	// Determine Scheduke instance to send requests to.
	baseURL := schedukeProdURL
	if pool == SchedukeDevPool {
		baseURL = schedukeDevURL
	}

	// Set up Scheduke client.
	schedukeRPCOpts := luciauth.WithIDTokenAudience(baseURL)
	sc, err := GCPHTTPClient(ctx, schedukeRPCOpts)
	if err != nil {
		return nil, errors.Annotate(err, "NewSchedukeClientForGCP: setting up Scheduke HTTP client").Err()
	}

	return &SchedukeClient{
		ctx:                ctx,
		baseURL:            baseURL,
		gerritClient:       gc,
		schedukeHTTPClient: sc,
		dmPools:            dmPools,
		blockedPools:       blockedPools,
	}, nil
}

func (s *SchedukeClient) parseSchedukeRequestResponse(response *http.Response) (*schedukeapi.CreateTaskStatesResponse, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Annotate(err, "parsing response").Err()
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.Reason("Scheduke server responsonse was not OK: %s", body).Err()
	}

	result := &schedukeapi.CreateTaskStatesResponse{}
	if err := proto.Unmarshal(body, result); err != nil {
		return nil, errors.Annotate(err, "unmarshal response").Err()
	}
	return result, nil

}

func (s *SchedukeClient) parseReadResponse(response *http.Response) (*schedukeapi.ReadTaskStatesResponse, error) {
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Annotate(err, "parsing response").Err()
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.Reason("Scheduke server responsonse was not OK: %s", body).Err()
	}
	result := &schedukeapi.ReadTaskStatesResponse{}
	if err := proto.Unmarshal(body, result); err != nil {
		return nil, errors.Annotate(err, "unmarshal response").Err()
	}
	return result, nil
}

// ScheduleExecution will schedule TR executions via scheduke.
func (s *SchedukeClient) ScheduleExecution(req *schedukeapi.KeyedTaskRequestEvents) (*schedukeapi.CreateTaskStatesResponse, error) {
	var pools []string
	for _, e := range req.GetEvents() {
		resolvePool(e)
		pools = append(pools, e.Pool)
	}
	poolsBlocked, err := AnyStringInGerritList(s.ctx, s.gerritClient, pools, BlockedPoolsURL, s.blockedPools)
	if err != nil {
		return nil, err
	}
	if poolsBlocked {
		return nil, fmt.Errorf("leasing is currently blocked for pools %s; try again later", pools)
	}

	endpoint, err := url.JoinPath(s.baseURL, schedukeExecutionEndpoint)
	if err != nil {
		return nil, errors.Annotate(err, "url.joinpath").Err()
	}

	data, err := protojson.Marshal(req)
	if err != nil {
		return nil, errors.Annotate(err, "marshal request").Err()
	}
	response, err := s.makeRequest(http.MethodPost, endpoint, bytes.NewReader(data), true)
	if err != nil {
		return nil, errors.Annotate(err, "HttpPost").Err()
	}
	return s.parseSchedukeRequestResponse(response)
}

// makeRequest makes the given HTTP request and returns an error if the response
// was not 200.
func (s *SchedukeClient) makeRequest(method string, url string, body io.Reader, useBackoff bool) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, errors.Annotate(err, "creating new HTTP request").Err()
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	r, err := sendHTTPRequestWithRetries(s.schedukeHTTPClient, req, useBackoff)
	if err != nil {
		return nil, errors.Annotate(err, "executing HTTP request").Err()
	}
	if r.StatusCode != 200 {
		if r.StatusCode == 400 || r.StatusCode == 401 || r.StatusCode == 403 {
			return nil, fmt.Errorf("scheduke returned %d; if this error persists, see http://go/crosfleet#obtaining-access)", r.StatusCode)
		}
		return nil, fmt.Errorf("scheduke returned %d", r.StatusCode)
	}
	return r, nil
}

// TestRunnerBBReqToSchedukeReq converts a ScheduleBuildRequest for a
// test_runner BB build to a Scheduke request.
func (s *SchedukeClient) TestRunnerBBReqToSchedukeReq(bbReq *buildbucketpb.ScheduleBuildRequest) (*schedukeapi.KeyedTaskRequestEvents, error) {
	bbReqBytes := []byte(protojson.Format(bbReq))
	compressedReqJSON, err := compressAndEncodeBBReq(bbReqBytes)
	if err != nil {
		return nil, fmt.Errorf("error compressing and encoding ScheduleBuildRequest %v: %w", bbReq, err)
	}
	deadlineStruct, err := getTRDeadlineStruct(bbReq)
	if err != nil {
		return nil, err
	}
	parentBBIDStr, err := getParentBBIDstr(bbReq)
	if err != nil {
		return nil, err
	}
	var parentBBID int64
	// Fail softly if parentBuildId field is not set on the request, as Scheduke
	// only uses this for metadata/logging.
	if parentBBIDStr != "" {
		parentBBID, err = strconv.ParseInt(parentBBIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid parent BBID found on ScheduleBuildRequest %v", bbReq)
		}
	}
	deadline, err := timeFromTimestampPBString(deadlineStruct.GetStringValue())
	if err != nil {
		return nil, fmt.Errorf("error parsing deadline for ScheduleBuildRequest %v: %w", bbReq, err)
	}
	tags := bbReq.GetTags()
	qsAccount := trQSAccount(tags)
	periodic := trBuildIsPeriodic(tags)
	asap := asap(qsAccount, periodic)
	dims, deviceName, pool := dimensionsDeviceNameAndPool(bbReq.GetDimensions())

	var experiments []string
	useDM, err := ShouldUseDM(s.ctx, s.gerritClient, pool, s.dmPools...)
	if err != nil {
		return nil, fmt.Errorf("error checking whether to use DM for pool %s: %w", pool, err)
	}
	if useDM {
		experiments = append(experiments, dmExperiment)
	}

	schedukeTask := &schedukeapi.TaskRequestEvent{
		EventTime:                time.Now().UnixMicro(),
		Deadline:                 deadline.UnixMicro(),
		Periodic:                 periodic,
		Priority:                 trPriority(tags),
		RequestedDimensions:      dims,
		RealExecutionMinutes:     0, // Unneeded outside of shadow mode.
		MaxExecutionMinutes:      30,
		QsAccount:                qsAccount,
		Pool:                     pool,
		Bbid:                     parentBBID,
		Asap:                     asap,
		ScheduleBuildRequestJson: compressedReqJSON,
		DeviceName:               deviceName,
		Experiments:              experiments,
		OsType:                   trOSType(tags),
	}

	return &schedukeapi.KeyedTaskRequestEvents{
		Events: map[int64]*schedukeapi.TaskRequestEvent{
			SchedukeTaskRequestKey: schedukeTask,
		},
	}, nil
}

// AdminTaskReqToSchedukeReq converts a ScheduleBuildRequest for a lab admin
// task BB build to a Scheduke request.
func (s *SchedukeClient) AdminTaskReqToSchedukeReq(bbReq *buildbucketpb.ScheduleBuildRequest, deviceName, pool string) (*schedukeapi.KeyedTaskRequestEvents, error) {
	bbReqBytes := []byte(protojson.Format(bbReq))
	compressedReqJSON, err := compressAndEncodeBBReq(bbReqBytes)
	if err != nil {
		return nil, fmt.Errorf("error compressing and encoding ScheduleBuildRequest %v: %w", bbReq, err)
	}

	now := time.Now()
	schedukeTask := &schedukeapi.TaskRequestEvent{
		EventTime: now.UnixMicro(),
		Deadline:  now.Add(40 * time.Hour).UnixMicro(),
		Periodic:  false,
		// Highest priority for admin tasks.
		Priority: 0,
		// No dimensions needed since all admin tasks specify device name directly.
		RequestedDimensions: nil,
		// Unneeded outside of shadow mode.
		RealExecutionMinutes: 0,
		MaxExecutionMinutes:  60,
		QsAccount:            "admin-task",
		Pool:                 pool,
		Bbid:                 0,
		// ASAP ensures these tasks cut the line and run first.
		Asap:                     true,
		ScheduleBuildRequestJson: compressedReqJSON,
		DeviceName:               deviceName,
		// If running through Scheduke, admin flow tasks always use DM.
		Experiments: []string{dmExperiment},
	}

	return &schedukeapi.KeyedTaskRequestEvents{
		Events: map[int64]*schedukeapi.TaskRequestEvent{
			SchedukeTaskRequestKey: schedukeTask,
		},
	}, nil
}

// LeaseRequest constructs a keyed TaskRequestEvent to request a lease from
// Scheduke with the given dimensions and lease length in minutes, for the given
// user, at the given time.
func (s *SchedukeClient) LeaseRequest(schedukeDims *schedukeapi.SwarmingDimensions, pool, deviceName, user string, mins int64, t time.Time) (*schedukeapi.KeyedTaskRequestEvents, error) {
	useDM, err := ShouldUseDM(s.ctx, s.gerritClient, pool, s.dmPools...)
	if err != nil {
		return nil, err
	}
	var (
		scheduleBuildReqJSON string
		experiments          []string
	)
	if useDM {
		experiments = append(experiments, dmExperiment)
	} else {
		req, err := leaseBBReq(schedukeDims, mins)
		if err != nil {
			return nil, err
		}
		reqByes := []byte(protojson.Format(req))
		scheduleBuildReqJSON, err = compressAndEncodeBBReq(reqByes)
		if err != nil {
			return nil, err
		}
	}

	return &schedukeapi.KeyedTaskRequestEvents{
		Events: map[int64]*schedukeapi.TaskRequestEvent{
			SchedukeTaskKey: {
				EventTime:                t.UnixMicro(),
				Deadline:                 t.Add(leaseSchedulingWindow).UnixMicro(),
				Periodic:                 false,
				Priority:                 leasePriority,
				RequestedDimensions:      schedukeDims,
				RealExecutionMinutes:     mins,
				MaxExecutionMinutes:      mins,
				ScheduleBuildRequestJson: scheduleBuildReqJSON,
				QsAccount:                leasesSchedulingAccount,
				Pool:                     pool,
				Bbid:                     0,
				Asap:                     false,
				TaskStateId:              0,
				DeviceName:               deviceName,
				User:                     user,
				Experiments:              experiments,
			},
		},
	}, nil
}

// ReadTaskStates calls Scheduke to read task states for the given task state
// IDs, users, and/or device names.
func (s *SchedukeClient) ReadTaskStates(taskStateIDs []int64, users, deviceNames []string) (*schedukeapi.ReadTaskStatesResponse, error) {
	readEndpoint, err := url.JoinPath(s.baseURL, schedukeGetExecutionEndpoint)
	if err != nil {
		return nil, errors.Annotate(err, "url.joinpath").Err()
	}

	fullReadURL := fmt.Sprintf("%s?%s", readEndpoint, schedukeParams(taskStateIDs, users, deviceNames))
	r, err := s.makeRequest(http.MethodGet, fullReadURL, nil, true)
	if err != nil {
		return nil, errors.Annotate(err, "executing HTTP request").Err()
	}
	return s.parseReadResponse(r)
}

// CancelTasks calls Scheduke to cancel tasks for the given task state IDs,
// users, and/or device names.
func (s *SchedukeClient) CancelTasks(taskStateIDs []int64, users, deviceNames []string) error {
	cancelEndpoint, err := url.JoinPath(s.baseURL, schedukeCancelExecutionEndpoint)
	if err != nil {
		return errors.Annotate(err, "url.joinpath").Err()
	}

	fullCancelURL := fmt.Sprintf("%s?%s", cancelEndpoint, schedukeParams(taskStateIDs, users, deviceNames))
	_, err = s.makeRequest(http.MethodPost, fullCancelURL, nil, true)
	if err != nil {
		return errors.Annotate(err, "executing HTTP request").Err()
	}
	return nil
}

// ShouldUseDM returns a bool indicating whether a task request with the given
// pool should enable the Device Manager experiment.
func ShouldUseDM(ctx context.Context, c clientThatSendsRequests, pool string, dmPools ...string) (bool, error) {
	return AnyStringInGerritList(ctx, c, []string{pool}, DmPoolsURL, dmPools)
}

// schedukeParams converts a list of task state IDs, users, and device names to
// params for a request to read task states or cancel tasks.
func schedukeParams(taskStateIDs []int64, users, deviceNames []string) string {
	var params []string
	if len(taskStateIDs) > 0 {
		stringIDs := make([]string, len(taskStateIDs))
		for i, num := range taskStateIDs {
			stringIDs[i] = strconv.FormatInt(num, 10)
		}
		params = append(params, fmt.Sprintf("ids=%s", strings.Join(stringIDs, ",")))
	}
	if len(deviceNames) > 0 {
		params = append(params, fmt.Sprintf("device_names=%s", strings.Join(deviceNames, ",")))
	}
	if len(taskStateIDs) == 0 && len(deviceNames) == 0 && len(users) > 0 {
		params = append(params, fmt.Sprintf("users=%s", strings.Join(users, ",")))
	} else {
		params = append(params, fmt.Sprintf("user-info=%s", strings.Join(users, ",")))
	}
	return strings.Join(params, "&")
}
