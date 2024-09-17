// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service implements a skylab.Client using calls to BuildBucket.
package service

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/config"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/buildbucket"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingapi "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/lucictx"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"

	"infra/cmd/cros_test_platform/internal/execution/types"
	"infra/cmd/cros_test_platform/internal/execution/vmlab"
	"infra/libs/skylab/request"
	"infra/libs/skylab/swarming"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
)

// TaskReference is an implementation-independent way to identify test_runner tasks.
type TaskReference string

// NewTaskReference creates a unique task reference.
func NewTaskReference() TaskReference {
	return TaskReference(uuid.New().String())
}

// FetchResultsResponse is an implementation-independent container for
// information about running and finished tasks.
type FetchResultsResponse struct {
	Result                      *skylab_test_runner.Result
	LifeCycle                   test_platform.TaskState_LifeCycle
	BuildBucketTransientFailure bool
}

// Client defines an interface used to interact with test_runner as a service.
type Client interface {
	// ValidateArgs validates that a test_runner build can be created with
	// the give arguments.
	ValidateArgs(context.Context, *request.Args) (bool, []types.TaskDimKeyVal, error)

	// CancelTasks cancels all tasks given.
	CancelTasks(ctx context.Context, taskReferences []TaskReference, reason string) error

	// LaunchTask creates a new test_runner task with the given arguments.
	LaunchTask(context.Context, *request.Args) (TaskReference, error)

	// FetchResults fetches results for a previously launched test_runner task.
	FetchResults(context.Context, TaskReference) (*FetchResultsResponse, error)

	// SwarmingTaskID returns the swarming task ID for a test_runner build.
	SwarmingTaskID(TaskReference) string

	// URL returns a canonical URL for a test_runner build.
	URL(TaskReference) string

	CheckFleetTestsPolicy(context.Context, *ufsapi.CheckFleetTestsPolicyRequest, ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error)
}

type task struct {
	bbID           int64
	swarmingTaskID string
	bbStatus       buildbucketpb.Status
}

// clientImpl is a concrete Client implementation to interact with
// test_runner as a service.
type clientImpl struct {
	swarmingClient swarmingClient
	bbClient       buildbucketpb.BuildsClient
	recorderClient resultpb.RecorderClient
	builder        *buildbucketpb.BuilderID
	// builderGce is the corresponding VMLab version to builder
	builderGce *buildbucketpb.BuilderID
	knownTasks map[TaskReference]*task
	ufsClient  ufsapi.FleetClient
}

// Ensure we satisfy the promised interface.
var _ Client = &clientImpl{}

// VersionNumber is the service version number for the cros-test-platform binary.
const VersionNumber = 4

type swarmingClient interface {
	BotExists(context.Context, []*swarmingapi.SwarmingRpcsStringPair) (bool, error)
	CancelTask(ctx context.Context, taskID string) error
}

// NewClient creates a concrete instance of a Client.
func NewClient(ctx context.Context, cfg *config.Config, rdbHost string) (Client, error) {
	sc, err := newSwarmingClient(ctx, cfg.SkylabSwarming)
	if err != nil {
		return nil, errors.Annotate(err, "create test_runner service client").Err()
	}
	bbc, err := newBBClient(ctx, cfg.TestRunner.Buildbucket)
	if err != nil {
		return nil, errors.Annotate(err, "create test_runner service client").Err()
	}

	ufsclient, err := NewUFSClient(ctx)
	rc, err := newRecorderClient(ctx, rdbHost)
	if err != nil {
		return nil, errors.Annotate(err, "create test_runner service client").Err()
	}
	return &clientImpl{
		swarmingClient: sc,
		bbClient:       bbc,
		recorderClient: rc,
		builder: &buildbucketpb.BuilderID{
			Project: cfg.TestRunner.Buildbucket.Project,
			Bucket:  cfg.TestRunner.Buildbucket.Bucket,
			Builder: cfg.TestRunner.Buildbucket.Builder,
		},
		builderGce: &buildbucketpb.BuilderID{
			Project: cfg.TestRunner.Buildbucket.Project,
			Bucket:  cfg.TestRunner.Buildbucket.Bucket,
			Builder: vmlab.ConvertBuilderName(cfg.TestRunner.Buildbucket.Builder),
		},
		knownTasks: make(map[TaskReference]*task),
		ufsClient:  ufsclient,
	}, nil
}

