// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/mock"
	moblabapipb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	swarmingapi "go.chromium.org/luci/swarming/proto/api_v2"

	"infra/cros/satlab/common/dut"
	"infra/cros/satlab/common/enumeration"
	"infra/cros/satlab/common/paths"
	"infra/cros/satlab/common/services"
	"infra/cros/satlab/common/services/build_service"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/executor"
	mk "infra/cros/satlab/satlabrpcserver/mocks"
	"infra/cros/satlab/satlabrpcserver/models"
	cpu "infra/cros/satlab/satlabrpcserver/platform/cpu_temperature"
	"infra/cros/satlab/satlabrpcserver/services/bucket_services"
	"infra/cros/satlab/satlabrpcserver/services/dut_services"
	"infra/cros/satlab/satlabrpcserver/utils"
	"infra/cros/satlab/satlabrpcserver/utils/constants"
	mon "infra/cros/satlab/satlabrpcserver/utils/monitor"
	ufsModels "infra/unifiedfleet/api/v1/models"
	ufsLabpb "infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsApi "infra/unifiedfleet/api/v1/rpc"
	ufspb "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

type mockDeleteClient struct {
	getMachineLSECalls    []*ufspb.GetMachineLSERequest
	deleteMachineLSECalls []*ufspb.DeleteMachineLSERequest
	deleteAssetCalls      []*ufspb.DeleteAssetRequest
	deleteRackCalls       []*ufspb.DeleteRackRequest
}

func (c *mockDeleteClient) DeleteMachineLSE(ctx context.Context, req *ufsApi.DeleteMachineLSERequest, ops ...grpc.CallOption) (*emptypb.Empty, error) {
	c.deleteMachineLSECalls = append(c.deleteMachineLSECalls, req)
	return &emptypb.Empty{}, nil
}

func (c *mockDeleteClient) DeleteRack(ctx context.Context, req *ufsApi.DeleteRackRequest, ops ...grpc.CallOption) (*emptypb.Empty, error) {
	c.deleteRackCalls = append(c.deleteRackCalls, req)
	return &emptypb.Empty{}, nil
}

func (c *mockDeleteClient) DeleteAsset(ctx context.Context, req *ufsApi.DeleteAssetRequest, ops ...grpc.CallOption) (*emptypb.Empty, error) {
	c.deleteAssetCalls = append(c.deleteAssetCalls, req)
	return &emptypb.Empty{}, nil
}

func (c *mockDeleteClient) GetMachineLSE(ctx context.Context, req *ufsApi.GetMachineLSERequest, opts ...grpc.CallOption) (*ufsModels.MachineLSE, error) {
	c.getMachineLSECalls = append(c.getMachineLSECalls, req)
	return &ufsModels.MachineLSE{
		Name:     req.Name,
		Machines: []string{fmt.Sprintf("asset-%s", ufsUtil.RemovePrefix(req.Name))},
		Rack:     fmt.Sprintf("rack-%s", ufsUtil.RemovePrefix(req.Name)),
	}, nil
}

// checkShouldRaiseError it is a helper function to check the response should raise error.
func checkShouldRaiseError(t *testing.T, err error, expectedErr error) {
	if err == nil {
		t.Errorf("Should return error, but got no error")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("Should return error, but get a different error. Expected %v, got %v", expectedErr, err)
	}
}

func createMockServer(t *testing.T) *SatlabRpcServiceServer {
	// Create a Mock `IBuildService`
	var mockBuildService = new(build_service.MockBuildService)

	// Create a Mock `IBucketService`
	var mockBucketService = new(bucket_services.MockBucketServices)

	// Create a Mock `IDUTService`
	var mockDUTService = new(mk.MockDUTServices)

	// Create a Mock `ISwarmingService`
	// var swarmingService = new(services.MockISwarmingService) //new(services.MockSwarmingService)
	ctl := gomock.NewController(t)
	defer ctl.Finish()

	var swarmingService = services.NewMockISwarmingService(ctl)

	// Create a SATLab Server
	return New(true, mockBuildService, mockBucketService, mockDUTService, nil, swarmingService)
}

// TestListBuildTargetsShouldSuccess test `ListBuildTargets` function.
//
// It should return some data without error.
func TestListBuildTargetsShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	expected := []string{"zork"}
	s.buildService.(*build_service.MockBuildService).On("ListBuildTargets", ctx).Return(
		expected, nil)

	req := &pb.ListBuildTargetsRequest{}

	res, err := s.ListBuildTargets(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if !reflect.DeepEqual(expected, res.BuildTargets) {
		t.Errorf("Expected %v != got %v", expected, res.BuildTargets)
	}
}

// TestListBuildTargetsShouldSuccess test `ListBuildTargets` function.
//
// It should return error because it mocks some network error on calling
// `BuildClient` to fetch the data.
func TestListBuildTargetsShouldFailWhenMakeARequestToBuildClientFailed(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	expectedErr := errors.New("network error")
	s.buildService.(*build_service.MockBuildService).On("ListBuildTargets", ctx).Return(
		[]string{}, expectedErr)

	req := &pb.ListBuildTargetsRequest{}

	_, err := s.ListBuildTargets(ctx, req)

	// Assert
	checkShouldRaiseError(t, err, expectedErr)
}

// TestListMilestonesShouldSuccess test `ListMilestones` function.
//
// It should return some data without error.
func TestListMilestonesShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	model := "dirinboz"
	expectedMilestones := []string{"114", "113"}
	s.buildService.(*build_service.MockBuildService).On("ListAvailableMilestones", ctx, board, model).Return(
		expectedMilestones, nil)

	localBucketMilestones := []string{"113"}
	s.bucketService.(*bucket_services.MockBucketServices).On("GetMilestones", ctx, board).Return(
		localBucketMilestones, nil)
	s.bucketService.(*bucket_services.MockBucketServices).On("IsBucketInAsia", ctx).Return(
		false, nil)

	req := &pb.ListMilestonesRequest{
		Board: board,
		Model: model,
	}

	res, err := s.ListMilestones(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if len(res.Milestones) != 2 {
		t.Errorf("Expected %v items, but got %v", 2, len(res.Milestones))
	}

	// Assert
	expected := []*pb.BuildItem{
		{
			Value:    "114",
			IsStaged: false,
		},
		{
			Value:    "113",
			IsStaged: true,
		},
	}

	if !reflect.DeepEqual(expected, res.Milestones) {
		t.Errorf("Expected %v != got %v", expected, res.Milestones)
	}
}

// TestListMilestonesShouldSuccessWhenBucketInAsia test `ListMilestones` function.
func TestListMilestonesShouldSuccessWhenBucketInAsia(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	model := "dirinboz"
	expectedMilestones := []string{"114", "113"}
	s.buildService.(*build_service.MockBuildService).On("ListAvailableMilestones", ctx, board, model).Return(
		expectedMilestones, nil)

	localBucketMilestones := []string{"113"}
	s.bucketService.(*bucket_services.MockBucketServices).On("GetMilestones", ctx, board).Return(
		localBucketMilestones, nil)
	s.bucketService.(*bucket_services.MockBucketServices).On("IsBucketInAsia", ctx).Return(
		true, nil)

	req := &pb.ListMilestonesRequest{
		Board: board,
		Model: model,
	}

	res, err := s.ListMilestones(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if len(res.Milestones) != 1 {
		t.Errorf("Expected %v items, but got %v", 2, len(res.Milestones))
	}

	// Assert
	expected := []*pb.BuildItem{
		{
			Value:    "113",
			IsStaged: true,
		},
	}

	if !reflect.DeepEqual(expected, res.Milestones) {
		t.Errorf("Expected %v != got %v", expected, res.Milestones)
	}
}

// TestListMilestonesShouldSuccessWhenBucketInAsia test `ListMilestones` function.
func TestListMilestonesShouldFailWhenMakeARequestToBucketFailed(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	model := "dirinboz"
	expectedMilestones := []string{"114", "113"}
	expectedErr := errors.New("can't make a request")
	s.buildService.(*build_service.MockBuildService).On("ListAvailableMilestones", ctx, board, model).Return(
		expectedMilestones, nil)

	localBucketMilestones := []string{"113"}
	s.bucketService.(*bucket_services.MockBucketServices).On("GetMilestones", ctx, board).Return(
		localBucketMilestones, nil)
	s.bucketService.(*bucket_services.MockBucketServices).On("IsBucketInAsia", ctx).Return(
		false, expectedErr)

	req := &pb.ListMilestonesRequest{
		Board: board,
		Model: model,
	}

	_, err := s.ListMilestones(ctx, req)

	// Assert
	checkShouldRaiseError(t, err, expectedErr)
}

// TestListAccessibleModelShouldSuccess test `ListAccessibleModel` function.
func TestListAccessibleModelShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	in := []string{"buildTargets/zork/models/model1", "buildTargets/zork/models/model2", "buildTargets/zork/models/dirinboz"}
	s.buildService.(*build_service.MockBuildService).On("ListModels", ctx, board).Return(
		in, nil)

	req := &pb.ListAccessibleModelsRequest{
		Board: board,
	}

	res, err := s.ListAccessibleModels(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if len(res.Models) != 3 {
		t.Errorf("Should got %v difference models", 3)
	}

	expected := &pb.ListAccessibleModelsResponse{
		Models: []*pb.Model{
			{
				Name:   "model1",
				Boards: []string{"zork"},
			},
			{
				Name:   "dirinboz",
				Boards: []string{"zork"},
			},
			{
				Name:   "model2",
				Boards: []string{"zork"},
			},
		},
	}

	// Assert
	// ignore generated pb code
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListAccessibleModelsResponse{}, pb.Model{})
	// Model ordering is not deterministic, need to sort before comparing
	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Model) bool {
			return x.GetName() > y.GetName()
		})

	if diff := cmp.Diff(expected, res, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("Expected %v, got %v", expected, res.Models)
	}
}

