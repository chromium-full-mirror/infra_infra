// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/uuid"
	"github.com/hashicorp/go-version"

	pb "go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/luci/common/logging"
	swarmingapi "go.chromium.org/luci/swarming/proto/api_v2"

	"infra/cmd/shivas/utils"
	"infra/cros/dutstate"
	"infra/cros/satlab/common/asset"
	"infra/cros/satlab/common/dns"
	"infra/cros/satlab/common/dut"
	"infra/cros/satlab/common/dut/shivas"
	"infra/cros/satlab/common/paths"
	"infra/cros/satlab/common/run"
	"infra/cros/satlab/common/satlabcommands"
	"infra/cros/satlab/common/services"
	"infra/cros/satlab/common/services/build_service"
	"infra/cros/satlab/common/services/ufs"
	"infra/cros/satlab/common/setup"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/collection"
	e "infra/cros/satlab/common/utils/errors"
	"infra/cros/satlab/common/utils/executor"
	"infra/cros/satlab/common/utils/parser"
	"infra/cros/satlab/satlabrpcserver/platform/cpu_temperature"
	"infra/cros/satlab/satlabrpcserver/services/bucket_services"
	"infra/cros/satlab/satlabrpcserver/services/dut_services"
	u "infra/cros/satlab/satlabrpcserver/utils"
	"infra/cros/satlab/satlabrpcserver/utils/constants"
)

// SatlabRpcServiceServer is the gRPC service that provides every function.
type SatlabRpcServiceServer struct {
	pb.UnimplementedSatlabRpcServiceServer
	// dev is a flag indicate which environment we want to run
	dev bool
	// buildService the connector to `BuildClient`
	buildService build_service.IBuildService
	// bucketService the connector to partner bucket
	bucketService bucket_services.IBucketServices
	// dutService the service to connect to DUTs
	dutService dut_services.IDUTServices
	// cpuTemperatureOrchestrator the CPU temperature orchestrator
	cpuTemperatureOrchestrator *cpu_temperature.CPUTemperatureOrchestrator
	// commandExecutor provides an interface to run a command. It is good for testing
	commandExecutor executor.IExecCommander
	// swarmingService provides the swarming API services
	swarmingService services.ISwarmingService
}

func New(
	dev bool,
	buildService build_service.IBuildService,
	bucketService bucket_services.IBucketServices,
	dutService dut_services.IDUTServices,
	cpuTemperatureOrchestrator *cpu_temperature.CPUTemperatureOrchestrator,
	swarmingService services.ISwarmingService,
) *SatlabRpcServiceServer {
	return &SatlabRpcServiceServer{
		dev:                        dev,
		bucketService:              bucketService,
		buildService:               buildService,
		dutService:                 dutService,
		cpuTemperatureOrchestrator: cpuTemperatureOrchestrator,
		commandExecutor:            &executor.ExecCommander{},
		swarmingService:            swarmingService,
	}
}

func (s *SatlabRpcServiceServer) validateServices() error {
	if s.buildService == nil || s.bucketService == nil || s.swarmingService == nil {
		return errors.New("authentication failed, please configure the credentials.")
	}

	return nil
}

// ListBuildTargets the gRPC server entry point to list all the build targets.
//
// ListBuildTargetsRequest _ we don't need use any parameter from the request, but we need to
// define it as a parameter to satisfy the compiler.
// To see more, we can look at the `src/satlab_rpcserver/satlabrpc.proto`
func (s *SatlabRpcServiceServer) ListBuildTargets(ctx context.Context, _ *pb.ListBuildTargetsRequest) (*pb.ListBuildTargetsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_build_targets")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	res, err := s.buildService.ListBuildTargets(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_build_targets: %w", err)
		return nil, err
	}

	return &pb.ListBuildTargetsResponse{
		BuildTargets: res,
	}, nil
}

// ListMilestones the gRPC server entry point to list all milestones from GCS bucket.
// TODO Add a cache for listing milestones
//
// pb.ListMilestonesRequest in the request from the client we use it as a filter to list the milestones.
func (s *SatlabRpcServiceServer) ListMilestones(ctx context.Context, in *pb.ListMilestonesRequest) (*pb.ListMilestonesResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_milestones")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	// Get the milestones from the partner bucket
	// If the milestones are in the partner bucket. they are staged.
	bucketMilestones, err := s.bucketService.GetMilestones(ctx, in.GetBoard())
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_milestones: %w", err)
		return nil, err
	}

	var remoteMilestones []string
	// Check the bucket is in asia, if it isn't in asia, we can fetch the milestones from `BuildClient`
	isBucketInAsia, err := s.bucketService.IsBucketInAsia(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_milestones: %w", err)
		return nil, err
	}

	if !isBucketInAsia {
		remoteMilestones, err = s.buildService.ListAvailableMilestones(ctx, in.GetBoard(), in.GetModel())
		if err != nil {
			logging.Errorf(ctx, "gRPC Service error: list_milestones: %w", err)
			return nil, err
		}
	}

	var res []*pb.BuildItem

	// Map bucketMilestones to response type `BuildItem`
	for _, item := range bucketMilestones {
		res = append(res, &pb.BuildItem{
			Value:    item,
			IsStaged: true,
			Status:   pb.BuildItem_BUILD_STATUS_PASS,
		})
	}

	// Filter the remoteMilestones not in the bucketMilestones,
	// and then mapping the milestones to response type `BuildItem`
	for _, item := range collection.Subtract(remoteMilestones, bucketMilestones, func(a string, b string) bool {
		return a == b
	}) {
		res = append(res, &pb.BuildItem{
			Value:    item,
			IsStaged: false,
			Status:   pb.BuildItem_BUILD_STATUS_PASS,
		})
	}

	// Sort the result
	sort.SliceStable(res, func(i, j int) bool {
		mA, errA := strconv.Atoi(res[i].Value)
		mB, errB := strconv.Atoi(res[j].Value)
		if errA != nil || errB != nil {
			return res[i].Value > res[j].Value
		}
		return mA > mB
	})

	return &pb.ListMilestonesResponse{
		Milestones: res,
	}, nil
}

// ListAccessibleModels the gRPC server entry point to list all models for a given board
//
// pb.ListAccessibleModelsRequest in the request from the client we use it as a filter to list the models.
func (s *SatlabRpcServiceServer) ListAccessibleModels(ctx context.Context, in *pb.ListAccessibleModelsRequest) (*pb.ListAccessibleModelsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_accessible_models")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	rawData, err := s.buildService.ListModels(ctx, in.GetBoard())
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_accessible_models: %w", err)
		return nil, err
	}

	data := make(map[string][]string)

	for _, item := range rawData {
		boardAndModelPair, err := parser.ExtractBoardAndModelFrom(item)
		if errors.Is(err, e.NotMatch) {
			logging.Warningf(ctx, "The model name (%s) doesn't match `buildTargets/{board}/models/{model}`", item)
		} else {
			data[boardAndModelPair.Model] = append(data[boardAndModelPair.Model], boardAndModelPair.Board)
		}
	}

	var res []*pb.Model

	for key, value := range data {
		res = append(res, &pb.Model{
			Name:   key,
			Boards: value,
		})
	}

	return &pb.ListAccessibleModelsResponse{
		Models: res,
	}, nil
}

