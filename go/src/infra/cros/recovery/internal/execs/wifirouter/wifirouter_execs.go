// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifirouter

import (
	"context"
	"strings"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/controller"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func setStateExec(ctx context.Context, info *execs.ExecInfo) error {
	const stateArgKey = "state"
	actionArgs := info.GetActionArgs(ctx)
	if !actionArgs.Has(stateArgKey) {
		return errors.Reason("missing required action argument %q", stateArgKey).Err()
	}
	stateName := actionArgs.AsString(ctx, stateArgKey, "")
	stateValue, ok := tlw.WifiRouterHost_State_value[stateName]
	if !ok {
		return errors.Reason("action argument %q (%s) does not match a known state", stateArgKey, stateName).Err()
	}
	wifiRouterHost, err := activeHost(info)
	if err != nil {
		return err
	}
	wifiRouterHost.State = tlw.WifiRouterHost_State(stateValue)
	return nil
}

func updatePeripheralWifiStateExec(ctx context.Context, info *execs.ExecInfo) error {
	chromeos := info.GetChromeos()
	if chromeos == nil {
		return errors.Reason("update peripheral wifi state: chromeos is not present").Err()
	}
	routers := chromeos.GetWifiRouters()
	pws := tlw.ChromeOS_PERIPHERAL_WIFI_STATE_NOT_APPLICABLE
	if len(routers) > 0 {
		pws = tlw.ChromeOS_PERIPHERAL_WIFI_STATE_WORKING
		for _, routerHost := range chromeos.GetWifiRouters() {
			if routerHost.GetState() != tlw.WifiRouterHost_WORKING {
				pws = tlw.ChromeOS_PERIPHERAL_WIFI_STATE_BROKEN
				break
			}
		}
	}
	chromeos.PeripheralWifiState = pws
	return nil
}

// updateWifiRouterFeaturesExec updates the overall testbed WifiRouterFeatures
// to only include features that are common among all routers in the testbed.
func updateWifiRouterFeaturesExec(ctx context.Context, info *execs.ExecInfo) error {
	chromeos := info.GetChromeos()
	if chromeos == nil {
		return errors.Reason("update peripheral wifi state: chromeos is not present").Err()
	}
	commonFeatures := controller.CollectOverallTestbedWifiRouterFeatures(chromeos.WifiRouters)
	controller.SortWifiRouterFeaturesByName(commonFeatures)
	chromeos.WifiRouterFeatures = commonFeatures
	return nil
}

func identifyDeviceTypeExec(ctx context.Context, info *execs.ExecInfo) error {
	host, err := activeHost(info)
	if err != nil {
		return errors.Annotate(err, "failed to get active wifi router host").Err()
	}
	deviceType, err := controller.IdentifyRouterDeviceType(ctx, info.GetAccess(), info.GetActiveResource())
	if err != nil {
		return errors.Annotate(err, "failed to analyze device type of wifirouter").Err()
	}
	host.DeviceType = deviceType
	if host.DeviceType == labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_INVALID {
		return errors.Reason("router not identified as a valid device type").Err()
	}
	log.Debugf(ctx, "WifiRouter %q device type identified as %q", host.Name, host.DeviceType)
	return nil
}

func deviceTypeInListExec(ctx context.Context, info *execs.ExecInfo) error {
	host, err := activeHost(info)
	if err != nil {
		return errors.Annotate(err, "failed to get active wifi router host").Err()
	}
	const deviceTypesArgKey = "device_types"
	actionArgs := info.GetActionArgs(ctx)
	if !actionArgs.Has(deviceTypesArgKey) {
		return errors.Reason("missing required action argument %q", deviceTypesArgKey).Err()
	}
	deviceTypesArg := actionArgs.AsStringSlice(ctx, deviceTypesArgKey, nil)
	if len(deviceTypesArg) == 0 {
		return errors.Reason("action argument %q must not be empty", deviceTypesArgKey).Err()
	}
	var deviceTypes []labapi.WifiRouterDeviceType
	hostHasMatchingDeviceType := false
	for i, deviceTypeName := range deviceTypesArg {
		deviceTypeValue, ok := labapi.WifiRouterDeviceType_value[deviceTypeName]
		if !ok {
			return errors.Reason("action argument %q[%d] %q does not match a known device type", deviceTypesArgKey, i, deviceTypeName).Err()
		}
		dt := labapi.WifiRouterDeviceType(deviceTypeValue)
		deviceTypes = append(deviceTypes, dt)
		if host.DeviceType == dt {
			hostHasMatchingDeviceType = true
		}
	}
	if !hostHasMatchingDeviceType {
		var deviceTypeNames []string
		for _, deviceType := range deviceTypes {
			deviceTypeNames = append(deviceTypeNames, deviceType.String())
		}
		return errors.Reason("wifi router type %q does match any of the desired device types [%s]", host.DeviceType.String(), strings.Join(deviceTypeNames, ",")).Err()
	}
	return nil
}

// rebootDeviceExec reboots the router host. Can take up to 4 minutes, as it
// waits for the host to come back up before completing.
func rebootDeviceExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostRouterController(ctx, info)
	if err != nil {
		return err
	}
	if err := c.Reboot(ctx); err != nil {
		return errors.Annotate(err, "failed to reboot device").Err()
	}
	return nil
}

func updateModelAndFeaturesExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostRouterController(ctx, info)
	if err != nil {
		return err
	}

	model, err := c.Model()
	if err != nil {
		return errors.Annotate(err, "failed to get device model").Err()
	}
	features, err := c.Features()
	if err != nil {
		return errors.Annotate(err, "failed to get device features").Err()
	}
	features = controller.CleanWifiRouterFeatures(features)

	wifiRouterHost := c.WifiRouterHost()
	wifiRouterHost.Model = model
	wifiRouterHost.Features = features

	return nil
}

func logUsageReportOfDirExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostRouterController(ctx, info)
	if err != nil {
		return err
	}
	const dirPathArgKey = "path"
	actionArgs := info.GetActionArgs(ctx)
	if !actionArgs.Has(dirPathArgKey) {
		return errors.Reason("missing required action argument %q", dirPathArgKey).Err()
	}
	dirPathArg := actionArgs.AsString(ctx, dirPathArgKey, "")
	if dirPathArg == "" {
		return errors.Reason("action argument %q must not be empty", dirPathArgKey).Err()
	}
	if err := logReportOfFilesInDir(ctx, info, c, dirPathArg); err != nil {
		return errors.Annotate(err, "log gale dir stateful partition of path %q", dirPathArg).Err()
	}
	return nil
}

func init() {
	execs.Register("update_peripheral_wifi_state", updatePeripheralWifiStateExec)
	execs.Register("update_wifi_router_features", updateWifiRouterFeaturesExec)
	execs.Register("wifi_router_set_state", setStateExec)
	execs.Register("wifi_router_identify_device_type", identifyDeviceTypeExec)
	execs.Register("wifi_router_device_type_in_list", deviceTypeInListExec)
	execs.Register("wifi_router_update_model_and_features", updateModelAndFeaturesExec)
	execs.Register("wifi_router_reboot", rebootDeviceExec)
	execs.Register("wifi_router_log_usage_report_of_dir", logUsageReportOfDirExec)
}