// TestListAccessibleModelShouldSuccess test `ListAccessibleModel` function.
func TestListAccessibleModelShouldFailWhenMakeARequestToBucketFailed(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	in := []string{"buildTargets/zork/models/model1", "buildTargets/zork/models/model2", "buildTargets/zork/models/dirinboz"}
	expectedErr := errors.New("can't make a request to bucket")
	s.buildService.(*build_service.MockBuildService).On("ListModels", ctx, board).Return(
		in, expectedErr)

	req := &pb.ListAccessibleModelsRequest{
		Board: board,
	}

	_, err := s.ListAccessibleModels(ctx, req)

	// Assert
	checkShouldRaiseError(t, err, expectedErr)
}

// TestListBuildVersionsShouldSuccess test `ListBuildVersions` function.
func TestListBuildVersionsShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork1"
	model := "dirinboz1"
	var milestone int32 = 105
	s.bucketService.(*bucket_services.MockBucketServices).
		On("GetBuilds", ctx, board, milestone).
		Return([]string{"14820.8.0"}, nil)

	s.buildService.(*build_service.MockBuildService).
		On("ListBuildsForMilestone", ctx, board, model, milestone).
		Return([]*build_service.BuildVersion{
			{
				Version: "14820.100.0",
				Status:  build_service.FAILED,
			},
			{
				Version: "14820.20.0",
				Status:  build_service.AVAILABLE,
			},
			{
				Version: "14820.8.0",
				Status:  build_service.AVAILABLE,
			},
		}, nil)

	s.bucketService.(*bucket_services.MockBucketServices).On("IsBucketInAsia", ctx).Return(
		false, nil)

	req := &pb.ListBuildVersionsRequest{Board: board, Model: model, Milestone: milestone}

	res, err := s.ListBuildVersions(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if len(res.BuildVersions) != 3 {
		t.Errorf("Should got %v difference models", 3)
	}

	expectedResult := []*pb.BuildItem{
		{
			Value:    "14820.100.0",
			Status:   pb.BuildItem_BUILD_STATUS_FAIL,
			IsStaged: false,
		},
		{
			Value:    "14820.20.0",
			Status:   pb.BuildItem_BUILD_STATUS_PASS,
			IsStaged: false,
		},
		{
			Value:    "14820.8.0",
			Status:   pb.BuildItem_BUILD_STATUS_PASS,
			IsStaged: true,
		},
	}

	// Assert
	if !reflect.DeepEqual(expectedResult, res.BuildVersions) {
		t.Errorf("Expected %v != got %v", expectedResult, res.BuildVersions)
	}
}

func Test_ListBuildVersionsStatusShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork1"
	model := "dirinboz1"
	var milestone int32 = 105
	s.bucketService.(*bucket_services.MockBucketServices).
		On("GetBuilds", ctx, board, milestone).
		Return([]string{"14820.8.0"}, nil)

	s.buildService.(*build_service.MockBuildService).
		On("ListBuildsForMilestone", ctx, board, model, milestone).
		Return([]*build_service.BuildVersion{
			{
				Version: "14820.8.0",
				Status:  build_service.FAILED,
			},
			{
				Version: "14820.20.0",
				Status:  build_service.AVAILABLE,
			},
		}, nil)

	s.bucketService.(*bucket_services.MockBucketServices).On("IsBucketInAsia", ctx).Return(
		false, nil)

	req := &pb.ListBuildVersionsRequest{Board: board, Model: model, Milestone: milestone}

	res, err := s.ListBuildVersions(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	expected := &pb.ListBuildVersionsResponse{
		BuildVersions: []*pb.BuildItem{
			{
				Value:    "14820.20.0",
				Status:   pb.BuildItem_BUILD_STATUS_PASS,
				IsStaged: false,
			},
			{
				Value:    "14820.8.0",
				Status:   pb.BuildItem_BUILD_STATUS_FAIL,
				IsStaged: true,
			},
		},
	}

	// Assert
	// ignore generated pb code
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListBuildVersionsResponse{}, pb.BuildItem{})
	if diff := cmp.Diff(expected, res, ignorePBFieldOpts); diff != "" {
		t.Errorf("Diff: %v", diff)
		return
	}
}

// TestListBuildVersionsShouldSuccess test `ListBuildVersions` function.
func TestListBuildVersionsShouldFailWhenMakeARequestToBuildClientFailed(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	model := "dirinboz"
	var milestone int32 = 105
	expectedErr := errors.New("can't make a request to bucket")
	s.bucketService.(*bucket_services.MockBucketServices).
		On("GetBuilds", ctx, board, milestone).
		Return([]string{"14826.0.0"}, nil)

	s.bucketService.(*bucket_services.MockBucketServices).On("IsBucketInAsia", ctx).Return(
		false, nil)

	s.buildService.(*build_service.MockBuildService).
		On("ListBuildsForMilestone", ctx, board, model, milestone).
		Return([]*build_service.BuildVersion{}, expectedErr)

	req := &pb.ListBuildVersionsRequest{Board: board, Model: model, Milestone: milestone}

	_, err := s.ListBuildVersions(ctx, req)

	// Assert
	checkShouldRaiseError(t, err, expectedErr)
}

// TestStageBuildShouldSuccess test `StageBuild` function.
func TestStageBuildShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	model := "dirinboz"
	build := "1234.0.0"
	bucketName := site.GetGCSImageBucket()
	expectedArtifact := &moblabapipb.BuildArtifact{
		Build:  build,
		Name:   "artifacts",
		Bucket: bucketName,
		Path:   "buildTargets/zork/models/dirinboz/builds/1234.0.0/artifacts/chromeos-image-archive",
	}

	s.buildService.(*build_service.MockBuildService).
		On("StageBuild", ctx, board, model, build, bucketName).
		Return(expectedArtifact, nil)

	req := &pb.StageBuildRequest{
		Board:        board,
		Model:        model,
		BuildVersion: build,
	}

	res, err := s.StageBuild(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if res.GetBuildBucket() != bucketName {
		t.Errorf("Expected %v, got: %v", bucketName, res.GetBuildBucket())
	}
}

// TestStageBuildShouldSuccess test `StageBuild` function.
func TestStageBuildShouldFailWhenMakeARequestToBuildClientFailed(t *testing.T) {
	t.Parallel()
	// Create a SATLab Server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Setup some data to Mock
	board := "zork"
	model := "dirinboz"
	build := "1234.0.0"
	bucketName := site.GetGCSImageBucket()
	expectedArtifact := &moblabapipb.BuildArtifact{
		Build:  build,
		Name:   "artifacts",
		Bucket: bucketName,
		Path:   "buildTargets/zork/models/dirinboz/builds/1234.0.0/artifacts/chromeos-image-archive",
	}
	expectedErr := errors.New("can't make a request")

	s.buildService.(*build_service.MockBuildService).
		On("StageBuild", ctx, board, model, build, bucketName).
		Return(expectedArtifact, expectedErr)

	req := &pb.StageBuildRequest{
		Board:        board,
		Model:        model,
		BuildVersion: build,
	}

	_, err := s.StageBuild(ctx, req)

	// Assert
	checkShouldRaiseError(t, err, expectedErr)
}

func TestListConnectedDUTsFirmwareShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cmdOut := "{\n  \"fwid\": \"Google_Lindar.13672.291.0\",\n  \"model\": \"lillipup\",\n  \"fw_update\": {\n    \"lillipup\": {\n      \"host\": {\n        \"versions\": {\n          \"ro\": \"Google_Lindar.13672.207.0\",\n          \"rw\": \"Google_Lindar.13672.291.0\"\n        },\n        \"keys\": {\n          \"root\": \"b11d74edd286c144e1135b49e7f0bc20cf041f10\",\n          \"recovery\": \"c14bd720b70d97394257e3e826bd8f43de48d4ed\"\n        },\n        \"image\": \"images/bios-lindar.ro-13672-207-0.rw-13672-291-0.bin\"\n      },\n      \"ec\": {\n        \"versions\": {\n          \"ro\": \"lindar_v2.0.7573-4cf04a534f\",\n          \"rw\": \"lindar_v2.0.10133-063f551128\"\n        },\n        \"image\": \"images/ec-lindar.ro-2-0-7573.rw-2-0-10133.bin\"\n      },\n      \"signature_id\": \"lillipup\"\n    }\n  }\n}\n"

	// Mock some data
	IP := "192.168.100.1"
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: IP, IsPingable: true, HasTestImage: true},
	}, nil)
	s.dutService.(*mk.MockDUTServices).
		On("RunCommandOnIPs", ctx, mock.Anything, constants.ListFirmwareCommand).
		Return([]*models.SSHResult{
			{IP: IP, Value: cmdOut},
		})

	req := &pb.ListConnectedDutsFirmwareRequest{}

	res, err := s.ListConnectedDutsFirmware(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	expected := []*pb.ConnectedDutFirmwareInfo{{
		Ip:              IP,
		CurrentFirmware: "Google_Lindar.13672.291.0",
		UpdateFirmware:  "Google_Lindar.13672.291.0",
	}}

	if !reflect.DeepEqual(expected, res.Duts) {
		t.Errorf("Expected: %v, got :%v", expected, res.Duts)
	}
}

func TestListConnectedDUTsFirmwareShouldGetEmptyListWhenCommandExecuteFailed(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	expectedError := errors.New("command execute failed")

	// Mock some data
	IP := "192.168.100.1"
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: IP, IsPingable: true, HasTestImage: true},
	}, nil)
	s.dutService.(*mk.MockDUTServices).
		On("RunCommandOnIPs", ctx, mock.Anything, constants.ListFirmwareCommand).
		Return([]*models.SSHResult{
			{IP: IP, Error: expectedError},
		})

	req := &pb.ListConnectedDutsFirmwareRequest{}

	res, err := s.ListConnectedDutsFirmware(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if len(res.Duts) != 0 {
		t.Errorf("Expected zero dut")
	}
}

func TestGetSystemInfoShouldWork(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)
	var mockCPUTemperature = new(mk.MockCPUTemperature)
	mockCPUTemperature.On("GetCurrentCPUTemperature").Return(float32(1.0), nil)
	var cpuOrchestrator = cpu.NewOrchestrator(mockCPUTemperature, 5)
	s.cpuTemperatureOrchestrator = cpuOrchestrator

	timeObjForTest := time.Now()
	s.commandExecutor = &executor.FakeCommander{CmdOutput: fmt.Sprintf("'%v'", timeObjForTest.Format(time.RFC3339Nano))}

	ctx := context.Background()

	// Make some data
	m := mon.New()
	m.Register(cpuOrchestrator, time.Second)
	time.Sleep(time.Second * 2)

	req := pb.GetSystemInfoRequest{}

	res, err := s.GetSystemInfo(ctx, &req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	expectedCpuTemp := 1.0
	if !utils.NearlyEqual(float64(res.GetCpuTemperature()), expectedCpuTemp) {
		t.Errorf("Expected %v, got %v", expectedCpuTemp, res.GetCpuTemperature())
	}

	expectedStartTime := timestamppb.New(timeObjForTest)
	if diff := cmp.Diff(expectedStartTime, res.GetStartTime(), cmpopts.IgnoreUnexported(timestamp.Timestamp{})); diff != "" {
		t.Errorf("Expected %v, got %v", expectedStartTime, res.GetStartTime())
	}
}

func TestGetSystemInfoShouldWorkWithoutCPUOrchestrator(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	timeObjForTest := time.Now()
	s.commandExecutor = &executor.FakeCommander{CmdOutput: fmt.Sprintf("'%v'", timeObjForTest.Format(time.RFC3339Nano))}

	ctx := context.Background()

	req := pb.GetSystemInfoRequest{}

	res, err := s.GetSystemInfo(ctx, &req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	expectedCpuTemp := -1.0
	if !utils.NearlyEqual(float64(res.GetCpuTemperature()), expectedCpuTemp) {
		t.Errorf("Expected %v, got %v", expectedCpuTemp, res.GetCpuTemperature())
	}

	expectedStartTime := timestamppb.New(timeObjForTest)
	if diff := cmp.Diff(expectedStartTime, res.GetStartTime(), cmpopts.IgnoreUnexported(timestamp.Timestamp{})); diff != "" {
		t.Errorf("Expected %v, got %v", expectedStartTime, res.GetStartTime())
	}
}

func TestGetPeripheralInformationShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	expectedResult := &models.SSHResult{
		Value: "<json data>",
		IP:    "192.168.231.100",
	}

	// Mock some data
	s.dutService.(*mk.MockDUTServices).
		On("RunCommandOnIP", ctx, mock.Anything, constants.GetPeripheralInfoCommand).
		Return(expectedResult, nil)

	req := &pb.GetPeripheralInformationRequest{}
	res, err := s.GetPeripheralInformation(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if diff := cmp.Diff(expectedResult.Value, res.JsonInfo); diff != "" {
		t.Errorf("Return difference result. Expected %v, got %v", expectedResult.Value, res.JsonInfo)
	}
}

func TestGetPeripheralInformationShouldFailWhenExecuteCommandFailed(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	expectedResult := &models.SSHResult{
		Error: errors.New("execute cmd failed"),
		IP:    "192.168.231.100",
	}

	// Mock some data
	s.dutService.(*mk.MockDUTServices).
		On("RunCommandOnIP", ctx, mock.Anything, constants.GetPeripheralInfoCommand).
		Return(expectedResult, nil)

	req := &pb.GetPeripheralInformationRequest{}
	_, err := s.GetPeripheralInformation(ctx, req)

	// Assert
	if diff := cmp.Diff(expectedResult.Error.Error(), err.Error()); diff != "" {
		t.Errorf("Return difference result. Expected %v, got %v", expectedResult.Error, err)
	}
}

func TestUpdateDUTsFirmwareShouldSuccess(t *testing.T) {
	// Run this testcase parallel
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	// Create a mockData data
	mockData := []*models.SSHResult{
		{IP: "192.168.231.1", Value: "execute command success"},
		{IP: "192.168.231.2", Error: errors.New("failed to execute command")},
	}
	s.dutService.(*mk.MockDUTServices).
		On("RunCommandOnIPs", ctx, mock.Anything, constants.UpdateFirmwareCommand).
		Return(mockData, nil)

	// Act
	req := &pb.UpdateDutsFirmwareRequest{Ips: []string{"192.168.231.1", "192.168.231.2"}}
	resp, err := s.UpdateDutsFirmware(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// Create a expected result
	expected := []*pb.FirmwareUpdateCommandOutput{
		{Ip: "192.168.231.1", CommandOutput: "execute command success"},
		{Ip: "192.168.231.2", CommandOutput: "failed to execute command"},
	}
	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.FirmwareUpdateCommandOutput{})
	// sort the response and expected result when comparasion
	sortOpts := cmpopts.SortSlices(
		func(x, y *pb.FirmwareUpdateCommandOutput) bool {
			return x.GetIp() > y.GetIp()
		},
	)

	if diff := cmp.Diff(expected, resp.Outputs, ignorePBFieldOpts, sortOpts); diff != "" {
		t.Errorf("Expected: {%v}, got: {%v}", expected, resp.Outputs)
	}
}

func TestGetVersionInfoShouldSuccess(t *testing.T) {
	// Run this testcase parallel
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)
	s.commandExecutor = &executor.FakeCommander{
		CmdOutput: "BUILD_VERSION=output\ndescription:description\nversion:v\ntrack:track",
	}

	ctx := context.Background()
	req := &pb.GetVersionInfoRequest{}
	resp, err := s.GetVersionInfo(ctx, req)

	if err != nil {
		t.Errorf("Should success, but got an error: %v\n", err)
	}

	expected := &pb.GetVersionInfoResponse{
		HostId:          "build_version=output\ndescription:description\nversion:v\ntrack:track",
		Description:     "description",
		Track:           "track",
		ChromeosVersion: "v",
		Version:         "output",
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.GetVersionInfoResponse{})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts); diff != "" {
		t.Errorf("Expected: {%v}, got: {%v}, {%v}", expected, resp, diff)
	}
}

