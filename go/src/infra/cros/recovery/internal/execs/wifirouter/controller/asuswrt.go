// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"regexp"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// nvramCmd is the path to the "nvram" command on AsusWrt devices.
	nvramCmd = "/bin/nvram"

	// asusWrtSSHUser is the ssh username to use when connecting to AsusWrt devices.
	asusWrtSSHUser = "admin"
)

var asuswrtModelToFeatures = map[string][]labapi.WifiRouterFeature{
	"RT-AX92U": {
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
	},
	"GT-AX11000": {
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
	},
	"GT-AXE11000": {
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
	},
}

// hostIsAsusWrtRouter checks if the remote host is an AsusWrt router.
func hostIsAsusWrtRouter(ctx context.Context, sshRunner ssh.Runner) (bool, error) {
	nvramCmdExists, err := ssh.TestPath(ctx, sshRunner, "-x", nvramCmd)
	if err != nil {
		return false, errors.Annotate(err, "failed to check for the existence of the command %q", nvramCmd).Err()
	}
	if !nvramCmdExists {
		return false, nil
	}

	// Verify that the manufacturer is ASUS.
	manufacturer, err := readNvramValueByKey(ctx, sshRunner, "wps_mfstring")
	if err != nil {
		return false, err
	}
	return manufacturer == "ASUSTeK Computer Inc.", nil
}

// readNvramValueByKey runs the "nvram show" command and parses the output for
// the value of the desired nvramKey.
func readNvramValueByKey(ctx context.Context, sshRunner ssh.Runner, nvramKey string) (string, error) {
	nvramShowOutput, err := sshRunner.Run(ctx, 0, nvramCmd, "show")
	if err != nil {
		return "", err
	}

	// Parse value of key.
	matchRegex := fmt.Sprintf("(?m)^%s=(.+)$", nvramKey)
	matcher, err := regexp.Compile(matchRegex)
	if err != nil {
		return "", errors.Annotate(err, "failed to compile regex string %q", matchRegex).Err()
	}
	valueMatch := matcher.FindStringSubmatch(nvramShowOutput)
	if valueMatch == nil {
		return "", errors.Reason("failed to parse %q from '%s show' output: %q", nvramKey, nvramCmd, nvramShowOutput).Err()
	}
	return valueMatch[1], nil
}

// AsusWrtRouterController is the RouterController implementation for
// AsusWrt router devices.
type AsusWrtRouterController struct {
	sshRunner      ssh.Runner
	wifiRouterHost *tlw.WifiRouterHost
	state          *tlw.AsusWrtRouterControllerState
}

func newAsusWrtRouterController(sshRunner ssh.Runner, wifiRouterHost *tlw.WifiRouterHost, state *tlw.AsusWrtRouterControllerState) *AsusWrtRouterController {
	return &AsusWrtRouterController{
		sshRunner:      sshRunner,
		wifiRouterHost: wifiRouterHost,
		state:          state,
	}
}

// WifiRouterHost returns the corresponding tlw.WifiRouterHost instance for
// this router. Changes to this instance are persisted across execs.
func (c *AsusWrtRouterController) WifiRouterHost() *tlw.WifiRouterHost {
	return c.wifiRouterHost
}

// FetchAsusModel retrieves the ASUS model from the device and stores it in the state.
func (c *AsusWrtRouterController) FetchAsusModel(ctx context.Context) error {
	if c.state == nil {
		return errors.Reason("fetch asus model: state is nil").Err()
	}
	var err error
	c.state.AsusModel, err = readNvramValueByKey(ctx, c.sshRunner, "wps_modelnum")
	if err != nil {
		return errors.Annotate(err, "failed to fetch ASUS model from ASUSWRT router").Err()
	}
	return nil
}

// DeviceType returns the labapi.WifiRouterDeviceType of the router.
func (c *AsusWrtRouterController) DeviceType() labapi.WifiRouterDeviceType {
	return labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_ASUSWRT
}

// Runner returns a components.Runner for running ssh commands on the router.
func (c *AsusWrtRouterController) Runner() components.Runner {
	return c.sshRunner.Run
}

// Model returns a unique name for the router model.
//
// For AsusWrt routers, this is a combination of the DeviceType and the Asus
// model name retrieved from the router.
func (c *AsusWrtRouterController) Model() (string, error) {
	if c.state.GetAsusModel() == "" {
		return "", errors.Reason("AsusModel is empty or not yet retrieved from device").Err()
	}
	return buildModelName(c.DeviceType(), c.state.AsusModel), nil
}

// Features returns the router features that this router supports.
//
// AsusWrt routers have a static mapping of known features for each supported
// Asus model. This mapping is maintained by the controller and is not retrieved
// from the device itself.
func (c *AsusWrtRouterController) Features() ([]labapi.WifiRouterFeature, error) {
	if c.state.GetAsusModel() == "" {
		return nil, errors.Reason("AsusModel is empty or not yet retrieved from device").Err()
	}
	features, ok := asuswrtModelToFeatures[c.state.AsusModel]
	if !ok {
		return []labapi.WifiRouterFeature{
			labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_UNKNOWN,
		}, nil
	}
	return features, nil
}

// Reboot will reboot the router and wait for it to come back up. A non-nil
// error indicates that the router was rebooted and is ssh-able again.
func (c *AsusWrtRouterController) Reboot(ctx context.Context) error {
	return ssh.Reboot(ctx, c.sshRunner, defaultPostRebootSSHDelay, defaultPostRebootSSHInterval, defaultPostRebootSSHTimeout)
}
