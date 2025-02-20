// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wifirouter initializes execs to be used with wifi routers.
package wifirouter

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/linux"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/controller"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// activeHost finds active host related to the executed plan.
func activeHost(info *execs.ExecInfo) (*tlw.WifiRouterHost, error) {
	resource := info.GetActiveResource()
	chromeos := info.GetChromeos()
	if chromeos == nil {
		return nil, errors.Reason("chromeos is not present").Err()
	}
	for _, router := range chromeos.GetWifiRouters() {
		if router.GetName() == resource {
			return router, nil
		}
	}
	return nil, errors.Reason("router: router not found").Err()
}

func activeHostRouterController(ctx context.Context, info *execs.ExecInfo) (controller.RouterController, error) {
	if info.GetDut() == nil {
		return nil, errors.Reason("dut is nil").Err()
	}
	wifiRouterHost, err := activeHost(info)
	if err != nil {
		return nil, err
	}
	return controller.NewRouterDeviceController(ctx, info.GetAccess(), info.GetAccess(), info.GetActiveResource(), info.GetDut().Name, wifiRouterHost)
}

func activeHostAsusWrtRouterController(ctx context.Context, info *execs.ExecInfo) (*controller.AsusWrtRouterController, error) {
	genericController, err := activeHostRouterController(ctx, info)
	if err != nil {
		return nil, err
	}
	asusWrtController, ok := genericController.(*controller.AsusWrtRouterController)
	if !ok {
		return nil, errors.Reason("active host is not an AsusWrt test router").Err()
	}
	return asusWrtController, nil
}

func activeHostOpenWrtRouterController(ctx context.Context, info *execs.ExecInfo) (*controller.OpenWrtRouterController, error) {
	genericController, err := activeHostRouterController(ctx, info)
	if err != nil {
		return nil, err
	}
	openWrtController, ok := genericController.(*controller.OpenWrtRouterController)
	if !ok {
		return nil, errors.Reason("active host is not an OpenWrt test router").Err()
	}
	return openWrtController, nil
}

func activeHostUbuntuRouterController(ctx context.Context, info *execs.ExecInfo) (*controller.UbuntuRouterController, error) {
	genericController, err := activeHostRouterController(ctx, info)
	if err != nil {
		return nil, err
	}
	ubuntuController, ok := genericController.(*controller.UbuntuRouterController)
	if !ok {
		return nil, errors.Reason("active host is not an Ubuntu test router").Err()
	}
	return ubuntuController, nil
}

func logReportOfFilesInDir(ctx context.Context, info *execs.ExecInfo, c controller.RouterController, dirPath string) error {
	csvReport, err := linux.StorageUtilizationReportOfFilesInDir(ctx, c.Runner(), dirPath)
	if err != nil {
		return errors.Annotate(err, "log report of files in dir: create report").Err()
	}
	logFileName := log.BuildFilename([]string{"storage_utilization_report", dirPath}, "csv")
	logPath, err := log.WriteResourceLogFile(ctx, info.GetLogRoot(), info.GetActiveResource(), logFileName, []byte(csvReport))
	if err != nil {
		return errors.Annotate(err, "log report of files in dir: write report").Err()
	}
	log.Infof(ctx, "Logged storage utilization report of router dir %q to %q", dirPath, logPath)
	return nil
}