// ListBuildVersions the gRPC server entry point to list all build versions for given board, model, and milestone.
// TODO Add a cache for listing build versions
//
// pb.ListBuildVersionsRequest in the request from the client we use to it as a filter to list the build versions.
func (s *SatlabRpcServiceServer) ListBuildVersions(ctx context.Context, in *pb.ListBuildVersionsRequest) (*pb.ListBuildVersionsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_build_versions")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	// Get the builds from the partner bucket
	// If the builds are in the partner bucket. they are staged.
	bucketBuilds, err := s.bucketService.GetBuilds(ctx, in.GetBoard(), in.GetMilestone())
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_build_versions: %w", err)
		return nil, err
	}

	var remoteBuilds []*build_service.BuildVersion
	// Check the bucket is in asia, if it isn't in asia, we can fetch the builds from `BuildClient`
	isBucketInAsia, err := s.bucketService.IsBucketInAsia(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_build_versions: %w", err)
		return nil, err
	}

	if !isBucketInAsia {
		remoteBuilds, err = s.buildService.ListBuildsForMilestone(ctx, in.GetBoard(), in.GetModel(), in.GetMilestone())
		if err != nil {
			logging.Errorf(ctx, "gRPC Service error: list_build_versions: %w", err)
			return nil, err
		}
	}

	var res []*pb.BuildItem
	buildDict := map[string]*pb.BuildItem{}

	// Map the bucketBuilds to response type `BuildItem`
	for _, item := range bucketBuilds {
		buildDict[item] = &pb.BuildItem{
			Status:   pb.BuildItem_BUILD_STATUS_PASS,
			IsStaged: true,
			Value:    item,
		}
	}

	// Filter the remoteBuilds not in the bucketBuilds,
	// and then mapping the remoteBuilds to response type `BuildItem`
	for _, build := range remoteBuilds {
		_, ok := buildDict[build.Version]
		item := &pb.BuildItem{
			Value:    build.Version,
			IsStaged: ok,
			Status:   constants.ToResponseBuildStatusMap[build.Status],
		}
		buildDict[build.Version] = item
	}

	for _, v := range buildDict {
		res = append(res, v)
	}

	// Sort the result
	sort.SliceStable(res, func(i, j int) bool {
		mA, errA := version.NewVersion(res[i].Value)
		mB, errB := version.NewVersion(res[j].Value)
		if errA != nil || errB != nil {
			return res[i].Value > res[j].Value
		}
		return mA.GreaterThanOrEqual(mB)
	})

	return &pb.ListBuildVersionsResponse{
		BuildVersions: res,
	}, nil
}

// StageBuild stage a build version in bucket.
//
// pb.StageBuildRequest in the request from client which we want to stage the artifact in the partner bucket.
func (s *SatlabRpcServiceServer) StageBuild(ctx context.Context, in *pb.StageBuildRequest) (*pb.StageBuildResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: stage_build")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	res, err := s.buildService.StageBuild(ctx, in.GetBoard(), in.GetModel(), in.GetBuildVersion(), site.GetGCSImageBucket())
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: stage_build: %w", err)
		return nil, err
	}

	return &pb.StageBuildResponse{
		BuildBucket: res.GetBucket(),
	}, nil

}

// ListConnectedDutsFirmware get current and firmware update on each DUT
func (s *SatlabRpcServiceServer) ListConnectedDutsFirmware(ctx context.Context, _ *pb.ListConnectedDutsFirmwareRequest) (*pb.ListConnectedDutsFirmwareResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_connected_duts_firmware")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	devices, err := s.dutService.GetConnectedIPs(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_connected_duts_firmware: %w", err)
		return nil, err
	}

	IPs := []string{}
	for _, d := range devices {
		if d.IsPingable && d.HasTestImage {
			IPs = append(IPs, d.IP)
		}
	}

	res := s.dutService.RunCommandOnIPs(ctx, IPs, constants.ListFirmwareCommand)

	var DUTsResponse []*pb.ConnectedDutFirmwareInfo

	for _, cmdRes := range res {
		if cmdRes.Error != nil {
			// If we execute the command failed, we can just continue others. Don't block.
			logging.Errorf(ctx, "Got an error when execute command: %v", cmdRes.Error)
			continue
		}
		var cmdResponse dut_services.ListFirmwareCommandResponse
		err = json.Unmarshal([]byte(cmdRes.Value), &cmdResponse)
		if err != nil {
			// If something wrong, we can continue to decode another ip result.
			logging.Errorf(ctx, "Json decode error: %v", err)
			continue
		}

		model := cmdResponse.Model
		currentFirmware := cmdResponse.FwId
		updateFirmware := "null"
		if _, ok := cmdResponse.FwUpdate[model]; ok {
			updateFirmware = cmdResponse.FwUpdate[model].Host.Versions.RW
		}
		DUTsResponse = append(DUTsResponse, &pb.ConnectedDutFirmwareInfo{
			Ip: cmdRes.IP, CurrentFirmware: currentFirmware, UpdateFirmware: updateFirmware,
		})
	}

	return &pb.ListConnectedDutsFirmwareResponse{Duts: DUTsResponse}, nil
}

// GetSystemInfo get the system information
func (s *SatlabRpcServiceServer) GetSystemInfo(ctx context.Context, _ *pb.GetSystemInfoRequest) (*pb.GetSystemInfoResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_system_info")

	var averageTemperature float32 = -1.0
	if s.cpuTemperatureOrchestrator == nil {
		logging.Errorf(ctx, "This platform doesn't support getting the temperature")
	} else {
		averageTemperature = s.cpuTemperatureOrchestrator.GetAverageCPUTemperature()
	}

	startTime, err := satlabcommands.GetSatlabStartTime(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_system_info: %w", err)
		return nil, err
	}

	return &pb.GetSystemInfoResponse{
		CpuTemperature: averageTemperature,
		StartTime:      startTime,
	}, nil
}

// GetPeripheralInformation get peripheral inforamtion by given DUT IP.
func (s *SatlabRpcServiceServer) GetPeripheralInformation(ctx context.Context, in *pb.GetPeripheralInformationRequest) (*pb.GetPeripheralInformationResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_peripheral_information")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	res, err := s.dutService.RunCommandOnIP(ctx, in.GetDutHostname(), constants.GetPeripheralInfoCommand)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_peripheral_information: %w", err)
		return nil, err
	}

	if res.Error != nil {
		logging.Errorf(ctx, "gRPC Service error: get_peripheral_information: %w", res.Error)
		return nil, res.Error
	}

	return &pb.GetPeripheralInformationResponse{
		JsonInfo: res.Value,
	}, nil
}

// UpdateDutsFirmware update Duts by given IPs
func (s *SatlabRpcServiceServer) UpdateDutsFirmware(ctx context.Context, in *pb.UpdateDutsFirmwareRequest) (*pb.UpdateDutsFirmwareResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: update_duts_firmware")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	// Run command on given IPs
	rawData := s.dutService.RunCommandOnIPs(ctx, in.GetIps(), constants.UpdateFirmwareCommand)

	// Create a response variable
	var resp = make([]*pb.FirmwareUpdateCommandOutput, len(rawData))

	// Loop over the raw data and then map to `FirmwareUpdateCommandOutput`
	for idx, cmdResp := range rawData {
		// Create a `FirmwareUpdateCommandOutput` object.
		out := &pb.FirmwareUpdateCommandOutput{
			Ip: cmdResp.IP,
		}
		// If the cmd response is an error,
		// we can show the error message to user.
		// Otherwise, we show the command output
		if cmdResp.Error != nil {
			out.CommandOutput = cmdResp.Error.Error()
		} else {
			out.CommandOutput = cmdResp.Value
		}

		resp[idx] = out
	}

	// Response the result to client
	return &pb.UpdateDutsFirmwareResponse{Outputs: resp}, nil
}

