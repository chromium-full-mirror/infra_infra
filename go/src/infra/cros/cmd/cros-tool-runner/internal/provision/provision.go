// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package provision run provisioning for DUT.
package provision

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/golang/protobuf/jsonpb"

	build_api "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/common"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/services"
)

const (
	// Parent temp dir
	tempDirPath = "/var/tmp"

	// Cros-dut result temp dir.
	crosDutResultsTempDir = "cros-dut-results"

	// Cros-provision result temp dir.
	crosProvisionResultsTempDir = "cros-provision-results"
)

// Result holds result data.
type Result struct {
	Out *api.CrosProvisionResponse
	Err error
}

// Run runs provisioning software dependencies per DUT.
func Run(ctx context.Context, device *api.CrosToolRunnerProvisionRequest_Device, crosDutContainer, crosProvisionContainer *build_api.ContainerImageInfo, tokenFile string) *Result {
	res := &Result{
		Out: &api.CrosProvisionResponse{
			Id: device.GetDut().GetId(),
			Outcome: &api.CrosProvisionResponse_Failure{
				Failure: &api.InstallFailure{
					Reason: api.InstallFailure_REASON_PROVISIONING_FAILED,
				},
			},
		},
	}
	if device == nil || device.GetProvisionState() == nil {
		res.Err = errors.Reason("run provision: DUT input is empty").Err()
		res.Out.Outcome = &api.CrosProvisionResponse_Failure{
			Failure: &api.InstallFailure{
				Reason: api.InstallFailure_REASON_INVALID_REQUEST,
			},
		}
		return res
	}
	dutName := res.Out.Id.GetValue()
	cacheServerInfo := device.GetDut().GetCacheServer()
	dutSshInfo := device.GetDut().GetChromeos().GetSsh()
	log.Printf("New Auth Steps.")
	log.Printf("Preparing for provisioning of %q, with: %s", dutName, device.GetProvisionState())

	// Use the host network.
	var networkName = "host"

	parentTempDir := ""
	if _, err := os.Stat(tempDirPath); err == nil {
		parentTempDir = tempDirPath
	}

	// Create temp results dir for cros-dut
	crosDutResultsDir, err := ioutil.TempDir(parentTempDir, crosDutResultsTempDir)
	if err != nil {
		log.Printf("cros-dut results temp directory creation failed with error: %s", err)
		res.Err = errors.Annotate(err, "create dut service: create temp dir").Err()
		return res
	}

	log.Printf("--> Starting cros-dut service for %q ...", dutName)
	dutService, err := services.CreateDutService(ctx, crosDutContainer, dutName, networkName, cacheServerInfo, dutSshInfo, crosDutResultsDir, tokenFile)
	defer func() {
		// Both errs will be ignored as they are non-critical and logged in the respective calls.
		dutService.Remove(ctx)
		common.AddContentsToLog("log.txt", crosDutResultsDir, "Reading cros-dut log file.")
	}()
	if err != nil {
		res.Err = errors.Annotate(err, "run provision").Err()
		var reason = api.InstallFailure_REASON_DOCKER_UNABLE_TO_START
		if dutService.Started {
			// If the err is nil, and the service is started (noted by the log.txt existing),
			// its extremely likely we are failing to connect to the dut. So for now this is the best err to use.
			reason = api.InstallFailure_REASON_DUT_UNREACHABLE_PRE_PROVISION
		}
		if common.IsCriticalPullCrash(dutService.PullExitCode) {
			reason = api.InstallFailure_REASON_SERVICE_CONTAINER_UNABLE_TO_PULL
		}
		res.Out.Outcome = &api.CrosProvisionResponse_Failure{
			Failure: &api.InstallFailure{
				Reason: reason,
			},
		}

		return res
	}

	log.Printf("--> Container of cros-dut was started for %q", dutName)

	// Create temp results dir for cros-provision
	crosProvisionResultsDir, err := ioutil.TempDir(parentTempDir, crosProvisionResultsTempDir)
	if err != nil {
		res.Err = errors.Annotate(err, "run provision: create temp dir").Err()
		return res
	}

	log.Printf("--> Starting cros-provision service for %q ...", dutName)
	provisionReq := &api.CrosProvisionRequest{
		Dut:            device.GetDut(),
		ProvisionState: device.GetProvisionState(),
		DutServer: &labapi.IpEndpoint{
			Address: "localhost",
			Port:    int32(dutService.ServicePort),
		},
	}

	provisionService, err := services.RunProvisionCLI(ctx, crosProvisionContainer, networkName, provisionReq, crosProvisionResultsDir, tokenFile)
	if err != nil {
		res.Err = errors.Annotate(err, "run provision").Err()
		if common.IsCriticalPullCrash(provisionService.PullExitCode) {
			res.Err = errors.Annotate(err, "run provision").Err()
			res.Out.Outcome = &api.CrosProvisionResponse_Failure{
				Failure: &api.InstallFailure{
					Reason: api.InstallFailure_REASON_SERVICE_CONTAINER_UNABLE_TO_PULL,
				},
			}
			return res
		}
		return res
	}
	defer func() {
		if provisionService != nil {
			provisionService.Remove(ctx)
		}
		common.AddContentsToLog("log.txt", crosProvisionResultsDir, "Reading cros-provision log file.")
	}()
	log.Printf("--> Started cros-provision service for %q", dutName)

	common.AddContentsToLog(services.InputFileName, crosProvisionResultsDir, "Reading cros-provision input file.")
	resultFileName := path.Join(crosProvisionResultsDir, services.OutputFileName)
	if _, err := os.Stat(resultFileName); os.IsNotExist(err) {
		res.Err = errors.Reason("run provision: result not found").Err()
		return res
	}
	out, err := readProvisionOutput(resultFileName)
	if err != nil {
		res.Err = errors.Annotate(err, "run provision").Err()
		return res
	}
	log.Printf("result file %s: found. %s", dutName, out)
	if f := out.GetFailure(); f != nil {
		res.Out.Outcome = &api.CrosProvisionResponse_Failure{
			Failure: f,
		}
		res.Err = errors.New("run provision")
	} else {
		res.Out.Outcome = &api.CrosProvisionResponse_Success{
			Success: &api.InstallSuccess{},
		}
		res.Err = nil
	}
	return res
}

// readProvisionOutput reads output file generated by cros-provision.
func readProvisionOutput(filePath string) (*api.CrosProvisionResponse, error) {
	r, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Annotate(err, "read output").Err()
	}
	out := &api.CrosProvisionResponse{}

	umrsh := jsonpb.Unmarshaler{AllowUnknownFields: true}
	err = umrsh.Unmarshal(r, out)

	log.Printf("cros-provision response:" + out.String())
	return out, errors.WrapIf(err, "read output")
}
