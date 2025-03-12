// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Executor defines a state initializer for each state. Usable by server to start.
// Under normal conditions this would be part of service, but go being go, it
// would create an impossible cycle
package executor

import (
	"fmt"
	"log"
	"strconv"
	"time"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	cross_over "go.chromium.org/chromiumos/test/provision/v2/common-utils/cross-over"
	"go.chromium.org/chromiumos/test/provision/v2/foil-provision/service"
	state_machine "go.chromium.org/chromiumos/test/provision/v2/foil-provision/state-machine"
	"go.chromium.org/chromiumos/test/util/adb"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
)

type FoilProvisionExecutor struct {
	Logger *log.Logger
}

func NewFoilProvisionExecutor(logger *log.Logger) (*FoilProvisionExecutor, error) {
	return &FoilProvisionExecutor{
		Logger: logger,
	}, nil
}

func (c *FoilProvisionExecutor) GetFirstState(dut *lab_api.Dut, dutClient api.DutServiceClient, servoNexusAddr string, req *api.InstallRequest) (common_utils.ServiceState, error) {
	crossOverRequired := c.crossOverRequired(dut, req)
	if crossOverRequired {
		state, err := crossOverProvisionState(dut, dutClient, servoNexusAddr, req)
		if err != nil {
			c.Logger.Println("Unable to perform crossover due to ", err)
		} else {
			return state, err
		}
	}

	c.Logger.Println("Attempting OTA update.")

	cs, err := service.NewFoilService(dut, req, dutClient, servoNexusAddr)
	if err != nil {
		return nil, err
	}
	return state_machine.NewFoilPreInitState(cs), nil
}

func crossOverProvisionState(dut *lab_api.Dut, dutClient api.DutServiceClient, servoNexusAddr string, req *api.InstallRequest) (common_utils.ServiceState, error) {
	if servoNexusAddr == "" {
		return nil, fmt.Errorf("servoNexusAdd required for crossover provision")
	}
	conn, err := grpc.Dial(servoNexusAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	servoNexusClient := api.NewServodServiceClient(conn)
	params := &cross_over.CrossOverParameters{
		Dut:              dut,
		TargetImagePath:  req.GetImagePath(),
		ServoNexusClient: servoNexusClient,
		PartnerMetadata:  req.GetPartnerMetadata(),
	}
	return cross_over.NewCrossOverInitState(params), nil
}

func (c *FoilProvisionExecutor) crossOverRequired(dut *lab_api.Dut, req *api.InstallRequest) bool {
	// Due to ongoing OTA issues; force the flash for now. b/378974495
	return true
	osType, err := cross_over.DetectOS(c.Logger, fmt.Sprintf("%v:%v", dut.GetChromeos().GetSsh().GetAddress(), dut.GetChromeos().GetSsh().GetPort()))
	if err != nil {
		c.Logger.Println("Could not detect the OS. Will perform CrossOver provision.")
		return true
	} else if osType == cross_over.ANDROID {
		c.Logger.Println("setting up adb")
		err := adb.RetrySetupAdb(c.Logger, dut.GetChromeos().GetSsh().GetAddress(), 10*time.Second)
		if err != nil {
			c.Logger.Println("ADB could not connect after detection. Suspected flaky device. Forcing the flash.")
			return true
		}
		c.Logger.Println("Trying to check target version")
		if targetOlder(dut.GetChromeos().GetSsh().GetAddress(), req.GetImagePath().GetPath(), c.Logger) == true {
			c.Logger.Println("Android Detected. However, current image is newer than target, flashing.")
			return true
		} else {
			c.Logger.Println("Android Detected. Will OTA")
			// Cleanup ADB before the provision so that it can start from a good known state.
			if err := adb.TeardownAdb(c.Logger, dut.GetChromeos().GetSsh().GetAddress()); err != nil {
				c.Logger.Println("Warning: TeardownAdb failed: ", err)
			}
		}
	} else {
		c.Logger.Println("CROS Detected")
		return true
	}
	return false
}

func targetOlder(dutAddr string, path string, log *log.Logger) bool {
	currentBuild, err := adb.GetBuildVersion(dutAddr, log)
	if err != nil {
		log.Println("error getting image, force the flash")
		return true
	}
	log.Println("Current build ID!", currentBuild)
	currentBuildInt, err := strconv.Atoi(currentBuild)
	if err != nil {
		log.Println("unable to parse current image, forcing a flash.")
		return true
	}
	targetBuild, err := service.TargetBuild(path)
	if err != nil {
		log.Println("unable to parse target image, forcing a flash.")
		return true
	}
	targetBuildInt, err := strconv.Atoi(targetBuild)
	if err != nil {
		log.Println("unable to parse current image, forcing a flash.")
		return true
	}
	log.Printf("Current build: %v, Target build: %v\n", currentBuildInt, targetBuildInt)

	return targetBuildInt < currentBuildInt
}

// Validate ensures the ProvisionStartupRequest meets specified requirements.
func (c *FoilProvisionExecutor) Validate(req *api.ProvisionStartupRequest) error {
	return nil
}