// Close clean up
func (s *SatlabRpcServiceServer) Close(ctx context.Context) {
	if err := s.buildService.Close(); err != nil {
		logging.Errorf(ctx, "Error while closing buildservice %v", err)
	}
}

// parseDims parse the dims input.
func parseDims(data []*pb.Dim) map[string]string {
	dims := map[string]string{}
	for _, d := range data {
		dims[d.GetKey()] = d.GetValue()
	}

	return dims
}

// Run suite triggers the test suite on the satlab. Right now, this is implemented using CTPBuildRequest
func (s *SatlabRpcServiceServer) RunSuite(ctx context.Context, in *pb.RunSuiteRequest) (*pb.RunSuiteResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: run_suite")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	r := &run.Run{
		Suite:         in.GetSuite(),
		Model:         in.GetModel(),
		Board:         in.GetBuildTarget(),
		Milestone:     in.GetMilestone(),
		Build:         in.GetBuildVersion(),
		Pool:          in.GetPool(),
		AddedDims:     parseDims(in.GetDims()),
		TimeoutMins:   site.MaxIshCTPTimeoutMins,
		Local:         true,
		CFT:           in.GetCft(),
		TRV2:          in.GetTrv2(),
		UploadToCpcon: in.GetUploadToCpcon(),
	}
	buildLink, err := r.TriggerRun(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: run_suite: %w", err)
		return nil, err
	}
	return &pb.RunSuiteResponse{BuildLink: buildLink}, nil
}

func (s *SatlabRpcServiceServer) RunTest(ctx context.Context, in *pb.RunTestRequest) (*pb.RunTestResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: run_test")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	r := &run.Run{
		Tests:         in.GetTests(),
		TestArgs:      in.GetTestArgs(),
		Board:         in.GetBoard(),
		Model:         in.GetModel(),
		Milestone:     in.GetMilestone(),
		Build:         in.GetBuild(),
		Pool:          in.GetPool(),
		AddedDims:     parseDims(in.GetDims()),
		TimeoutMins:   site.MaxIshCTPTimeoutMins,
		Local:         true,
		CFT:           in.GetCft(),
		TRV2:          in.GetTrv2(),
		UploadToCpcon: in.GetUploadToCpcon(),
	}
	buildLink, err := r.TriggerRun(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: run_test: %w", err)
		return nil, err
	}
	return &pb.RunTestResponse{BuildLink: buildLink}, nil
}

func (s *SatlabRpcServiceServer) GetVersionInfo(ctx context.Context, _ *pb.GetVersionInfoRequest) (*pb.GetVersionInfoResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_version_info")

	resp := pb.GetVersionInfoResponse{}
	hostId, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_version_info: %w", err)
		return nil, err
	}
	resp.HostId = hostId
	osVersion, err := satlabcommands.GetOsVersion(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_version_info: %w", err)
		return nil, err
	}
	resp.Description = osVersion.Description
	resp.ChromeosVersion = osVersion.Version
	resp.Track = osVersion.Track
	version, err := satlabcommands.GetSatlabVersion(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_version_info: %w", err)
		return nil, err
	}
	resp.Version = version
	return &resp, nil
}

func addPoolsToDUT(ctx context.Context, executor executor.IExecCommander, hostname string, pools []string) error {
	req := dut.UpdateDUT{
		Pools:    pools,
		Hostname: hostname,
	}
	return req.TriggerRun(ctx, executor)
}

func (s *SatlabRpcServiceServer) AddPool(ctx context.Context, in *pb.AddPoolRequest) (*pb.AddPoolResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: add_pool")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	for _, hostname := range in.GetHostnames() {
		if err := addPoolsToDUT(ctx, s.commandExecutor, hostname, []string{in.GetPool()}); err != nil {
			logging.Errorf(ctx, "gRPC Service error: add_pool: %w", err)
			return nil, err
		}
	}

	return &pb.AddPoolResponse{}, nil
}

func removeAllPoolsFromDUT(ctx context.Context, executor executor.IExecCommander, satlabID, hostname string) error {
	err := addPoolsToDUT(ctx, executor, hostname, []string{"-"})
	// As the CL changed, it returns an error even though the command removes all pools from the DUTs.
	// Ref: https://chromium-review.googlesource.com/c/infra/infra/+/5673141
	if err != nil {
		// We try to get the bot info by hostname
		satlabRackFilter := []string{site.MaybePrepend(site.Satlab, satlabID, "rack")}
		d := dut.GetDUT{
			SatlabID: satlabID,
			Racks:    satlabRackFilter,
		}
		dut, ufsErr := d.TriggerRun(ctx, executor, []string{site.MaybePrepend(site.Satlab, satlabID, hostname)})
		if ufsErr != nil {
			logging.Infof(ctx, "remove pool error: %v", err)
			logging.Infof(ctx, "can't get the bot info from hostname: %s, got an error: %v", hostname, ufsErr)
			return errors.New("Internal server error")
		}

		if len(dut) == 0 {
			logging.Infof(ctx, "remove pool error, can't get dut from ufs by hostname")
			return errors.New("Internal server error")
		}

		pools := dut[0].GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools()
		if len(pools) != 0 {
			return err
		}
	}

	return nil
}

// validateUpdatePools validate remove pools from UI
// that shouldn't be remove all pools from a DUT
func validateUpdatePools(pools []string) bool {
	return len(pools) >= 1
}

func (s *SatlabRpcServiceServer) UpdatePool(ctx context.Context, in *pb.UpdatePoolRequest) (*pb.UpdatePoolResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: update_pool")

	if err := s.validateServices(); err != nil {
		return nil, err
	}
	satlabID, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}

	for _, item := range in.GetItems() {
		if validateUpdatePools(item.GetPools()) {
			// According to `shivas` CLI. If we add a pool ("-"). It will remove all pools from the
			// host.
			if err := removeAllPoolsFromDUT(ctx, s.commandExecutor, satlabID, item.GetHostname()); err != nil {
				logging.Errorf(ctx, "gRPC Service error: update_pool: %w", err)
				return nil, err
			}

			// After removing the pools, we can add it the pools that we want to keep
			if err := addPoolsToDUT(ctx, s.commandExecutor, item.GetHostname(), item.GetPools()); err != nil {
				logging.Errorf(ctx, "gRPC Service error: update_pool: %w", err)
				return nil, err
			}
		}
	}

	return &pb.UpdatePoolResponse{}, nil
}

func (s *SatlabRpcServiceServer) GetDutDetail(ctx context.Context, in *pb.GetDutDetailRequest) (*pb.GetDutDetailResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_dut_detail")

	if err := s.validateServices(); err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_dut_detail: need to login before using this")
		return nil, err
	}

	IPToHostResult, err := dns.IPToHostname(ctx, s.commandExecutor, []string{in.GetAddress()})
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_dut_detail: %w", err)
		return nil, err
	}

	if len(IPToHostResult.InvalidAddresses) != 0 {
		logging.Errorf(ctx, "gRPC Service error: get_dut_detail: can't find the host by ip address {%v}", IPToHostResult.InvalidAddresses)
		return nil, fmt.Errorf("can't find the host by ip address {%v}", IPToHostResult.InvalidAddresses)
	}

	r, err := s.swarmingService.GetBot(ctx, IPToHostResult.Hostnames[0])
	if err != nil {
		return nil, err
	}

	dimensions := []*pb.StringListPair{}

	for _, d := range r.GetDimensions() {
		dimensions = append(dimensions, &pb.StringListPair{
			Key:    d.GetKey(),
			Values: d.GetValue(),
		})
	}

	resp := pb.GetDutDetailResponse{
		BotId:           r.GetBotId(),
		TaskId:          r.GetTaskId(),
		ExternalIp:      r.GetExternalIp(),
		AuthenticatedAs: r.GetAuthenticatedAs(),
		FirstSeenTs:     r.GetFirstSeenTs(),
		IsDead:          r.GetIsDead(),
		LastSeenTs:      r.GetLastSeenTs(),
		Quarantined:     r.GetQuarantined(),
		MaintenanceMsg:  r.GetMaintenanceMsg(),
		TaskName:        r.GetTaskName(),
		Version:         r.GetVersion(),
		Dimensions:      dimensions,
	}

	return &resp, nil
}

