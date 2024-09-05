// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/buildbucket"
	buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/memlogger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/lucictx"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"

	"infra/cmd/cros_test_platform/internal/execution/types"
	"infra/libs/skylab/inventory"
	"infra/libs/skylab/request"
	ufsapi "infra/unifiedfleet/api/v1/rpc"
)

// fakeSwarming implements skylab_api.Swarming.
type fakeSwarming struct {
	botBoardsPerPool map[string][]string // pool -> list of boards
}

func newFakeSwarming() *fakeSwarming {
	return &fakeSwarming{
		botBoardsPerPool: make(map[string][]string),
	}
}

// CancelTask implements swarmingClient interface.
func (f *fakeSwarming) CancelTask(_ context.Context, taskID string) error {
	return nil
}

// BotExists implements swarmingClient interface.
func (f *fakeSwarming) BotExists(_ context.Context, dims []*swarming_api.SwarmingRpcsStringPair) (bool, error) {
	pool := ""
	board := ""
	for _, dim := range dims {
		switch dim.Key {
		case "label-board":
			board = dim.Value
		case "pool":
			pool = dim.Value
		}
	}
	return contains(f.botBoardsPerPool[pool], board), nil
}

// contains scans `s` for presence of `e`
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (f *fakeSwarming) addBot(board string, pool string) {
	f.botBoardsPerPool[pool] = append(f.botBoardsPerPool[pool], board)
}

func TestNonExistentBot(t *testing.T) {
	ftt.Run("When arguments ask for a non-existent bot", t, func(t *ftt.Test) {
		swarming := newFakeSwarming()
		swarming.addBot("existing-board", "ChromeOSSkylab")
		skylab := &clientImpl{
			swarmingClient: swarming,
		}
		var ml memlogger.MemLogger
		ctx := setLogger(context.Background(), &ml)
		var args request.Args
		args.SchedulableLabels = &inventory.SchedulableLabels{}
		addBoard(&args, "nonexistent-board")
		expectedRejectedTaskDims := []types.TaskDimKeyVal{
			{Key: "label-board", Val: "nonexistent-board"},
			{Key: "pool", Val: "ChromeOSSkylab"},
		}
		t.Run("the validation fails.", func(t *ftt.Test) {
			botExists, rejectedTaskDims, err := skylab.ValidateArgs(ctx, &args)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rejectedTaskDims, should.Resemble(expectedRejectedTaskDims))
			assert.Loosely(t, botExists, should.BeFalse)
			assert.Loosely(t, loggerOutput(ml, logging.Warning), should.ContainSubstring("nonexistent-board"))
		})
	})
}

func setLogger(ctx context.Context, l logging.Logger) context.Context {
	return logging.SetFactory(ctx, func(context.Context) logging.Logger {
		return l
	})
}

func loggerOutput(ml memlogger.MemLogger, level logging.Level) string {
	out := ""
	for _, m := range ml.Messages() {
		if m.Level == level {
			out = out + m.Msg
		}
	}
	return out
}

func TestExistingBot(t *testing.T) {
	ftt.Run("When arguments ask for an existing bot", t, func(t *ftt.Test) {
		swarming := newFakeSwarming()
		swarming.addBot("existing-board", "ChromeOSSkylab")
		skylab := &clientImpl{
			swarmingClient: swarming,
		}
		var args request.Args
		args.SchedulableLabels = &inventory.SchedulableLabels{}
		addBoard(&args, "existing-board")
		t.Run("the validation passes.", func(t *ftt.Test) {
			botExists, rejectedTaskDims, err := skylab.ValidateArgs(context.Background(), &args)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rejectedTaskDims, should.BeNil)
			assert.Loosely(t, botExists, should.BeTrue)
		})
	})
}

// TestValidateArgsExplicitPool verifies behavior when a specific pool is given
// as an argument (instead of implicitly being `ChromeOSSkylab`)
func TestValidateArgsExplicitPool(t *testing.T) {
	ftt.Run("When validating args with a specific pool with no bot", t, func(t *ftt.Test) {
		swarming := newFakeSwarming()
		swarming.addBot("existing-board", "ChromeOSSkylab")
		skylab := &clientImpl{
			swarmingClient: swarming,
		}
		var ml memlogger.MemLogger
		ctx := setLogger(context.Background(), &ml)
		var args request.Args
		args.SchedulableLabels = &inventory.SchedulableLabels{}
		args.SwarmingPool = "OtherPool"
		addBoard(&args, "existing-board")
		expectedRejectedTaskDims := []types.TaskDimKeyVal{
			{Key: "label-board", Val: "existing-board"},
			{Key: "pool", Val: "OtherPool"},
		}
		t.Run("the validation fails.", func(t *ftt.Test) {
			botExists, rejectedTaskDims, err := skylab.ValidateArgs(ctx, &args)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rejectedTaskDims, should.Resemble(expectedRejectedTaskDims))
			assert.Loosely(t, botExists, should.BeFalse)
			assert.Loosely(t, loggerOutput(ml, logging.Warning), should.ContainSubstring("existing-board"))
		})
		t.Run("Once the bot is added, the validation succeeds.", func(t *ftt.Test) {
			swarming.addBot("existing-board", "OtherPool")
			botExists, rejectedTaskDims, err := skylab.ValidateArgs(ctx, &args)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rejectedTaskDims, should.BeNil)
			assert.Loosely(t, botExists, should.BeTrue)
		})
	})
}