func TestGetVersionInfoShouldFail(t *testing.T) {
	// Run this testcase parallel
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)
	s.commandExecutor = &executor.FakeCommander{
		Err: errors.New("exec command failed"),
	}

	ctx := context.Background()
	req := &pb.GetVersionInfoRequest{}
	res, err := s.GetVersionInfo(ctx, req)

	if err == nil {
		t.Errorf("Expected error")
	}

	if res != nil {
		t.Errorf("Expected the reuslt should be nil")
	}
}

func TestGetDUTDetailShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	mockSwarm := services.NewMockISwarmingService(ctl)

	s := createMockServer(t)
	s.swarmingService = mockSwarm
	s.commandExecutor = &executor.FakeCommander{
		CmdOutput: `
192.168.231.137	satlab-0wgtfqin1846803b-one
192.168.231.137	satlab-0wgtfqin1846803b-host5
192.168.231.222	satlab-0wgtfqin1846803b-host11
192.168.231.222	satlab-0wgtfqin1846803b-host12
  `,
	}
	mockData := &swarmingapi.BotInfo{BotId: "test bot"}

	ftt.Run("GetDUTDetailShouldSuccess", t, func(t *ftt.Test) {
		mockSwarm.EXPECT().GetBot(ctx, "satlab-0wgtfqin1846803b-host12").Return(mockData, nil)

		// Act
		req := &pb.GetDutDetailRequest{
			Address: "192.168.231.222",
		}
		resp, err := s.GetDutDetail(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&pb.GetDutDetailResponse{
			BotId:      "test bot",
			Dimensions: []*pb.StringListPair{},
		}))
	})

}

func TestListDutTasksShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	mockSwarm := services.NewMockISwarmingService(ctl)

	mockData := &services.TasksIterator{
		Cursor: "next_cursor",
		Tasks: []services.Task{
			{
				Id: "task id",
			},
		},
	}

	// Create a mock data
	s := createMockServer(t)
	s.swarmingService = mockSwarm

	s.commandExecutor = &executor.FakeCommander{
		CmdOutput: `
192.168.231.222	satlab-0wgtfqin1846803b-host12
  `,
	}

	ftt.Run("ListDutTasksShouldSuccess", t, func(t *ftt.Test) {
		mockSwarm.EXPECT().ListBotTasks(ctx, "satlab-0wgtfqin1846803b-host12", "", 1).Return(mockData, nil)
		// Act
		req := &pb.ListDutTasksRequest{
			PageToken: "",
			PageSize:  1,
			Address:   "192.168.231.222",
		}
		resp, err := s.ListDutTasks(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&pb.ListDutTasksResponse{
			NextPageToken: "next_cursor",
			Tasks: []*pb.Task{
				{
					Id: "task id",
				},
			}}))
	})
}

func TestListDutEventsShouldSuccess(t *testing.T) {

	t.Parallel()
	ctx := context.Background()
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	mockSwarm := services.NewMockISwarmingService(ctl)

	s := createMockServer(t)
	s.swarmingService = mockSwarm
	s.commandExecutor = &executor.FakeCommander{
		CmdOutput: `
192.168.231.222	satlab-0wgtfqin1846803b-host12
  `,
	}
	mockData := &services.BotEventsIterator{
		Cursor: "next_cursor",
		Events: []services.BotEvent{
			{
				TaskID: "task id",
			},
		},
	}

	ftt.Run("ListDutEventsShouldSuccess", t, func(t *ftt.Test) {
		mockSwarm.EXPECT().ListBotEvents(ctx, "satlab-0wgtfqin1846803b-host12", "", 1).Return(mockData, nil)

		// Act
		req := &pb.ListDutEventsRequest{
			PageToken: "",
			PageSize:  1,
			Address:   "192.168.231.222",
		}
		resp, err := s.ListDutEvents(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&pb.ListDutEventsResponse{
			NextPageToken: "next_cursor",
			Events: []*pb.BotEvent{
				{
					TaskId: "task id",
				},
			},
		}))
	})

}

func getDUTOutput() []byte {
	return []byte(`[{
"name": "satlab-0wgatfqi21498062-jeff137-c",
"machineLsePrototype": "",
"hostname": "satlab-0wgatfqi21498062-jeff137-c",
"chromeosMachineLse": {
	"deviceLse": {
        "dut": {
			"hostname": "satlab-0wgatfqi21498062-jeff137-c",
			"pools": [
				"jev-satlab"
			]
        }
	}
},
"machines": [
      "JEFF137-c"
]}]`)
}

func getAssetOutput() []byte {
	return []byte(`[{
"name": "JEFF137-c",
"type": "DUT",
"model": "atlas",
"info": {
	"model": "atlas",
	"buildTarget": "atlas"
}}]`)
}

func shivasTestHelper(hasData bool) executor.IExecCommander {
	return &executor.FakeCommander{
		FakeFn: func(in *exec.Cmd) ([]byte, error) {
			if in.Path == "/usr/local/bin/shivas" {
				for _, arg := range in.Args {
					if arg == "dut" {
						// execute a command to get dut
						if hasData {
							return getDUTOutput(), nil
						} else {
							return []byte("[]"), nil
						}
					}

					if arg == "asset" {
						// execute a command to get asset
						if hasData {
							return getAssetOutput(), nil
						} else {
							return []byte("[]"), nil
						}
					}
				}
			}

			if in.Path == "/usr/local/bin/get_host_identifier" {
				return []byte("satlab-id"), nil
			}
			if in.Path == "/usr/local/bin/docker" {
				return []byte("192.168.231.222	satlab-0wgatfqi21498062-jeff137-c"), nil
			}

			return nil, errors.New(fmt.Sprintf("handle command: %v", in.Path))
		},
	}

}

func TestListEnrolledDutsShouldSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Create a mock data
	s := createMockServer(t)
	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, gomock.Any()).Return(nil, nil)

	s.commandExecutor = shivasTestHelper(true)
	req := &pb.ListEnrolledDutsRequest{}
	resp, err := s.ListEnrolledDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListEnrolledDutsResponse{}, pb.Dut{})

	// Create a expected result
	expected := &pb.ListEnrolledDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:     "satlab-0wgatfqi21498062-jeff137-c",
				Hostname: "satlab-0wgatfqi21498062-jeff137-c",
				Address:  "192.168.231.222",
				Pools:    []string{"jev-satlab"},
				Model:    "atlas",
				Board:    "atlas",
				State:    "unknown",
			},
		},
	}

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListEnrolledDutsShouldFail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Create a mock data
	s := createMockServer(t)

	s.commandExecutor = &executor.FakeCommander{
		Err: errors.New("execute command failed"),
	}
	req := &pb.ListEnrolledDutsRequest{}
	resp, err := s.ListEnrolledDuts(ctx, req)

	// Assert
	if err == nil {
		t.Errorf("Should be failed in this test case")
	}

	if resp != nil {
		t.Errorf("Should get the empty result.")
	}
}

func TestListConnectedAndEnrolledDutsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Create a mock data
	s := createMockServer(t)

	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, gomock.Any()).Return(nil, nil)
	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: true, HasTestImage: true},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: true, HasTestImage: true},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, "192.168.231.2").Return("board", nil)
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, "192.168.231.2").Return("model", nil)
	s.dutService.(*mk.MockDUTServices).On("GetServoSerial", ctx, "192.168.231.2", mock.Anything).Return(true, "SERVOSERIAL", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)

	s.commandExecutor = shivasTestHelper(true)

	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListDutsResponse{}, pb.Dut{})
	// Create a expected result
	expected := &pb.ListDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:         "satlab-0wgatfqi21498062-jeff137-c",
				Hostname:     "satlab-0wgatfqi21498062-jeff137-c",
				Address:      "192.168.231.222",
				Pools:        []string{"jev-satlab"},
				Model:        "atlas",
				Board:        "atlas",
				IsPingable:   true,
				HasTestImage: true,
				MacAddress:   "00:14:3d:14:c4:02",
				State:        "unknown",
				BotInfo:      nil,
				CcdStatus:    "Unknown",
			},
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.2",
				Pools:        nil,
				Model:        "model",
				Board:        "board",
				MacAddress:   "e8:9f:80:83:3d:c8",
				ServoSerial:  "SERVOSERIAL",
				IsPingable:   true,
				HasTestImage: true,
				State:        "",
				BotInfo:      nil,
				CcdStatus:    "Opened",
			},
		},
	}

	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Dut) bool {
			return x.GetAddress() > y.GetAddress()
		})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListDisconnectedAndEnrolledDutsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Create a mock data

	s := createMockServer(t)

	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, gomock.Any()).Return(nil, nil)
	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: false, HasTestImage: false},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: false, HasTestImage: false},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, "192.168.231.2").Return("", nil)
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, "192.168.231.2").Return("", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)

	s.commandExecutor = shivasTestHelper(true)

	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListDutsResponse{}, pb.Dut{})

	// Create a expected result
	expected := &pb.ListDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:         "satlab-0wgatfqi21498062-jeff137-c",
				Hostname:     "satlab-0wgatfqi21498062-jeff137-c",
				Address:      "192.168.231.222",
				Pools:        []string{"jev-satlab"},
				Model:        "atlas",
				Board:        "atlas",
				IsPingable:   false,
				HasTestImage: false,
				MacAddress:   "00:14:3d:14:c4:02",
				State:        "unknown",
				CcdStatus:    "Unknown",
			},
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.2",
				Pools:        nil,
				Model:        "",
				Board:        "",
				MacAddress:   "e8:9f:80:83:3d:c8",
				IsPingable:   false,
				HasTestImage: false,
				State:        "",
			},
		},
	}

	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Dut) bool {
			return x.GetAddress() > y.GetAddress()
		})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListConnectedAndUnenrolledDutsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	s := createMockServer(t)

	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, gomock.Any()).Return(nil, nil)
	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: true, HasTestImage: true},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: true, HasTestImage: true},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, mock.Anything).Return("board", nil)
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, mock.Anything).Return("model", nil)
	s.dutService.(*mk.MockDUTServices).On("GetServoSerial", ctx, "192.168.231.222").Return(false, "", nil)
	s.dutService.(*mk.MockDUTServices).On("GetServoSerial", ctx, "192.168.231.2").Return(true, "SERVOSERIAL", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)
	s.commandExecutor = shivasTestHelper(false)
	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListDutsResponse{}, pb.Dut{})

	// Create a expected result
	expected := &pb.ListDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.222",
				Pools:        nil,
				Model:        "model",
				Board:        "board",
				IsPingable:   true,
				HasTestImage: true,
				MacAddress:   "00:14:3d:14:c4:02",
				CcdStatus:    "Unknown",
			},
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.2",
				Pools:        nil,
				Model:        "model",
				Board:        "board",
				MacAddress:   "e8:9f:80:83:3d:c8",
				IsPingable:   true,
				HasTestImage: true,
				ServoSerial:  "SERVOSERIAL",
				CcdStatus:    "Opened",
			},
		},
	}

	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Dut) bool {
			return x.GetAddress() > y.GetAddress()
		})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListDisconnectedAndUnenrolledDutsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	s := createMockServer(t)
	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, gomock.Any()).Return(nil, nil)
	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: false, HasTestImage: false},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: false, HasTestImage: false},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, mock.Anything).Return("", nil)
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, mock.Anything).Return("", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)

	s.commandExecutor = shivasTestHelper(false)

	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListDutsResponse{}, pb.Dut{})

	// Create a expected result
	expected := &pb.ListDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.222",
				Pools:        nil,
				Model:        "",
				Board:        "",
				IsPingable:   false,
				HasTestImage: false,
				MacAddress:   "00:14:3d:14:c4:02",
			},
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.2",
				Pools:        nil,
				Model:        "",
				Board:        "",
				MacAddress:   "e8:9f:80:83:3d:c8",
				IsPingable:   false,
				HasTestImage: false,
			},
		},
	}

	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Dut) bool {
			return x.GetAddress() > y.GetAddress()
		})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListConnectedAndEnrolledDutsWithoutGetBoardAndModelInformationShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	s := createMockServer(t)
	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, gomock.Any()).Return(nil, nil)
	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: true, HasTestImage: true},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: true, HasTestImage: true},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, "192.168.231.2").Return("", errors.New("can't get board"))
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, "192.168.231.2").Return("", errors.New("can't get model"))
	s.dutService.(*mk.MockDUTServices).On("GetServoSerial", ctx, "192.168.231.2", mock.Anything).Return(true, "SERVOSERIAL", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)
	s.commandExecutor = shivasTestHelper(true)

	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListDutsResponse{}, pb.Dut{})
	// Create a expected result
	expected := &pb.ListDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:         "satlab-0wgatfqi21498062-jeff137-c",
				Hostname:     "satlab-0wgatfqi21498062-jeff137-c",
				Address:      "192.168.231.222",
				Pools:        []string{"jev-satlab"},
				Model:        "atlas",
				Board:        "atlas",
				IsPingable:   true,
				HasTestImage: true,
				MacAddress:   "00:14:3d:14:c4:02",
				State:        "unknown",
				CcdStatus:    "Unknown",
			},
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.2",
				Pools:        nil,
				Model:        "",
				Board:        "",
				MacAddress:   "e8:9f:80:83:3d:c8",
				IsPingable:   true,
				HasTestImage: true,
				ServoSerial:  "SERVOSERIAL",
				State:        "",
				CcdStatus:    "Opened",
			},
		},
	}

	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Dut) bool {
			return x.GetAddress() > y.GetAddress()
		})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListConnectedDutsShouldFail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	s := createMockServer(t)

	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: false, HasTestImage: false},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: false, HasTestImage: false},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, mock.Anything).Return("board", nil)
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, mock.Anything).Return("model", nil)
	s.dutService.(*mk.MockDUTServices).On("GetServoSerial", ctx, mock.Anything).Return(true, "SERVOSERIAL", nil)
	s.commandExecutor = &executor.FakeCommander{
		Err: errors.New("execute command failed"),
	}
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)

	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err == nil {
		t.Errorf("should fail")
	}
	if resp != nil {
		t.Errorf("response should be empty, but got %v", resp)
	}
}

func TestDeleteDutsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			// 192.168.231.222	satlab-0wgtfqin1846803b-host12
			if c.Path == paths.GetHostIdentifierScript {
				return []byte("0wgtfqin1846803b"), nil
			} else {
				return nil, errors.New(fmt.Sprintf("execute a command %v\n", c.Path))
			}
		},
	}

	hostnames := []string{"satlab-0wgtfqin1846803b-host12"}
	ufs := mockDeleteClient{}

	resp, err := innerDeleteDuts(ctx, e, &ufs, hostnames, false)
	if err != nil {
		t.Errorf("unexpected error: %v\n", err)
		return
	}

	expected := &dut.DeleteDUTResult{
		MachineLSEs: []*ufsModels.MachineLSE{
			{
				Name:     "machineLSEs/satlab-0wgtfqin1846803b-host12",
				Machines: []string{"asset-satlab-0wgtfqin1846803b-host12"},
				Rack:     "rack-satlab-0wgtfqin1846803b-host12",
			},
		},
		DutResults: &dut.Result{
			Pass: []string{"satlab-0wgtfqin1846803b-host12"},
			Fail: []string{},
		},
		AssetResults: &dut.Result{},
		RackResults:  &dut.Result{},
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(ufsModels.MachineLSE{})

	if diff := cmp.Diff(resp, expected, ignorePBFieldOpts); diff != "" {
		t.Errorf("unexpected diff: %v\n", diff)
	}
}

func TestFullDeleteDutsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			// 192.168.231.222	satlab-0wgtfqin1846803b-host12
			if c.Path == paths.GetHostIdentifierScript {
				return []byte("0wgtfqin1846803b"), nil
			} else {
				return nil, errors.New(fmt.Sprintf("execute a command %v\n", c.Path))
			}
		},
	}

	hostnames := []string{"host12"}
	ufs := mockDeleteClient{}

	resp, err := innerDeleteDuts(ctx, e, &ufs, hostnames, true)
	if err != nil {
		t.Errorf("unexpected error: %v\n", err)
		return
	}

	expected := &dut.DeleteDUTResult{
		MachineLSEs: []*ufsModels.MachineLSE{
			{
				Name:     "machineLSEs/satlab-0wgtfqin1846803b-host12",
				Machines: []string{"asset-satlab-0wgtfqin1846803b-host12"},
				Rack:     "rack-satlab-0wgtfqin1846803b-host12",
			},
		},
		DutResults: &dut.Result{
			Pass: []string{"satlab-0wgtfqin1846803b-host12"},
			Fail: []string{},
		},
		AssetResults: &dut.Result{
			Pass: []string{"asset-satlab-0wgtfqin1846803b-host12"},
			Fail: []string{},
		},
		RackResults: &dut.Result{
			Pass: []string{"rack-satlab-0wgtfqin1846803b-host12"},
			Fail: []string{},
		},
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(ufsModels.MachineLSE{})

	if diff := cmp.Diff(resp, expected, ignorePBFieldOpts); diff != "" {
		t.Errorf("unexpected diff: %v\n", diff)
	}
}

func TestDeleteDutsShouldFail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock data
	e := &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			return nil, errors.New("fail")
		},
	}

	hostnames := []string{"hostname"}
	ufs := mockDeleteClient{}

	resp, err := innerDeleteDuts(ctx, e, &ufs, hostnames, true)
	if err == nil {
		t.Errorf("should get an err")
	}

	if resp != nil {
		t.Errorf("result should be empty")
	}
}

func TestGetNetworkInfoShouldSuccess(t *testing.T) {
	t.Parallel()
	// Create a mock server
	s := createMockServer(t)

	expected := &pb.GetNetworkInfoResponse{
		Hostname:    "127.0.0.1",
		MacAddress:  "aa:bb:cc:dd:ee:ff",
		IsConnected: true,
	}

	s.commandExecutor = &executor.FakeCommander{
		FakeFn: func(in *exec.Cmd) ([]byte, error) {
			cmd := strings.Join(in.Args, " ")
			if in.Path == paths.GetHostIPScript {
				return []byte(expected.Hostname), nil
			} else if cmd == fmt.Sprintf("%s exec dhcp cat %s", paths.DockerPath, fmt.Sprintf(paths.NetInfoPathTemplate, "eth0")) {
				return []byte(expected.MacAddress), nil
			} else if cmd == fmt.Sprintf(fmt.Sprintf("%s exec dhcp ip route show", paths.DockerPath)) {
				return []byte(
					fmt.Sprintf("%v/24 dev eth0 scope link  src %v", expected.Hostname, expected.Hostname),
				), nil
			} else if in.Path == paths.Grep {
				return []byte(expected.Hostname), nil
			} else if in.Path == paths.GetHostMACScript {
				return []byte(expected.MacAddress), nil
			}
			return nil, errors.New(fmt.Sprintf("handle command: %v", in.Path))
		},
		CmdOutput: fmt.Sprintf("%v/24 dev eth0 scope link  src %v", expected.Hostname, expected.Hostname),
	}

	ctx := context.Background()

	req := &pb.GetNetworkInfoRequest{}

	res, err := s.GetNetworkInfo(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: %v", err)
	}

	if diff := cmp.Diff(expected, res, cmpopts.IgnoreUnexported(pb.GetNetworkInfoResponse{})); diff != "" {
		t.Errorf("Expected %v, got %v", expected, res)
	}
}

func Test_ListTestPlansShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock server and mock services
	s := createMockServer(t)

	expected := &pb.ListTestPlansResponse{Names: []string{"testplan1.json"}}

	s.bucketService.(*bucket_services.MockBucketServices).
		On("ListTestplans", ctx).
		Return([]string{"testplan1.json"}, nil)

	// Act
	req := &pb.ListTestPlansRequest{}
	resp, err := s.ListTestPlans(ctx, req)

	// Asset
	if err != nil {
		t.Errorf("unexpected error: %v\n", err)
	}

	// ignore generated pb code
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(
		pb.ListTestPlansResponse{},
	)

	if diff := cmp.Diff(resp, expected, ignorePBFieldOpts); diff != "" {
		t.Errorf("unexpected diff: %v\n", diff)
	}
}

// Test_ValidateCloudConfiguration test the user input the
// configuration is correct or not.
func Test_ValidateCloudConfiguration(t *testing.T) {
	cases := []struct {
		name  string
		req   *pb.SetCloudConfigurationRequest
		valid bool
	}{
		{
			name: "empty boto key",
			req: &pb.SetCloudConfigurationRequest{
				BotoKeyId:     "  ",
				BotoKeySecret: "secret",
				GcsBucketUrl:  "bucket",
			},
			valid: false,
		},
		{
			name: "empty secret id",
			req: &pb.SetCloudConfigurationRequest{
				BotoKeyId:     "boto key",
				BotoKeySecret: "",
				GcsBucketUrl:  "bucket",
			},
			valid: false,
		},
		{
			name: "empty bucket",
			req: &pb.SetCloudConfigurationRequest{
				BotoKeyId:     "boto key",
				BotoKeySecret: "secret",
				GcsBucketUrl:  "",
			},
			valid: false,
		},
		{
			name: "correct input",
			req: &pb.SetCloudConfigurationRequest{
				BotoKeyId:     "boto key",
				BotoKeySecret: "secret",
				GcsBucketUrl:  "bucket",
			},
			valid: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCloudConfiguration(tt.req)
			valid := err == nil
			if valid != tt.valid {
				t.Errorf("unexpected error.")
			}
		})
	}

}

func Test_RemoveBucketPrefixAndSuffix(t *testing.T) {
	cases := []struct {
		name       string
		bucket_url string
		expected   string
	}{
		{
			name:       "full url",
			expected:   "bucket",
			bucket_url: "gs://bucket/",
		},
		{
			name:       "without suffix",
			expected:   "bucket",
			bucket_url: "gs://bucket",
		},
		{
			name:       "without prefix",
			expected:   "bucket",
			bucket_url: "bucket/",
		},
		{
			name:       "correct input",
			expected:   "bucket",
			bucket_url: "bucket",
		},
		{
			name:       "with many suffix",
			expected:   "bucket",
			bucket_url: "bucket///////",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			out := removeGCSBucketPrefixAndSuffix(tt.bucket_url)
			if out != tt.expected {
				t.Errorf("unexpected error.")
			}
		})
	}
}

func Test_Reboot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a mock server and mock services
	s := createMockServer(t)
	s.commandExecutor = &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			if c.Path == paths.Reboot {
				return []byte(""), nil
			}
			return nil, errors.New(fmt.Sprintf("unknow command %v", c))
		},
	}

	req := &pb.RebootRequest{}

	_, err := s.Reboot(ctx, req)

	if err != nil {
		t.Errorf("unexpected error, got an error: %v", err)
	}
}

func Test_validateUpdatePools(t *testing.T) {
	cases := []struct {
		name     string
		in       []string
		expected bool
	}{
		{
			name:     "remove pool1 from [pool1, pool2]",
			expected: true,
			in:       []string{"pool2"},
		},
		{
			name:     "remove all pools",
			expected: false,
			in:       []string{},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			out := validateUpdatePools(tt.in)
			if out != tt.expected {
				t.Errorf("unexpected error.")
			}
		})
	}

}

func Test_RepairDutsShouldWork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a mock server and mock services
	s := createMockServer(t)
	s.commandExecutor = &executor.FakeCommander{
		FakeFn: func(c *exec.Cmd) ([]byte, error) {
			if c.Path == paths.GetHostIdentifierScript {
				return []byte("satlab-id"), nil
			} else if c.Args[0] == paths.ShivasCLI && c.Args[1] == "repair-duts" && c.Args[len(c.Args)-1] == "satlab-satlab-id-dut1" {
				return []byte(`Build Link: https://ci.chromium.org/p/chromeos/builders/external-cienet/repair/build-id1
### Batch tasks URL ###
Task Link: https://chromeos-swarming.appspot.com/1`), nil
			} else if c.Args[0] == paths.ShivasCLI && c.Args[1] == "repair-duts" && c.Args[len(c.Args)-1] == "satlab-satlab-id-dut2" {
				return nil, errors.New("Not found")
			}
			return nil, errors.New(fmt.Sprintf("unknow command %v", c))
		},
	}

	// Act
	req := &pb.RepairDutsRequest{
		Hostnames: []string{"dut1", "dut2"},
		Deep:      true,
	}

	resp, err := s.RepairDuts(ctx, req)
	if err != nil {
		t.Errorf("unexpected error, got an error: %v", err)
	}

	// Assert
	expected := &pb.RepairDutsResponse{
		Result: []*pb.RepairDutsResponse_RepairResult{
			{
				Hostname:  "dut1",
				BuildLink: "https://ci.chromium.org/p/chromeos/builders/external-cienet/repair/build-id1",
				TaskLink:  "https://chromeos-swarming.appspot.com/1",
				IsSuccess: true,
			},
			{
				Hostname:  "dut2",
				IsSuccess: false,
			},
		},
	}

	// ignore generated pb code
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(
		pb.RepairDutsResponse{},
		pb.RepairDutsResponse_RepairResult{},
	)

	if diff := cmp.Diff(resp, expected, ignorePBFieldOpts); diff != "" {
		t.Errorf("unexpected diff: %v\n", diff)
	}

}