func newBBClient(ctx context.Context, cfg *config.Config_Buildbucket) (buildbucketpb.BuildsClient, error) {
	hClient, err := httpClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "create buildbucket client").Err()
	}
	pClient := &prpc.Client{
		C:    hClient,
		Host: cfg.Host,
	}
	return buildbucketpb.NewBuildsPRPCClient(pClient), nil
}

func newRecorderClient(ctx context.Context, host string) (resultpb.RecorderClient, error) {
	hClient, err := httpClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "create recorder client").Err()
	}
	pClient := &prpc.Client{
		C:    hClient,
		Host: host,
	}
	return resultpb.NewRecorderPRPCClient(pClient), nil
}

// TODO(crbug.com/1115207): dedupe with swarmingHTTPClient.
func httpClient(ctx context.Context) (*http.Client, error) {
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, auth.Options{
		Scopes: []string{auth.OAuthScopeEmail},
	})
	h, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "create http client").Err()
	}
	return h, nil
}

func newSwarmingClient(ctx context.Context, c *config.Config_Swarming) (*swarming.Client, error) {
	logging.Infof(ctx, "Creating swarming client from config %v", c)
	hClient, err := swarmingHTTPClient(ctx, c.AuthJsonPath)
	if err != nil {
		return nil, err
	}

	client, err := swarming.NewClientUpdated(ctx, hClient, c.Server)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// TODO(crbug.com/1115207): dedupe with httpClient
func swarmingHTTPClient(ctx context.Context, authJSONPath string) (*http.Client, error) {
	options := auth.Options{
		ServiceAccountJSONPath: authJSONPath,
		Scopes:                 []string{auth.OAuthScopeEmail},
	}
	a := auth.NewAuthenticator(ctx, auth.SilentLogin, options)
	h, err := a.Client()
	if err != nil {
		return nil, errors.Annotate(err, "create http client").Err()
	}
	return h, nil
}

// NewClient returns a new client to interact with UFS .
func NewUFSClient(ctx context.Context) (ufsapi.FleetClient, error) {
	hClient, err := httpClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "create UFS client").Err()
	}

	options := *prpc.DefaultOptions()
	options.UserAgent = fmt.Sprintf("cros_test_platform/%d", VersionNumber)

	return ufsapi.NewFleetPRPCClient(&prpc.Client{
		C: hClient,
		// TODO: Change Host to be passed in as a command line argument if a non-prod UFS host is needed
		Host:    "ufs.api.cr.dev",
		Options: &options,
	}), nil
}

