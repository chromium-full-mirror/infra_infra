// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cross_over

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"

	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
	"go.chromium.org/chromiumos/test/util/adb"
	"go.chromium.org/chromiumos/test/util/common"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	adbInterval = 3 * time.Minute
	adbTimeout  = 15 * time.Minute
)

// ValidateState checks if the correct version was provisioned or not.
type ValidateState struct {
	params *CrossOverParameters
}

func NewValidateState(params *CrossOverParameters) common_utils.ServiceState {
	return &ValidateState{
		params: params,
	}
}

func (s ValidateState) Execute(ctx context.Context, log *log.Logger) (*anypb.Any, api.InstallResponse_Status, error) {
	// TODO: Refactor verfication logic into tow separate commands one for Android and another one for cros.
	log.Println("Executing " + s.Name())

	// TODO: Use TargetImagePath's HostType to identify installation type.
	if strings.Contains(s.params.TargetImagePath.GetPath(), "android-build") {
		log.Println("Performing validation check for AL.")
		// Refer to b/382122013 for more details on why such a high timeout is needed.
		dutAddress := fmt.Sprintf("%s:%v", s.params.Dut.GetChromeos().GetSsh().GetAddress(), "5555")
		adbCtx, cancel := context.WithTimeout(ctx, adbTimeout)
		defer cancel()
	adbLoop:
		for {
			select {
			case <-adbCtx.Done():
				collectUARTLogs(ctx, log, s.params)
				return common_utils.WrapStringInAny("PLATFORM: unable to connect to device post al-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, fmt.Errorf("adb timeout into device=%s", dutAddress)
			default:
				if adberr := adb.RetrySetupAdb(log, dutAddress, adbInterval); adberr != nil {
					log.Println("Error while testing adb connection ", adberr)
					// Hard off followed by on to refresh the device boot state.
					// Sometimes the networking services don't seem to be always providing a valid connection.
					if err := callServodRetry(ctx, log, "power_state", "off", s.params); err != nil {
						log.Println("Error while powering off via servo")
					}
					if err := callServodRetry(ctx, log, "power_state", "on", s.params); err != nil {
						log.Println("Error while powering on via servo")
					}
				} else {
					break adbLoop
				}
			}
		}
		collectUARTLogs(ctx, log, s.params)
		// As soon as we get the ADB connection, immediately set the keep awake settings.
		c1, err := adb.AdbShellCmd([]string{"svc", "power", "stayon", "true"}, dutAddress, log)
		if err != nil {
			log.Println("Unable to set stay awake:", c1)
			return common_utils.WrapStringInAny("PLATFORM: setting stay awake failed"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
		c2, err := adb.AdbShellCmd([]string{"settings", "put", "global", "stay_on_while_plugged_in", "7"}, dutAddress, log)
		if err != nil {
			log.Println("Unable to set stay awake:", c2)
			return common_utils.WrapStringInAny("PLATFORM: setting stay awake failed"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}

		dutVersion, err := adb.AdbShellCmd([]string{"getprop", "ro.system.build.version.incremental"}, dutAddress, log)
		if err != nil {
			return common_utils.WrapStringInAny("PLATFORM: unable to get build from device post install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
		dutVersion = strings.Trim(dutVersion, " \n")
		targetBuild, err := targetBuild(s.params.TargetImagePath.GetPath())
		if err != nil {
			return nil, api.InstallResponse_STATUS_INVALID_REQUEST, err
		}
		if targetBuild != dutVersion {
			err := fmt.Errorf("post provision verification failed. Installed version on dut is %s but want %s", dutVersion, targetBuild)
			return common_utils.WrapStringInAny("PLATFORM: device version not correct post al-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
		if err := adb.TeardownAdb(log, dutAddress); err != nil {
			log.Println("Warning: TeardownAdb failed in ValidateState ", err)
		}
	} else {
		log.Println("Calling force connect on cros-dut")
		if err := retryForceReconnect(ctx, log, s.params.DutClient, bootWaitRetryCount, bootWaitRetryInterval); err != nil {
			return common_utils.WrapStringInAny("PLATFORM: unable to connect to device post cros-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
		osReleaseFile, err := common.GetFile(ctx, "/etc/os-release", s.params.DutClient)
		if err != nil {
			log.Printf("Could not fetch /etc/os-release : %s\n", err)
			return common_utils.WrapStringInAny("PLATFORM: unable to obtain /etc/os-release post cros-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
		contentBytes, err := ioutil.ReadFile(osReleaseFile)
		if err != nil {
			log.Printf("Could not read %v : %s\n", osReleaseFile, err)
			return common_utils.WrapStringInAny("PLATFORM: unable to read /etc/os-release post cros-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
		content := string(contentBytes)
		log.Println("Content of /etc/os-release ", content)
		buildIDRegex := regexp.MustCompile("BUILD_ID=(.*)")
		buildIDResult := buildIDRegex.FindStringSubmatch(content)
		if len(buildIDResult) == 0 {
			return common_utils.WrapStringInAny("PLATFORM: device version not correct post cros-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, fmt.Errorf("could not find buildId")
		}
		targetImgPath := s.params.TargetImagePath.GetPath()
		if !strings.Contains(targetImgPath, buildIDResult[1]) {
			err = fmt.Errorf("expected version %s but got %s", targetImgPath, buildIDResult[1])
			return common_utils.WrapStringInAny("PLATFORM: device version not correct post cros-install"), api.InstallResponse_STATUS_POST_PROVISION_SETUP_FAILED, err
		}
	}
	return nil, api.InstallResponse_STATUS_SUCCESS, nil
}

func (s ValidateState) Next() common_utils.ServiceState {
	return NewCrossOverTerminateState(s.params)
}

func (s ValidateState) Name() string {
	return "Validate State"
}

func retryForceReconnect(ctx context.Context, log *log.Logger, dutClient api.DutServiceClient, retryCount int, retryInterval time.Duration) error {
	log.Println("Trying out ForceReconnect with retry count: ", retryCount)
	var err error
	for ; retryCount >= 0; retryCount-- {
		_, err = dutClient.ForceReconnect(ctx, &api.ForceReconnectRequest{})
		if err == nil {
			return nil
		}
		time.Sleep(retryInterval)
	}
	return err
}