func (s *SatlabRpcServiceServer) ListDutTasks(ctx context.Context, in *pb.ListDutTasksRequest) (*pb.ListDutTasksResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_dut_tasks")

	if err := s.validateServices(); err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_dut_tasks: need to login before using this")
		return nil, err
	}

	IPToHostResult, err := dns.IPToHostname(ctx, s.commandExecutor, []string{in.GetAddress()})
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_dut_tasks: %w", err)
		return nil, err
	}

	if len(IPToHostResult.InvalidAddresses) != 0 {
		logging.Errorf(ctx, "gRPC Service error: list_dut_tasks: can't find the host by ip address {%v}", IPToHostResult.InvalidAddresses)
		return nil, fmt.Errorf("can't find the host by ip address {%v}", IPToHostResult.InvalidAddresses)
	}

	r, err := s.swarmingService.ListBotTasks(ctx, IPToHostResult.Hostnames[0], in.GetPageToken(), int(in.GetPageSize()))
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_dut_tasks: %w", err)
		return nil, err
	}

	tasks := []*pb.Task{}

	for _, t := range r.Tasks {
		tasks = append(tasks, &pb.Task{
			Id:        t.Id,
			Name:      t.Name,
			StartAt:   t.StartAt,
			Duration:  t.Duration,
			Url:       t.Url,
			IsSuccess: t.IsSuccess,
		})
	}

	return &pb.ListDutTasksResponse{
		NextPageToken: r.Cursor,
		Tasks:         tasks,
	}, nil
}

func (s *SatlabRpcServiceServer) ListDutEvents(ctx context.Context, in *pb.ListDutEventsRequest) (*pb.ListDutEventsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_dut_events")

	if err := s.validateServices(); err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_dut_events: need to login before using this")
		return nil, err
	}

	IPToHostResult, err := dns.IPToHostname(ctx, s.commandExecutor, []string{in.GetAddress()})
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_dut_events: %w", err)
		return nil, err
	}

	if len(IPToHostResult.InvalidAddresses) != 0 {
		logging.Errorf(ctx, "gRPC Service error: list_dut_events: can't find the host by ip address {%v}", IPToHostResult.InvalidAddresses)
		return nil, fmt.Errorf("can't find the host by ip address {%v}", IPToHostResult.InvalidAddresses)
	}

	r, err := s.swarmingService.ListBotEvents(ctx, IPToHostResult.Hostnames[0], in.GetPageToken(), int(in.GetPageSize()))
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_dut_events: %w", err)
		return nil, err
	}

	events := []*pb.BotEvent{}
	for _, e := range r.Events {
		events = append(events, &pb.BotEvent{
			Msg:       e.Message,
			EventType: e.Type,
			CreatedAt: e.Ts,
			TaskId:    e.TaskID,
			TaskLink:  e.TaskLink,
			Version:   e.Version,
		})
	}

	return &pb.ListDutEventsResponse{
		NextPageToken: r.Cursor,
		Events:        events,
	}, nil
}

func (s *SatlabRpcServiceServer) GetConnectedDuts(ctx context.Context, executor executor.IExecCommander) ([]*pb.Dut, error) {
	satlabID, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
	if err != nil {
		return nil, err
	}
	// Use rack and satlab id to filter
	satlabRackFilter := []string{site.MaybePrepend(site.Satlab, satlabID, "rack")}
	d := dut.GetDUT{
		Racks: satlabRackFilter,
	}
	a := asset.GetAsset{
		Racks: satlabRackFilter,
	}
	// All Satlab drone names are set as "satlab-<serial number of satlab box>"
	droneName := fmt.Sprintf("%s-%s", site.Satlab, satlabID)
	req := &swarmingapi.BotsRequest{
		Limit: int32(25),
		Dimensions: []*swarmingapi.StringPair{
			{
				Key:   "drone",
				Value: droneName,
			},
			{
				Key:   "pool",
				Value: site.GetTestRunnerSwarmingPool(),
			},
		},
	}
	botList, err := s.swarmingService.ListBots(ctx, req)
	if err != nil {
		logging.Errorf(ctx, "gRPC ListBots failed, couldn't determine swaming bot info", err)
	}

	HostMap, err := dns.ReadHostsToHostMap(ctx, executor)
	if err != nil {
		return nil, err
	}

	duts, err := d.TriggerRun(ctx, executor, []string{})
	if err != nil {
		return nil, err
	}

	assets, err := a.TriggerRun(ctx, executor)
	if err != nil {
		return nil, err
	}

	res := []*pb.Dut{}

	for _, dut := range duts {
		e := &pb.Dut{
			Name:        dut.Name,
			Hostname:    dut.Hostname,
			Pools:       dut.GetChromeosMachineLse().GetDeviceLse().GetDut().Pools,
			ServoSerial: dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoSerial(),
			ServoType:   dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoType(),
			ServoPort:   dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo().GetServoPort(),
			State:       dutstate.ConvertFromUFSState(dut.GetResourceState()).String(),
		}

		address := HostMap[dut.Hostname]
		e.Address = address
		for _, asset := range assets {
			if len(dut.Machines) > 0 {
				if asset.Name == dut.Machines[0] {
					e.Model = asset.Model
					e.Board = asset.Info.BuildTarget
				}
			}
		}
		// Set the swarming bot status information for the DUT
		if botList != nil && botList.Items != nil {
			botInfo, err := getBotStatusInfo(ctx, botList.Items, dut.Hostname)
			if err == nil && botInfo != nil {
				e.BotInfo = botInfo
			} else {
				logging.Errorf(ctx, "couldn't determine bot status for %s, ignores BotInfo setting", dut.Hostname, err)
			}
		}

		res = append(res, e)
	}

	return res, nil
}

func (s *SatlabRpcServiceServer) ListEnrolledDuts(ctx context.Context, in *pb.ListEnrolledDutsRequest) (*pb.ListEnrolledDutsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_enrolled_duts")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	duts, err := s.GetConnectedDuts(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_enrolled_duts: %w", err)
		return nil, err
	}

	return &pb.ListEnrolledDutsResponse{Duts: duts}, nil
}