// ValidateArgs checks whether this test has dependencies satisfied by
// at least one Skylab bot.
//
// Any changes to this implementation should be also reflected in
// rawSwarmingSkylabClient.ValidateArgs
// TODO(crbug.com/1033287): Remove the rawSwarmingSkylabClient implementation.
func (c *clientImpl) ValidateArgs(ctx context.Context, args *request.Args) (botExists bool, rejectedTaskDims []types.TaskDimKeyVal, err error) {
	dims, err := args.StaticDimensions()
	if err != nil {
		err = errors.Annotate(err, "validate dependencies").Err()
		return
	}
	// Take out dut_state dim before checking bot availability
	potentialRejectedTaskDims := []types.TaskDimKeyVal{}
	dimsWithoutDutState := []*swarmingapi.SwarmingRpcsStringPair{}
	for _, dim := range dims {
		if dim.Key != "dut_state" {
			dimsWithoutDutState = append(dimsWithoutDutState, dim)
			potentialRejectedTaskDims = append(potentialRejectedTaskDims, types.TaskDimKeyVal{Key: dim.Key, Val: dim.Value})
		}
	}
	logging.Infof(ctx, "Checking if bot exists for dims: %v", potentialRejectedTaskDims)
	botExists, err = c.swarmingClient.BotExists(ctx, dimsWithoutDutState)
	if err != nil {
		err = errors.Annotate(err, "validate dependencies").Err()
		return
	}
	if !botExists {
		rejectedTaskDims = potentialRejectedTaskDims
		// sort by key, then by val
		sort.Slice(rejectedTaskDims, func(i, j int) bool {
			if rejectedTaskDims[i].Key != rejectedTaskDims[j].Key {
				return rejectedTaskDims[i].Key < rejectedTaskDims[j].Key
			}
			return rejectedTaskDims[i].Val < rejectedTaskDims[j].Val
		})
		logging.Warningf(ctx, "Dependency validation failed for %s: no bot exists with dimensions: %v", args.TestRunnerRequest.GetTest().GetAutotest().GetName(), rejectedTaskDims)
	}
	return
}

// CancelTasks cancels all tasks given.
func (c *clientImpl) CancelTasks(ctx context.Context, taskReferences []TaskReference, reason string) error {
	for _, ref := range taskReferences {
		req := &buildbucketpb.CancelBuildRequest{
			Id:              c.knownTasks[ref].bbID,
			SummaryMarkdown: reason,
		}
		_, err := c.bbClient.CancelBuild(ctx, req)
		if err != nil {
			return err
		}
	}

	return nil
}

// LaunchTask sends an RPC request to start the task.
func (c *clientImpl) LaunchTask(ctx context.Context, args *request.Args) (TaskReference, error) {
	builderId := c.builder
	// VmLab runs CFT via test runner v2, and VmLab rollout is ahead of TRv2 rollout for HW
	shouldRunOnVmLab := vmlab.ShouldRun(args)
	if shouldRunOnVmLab {
		args.CFTTestRunnerRequest.RunViaTrv2 = true
		builderId = c.builderGce
	}
	req, err := args.NewBBRequest(builderId)
	if err != nil {
		return "", errors.Annotate(err, "launch task for %s", args.TestRunnerRequest.GetTest().GetAutotest().GetName()).Err()
	}
	// Clear dimensions in the request as the GCE builders populate all required
	// dimensions for VM tests
	if shouldRunOnVmLab {
		req.Dimensions = make([]*buildbucketpb.RequestedDimension, 0)
	}

	// Check if there's a parent build for the task to be launched.
	// If a ScheduleBuildToken can be found in the Buildbucket section of LUCI_CONTEXT,
	// it will be the token for the parent build.
	// Attaching the token to the ScheduleBuild request will enable Buildbucket to
	// track the parent/child build relationship between the build with the token
	// and this new build.
	bbCtx := lucictx.GetBuildbucket(ctx)
	// Do not attach the buildbucket token if it's empty or the build is a led build.
	// Because led builds are not real Buildbucket builds and they don't have
	// real buildbucket tokens, so we cannot make them  any builds's parent,
	// even for the builds they scheduled.
	if bbCtx != nil && bbCtx.GetScheduleBuildToken() != "" && bbCtx.GetScheduleBuildToken() != buildbucket.DummyBuildbucketToken {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(buildbucket.BuildbucketTokenHeader, bbCtx.ScheduleBuildToken))

		// Decide if the child can outlive its parent or not.
		// For details see https://source.chromium.org/chromium/infra/infra/+/main:go/src/go.chromium.org/luci/buildbucket/proto/builds_service.proto;l=458;drc=79232ce0a0af1f7ab9ae78efa9ab3931a520d2bc.
		if req.GetCanOutliveParent() == buildbucketpb.Trinary_UNSET {
			// We do not want test_runner runs to outrun parent CTP.
			req.CanOutliveParent = buildbucketpb.Trinary_NO
		}
	}

	resp, err := c.bbClient.ScheduleBuild(ctx, req)
	if err != nil {
		return "", errors.Annotate(err, "launch task for %s", args.TestRunnerRequest.GetTest().GetAutotest().GetName()).Err()
	}

	c.inheritResultdbInvocation(ctx, resp.Id)

	tr := NewTaskReference()
	c.knownTasks[tr] = &task{
		bbID:     resp.Id,
		bbStatus: resp.Status,
	}
	return tr, nil
}