func TestListTasksShouldSuccess(t *testing.T) {
	// Reopen the tree! Make this test non-parallel.
	//
	// t.Parallel()
	ctx := context.Background()
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	mockSwarm := services.NewMockISwarmingService(ctl)

	expectedData := &services.JobsIterator{
		Cursor: "next_cursor",
		Jobs: []*pb.Job{
			{
				JobId:       "1234",
				Name:        "suite:audio",
				CreatedTime: &timestamppb.Timestamp{Seconds: 1517260502, Nanos: 649750000},
				Status:      pb.Job_PENDING,
				Hostname:    "",
				LabelPool:   "pool_1",
				SatlabId:    "satlab-id",
				TaskUrl:     "http://ci.chromium.org/b/1234",
			},
		},
	}

	mockTr := []*swarmingapi.TaskResultResponse{
		{
			State:     swarmingapi.TaskState_PENDING,
			CreatedTs: &timestamppb.Timestamp{Seconds: 1517260502, Nanos: 649750000},
			Tags: []string{
				"authenticated:project:crproj",
				"buildbucket_bucket:proj/external-abc-xyz",
				"buildbucket_build_id:1234",
				"builder:cros_test_platform",
				"label-pool:pool_1",
				"luci_project:crproj",
				"pool:external-pool",
				"satlab-id:satlab-id",
				"test-type:suite",
				"label-suite:audio",
			},
			BotDimensions: []*swarmingapi.StringListPair{
				{
					Key:   "dut_name",
					Value: []string{"dut1"},
				},
			},
		},
	}

	mockTaskResults := &swarmingapi.TaskListResponse{
		Items:  mockTr,
		Cursor: "next_cursor",
	}

	// Create a mock data
	s := createMockServer(t)
	s.swarmingService = mockSwarm

	ftt.Run("TestListTasksShouldSuccess", t, func(t *ftt.Test) {
		req := &pb.ListJobsRequest{
			PageToken: "",
			Limit:     1,
			Tags: []*pb.Tag{
				{Key: "pool", Value: "ext"},
				{Key: "satlab-id", Value: "satlab-id"}},
		}
		mockSwarm.EXPECT().ListTasks(ctx, getTaskListReq(req)).Return(mockTaskResults, nil)
		resp, err := s.ListJobs(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&pb.ListJobsResponse{
			NextPageToken: expectedData.Cursor,
			Jobs:          expectedData.Jobs,
		}))
	})
}

func getTaskListReq(in *pb.ListJobsRequest) *swarmingapi.TasksWithPerfRequest {
	return &swarmingapi.TasksWithPerfRequest{
		Start:                   in.GetCreatedTimeGt(),
		End:                     in.GetCreatedTimeLt(),
		Tags:                    getListTasksRequestTags(in),
		Limit:                   int32(in.GetLimit()),
		State:                   swarmingapi.StateQuery(in.GetQueryStatus()), // default is PENDING
		Sort:                    swarmingapi.SortQuery(in.GetSortBy()),       // default is CREATED_TS
		Cursor:                  in.GetPageToken(),
		IncludePerformanceStats: false,
	}
}

func TestListConnectedAndEnrolledDutsShouldSuccessWithBotInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Create a mock data
	s := createMockServer(t)

	mockBotList := &swarmingapi.BotInfoListResponse{
		Items: []*swarmingapi.BotInfo{
			{
				Dimensions: []*swarmingapi.StringListPair{
					{
						Key:   "drone",
						Value: []string{"satlab-satlab-id"},
					},
				},
				BotId:       "satlab-0wgatfqi21498062-jeff137-c",
				IsDead:      false,
				Quarantined: false,
				TaskId:      "abcd",
				TaskName:    "bb-21344/proj/test_runner",
			},
		},
	}
	reqListBot := &swarmingapi.BotsRequest{
		Limit: 25,
		Dimensions: []*swarmingapi.StringPair{
			{
				Key:   "drone",
				Value: "satlab-satlab-id",
			},
			{
				Key: "pool",
			},
		},
	}

	s.swarmingService.(*services.MockISwarmingService).EXPECT().ListBots(ctx, reqListBot).Return(mockBotList, nil).AnyTimes()
	s.dutService.(*mk.MockDUTServices).On("GetUSBDevicePaths", ctx).Return([]enumeration.USBDevice{}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetConnectedIPs", ctx).Return([]dut_services.Device{
		{IP: "192.168.231.222", MACAddress: "00:14:3d:14:c4:02", IsPingable: true, HasTestImage: true},
		{IP: "192.168.231.2", MACAddress: "e8:9f:80:83:3d:c8", IsPingable: true, HasTestImage: true},
	}, nil)
	s.dutService.(*mk.MockDUTServices).On("GetBoard", ctx, "192.168.231.2").Return("board", nil)
	s.dutService.(*mk.MockDUTServices).On("GetModel", ctx, "192.168.231.2").Return("model", nil)
	s.dutService.(*mk.MockDUTServices).On("GetServoSerial", ctx, "192.168.231.2", mock.Anything).Return(true, "SERVOSERIAL", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.2").Return("Opened", nil)
	s.dutService.(*mk.MockDUTServices).On("GetCCDStatus", ctx, "192.168.231.222").Return("Unknown", nil)

	s.commandExecutor = shivasTestHelper(true)

	req := &pb.ListDutsRequest{}
	resp, err := s.ListDuts(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}

	// ignore pb fields in `FirmwareUpdateCommandOutput`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(pb.ListDutsResponse{}, pb.Dut{}, pb.BotInfo{}, pb.StringListPair{})
	// Create a expected result
	expected := &pb.ListDutsResponse{
		Duts: []*pb.Dut{
			{
				Name:         "satlab-0wgatfqi21498062-jeff137-c",
				Hostname:     "satlab-0wgatfqi21498062-jeff137-c",
				Address:      "192.168.231.222",
				Pools:        []string{"jev-satlab"},
				Model:        "atlas",
				Board:        "atlas",
				IsPingable:   true,
				HasTestImage: true,
				MacAddress:   "00:14:3d:14:c4:02",
				State:        "unknown",
				BotInfo: &pb.BotInfo{
					BotState:    pb.BotInfo_BUSY,
					CurrentTask: "https://chromeos-swarming.appspot.com/task?id=abcd",
					TaskName:    "Running",
					Dimensions: []*pb.StringListPair{
						{
							Key:    "drone",
							Values: []string{"satlab-satlab-id"},
						},
					},
				},
				CcdStatus: "Unknown",
			},
			{
				Name:         "",
				Hostname:     "",
				Address:      "192.168.231.2",
				Pools:        nil,
				Model:        "model",
				Board:        "board",
				MacAddress:   "e8:9f:80:83:3d:c8",
				ServoSerial:  "SERVOSERIAL",
				IsPingable:   true,
				HasTestImage: true,
				State:        "",
				BotInfo:      nil,
				CcdStatus:    "Opened",
			},
		},
	}

	sortModelsOpts := cmpopts.SortSlices(
		func(x, y *pb.Dut) bool {
			return x.GetAddress() > y.GetAddress()
		})

	if diff := cmp.Diff(expected, resp, ignorePBFieldOpts, sortModelsOpts); diff != "" {
		t.Errorf("diff: %v\n", diff)
	}
}