func addBoard(args *request.Args, board string) {
	args.SchedulableLabels.Board = &board
}

func TestLaunchRequest_TaskLaunched(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	setBuilder(tf.skylab, "foo-project", "foo-bucket", "foo-builder-name")
	args := newArgs()
	addTestName(args, "foo-test")

	var gotRequest *buildbucket_pb.ScheduleBuildRequest
	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Do(
		func(_ context.Context, r *buildbucket_pb.ScheduleBuildRequest, opts ...grpc.CallOption) {
			gotRequest = r
		},
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.ctx = lucictx.SetResultDB(tf.ctx, &lucictx.ResultDB{
		Hostname: "host",
		CurrentInvocation: &lucictx.ResultDBInvocation{
			Name:        "parent-invocation",
			UpdateToken: "fake-token",
		},
	})
	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	)
	tf.rc.EXPECT().UpdateIncludedInvocations(
		gomock.Any(),
		gomock.Any(),
	)

	task, err := tf.skylab.LaunchTask(tf.ctx, args)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, gotRequest, should.NotBeNil)
	assert.Loosely(t, gotRequest.Properties, should.NotBeNil)
	assert.Loosely(t, gotRequest.Properties.Fields, should.NotBeNil)
	assert.Loosely(t, gotRequest.Properties.Fields["request"], should.NotBeNil)
	req, err := structPBToTestRunnerRequest(gotRequest.Properties.Fields["request"])
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, req.GetTest().GetAutotest().GetName(), should.Equal("foo-test"))
	assert.Loosely(t, tf.skylab.URL(task), should.Equal(
		"https://ci.chromium.org/p/foo-project/builders/foo-bucket/foo-builder-name/b42"))
}

func TestLaunchRequest_ChildTaskLaunched(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	setBuilder(tf.skylab, "foo-project", "foo-bucket", "foo-builder-name")
	args := newArgs()
	addTestName(args, "foo-test")

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Do(
		func(ctx context.Context, r *buildbucket_pb.ScheduleBuildRequest, opts ...grpc.CallOption) {
			//Confirm that the parent's buildbucket-token is attached.
			md, _ := metadata.FromOutgoingContext(ctx)
			buildToks := md.Get(buildbucket.BuildbucketTokenHeader)
			assert.Loosely(t, len(buildToks), should.Equal(1))
			assert.Loosely(t, buildToks[0], should.Equal("parent-token"))

			assert.Loosely(t, r.CanOutliveParent, should.Equal(buildbucket_pb.Trinary_NO))
		},
	).Return(&buildbucket_pb.Build{Id: 42}, nil)
	tf.ctx = lucictx.SetBuildbucket(tf.ctx, &lucictx.Buildbucket{
		Hostname:           "host",
		ScheduleBuildToken: "parent-token",
	})

	_, err := tf.skylab.LaunchTask(tf.ctx, args)
	assert.Loosely(t, err, should.BeNil)
}

func setBuilder(skylab *clientImpl, project string, bucket string, builder string) {
	skylab.builder = &buildbucket_pb.BuilderID{
		Project: project,
		Bucket:  bucket,
		Builder: builder,
	}
}

func addTestName(args *request.Args, name string) {
	if args.TestRunnerRequest.Test == nil {
		args.TestRunnerRequest.Test = &skylab_test_runner.Request_Test{
			Harness: &skylab_test_runner.Request_Test_Autotest_{
				Autotest: &skylab_test_runner.Request_Test_Autotest{},
			},
		}
	}
	args.TestRunnerRequest.Test.GetAutotest().Name = name
}

func structPBToTestRunnerRequest(from *structpb.Value) (*skylab_test_runner.Request, error) {
	m := jsonpb.Marshaler{}
	json, err := m.MarshalToString(from)
	if err != nil {
		return nil, errors.Annotate(err, "structPBToTestRunnerRequest").Err()
	}
	var req skylab_test_runner.Request
	if err := jsonpb.UnmarshalString(json, &req); err != nil {
		return nil, errors.Annotate(err, "structPBToTestRunnerRequest").Err()
	}
	return &req, nil
}