// Inherit the test_runner build's ResultDB invocation.
func (c *clientImpl) inheritResultdbInvocation(ctx context.Context, buildId int64) {
	rdbCtx := lucictx.GetResultDB(ctx)

	// Do not attach the resultdb token if it's empty.
	if rdbCtx == nil || rdbCtx.CurrentInvocation.UpdateToken == "" {
		return
	}

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(resultpb.UpdateTokenMetadataKey, rdbCtx.CurrentInvocation.UpdateToken))
	parentInv := rdbCtx.CurrentInvocation.Name

	var inv string
	// The ScheduleBuild response does not populate the invocation, so we need
	// to request it from buildbucket.
	req := &buildbucketpb.GetBuildRequest{
		Id: buildId,
		Fields: &field_mask.FieldMask{Paths: []string{
			"id",
			"infra.resultdb.invocation",
		}},
	}
	b, err := c.bbClient.GetBuild(ctx, req)
	if err != nil && b.GetInfra().GetResultdb().GetInvocation() != "" {
		inv = b.GetInfra().GetResultdb().GetInvocation()
	} else {
		// As a fallback, the ResultDB invocation for the child build can be
		// inferred using the Buildbucket id.
		inv = fmt.Sprintf("invocations/build-%d", buildId)
	}

	rreq := resultpb.UpdateIncludedInvocationsRequest{IncludingInvocation: parentInv, AddInvocations: []string{inv}}
	c.recorderClient.UpdateIncludedInvocations(ctx, &rreq)

}

// getBuildFieldMask is the list of buildbucket fields that are needed.
var getBuildFieldMask = []string{
	"id",
	// TODO(b/40949135): remove after swarming -> backend migration completes.
	"infra.swarming.task_id",
	"infra.backend.task.id.id",
	// Build details are parsed from the build's output properties.
	"output.properties",
	// Build status is used to determine whether the build is complete.
	"status",
}

// FetchResults fetches the latest state and results of the given task.
func (c *clientImpl) FetchResults(ctx context.Context, t TaskReference) (*FetchResultsResponse, error) {
	task, ok := c.knownTasks[t]
	if !ok {
		return nil, errors.Reason("fetch results: could not find task among launched tasks").Err()
	}

	// Check Build status.
	statusReq := &buildbucketpb.GetBuildStatusRequest{
		Id: task.bbID,
	}
	b, err := c.bbClient.GetBuildStatus(ctx, statusReq)
	if err != nil {
		return &FetchResultsResponse{
			nil,
			test_platform.TaskState_LIFE_CYCLE_ABORTED,
			true,
		}, errors.Annotate(err, "fetch results for build %d", task.bbID).Err()
	}

	// Build status changes, call GetBuild to get more info on the build.
	if b.GetStatus() != task.bbStatus {
		req := &buildbucketpb.GetBuildRequest{
			Id:     task.bbID,
			Fields: &field_mask.FieldMask{Paths: getBuildFieldMask},
		}
		b, err = c.bbClient.GetBuild(ctx, req)
		if err != nil {
			return &FetchResultsResponse{
				nil,
				test_platform.TaskState_LIFE_CYCLE_ABORTED,
				true,
			}, errors.Annotate(err, "fetch results for build %d", task.bbID).Err()
		}
		task.bbStatus = b.Status
	}

	task.swarmingTaskID = b.GetInfra().GetBackend().GetTask().GetId().GetId()
	if task.swarmingTaskID == "" {
		// TODO(b/40949135): remove after swarming -> backend migration completes.
		task.swarmingTaskID = b.GetInfra().GetSwarming().GetTaskId()
	}

	lc := bbStatusToLifeCycle[b.Status]
	if !lifeCyclesWithResults[lc] {
		return &FetchResultsResponse{LifeCycle: lc}, nil
	}

	res, err := extractResult(b)
	if err != nil {
		return nil, errors.Annotate(err, "fetch results for build %d", task.bbID).Err()
	}

	return &FetchResultsResponse{
		Result:    res,
		LifeCycle: lc,
	}, nil
}