func (s *SatlabRpcServiceServer) ListDuts(ctx context.Context, in *pb.ListDutsRequest) (*pb.ListDutsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_duts")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	connectedDevices, err := s.dutService.GetConnectedIPs(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_duts: %w", err)
		return nil, err
	}

	duts, err := s.GetConnectedDuts(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_duts: %w", err)
		return nil, err
	}

	// Get the USB device connected to extract Cr50/Ti50 and Servo serials serial numbers
	usbDevices, err := s.dutService.GetUSBDevicePaths(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_duts: %w", err)
		return nil, err
	}

	enrolledIPs := []string{}

	for _, dut := range duts {
		for _, device := range connectedDevices {
			if dut.Address == device.IP {
				dut.IsPingable = device.IsPingable
				dut.HasTestImage = device.HasTestImage
				dut.MacAddress = device.MACAddress
				enrolledIPs = append(enrolledIPs, dut.Address)
				ccdStatus, err := s.dutService.GetCCDStatus(ctx, dut.Address)
				if err != nil {
					ccdStatus = "Unknown"
				}
				dut.CcdStatus = ccdStatus
			}
		}
	}

	unenrolledDevices := collection.Subtract(connectedDevices, enrolledIPs, func(a dut_services.Device, b string) bool {
		return a.IP == b
	})

	for _, device := range unenrolledDevices {

		// TODO optimize we don't need to wait for
		// out dut executing command complete to fetch
		// the next dut board and model.
		var servoSerial = ""
		var board = ""
		var model = ""
		var ccdStatus = ""
		if device.IsPingable && device.HasTestImage {
			board, err = s.dutService.GetBoard(ctx, device.IP)
			if err != nil {
				// Skip when we can't get the board from the CLI.
				board = ""
			}
			model, err = s.dutService.GetModel(ctx, device.IP)
			if err != nil {
				// Skip when we can't get the model from the CLI.
				model = ""
			}
			var isServoConnected = false
			isServoConnected, servoSerial, err = s.dutService.GetServoSerial(ctx, device.IP, usbDevices)
			if err != nil {
				logging.Errorf(ctx, "gRPC Service error: list_duts: failed to find servo serial for %s: %v", device.IP, err)
			}
			// TODO Make UI handle this to display appropriate thing instead of setting it here.
			if isServoConnected && servoSerial == "" {
				servoSerial = "NOT DETECTED"
			}
			ccdStatus, err = s.dutService.GetCCDStatus(ctx, device.IP)
			if err != nil {
				ccdStatus = "Unknown"
			}
		}
		duts = append(duts, &pb.Dut{
			Board:        board,
			Model:        model,
			Address:      device.IP,
			MacAddress:   device.MACAddress,
			IsPingable:   device.IsPingable,
			HasTestImage: device.HasTestImage,
			ServoSerial:  servoSerial,
			CcdStatus:    ccdStatus,
		})
	}

	return &pb.ListDutsResponse{Duts: duts}, nil
}

func createTaskLink(taskID string) string {
	// If task ID is empty, we can return an empty string
	if taskID == "" {
		return ""
	}
	return fmt.Sprintf("%s%s", site.TaskLinkTemplate, taskID)
}

// DeleteDuts the RPC service for deleting DUTs
func (s *SatlabRpcServiceServer) DeleteDuts(ctx context.Context, in *pb.DeleteDutsRequest) (*pb.DeleteDutsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: delete_duts")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	ctx = utils.SetupContext(ctx, site.GetNamespace(""))
	ufs, err := ufs.NewUFSClientWithDefaultOptions(ctx, site.GetUFSService(s.dev))
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: delete_duts: %w", err)
		return nil, err
	}

	res, err := innerDeleteDuts(ctx, s.commandExecutor, ufs, in.GetHostnames(), false)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: delete_duts: %w", err)
		return nil, err
	}

	return &pb.DeleteDutsResponse{
		Pass: res.DutResults.Pass,
		Fail: res.DutResults.Fail,
	}, nil
}

// innerDeleteDuts the main logic of deleting the DUTs by given IP addresses.
// Create this function for testing easily
// This function returns a result of deleting DUTs result that contains pass and fail.
func innerDeleteDuts(ctx context.Context, executor executor.IExecCommander, ufs dut.DeleteClient, hostnames []string, full bool) (*dut.DeleteDUTResult, error) {
	d := dut.DeleteDUT{
		Names: hostnames,
		Full:  full,
	}

	if err := d.Validate(); err != nil {
		return nil, err
	}

	return d.TriggerRun(ctx, executor, ufs)
}

// GetNetworkInfo gets newwork information of satlab.
func (s *SatlabRpcServiceServer) GetNetworkInfo(ctx context.Context, _ *pb.GetNetworkInfoRequest) (*pb.GetNetworkInfoResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_network_info")

	hostname, err := satlabcommands.GetHostIP(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_network_info: %w", err)
		return nil, err
	}
	macAddress, err := satlabcommands.GetMacAddress(ctx, s.commandExecutor)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_network_info: %w", err)
		return nil, err
	}

	return &pb.GetNetworkInfoResponse{
		Hostname:    hostname,
		MacAddress:  macAddress,
		IsConnected: hostname != "" && hostname != "localhost",
	}, nil

}

func (s *SatlabRpcServiceServer) ListTestPlans(ctx context.Context, _ *pb.ListTestPlansRequest) (*pb.ListTestPlansResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: list_test_plans")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	res, err := s.bucketService.ListTestplans(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: list_test_plans: %w", err)
		return nil, err
	}

	return &pb.ListTestPlansResponse{
		Names: res,
	}, nil
}

func (s *SatlabRpcServiceServer) AddDuts(ctx context.Context, in *pb.AddDutsRequest) (*pb.AddDutsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: add_duts")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	var fail = make([]*pb.AddDutsResponse_FailedData, 0, len(in.GetDuts()))
	var pass = make([]*pb.AddDutsResponse_PassedData, 0, len(in.GetDuts()))

	for _, d := range in.GetDuts() {
		// The buffer we want to get the command output
		// we use this buffer to parse the deploy URL.
		var buf bytes.Buffer
		err := (&dut.AddDUT{
			Hostname:    d.GetHostname(),
			Address:     d.GetAddress(),
			Board:       d.GetBoard(),
			Model:       d.GetModel(),
			AssetType:   "dut",
			Asset:       uuid.NewString(),
			DeployTags:  []string{"satlab:true"},
			ServoSerial: d.GetServoSerial(),
		}).TriggerRun(ctx, s.commandExecutor, &buf)
		if err != nil {
			fail = append(fail, &pb.AddDutsResponse_FailedData{
				Hostname: d.GetHostname(),
				Reason:   err.Error(),
			})
		} else {
			// There are two results in the command output.
			// The first one is a success. Another one has failed.
			out := buf.String()
			logging.Infof(ctx, "add DUT result\n========\n%s\n========\n", out)
			rows := strings.Split(out, "\n")
			// If it deployed failed, The output
			// will contain the message `Failed to add DUT...`
			failed := ""
			for i := len(rows) - 1; i >= 0; i-- {
				if strings.HasPrefix(rows[i], "Failed to add DUT") {
					failed = rows[i]
					break
				}
			}

			if failed != "" {
				fail = append(fail, &pb.AddDutsResponse_FailedData{
					Hostname: d.GetHostname(),
					Reason:   failed,
				})
			} else {
				url, err := parser.ParseDeployURL(out)
				if err != nil {
					// Skip parsing error here, we don't want to
					// block user if any dut has been deployed successfully,
					// but we can't parse the url from the command output.
					url = ""
				}
				pass = append(pass, &pb.AddDutsResponse_PassedData{
					Hostname: d.GetHostname(),
					Url:      url,
				})
			}
		}
	}

	return &pb.AddDutsResponse{Pass: pass, Fail: fail}, nil
}