func TestFetchRequest_TaskLaunchedAndCompleted(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_SUCCESS,
	}, nil)

	var gotRequest *buildbucket_pb.GetBuildRequest
	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	).Do(
		func(_ context.Context, r *buildbucket_pb.GetBuildRequest, opts ...grpc.CallOption) {
			gotRequest = r
		},
	).Return(&buildbucket_pb.Build{}, nil)

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	_, err = tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, gotRequest.Id, should.Equal(42))
	assert.Loosely(t, gotRequest.Fields, should.NotBeNil)
	assert.Loosely(t, gotRequest.Fields.Paths, should.Contain("id"))
	assert.Loosely(t, gotRequest.Fields.Paths, should.Contain("infra.swarming.task_id"))
	assert.Loosely(t, gotRequest.Fields.Paths, should.Contain("output.properties"))
	assert.Loosely(t, gotRequest.Fields.Paths, should.Contain("status"))
	assert.Loosely(t, gotRequest.Fields.Paths, should.Contain("infra.backend.task.id.id"))
}

func TestFetchRequest_LaunchedAndPending(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_SCHEDULED,
	}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_SCHEDULED,
	}, nil)

	// GetBuild is not called because the build status is not changed.

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	_, err = tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.BeNil)
}

func TestFetchRequestBuildBucketFailure_TaskLaunchedGetBuildStatusFails(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, errors.Reason("Transient failure").Err())

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	resp, err := tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.NotBeNil)
	assert.Loosely(t, resp.BuildBucketTransientFailure, should.BeTrue)
}

func TestFetchRequestBuildBucketFailure_TaskLaunchGetBuildFails(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_SUCCESS,
	}, nil)

	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, errors.Reason("Transient failure").Err())

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	resp, err := tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.NotBeNil)
	assert.Loosely(t, resp.BuildBucketTransientFailure, should.BeTrue)
}

func TestCompletedTask_LaunchAndCompletes(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_SUCCESS,
	}, nil)

	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id: 42,
		Infra: &buildbucket_pb.BuildInfra{
			Swarming: &buildbucket_pb.BuildInfra_Swarming{
				TaskId: "foo-swarming-task-id",
			},
		},
		Status: buildbucket_pb.Status_SUCCESS,
		Output: outputProperty("foo-test-case"),
	}, nil)

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	res, err := tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, res, should.NotBeNil)
	assert.Loosely(t, res.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
	assert.Loosely(t, res.Result, should.NotBeNil)
	assert.Loosely(t, res.Result.GetAutotestResult().GetTestCases(), should.HaveLength(1))
	assert.Loosely(t, res.Result.GetAutotestResult().GetTestCases()[0].GetName(), should.Equal("foo-test-case"))
	assert.Loosely(t, tf.skylab.SwarmingTaskID(task), should.Equal("foo-swarming-task-id"))
}

func TestCompletedTask_BackendBuild(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_SUCCESS,
	}, nil)

	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id: 42,
		Infra: &buildbucket_pb.BuildInfra{
			Backend: &buildbucket_pb.BuildInfra_Backend{
				Task: &buildbucket_pb.Task{
					Id: &buildbucket_pb.TaskID{
						Id: "foo-swarming-task-id",
					},
				},
			},
		},
		Status: buildbucket_pb.Status_SUCCESS,
		Output: outputProperty("foo-test-case"),
	}, nil)

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	res, err := tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, res, should.NotBeNil)
	assert.Loosely(t, res.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
	assert.Loosely(t, res.Result, should.NotBeNil)
	assert.Loosely(t, res.Result.GetAutotestResult().GetTestCases(), should.HaveLength(1))
	assert.Loosely(t, res.Result.GetAutotestResult().GetTestCases()[0].GetName(), should.Equal("foo-test-case"))
	assert.Loosely(t, tf.skylab.SwarmingTaskID(task), should.Equal("foo-swarming-task-id"))
}

func TestCompletedTaskMissingResults(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_INFRA_FAILURE,
	}, nil)

	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_INFRA_FAILURE,
	}, nil)

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	_, err = tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.BeNil)
}

func TestAbortedTask(t *testing.T) {
	tf, cleanup := newTestFixture(t)
	defer cleanup()

	tf.bb.EXPECT().ScheduleBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{Id: 42}, nil)

	tf.bb.EXPECT().GetBuildStatus(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_INFRA_FAILURE,
	}, nil)

	tf.bb.EXPECT().GetBuild(
		gomock.Any(),
		gomock.Any(),
	).Return(&buildbucket_pb.Build{
		Id:     42,
		Status: buildbucket_pb.Status_INFRA_FAILURE,
		Output: outputProperty("foo-test-case"),
	}, nil)

	task, err := tf.skylab.LaunchTask(tf.ctx, newArgs())
	assert.Loosely(t, err, should.BeNil)
	res, err := tf.skylab.FetchResults(tf.ctx, task)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, res, should.NotBeNil)
	assert.Loosely(t, res.LifeCycle, should.Equal(test_platform.TaskState_LIFE_CYCLE_COMPLETED))
	assert.Loosely(t, res.Result, should.NotBeNil)
}