// CheckFleetTestsPolicy checks the fleet test policy for the given test parameters.
func (c *clientImpl) CheckFleetTestsPolicy(ctx context.Context, req *ufsapi.CheckFleetTestsPolicyRequest, opt ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error) {
	return c.ufsClient.CheckFleetTestsPolicy(ctx, req)
}

// lifeCyclesWithResults lists all task states which have a chance of producing
// test cases results. E.g. this excludes killed tasks.
var lifeCyclesWithResults = map[test_platform.TaskState_LifeCycle]bool{
	test_platform.TaskState_LIFE_CYCLE_COMPLETED: true,
}

var bbStatusToLifeCycle = map[buildbucketpb.Status]test_platform.TaskState_LifeCycle{
	buildbucketpb.Status_SCHEDULED:     test_platform.TaskState_LIFE_CYCLE_PENDING,
	buildbucketpb.Status_STARTED:       test_platform.TaskState_LIFE_CYCLE_RUNNING,
	buildbucketpb.Status_SUCCESS:       test_platform.TaskState_LIFE_CYCLE_COMPLETED,
	buildbucketpb.Status_FAILURE:       test_platform.TaskState_LIFE_CYCLE_COMPLETED,
	buildbucketpb.Status_INFRA_FAILURE: test_platform.TaskState_LIFE_CYCLE_COMPLETED,
	buildbucketpb.Status_CANCELED:      test_platform.TaskState_LIFE_CYCLE_CANCELLED,
}

func extractResult(from *buildbucketpb.Build) (*skylab_test_runner.Result, error) {
	op := from.GetOutput().GetProperties().GetFields()
	if op == nil {
		return nil, nil
	}
	cr := op["compressed_result"].GetStringValue()
	if cr == "" {
		return nil, nil
	}
	pb, err := decompress(cr)
	if err != nil {
		return nil, errors.Annotate(err, "extract results from build %d", from.Id).Err()
	}
	var r skylab_test_runner.Result
	if err := proto.Unmarshal(pb, &r); err != nil {
		return nil, errors.Annotate(err, "extract results from build %d", from.Id).Err()
	}
	return &r, nil
}

func decompress(from string) ([]byte, error) {
	bs, err := base64.StdEncoding.DecodeString(from)
	if err != nil {
		return nil, errors.Annotate(err, "decompress").Err()
	}
	reader, err := zlib.NewReader(bytes.NewReader(bs))
	if err != nil {
		return nil, errors.Annotate(err, "decompress").Err()
	}
	bs, err = ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Annotate(err, "decompress").Err()
	}
	return bs, nil
}

// URL is the Buildbucket URL of the task.
func (c *clientImpl) URL(t TaskReference) string {
	return fmt.Sprintf("https://ci.chromium.org/p/%s/builders/%s/%s/b%d",
		c.builder.Project, c.builder.Bucket, c.builder.Builder, c.knownTasks[t].bbID)
}

// SwarmingTaskID is the Swarming ID of the underlying task.
func (c *clientImpl) SwarmingTaskID(t TaskReference) string {
	return c.knownTasks[t].swarmingTaskID
}