func (s *SatlabRpcServiceServer) RunTestPlan(ctx context.Context, in *pb.RunTestPlanRequest) (*pb.RunTestPlanResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: run_test_plan")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	r := &run.Run{
		Board:         in.GetBoard(),
		Model:         in.GetModel(),
		Milestone:     in.GetMilestone(),
		Build:         in.GetBuild(),
		Pool:          in.GetPool(),
		Testplan:      in.GetTestPlanName(),
		AddedDims:     parseDims(in.GetDims()),
		TimeoutMins:   site.MaxIshCTPTimeoutMins,
		Local:         true,
		CFT:           in.GetCft(),
		TRV2:          in.GetTrv2(),
		UploadToCpcon: in.GetUploadToCpcon(),
	}

	buildLink, err := r.TriggerRun(ctx)
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: run_test_plan: %w", err)
		return nil, err
	}
	return &pb.RunTestPlanResponse{BuildLink: buildLink}, nil
}

func (s *SatlabRpcServiceServer) GetTestPlan(ctx context.Context, in *pb.GetTestPlanRequest) (*pb.GetTestPlanResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_test_plan")

	if err := s.validateServices(); err != nil {
		return nil, err
	}

	tp, err := s.bucketService.GetTestPlan(ctx, in.GetName())
	if err != nil {
		logging.Errorf(ctx, "gRPC Service error: get_test_plan: %w", err)
		return nil, err
	}

	return &pb.GetTestPlanResponse{Plan: tp}, nil
}

func (s *SatlabRpcServiceServer) SetCloudConfiguration(ctx context.Context, in *pb.SetCloudConfigurationRequest) (*pb.SetCloudConfigurationResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: set_cloud_configuration")

	if err := validateCloudConfiguration(in); err != nil {
		logging.Errorf(ctx, "gRPC Service error: set_cloud_configuration: %w", err)
		return nil, err
	}

	bucket := removeGCSBucketPrefixAndSuffix(in.GetGcsBucketUrl())

	r := setup.Setup{
		Bucket:            bucket,
		GSAccessKeyId:     in.GetBotoKeyId(),
		GSSecretAccessKey: in.GetBotoKeySecret(),
	}

	err := r.StartSetup(ctx)
	if err != nil {
		logging.Errorf(ctx, "StartSetup failed. got an error: %v\n", err)
		return nil, err
	}

	return &pb.SetCloudConfigurationResponse{}, nil
}

// removeGCSBucketPrefixAndSuffix remove the gcs bucket url
// e.g.
// gs://bucket/ -> bucket
// gs://bucket  -> bucket
// bucket/      -> bucket
func removeGCSBucketPrefixAndSuffix(bucket string) string {
	s := strings.TrimPrefix(bucket, "gs://")
	s = strings.TrimRight(s, "/")
	return s
}

// validateCloudConfiguration validate the config form the GRPC call.
func validateCloudConfiguration(in *pb.SetCloudConfigurationRequest) error {
	if strings.TrimSpace(in.GetGcsBucketUrl()) == "" {
		return errors.New("bucket is empty")
	}

	if strings.TrimSpace(in.GetBotoKeyId()) == "" {
		return errors.New("boto key is empty")
	}

	if strings.TrimSpace(in.GetBotoKeySecret()) == "" {
		return errors.New("secret key is empty")
	}

	return nil
}

// GetCloudConfiguration get the cloud configuration from env and boto file.
func (s *SatlabRpcServiceServer) GetCloudConfiguration(ctx context.Context, in *pb.GetCloudConfigurationRequest) (*pb.GetCloudConfigurationResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: get_cloud_configuration")

	bucket := site.GetGCSPartnerBucket()
	p := site.GetBotoPath()

	f, err := os.Open(p)
	if err != nil {
		// If `boto` file doesn't exist, it means the user
		// doesn't login. we return empty information
		return &pb.GetCloudConfigurationResponse{}, nil
	}
	key := setup.ReadBotoKey(f)

	return &pb.GetCloudConfigurationResponse{
		GcsBucketUrl: bucket,
		BotoKeyId:    key,
	}, nil
}

func innerReboot(executor executor.IExecCommander) {
	// Start the proccess immediately
	// because `reboot` won't return anything.
	// we don't want to wait for the stdout
	cmd := exec.Command(paths.Reboot, "-h", "now")
	executor.CombinedOutput(cmd)
}

// Reboot call a reboot command on RPC container
func (s *SatlabRpcServiceServer) Reboot(context.Context, *pb.RebootRequest) (*pb.RebootResponse, error) {
	// use defer and go rountine to make sure
	// UI receive the response first, and reboot after.
	defer func() {
		go innerReboot(s.commandExecutor)
	}()

	return &pb.RebootResponse{}, nil
}

func (s *SatlabRpcServiceServer) UploadLog(ctx context.Context, _ *pb.UploadLogRequest) (*pb.UploadLogResponse, error) {
	if err := s.validateServices(); err != nil {
		return nil, err
	}

	hostId, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}

	// prepare the tar filename and bucket path
	filename := fmt.Sprintf("%d.tar.gz", time.Now().Unix())
	now := time.Now().Format("2006-01-02_15:04:05")
	out := fmt.Sprintf("/tmp/%s", filename)
	if err := u.TarGz(constants.LogDirectory, out); err != nil {
		return nil, err
	}
	// clean up the tar file
	defer os.Remove(out)

	bPath := fmt.Sprintf("%s/%s/%s", hostId, now, filename)
	// upload the tar file to the bucket
	gsPath, err := s.bucketService.UploadLog(ctx, bPath, out)
	if err != nil {
		return nil, err
	}

	return &pb.UploadLogResponse{
		BucketLink: fmt.Sprintf(constants.GCSObjectURLTemplate, gsPath),
	}, nil
}

func (s *SatlabRpcServiceServer) DownloadLog(_ *pb.DownloadLogRequest, server pb.SatlabRpcService_DownloadLogServer) error {
	filename := fmt.Sprintf("%d.tar.gz", time.Now().Unix())
	out := fmt.Sprintf("/tmp/%s", filename)
	if err := u.TarGz(constants.LogDirectory, out); err != nil {
		return err
	}

	bufSize := 64 * 1024
	f, err := os.Open(out)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, bufSize)
	for {
		c, err := f.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		resp := &pb.DownloadLogResponse{
			FileChunk: buf[:c],
		}
		err = server.Send(resp)
		if err != nil {
			return err
		}
	}

	return nil
}

// RepairDuts a GRPC call for repairing the DUTs.
func (s *SatlabRpcServiceServer) RepairDuts(ctx context.Context, in *pb.RepairDutsRequest) (*pb.RepairDutsResponse, error) {
	if err := s.validateServices(); err != nil {
		return nil, err
	}
	satlabID, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, s.commandExecutor)
	if err != nil {
		return nil, err
	}
	action := shivas.Normal
	if in.GetDeep() {
		action = shivas.DeepRepair
	}

	res := []*pb.RepairDutsResponse_RepairResult{}
	for _, h := range in.GetHostnames() {
		r := shivas.DUTRepairer{
			Name:      site.MaybePrepend(site.Satlab, satlabID, h),
			Namespace: site.GetNamespace(""),
			Executor:  s.commandExecutor,
		}
		resp, err := r.Repair(ctx, action)
		buildLink := ""
		taskLink := ""
		if err == nil {
			buildLink = resp.BuildLink
			taskLink = resp.TaskLink
		}
		res = append(res, &pb.RepairDutsResponse_RepairResult{
			Hostname:  h,
			BuildLink: buildLink,
			TaskLink:  taskLink,
			IsSuccess: err == nil,
		})

	}

	return &pb.RepairDutsResponse{Result: res}, nil
}

