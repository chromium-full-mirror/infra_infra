// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	// lsbReleaseMatchIfUbuntu is a regex string that matches the /etc/lsb-release
	// file on the device if it is an ubuntu-based device.
	lsbReleaseMatchIfUbuntu = "(?m)^DISTRIB_ID=Ubuntu$"
)

// pCINetworkControllerFullToShortName is a mapping of the name of network
// controller name provided by the lspci command to the name used in the model
// name. If no mapping is found for a given name, the full name is used in the
// model name.
var pCINetworkControllerFullToShortName = map[string]string{
	"Intel Corporation Device 272b": "be200",
}

// ubuntuModelToFeatures is a mapping of full model name to the
// WifiRouterFeatures supported by the model.
var ubuntuModelToFeatures = map[string][]labapi.WifiRouterFeature{
	"UBUNTU[NUC11TNHi5_be200]": {
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_A,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_B,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_G,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_N,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AC,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_AX_E,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_IEEE_802_11_BE,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_DOUBLE_BRIDGE_OVER_VETH,
		labapi.WifiRouterFeature_WIFI_ROUTER_FEATURE_GCMP,
	},
}

// hostIsUbuntuRouter checks if the remote host is an Ubuntu router.
func hostIsUbuntuRouter(ctx context.Context, sshRunner ssh.Runner) (bool, error) {
	return lsbReleaseFileMatches(ctx, sshRunner, lsbReleaseMatchIfUbuntu)
}

// UbuntuRouterController is the RouterController implementation for
// Ubuntu-based router devices.
type UbuntuRouterController struct {
	sshRunner      ssh.Runner
	wifiRouterHost *tlw.WifiRouterHost
	state          *tlw.UbuntuRouterControllerState
}

func newUbuntuRouterController(sshRunner ssh.Runner, wifiRouterHost *tlw.WifiRouterHost, state *tlw.UbuntuRouterControllerState) *UbuntuRouterController {
	return &UbuntuRouterController{
		wifiRouterHost: wifiRouterHost,
		sshRunner:      sshRunner,
		state:          state,
	}
}

// WifiRouterHost returns the corresponding tlw.WifiRouterHost instance for
// this router. Changes to this instance are persisted across execs.
func (c *UbuntuRouterController) WifiRouterHost() *tlw.WifiRouterHost {
	return c.wifiRouterHost
}

// DeviceType returns the labapi.WifiRouterDeviceType of the router.
func (c *UbuntuRouterController) DeviceType() labapi.WifiRouterDeviceType {
	return labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_UBUNTU
}

// Runner returns a components.Runner for running ssh commands on the router.
func (c *UbuntuRouterController) Runner() components.Runner {
	return c.sshRunner.Run
}

// Model returns a unique name for the router model.
//
// For Ubuntu-based routers, model names are a combination of DeviceType, the
// system product name, and their network controller name.
func (c *UbuntuRouterController) Model() (string, error) {
	if c.state.GetSystemProductName() == "" {
		return "", errors.Reason("SystemProductName is empty or not yet retrieved from device").Err()
	}
	if c.state.GetNetworkControllerName() == "" {
		return "", errors.Reason("NetworkControllerName is empty or not yet retrieved from device").Err()
	}
	deviceName := fmt.Sprintf("%s_%s", c.state.GetSystemProductName(), c.state.GetNetworkControllerName())
	return buildModelName(c.DeviceType(), deviceName), nil
}

// FetchSystemProductName retrieves the system product name of the Ubuntu device
// from its DMI table and stores it in the state for later reference in model
// name creation.
func (c *UbuntuRouterController) FetchSystemProductName(ctx context.Context) error {
	systemProductName, err := c.sshRunner.Run(ctx, 30*time.Second, "dmidecode", "-s", "system-product-name")
	if err != nil {
		return errors.Annotate(err, "fetch system product name: failed to get system-product-name from DMI table").Err()
	}
	systemProductName = strings.TrimSpace(systemProductName)
	if systemProductName == "" {
		return errors.Reason("fetch system product name: got empty system-product-name from DMI table").Err()
	}
	c.state.SystemProductName = systemProductName
	return nil
}

// FetchNetworkControllerName retrieves the name of the PCI network controller
// on the device and stores it (or a translated version of it) in the state for
// later reference in model name creation.
func (c *UbuntuRouterController) FetchNetworkControllerName(ctx context.Context) error {
	// Get raw name of PCI network controller.
	lscpiOutput, err := c.sshRunner.Run(ctx, 30*time.Second, "lspci")
	if err != nil {
		return errors.Annotate(err, "fetch network controller name: failed to run lspci").Err()
	}
	networkControllerName, err := c.parseNetworkControllerName(lscpiOutput)
	if err != nil {
		return errors.Annotate(err, "fetch network controller name: failed to parse raw network controller name from lscpi output %q", lscpiOutput).Err()
	}
	// Translate raw name to more commonly known (if available).
	if rename, ok := pCINetworkControllerFullToShortName[networkControllerName]; ok {
		networkControllerName = rename
	}
	c.state.NetworkControllerName = networkControllerName
	return nil
}

func (c *UbuntuRouterController) parseNetworkControllerName(lscpiOutput string) (string, error) {
	matcher, err := regexp.Compile(`(?m)^.+ Network controller: (.+) \(rev .*\)$`)
	if err != nil {
		return "", errors.Annotate(err, "parse network controller name: regex compilation failed").Err()
	}
	match := matcher.FindStringSubmatch(lscpiOutput)
	if len(match) != 2 {
		return "", errors.Reason("parse network controller name: no regex match").Err()
	}
	networkControllerName := strings.TrimSpace(match[1])
	if networkControllerName == "" {
		return "", errors.Reason("parse network controller name: got empty network controller name").Err()
	}
	return networkControllerName, nil
}

// Features returns the router features that this router supports.
//
// ChromeOS Gale routers have a static list of known features. This list is
// maintained by the controller and is not retrieved from the device itself.
func (c *UbuntuRouterController) Features() ([]labapi.WifiRouterFeature, error) {
	model, err := c.Model()
	if err != nil {
		return nil, errors.Annotate(err, "features: failed to get model name").Err()
	}
	features, ok := ubuntuModelToFeatures[model]
	if !ok {
		return nil, errors.Annotate(err, "features: no features mapped for model %s", model).Err()
	}
	return features, nil
}

// Reboot will reboot the router and wait for it to come back up. A non-nil
// error indicates that the router was rebooted and is ssh-able again.
func (c *UbuntuRouterController) Reboot(ctx context.Context) error {
	return ssh.Reboot(ctx, c.sshRunner, defaultPostRebootSSHDelay, defaultPostRebootSSHInterval, defaultPostRebootSSHTimeout)
}
