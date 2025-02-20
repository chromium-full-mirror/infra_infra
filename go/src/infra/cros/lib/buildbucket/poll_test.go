package buildbucket_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"gotest.tools/assert"

	bbpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cros/lib/buildbucket"
)

var (
	expectedFieldMask = &fieldmaskpb.FieldMask{Paths: []string{
		"builder",
		"create_time",
		"created_by",
		"critical",
		"end_time",
		"id",
		"input",
		"number",
		"output",
		"start_time",
		"status",
		"update_time",
		"tags",
	},
	}
)

func TestPollForOutputProp(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := bbpb.NewMockBuildsClient(ctrl)

	// First call requests all 4 builds.
	firstReq := &bbpb.BatchRequest{Requests: []*bbpb.BatchRequest_Request{
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 1, Fields: expectedFieldMask},
		}},
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 2, Fields: expectedFieldMask},
		}},
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 3, Fields: expectedFieldMask},
		}},
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 4, Fields: expectedFieldMask},
		}},
	}}

	// On the first iteration, one build is completed, one build is running w/o
	// the output prop, and two builds are running w/ the output prop.
	firstResp := &bbpb.BatchResponse{
		Responses: []*bbpb.BatchResponse_Response{
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 1, Status: bbpb.Status_SUCCESS},
			}},
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 2, Status: bbpb.Status_SCHEDULED},
			}},
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 3, Status: bbpb.Status_STARTED,
					Output: &bbpb.Build_Output{
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							"testprop": structpb.NewBoolValue(false),
						}},
					}},
			}},
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 4, Status: bbpb.Status_STARTED,
					Output: &bbpb.Build_Output{
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							"testprop": structpb.NewBoolValue(true),
						}},
					}},
			}},
		}}

	// Second call requests only the 3 builds still running.
	secondReq := &bbpb.BatchRequest{Requests: []*bbpb.BatchRequest_Request{
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 2, Fields: expectedFieldMask},
		}},
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 3, Fields: expectedFieldMask},
		}},
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 4, Fields: expectedFieldMask},
		}},
	}}

	// On the second call, all builds are completed or have set the output prop.
	secondResp := &bbpb.BatchResponse{
		Responses: []*bbpb.BatchResponse_Response{
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 1, Status: bbpb.Status_SUCCESS},
			}},
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 2, Status: bbpb.Status_FAILURE},
			}},
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 3, Status: bbpb.Status_STARTED,
					Output: &bbpb.Build_Output{
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							"testprop": structpb.NewBoolValue(false),
						}},
					}},
			}},
			{Response: &bbpb.BatchResponse_Response_GetBuild{
				GetBuild: &bbpb.Build{Id: 4, Status: bbpb.Status_SUCCESS,
					Output: &bbpb.Build_Output{
						Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
							"testprop": structpb.NewBoolValue(false),
						}},
					}},
			}},
		}}

	gomock.InOrder(
		client.EXPECT().Batch(gomock.AssignableToTypeOf(ctx), firstReq).Return(firstResp, nil),
		client.EXPECT().Batch(gomock.AssignableToTypeOf(ctx), secondReq).Return(secondResp, nil),
	)

	builds, err := buildbucket.PollForOutputProp(ctx, client, []int64{1, 2, 3, 4}, "testprop", time.Millisecond*10)
	if err != nil {
		t.Fatal(err)
	}

	expectedBuilds := map[int64]*bbpb.Build{
		1: {Id: 1, Status: bbpb.Status_SUCCESS},
		2: {Id: 2, Status: bbpb.Status_FAILURE},
		3: {Id: 3, Status: bbpb.Status_STARTED,
			Output: &bbpb.Build_Output{
				Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
					"testprop": structpb.NewBoolValue(false),
				}},
			}},
		4: {Id: 4, Status: bbpb.Status_SUCCESS,
			Output: &bbpb.Build_Output{
				Properties: &structpb.Struct{Fields: map[string]*structpb.Value{
					"testprop": structpb.NewBoolValue(false),
				}},
			}},
	}

	if diff := cmp.Diff(expectedBuilds, builds, protocmp.Transform()); diff != "" {
		t.Errorf("PollForOutputProp diff (-want +got):\n%s", diff)
	}
}

func TestPollForOutputPropError(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := bbpb.NewMockBuildsClient(ctrl)

	req := &bbpb.BatchRequest{Requests: []*bbpb.BatchRequest_Request{
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 1, Fields: expectedFieldMask},
		}},
	}}

	client.EXPECT().
		Batch(gomock.AssignableToTypeOf(ctx), req).
		Return(&bbpb.BatchResponse{
			Responses: []*bbpb.BatchResponse_Response{
				{
					Response: &bbpb.BatchResponse_Response_Error{Error: &status.Status{
						Code:    int32(codes.InvalidArgument),
						Message: "error in request",
					}},
				},
			},
		}, nil)

	_, err := buildbucket.PollForOutputProp(ctx, client, []int64{1}, "testprop", time.Millisecond*10)

	assert.ErrorContains(t, err, "got error in BatchResponse")
}

func TestPollForOutputPropRetriesTransientError(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := bbpb.NewMockBuildsClient(ctrl)

	req := &bbpb.BatchRequest{Requests: []*bbpb.BatchRequest_Request{
		{Request: &bbpb.BatchRequest_Request_GetBuild{
			GetBuild: &bbpb.GetBuildRequest{Id: 1, Fields: expectedFieldMask},
		}},
	}}

	firstResp := &bbpb.BatchResponse{
		Responses: []*bbpb.BatchResponse_Response{
			{
				Response: &bbpb.BatchResponse_Response_Error{Error: &status.Status{
					Code:    int32(codes.Internal),
					Message: "internal error",
				}},
			},
		},
	}

	secondResp := &bbpb.BatchResponse{
		Responses: []*bbpb.BatchResponse_Response{
			{
				Response: &bbpb.BatchResponse_Response_Error{Error: &status.Status{
					Code:    int32(codes.DeadlineExceeded),
					Message: "deadline exceeded",
				}},
			},
		},
	}

	thirdResp := &bbpb.BatchResponse{
		Responses: []*bbpb.BatchResponse_Response{
			{
				Response: &bbpb.BatchResponse_Response_GetBuild{
					GetBuild: &bbpb.Build{Id: 1, Status: bbpb.Status_SUCCESS},
				},
			},
		},
	}

	gomock.InOrder(
		client.EXPECT().Batch(gomock.AssignableToTypeOf(ctx), req).Return(firstResp, nil),
		client.EXPECT().Batch(gomock.AssignableToTypeOf(ctx), req).Return(secondResp, nil),
		client.EXPECT().Batch(gomock.AssignableToTypeOf(ctx), req).Return(thirdResp, nil),
	)

	builds, err := buildbucket.PollForOutputProp(ctx, client, []int64{1}, "testprop", time.Millisecond*10)
	if err != nil {
		t.Fatal(err)
	}

	expectedBuilds := map[int64]*bbpb.Build{
		1: {Id: 1, Status: bbpb.Status_SUCCESS},
	}

	if diff := cmp.Diff(expectedBuilds, builds, protocmp.Transform()); diff != "" {
		t.Errorf("PollForOutputProp diff (-want +got):\n%s", diff)
	}
}