type testFixture struct {
	ctx    context.Context
	bb     *buildbucket_pb.MockBuildsClient
	rc     *resultpb.MockRecorderClient
	skylab *clientImpl
}

func newTestFixture(t *testing.T) (*testFixture, func()) {
	ctrl := gomock.NewController(t)
	bb := buildbucket_pb.NewMockBuildsClient(ctrl)
	rc := resultpb.NewMockRecorderClient(ctrl)
	ctx := context.Background()
	ctx = lucictx.SetResultDB(ctx, &lucictx.ResultDB{
		Hostname: "host",
		// The test context will not have an update token by default.
		CurrentInvocation: &lucictx.ResultDBInvocation{
			Name: "parent-invocation",
		},
	})
	return &testFixture{
		ctx: ctx,
		bb:  bb,
		rc:  rc,
		skylab: &clientImpl{
			bbClient:       bb,
			recorderClient: rc,
			knownTasks:     make(map[TaskReference]*task),
		},
	}, ctrl.Finish
}

func newArgs() *request.Args {
	return &request.Args{
		TestRunnerRequest: &skylab_test_runner.Request{},
	}
}

func outputProperty(testCase string) *buildbucket_pb.Build_Output {
	res := &skylab_test_runner.Result{
		Harness: &skylab_test_runner.Result_AutotestResult{
			AutotestResult: &skylab_test_runner.Result_Autotest{
				TestCases: []*skylab_test_runner.Result_Autotest_TestCase{
					{
						Name:    testCase,
						Verdict: skylab_test_runner.Result_Autotest_TestCase_VERDICT_PASS,
					},
				},
			},
		},
	}
	m, _ := proto.Marshal(res)
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(m)
	w.Close()
	return &buildbucket_pb.Build_Output{
		Properties: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"compressed_result": {
					Kind: &structpb.Value_StringValue{
						StringValue: base64.StdEncoding.EncodeToString(b.Bytes()),
					},
				},
			},
		},
	}
}

// fakeSwarming implements skylab_api.Swarming.
type fakeUFS struct {
	policyByBoard map[string]bool
}

func newFakeUFS() *fakeUFS {
	return &fakeUFS{
		policyByBoard: make(map[string]bool),
	}
}

// CheckFleetTestsPolicy implements fleetClient interface.
func (f *fakeUFS) CheckFleetTestsPolicy(_ context.Context, req *ufsapi.CheckFleetTestsPolicyRequest) (*ufsapi.CheckFleetTestsPolicyResponse, error) {
	status := ufsapi.TestStatus_OK
	if !f.policyByBoard[req.Board] {
		status = ufsapi.TestStatus_NOT_A_PUBLIC_BOARD
	}
	return &ufsapi.CheckFleetTestsPolicyResponse{
		TestStatus: &ufsapi.TestStatus{
			Code: status,
		}}, nil
}

func (f *fakeUFS) addPolicy(board string) {
	f.policyByBoard[board] = true
}

func TestFleetPolicyCheckFailed(t *testing.T) {
	ftt.Run("When Invalid arguments are passed to fleet check policy", t, func(t *ftt.Test) {
		ufs := newFakeUFS()
		ufs.addPolicy("board1")
		t.Run("the validation fails.", func(t *ftt.Test) {
			policyResponse, err := ufs.CheckFleetTestsPolicy(context.Background(), &ufsapi.CheckFleetTestsPolicyRequest{
				TestName: "testName",
				Board:    "board",
				Model:    "model",
				Image:    "image",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, policyResponse.TestStatus.Code, should.Equal(ufsapi.TestStatus_NOT_A_PUBLIC_BOARD))
		})
	})
}

func TestFleetPolicyCheckSucceeded(t *testing.T) {
	ftt.Run("When valid arguments are passed to fleet check policy", t, func(t *ftt.Test) {
		ufs := newFakeUFS()
		ufs.addPolicy("board1")
		t.Run("the validation succeeds.", func(t *ftt.Test) {
			policyResponse, err := ufs.CheckFleetTestsPolicy(context.Background(), &ufsapi.CheckFleetTestsPolicyRequest{
				TestName: "testName",
				Board:    "board1",
				Model:    "model",
				Image:    "image",
			})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, policyResponse.TestStatus.Code, should.Equal(ufsapi.TestStatus_OK))
		})
	})
}