// ListJobs a GRPC call to list all jobs(swarming tasks) based on filters.
func (s *SatlabRpcServiceServer) ListJobs(ctx context.Context, in *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	logging.Infof(ctx, "gRPC Service triggered: ListJobs")

	if err := s.validateServices(); err != nil {
		logging.Errorf(ctx, "gRPC Service error: ListJobs: need to login before using this")
		return nil, err
	}

	limit := in.GetLimit()
	if limit == 0 {
		limit = 30
	}
	// Swarming API to list all tasks for given filters.
	resp, err := s.swarmingService.ListTasks(ctx, &swarmingapi.TasksWithPerfRequest{
		Start:                   in.GetCreatedTimeGt(),
		End:                     in.GetCreatedTimeLt(),
		Tags:                    getListTasksRequestTags(in),
		Limit:                   int32(limit),
		State:                   swarmingapi.StateQuery(in.GetQueryStatus()), // default is PENDING
		Sort:                    swarmingapi.SortQuery(in.GetSortBy()),       // default is CREATED_TS
		Cursor:                  in.GetPageToken(),
		IncludePerformanceStats: false,
	})

	if err != nil {
		logging.Errorf(ctx, "failed to list tasks for the request %v, error: %v", in, err)
		return nil, err
	}

	jobs := []*pb.Job{}
	// Iterate through the list of tasks fetched from Swarming
	for _, row := range resp.GetItems() {
		// Extract the required information for UI from the Swarming task
		job, err := s.GetJobDetails(ctx, row)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return &pb.ListJobsResponse{
		NextPageToken: resp.Cursor,
		Jobs:          jobs,
	}, nil

}

// getListTasksRequestTags return filtering tags for ListTasks.
func getListTasksRequestTags(in *pb.ListJobsRequest) []string {
	// Define a tags list by setting a default tag of LUCI buildbucket for a given account.
	tags := []string{fmt.Sprintf("buildbucket_bucket:%s/%s", site.GetLUCIProject(), site.GetCTPBucket())}
	//  Swarming ListTasks RPC uses concatenated key:value pair as list of string for tags. eg: ["drone:xyz", "pool:abc"]
	if in.GetTags() != nil {
		for _, v := range in.GetTags() {
			tagPair := fmt.Sprintf("%s:%v", v.GetKey(), v.GetValue())
			tags = append(tags, tagPair)
		}
	}

	// Fine tune filters to get only CTP builds when search is for suite or testplan
	if in.JobType == pb.Job_SUITE || in.JobType == pb.Job_TESTPLAN {
		tags = append(tags, fmt.Sprintf("%s:%s", site.BuilderTag, site.GetCTPBuilder()))
		tags = append(tags, fmt.Sprintf("pool:%s", site.GetCTPSwarmingPool()))
	} else if in.JobType == pb.Job_TEST {
		tags = append(tags, fmt.Sprintf("%s:%s", site.BuilderTag, site.DefaultTestRunnerBuilderName))
		tags = append(tags, fmt.Sprintf("pool:%s", site.GetTestRunnerSwarmingPool()))
	} else { // Filter for both CTP and Testrunner builds
		tags = append(tags, fmt.Sprintf("%s:%s|%s", site.BuilderTag, site.DefaultTestRunnerBuilderName, site.GetCTPBuilder()))
		tags = append(tags, fmt.Sprintf("pool:%s|%s", site.GetTestRunnerSwarmingPool(), site.GetCTPSwarmingPool()))
	}

	return tags
}

// GetJobDetails extracts tasks details used by Satlab UI.
func (s *SatlabRpcServiceServer) GetJobDetails(ctx context.Context, taskInfo *swarmingapi.TaskResultResponse) (*pb.Job, error) {
	if taskInfo == nil {
		return nil, errors.New("task is empty, cannot process task details")
	}

	tagsInfo := convertTagsSliceToMap(taskInfo.GetTags())

	if tagsInfo == nil {
		return nil, errors.New("could not determine task tags")
	}

	job := pb.Job{}
	job.CreatedTime = taskInfo.GetCreatedTs()
	job.StartTime = taskInfo.GetStartedTs()
	job.FinishedTime = taskInfo.GetCompletedTs()
	job.Status = getJobStatus(taskInfo)

	job.JobId = tagsInfo[site.BuildBucketIDTag]
	job.SatlabId = tagsInfo[site.SatlabIDTag]
	job.LabelPool = tagsInfo[site.LabelPoolTag]
	job.ParentJobId = tagsInfo[site.ParentBuildBucketIDTag]
	job.Board = tagsInfo[site.LabelBoardTag]
	job.Model = tagsInfo[site.LabelModelTag]
	job.Build = tagsInfo[site.LabelImageTag]

	if tagsInfo[site.BuilderTag] == site.GetCTPBuilder() {
		if tagsInfo[site.TestTypeTag] == site.TestPlan {
			job.Name = fmt.Sprintf("%s:%s", site.TestPlan, tagsInfo[site.TestPlanIDTag])
		} else if tagsInfo[site.TestTypeTag] == site.Suite {
			job.Name = fmt.Sprintf("%s:%s", site.Suite, tagsInfo[site.LabelSuiteTag])

		}
	} else {
		job.Name = tagsInfo[site.DisplayNameTag]
		// CTP task we don't need hostname as it is ran on GCE VM bots
		job.Hostname = getDutHostName(taskInfo.GetBotDimensions())
	}
	job.TaskUrl = getMiloBuildLink(tagsInfo)

	// Set results URL only if the task is completed.
	if taskInfo.GetState() == swarmingapi.TaskState_COMPLETED {
		job.ResultsUrl = getTestResultsLink(tagsInfo)
	}

	job.CpconUrl = getStorageQualCPCONLink(tagsInfo)

	// Only a task in RUNNINING state can have child tasks.
	if tagsInfo[site.BuilderTag] == site.GetCTPBuilder() && (job.Status == pb.Job_RUNNING) {

		// Define tags to fetch Child tasks count CTP builds
		tcTags := []string{
			fmt.Sprintf("buildbucket_bucket:%s/%s", site.GetLUCIProject(), site.GetCTPBucket()),
			fmt.Sprintf("%s:%s", site.ParentBuildBucketIDTag, tagsInfo[site.BuildBucketIDTag]),
			fmt.Sprintf("%s:%s", site.BuilderTag, site.DefaultTestRunnerBuilderName),
			fmt.Sprintf("pool:%s", site.GetTestRunnerSwarmingPool()),
		}

		tc, err := s.GetChildTasksCountByStatus(
			ctx,
			taskInfo.GetStartedTs(),
			tagsInfo[site.BuildBucketIDTag],
			tcTags,
			[]swarmingapi.StateQuery{
				swarmingapi.StateQuery_QUERY_PENDING_RUNNING,
				swarmingapi.StateQuery_QUERY_ALL,
			},
		)
		if err != nil {
			logging.Infof(ctx, "job.StatusCount Error %v", err)
		} else {
			job.ChildStatusCount = tc
		}
	}

	return &job, nil
}

func getStorageQualCPCONLink(tagsInfo map[string]string) string {
	accountID := site.GetCPConAccountID()
	bugID := tagsInfo[site.BugIDTag]
	runID := tagsInfo[site.QualRunIDTag]
	if accountID == "" || bugID == "" || runID == "" {
		return ""
	}
	return fmt.Sprintf(site.CPConLinkTemplate, accountID, runID, bugID)
}

// getJobStatus converts Swarming Task state to Satlab Job status.
func getJobStatus(taskInfo *swarmingapi.TaskResultResponse) pb.Job_JobStatus {
	// Map the swarming task State to Job status
	switch taskInfo.GetState() {
	case swarmingapi.TaskState_COMPLETED:
		if !taskInfo.GetFailure() {
			return pb.Job_COMPLETE_SUCCESS
		} else {
			return pb.Job_COMPLETE_FAILURE
		}
	case swarmingapi.TaskState_RUNNING:
		return pb.Job_RUNNING
	case swarmingapi.TaskState_PENDING:
		return pb.Job_PENDING
	case swarmingapi.TaskState_EXPIRED:
		return pb.Job_EXPIRED
	case swarmingapi.TaskState_TIMED_OUT:
		return pb.Job_TIMED_OUT
	default:
		return pb.Job_ABORTED
	}
}

// getDutHostName gets the DUT hostname from swarming task's bot dimensions.
func getDutHostName(botDims []*swarmingapi.StringListPair) string {
	for _, dim := range botDims {
		if dim.Key != site.DutNameTag {
			continue
		}
		if len(dim.Value) != 1 {
			return ""
		}
		return dim.Value[0]
	}
	return ""
}

// convertTagsSliceToMap converts the list of key:value strings to map.
func convertTagsSliceToMap(tags []string) map[string]string {
	// Create an empty map
	tagsMap := map[string]string{}
	// Iterate over the array and split each key-value pair
	for _, kv := range tags {
		parts := strings.SplitN(kv, ":", 2) // Split using a colon as the separator
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		value := parts[1]
		// Add the key-value pair to the map
		tagsMap[key] = value
	}

	return tagsMap
}

// getMiloBuildLink returns the LUCI Milo build link.
func getMiloBuildLink(info map[string]string) string {
	return fmt.Sprintf("%s%s", site.MiloSite, info[site.BuildBucketIDTag])
}

// getTestResultsLink returns the testhaus result link.
func getTestResultsLink(info map[string]string) string {
	filters := fmt.Sprintf("%s~%s", site.BuildBucketID, info[site.BuildBucketIDTag])
	if info[site.BuilderTag] == site.GetCTPBuilder() {
		filters = fmt.Sprintf("%s~%s", site.ParentBuildBucketIDTag, info[site.BuildBucketIDTag])
	}
	daysToFilter := 30
	// TODO (prasadv): Add account to the satlab-config.json to avoid parsing.
	accountID := site.GetUFSZone()
	re := regexp.MustCompile("[0-9]+")
	n := re.FindAllString(accountID, -1)
	if len(n) > 0 {
		accountID = n[0]
	}

	return fmt.Sprintf("%sfilters=%s&accountId=%s&days=%d", site.TesthausURLTemplate, filters, accountID, daysToFilter)
}

// getBotStatusInfo returns the bot info for a given DUT hostname.
func getBotStatusInfo(ctx context.Context, botList []*swarmingapi.BotInfo, hostname string) (*pb.BotInfo, error) {
	if botList == nil {
		return nil, errors.New("bot list is empty, cannot process bot info")
	}

	for _, bot := range botList {
		// site.GetBotPrefix() is always suffixed with '-'
		dutBotName := fmt.Sprintf("%s%s", site.GetBotPrefix(), hostname)
		if dutBotName == bot.GetBotId() {
			return generateBotInfo(bot), nil
		}
	}

	return nil, errors.New("could not find bot for the given hostname")
}

// generateBotInfo checks possible different statuses for a bot, and returns the status and task (if applicable).
func generateBotInfo(bot *swarmingapi.BotInfo) *pb.BotInfo {
	botInfo := &pb.BotInfo{}
	if bot.GetIsDead() {
		botInfo.BotState = pb.BotInfo_DEAD
	} else if bot.GetQuarantined() {
		botInfo.BotState = pb.BotInfo_QUARANTINED
	} else if bot.GetTaskId() != "" {
		botInfo.BotState = pb.BotInfo_BUSY
		botInfo.TaskName = getTaskName(bot.GetTaskName())
		botInfo.CurrentTask = createTaskLink(bot.TaskId)
	} else if bot.GetTaskId() == "" {
		botInfo.BotState = pb.BotInfo_IDLE
	} else {
		botInfo.BotState = pb.BotInfo_NO_STATE_SET
	}

	for _, dim := range bot.GetDimensions() {
		stringListPair := &pb.StringListPair{
			Key:    dim.GetKey(),
			Values: dim.GetValue(),
		}
		botInfo.Dimensions = append(botInfo.Dimensions, stringListPair)
	}

	return botInfo
}

// getTaskName returns the swarming task name for Satlab UI.
func getTaskName(taskName string) string {
	if strings.Contains(taskName, site.DeployBuilderName) {
		return site.TaskRunningDeploy
	} else if strings.Contains(taskName, site.RepairBuilderName) {
		return site.TaskRunningRepair
	} else if strings.Contains(taskName, site.DefaultTestRunnerBuilderName) {
		return site.TaskRunningTest
	}

	return ""
}

// GetChildTasksCountByStatus returns child tasks count for given states.
func (s *SatlabRpcServiceServer) GetChildTasksCountByStatus(
	ctx context.Context,
	startTime *timestamp.Timestamp,
	jobID string,
	tags []string,
	jobStates []swarmingapi.StateQuery,
) (*pb.TasksStatusCount, error) {
	if startTime == nil || jobID == "" {
		return nil, errors.New("start time and id cannot be empty, failed to process child task count")
	}
	taskCounts := []*pb.TasksStatusCount_TaskCount{}
	for _, state := range jobStates {
		resp, err := s.swarmingService.CountTasks(ctx, &swarmingapi.TasksCountRequest{
			Start: startTime,
			Tags:  tags,
			State: state,
		})
		if err != nil {
			logging.Errorf(ctx, "RPC Service error: CountsTasks: %w", err)
			continue
		}
		taskCounts = append(taskCounts, &pb.TasksStatusCount_TaskCount{
			State: pb.StateQuery(state),
			Count: resp.GetCount()})
	}
	return &pb.TasksStatusCount{
		TaskCount: taskCounts,
	}, nil
}

// CancelJobs handles the request that users want to cancel jobs.
func (s *SatlabRpcServiceServer) AbortJobs(ctx context.Context, in *pb.AbortJobsRequest) (*pb.AbortJobsResponse, error) {
	tags := []string{}

	if len(in.GetIds()) == 0 {
		return nil, errors.New("id can not be empty")
	}

	ids := strings.Join(in.GetIds(), "|")
	tags = append(tags, fmt.Sprintf("buildbucket_build_id:%s", ids))

	pool := ""
	// Fine tune filters to get only CTP builds when search is for suite or testplan
	if in.JobType == pb.Job_SUITE || in.JobType == pb.Job_TESTPLAN {
		pool = site.GetCTPSwarmingPool()
	} else if in.JobType == pb.Job_TEST {
		pool = site.GetTestRunnerSwarmingPool()
	}

	if pool == "" {
		return nil, errors.New("Pool can not be empty")
	}

	tags = append(tags, fmt.Sprintf("pool:%s", pool))

	req := services.CancelTasksRequest{
		Tags:  tags,
		Start: in.CreatedTimeGt,
		End:   in.CreatedTimeLt,
	}

	err := s.swarmingService.CancelTasks(ctx, req)
	if err != nil {
		return nil, err
	}

	return &pb.AbortJobsResponse{}, nil
}