func TestListTasksWithChildTaskStatusShouldSuccess(t *testing.T) {
	// Reopen the tree! Make this test non-parallel.
	// Tree closed on 2024-03-19 at 15:42 America/Los_Angeles time.
	//
	// t.Parallel()
	ctx := context.Background()
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	mockSwarm := services.NewMockISwarmingService(ctl)

	expectedData := &services.JobsIterator{
		Cursor: "next_cursor",
		Jobs: []*pb.Job{
			{
				JobId:       "1234",
				Name:        "suite:audio",
				CreatedTime: &timestamppb.Timestamp{Seconds: 1517260502, Nanos: 649750000},
				StartTime:   &timestamppb.Timestamp{Seconds: 1517260502, Nanos: 649750000},
				Status:      pb.Job_RUNNING,
				Hostname:    "",
				LabelPool:   "pool_1",
				SatlabId:    "satlab-id",
				TaskUrl:     "http://ci.chromium.org/b/1234",
				ChildStatusCount: &pb.TasksStatusCount{
					TaskCount: []*pb.TasksStatusCount_TaskCount{
						{
							State: pb.StateQuery_QUERY_PENDING_RUNNING,
							Count: 1,
						},
						{
							State: pb.StateQuery_QUERY_ALL,
							Count: 1,
						},
					},
				},
			},
		},
	}
	mockTr := []*swarmingapi.TaskResultResponse{
		{
			State:     swarmingapi.TaskState_RUNNING,
			CreatedTs: &timestamppb.Timestamp{Seconds: 1517260502, Nanos: 649750000},
			StartedTs: &timestamppb.Timestamp{Seconds: 1517260502, Nanos: 649750000},
			Tags: []string{
				"authenticated:project:crproj",
				"buildbucket_bucket:proj/external-abc-xyz",
				"buildbucket_build_id:1234",
				"builder:cros_test_platform",
				"label-pool:pool_1",
				"luci_project:crproj",
				"pool:external-pool",
				"satlab-id:satlab-id",
				"test-type:suite",
				"label-suite:audio",
			},
			BotDimensions: []*swarmingapi.StringListPair{
				{
					Key:   "dut_name",
					Value: []string{"dut1"},
				},
			},
		},
	}

	mockTaskResults := &swarmingapi.TaskListResponse{
		Items:  mockTr,
		Cursor: "next_cursor",
	}

	// Create a mock data
	s := createMockServer(t)
	s.swarmingService = mockSwarm

	ftt.Run("TestListTasksShouldSuccess", t, func(t *ftt.Test) {
		req := &pb.ListJobsRequest{
			PageToken: "",
			Limit:     1,
			JobType:   1,
			Tags: []*pb.Tag{
				{Key: "pool", Value: "ext"},
				{Key: "satlab-id", Value: "satlab-id"}},
		}

		mockSwarm.EXPECT().ListTasks(ctx, getTaskListReq(req)).Return(mockTaskResults, nil)
		s.swarmingService.(*services.MockISwarmingService).EXPECT().CountTasks(ctx, gomock.Any()).Return(&swarmingapi.TasksCount{
			Count: 1,
		}, nil).AnyTimes()

		resp, err := s.ListJobs(ctx, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(&pb.ListJobsResponse{
			NextPageToken: expectedData.Cursor,
			Jobs:          expectedData.Jobs,
		}))
	})
}

func Test_AbortJobsShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Create a mock data
	s := createMockServer(t)

	os.Setenv(site.CTPSwarmingPoolEnv, "pool")
	defer os.Unsetenv(site.CTPSwarmingPoolEnv)
	start := timestamppb.Now()
	end := timestamppb.Now()

	tags := []string{}
	tags = append(tags, fmt.Sprintf("buildbucket_build_id:%s", "task-1|task-2"))
	tags = append(tags, fmt.Sprintf("pool:%s", site.GetCTPSwarmingPool()))
	reqCancelTasks := services.CancelTasksRequest{
		Tags:  tags,
		Start: start,
		End:   end,
	}
	s.swarmingService.(*services.MockISwarmingService).EXPECT().CancelTasks(ctx, reqCancelTasks).Return(nil).AnyTimes()

	req := &pb.AbortJobsRequest{
		Ids:           []string{"task-1", "task-2"},
		JobType:       pb.Job_SUITE,
		CreatedTimeGt: start,
		CreatedTimeLt: end,
	}
	_, err := s.AbortJobs(ctx, req)

	// Assert
	if err != nil {
		t.Errorf("Should not return error, but got an error: {%v}", err)
	}
}

func newMachineLSE(name string, pools []string) *ufsModels.MachineLSE {
	return &ufsModels.MachineLSE{
		Name:     name,
		Hostname: name,
		Machines: []string{"machine"},
		Lse: &ufsModels.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufsModels.ChromeOSMachineLSE{
				ChromeosLse: &ufsModels.ChromeOSMachineLSE_Dut{
					Dut: &ufsModels.ChromeOSDeviceLSE{
						Device: &ufsModels.ChromeOSDeviceLSE_Dut{
							Dut: &ufsLabpb.DeviceUnderTest{
								Hostname: name,
								Pools:    pools,
								Peripherals: &ufsLabpb.Peripherals{
									Servo: &ufsLabpb.Servo{
										ServoFwChannel: ufsLabpb.ServoFwChannel_SERVO_FW_PREV,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func marshallMachineLSESlice(expected []*ufsModels.MachineLSE) []string {
	m := protojson.MarshalOptions{
		Indent: "  ",
	}
	s := []string{}
	for _, elem := range expected {
		js, _ := m.Marshal(elem)
		s = append(s, string(js))
	}

	return s
}

func Test_removeAllPoolShouldSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := createMockServer(t)
	hostname := "hostname1"
	satlabID := "satlab-id"

	s.commandExecutor = &executor.FakeCommander{FakeFn: func(c *exec.Cmd) ([]byte, error) {
		if c.Args[0] == paths.ShivasCLI {
			if c.Args[1] == "update" {
				return nil, errors.New("found no pool for device satlab-<satlab-id>")
			} else if c.Args[1] == "get" {
				gotMachineLSE := []*ufsModels.MachineLSE{newMachineLSE(hostname, []string{})}
				marshalled := marshallMachineLSESlice(gotMachineLSE)

				return []byte(fmt.Sprintf("[%v]", strings.Join(marshalled, ","))), nil
			} else {
				return nil, errors.New(fmt.Sprintf("Un-support command: %v", c.Args))
			}
		}
		return nil, errors.New(fmt.Sprintf("Un-support command: %v", c.Args))
	}}

	err := removeAllPoolsFromDUT(ctx, s.commandExecutor, satlabID, hostname)

	if err != nil {
		t.Errorf("should sucess, but got an error: %v", err)
		return
	}
}

func Test_removeAllPoolShouldFailWhenGettingBotInfoFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := createMockServer(t)
	hostname := "hostname1"
	satlabID := "satlab-id"

	s.commandExecutor = &executor.FakeCommander{FakeFn: func(c *exec.Cmd) ([]byte, error) {
		if c.Args[0] == paths.ShivasCLI {
			if c.Args[1] == "update" {
				return nil, errors.New("found no pool for device satlab-<satlab-id>")
			} else if c.Args[1] == "get" {
				return nil, errors.New("execute the command failed")
			} else {
				return nil, errors.New(fmt.Sprintf("Un-support command: %v", c.Args))
			}
		}
		return nil, errors.New(fmt.Sprintf("Un-support command: %v", c.Args))
	}}

	err := removeAllPoolsFromDUT(ctx, s.commandExecutor, satlabID, hostname)

	if err == nil {
		t.Errorf("should get an error")
		return
	}
}

func Test_removeAllPoolShouldSuccessWhenCommandSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := createMockServer(t)
	hostname := "hostname1"
	satlabID := "satlab-id"

	s.commandExecutor = &executor.FakeCommander{FakeFn: func(c *exec.Cmd) ([]byte, error) {
		if c.Args[0] == paths.ShivasCLI {
			if c.Args[1] == "update" {
				return []byte("success"), nil
			} else if c.Args[1] == "get" {
				return nil, errors.New("should not reach here")
			} else {
				return nil, errors.New(fmt.Sprintf("Un-support command: %v", c.Args))
			}
		} else if c.Args[0] == paths.GetHostIdentifierScript {
			return []byte(satlabID), nil
		}
		return nil, errors.New(fmt.Sprintf("Un-support command: %v", c.Args))
	}}

	err := removeAllPoolsFromDUT(ctx, s.commandExecutor, satlabID, hostname)

	if err != nil {
		t.Errorf("should sucess, but got an error: %v", err)
		return
	}
}
