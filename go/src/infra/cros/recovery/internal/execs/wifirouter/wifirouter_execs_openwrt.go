// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifirouter

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
)

func fetchOpenWrtBuildInfoExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostOpenWrtRouterController(ctx, info)
	if err != nil {
		return errors.Annotate(err, "fetch openwrt build info").Err()
	}
	if err := c.FetchDeviceBuildInfo(ctx); err != nil {
		return errors.Annotate(err, "failed to fetch device build info").Err()
	}
	return nil
}

func fetchOpenWrtConfigExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostOpenWrtRouterController(ctx, info)
	if err != nil {
		return errors.Annotate(err, "fetch openwrt config").Err()
	}
	if err := c.FetchGlobalImageConfig(ctx); err != nil {
		return errors.Annotate(err, "failed to fetch device build info").Err()
	}
	return nil
}

func identifyExpectedOpenWrtImageExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostOpenWrtRouterController(ctx, info)
	if err != nil {
		return errors.Annotate(err, "identify expected openwrt image").Err()
	}
	dut := info.GetDut()
	if dut == nil {
		return errors.Reason("dut is not present").Err()
	}
	// For now, we always want repair to select the highest non-next version, so
	// a very high version is used for selection to force this behavior.
	actionArgs := info.GetActionArgs(ctx)
	const crosVersionActionArgKey = "cros_version"
	crosVersion := actionArgs.AsString(ctx, crosVersionActionArgKey, "999999999")
	if err := c.IdentifyExpectedImage(ctx, dut.Name, crosVersion); err != nil {
		return errors.Annotate(err, "failed to identify expected OpenWrt OS image for device").Err()
	}
	return nil
}

func assertOpenWrtRouterHasExpectedImageExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostOpenWrtRouterController(ctx, info)
	if err != nil {
		return errors.Annotate(err, "assert openwrt router has expected image").Err()
	}
	return c.AssertHasExpectedImage()
}

func updateOpenWrtRouterToExpectedImageExec(ctx context.Context, info *execs.ExecInfo) error {
	c, err := activeHostOpenWrtRouterController(ctx, info)
	if err != nil {
		return errors.Annotate(err, "update openwrt router to expected image").Err()
	}
	return c.UpdateToExpectedImage(ctx)
}

func init() {
	execs.Register("wifi_router_openwrt_fetch_build_info", fetchOpenWrtBuildInfoExec)
	execs.Register("wifi_router_openwrt_fetch_config", fetchOpenWrtConfigExec)
	execs.Register("wifi_router_openwrt_identify_expected_image", identifyExpectedOpenWrtImageExec)
	execs.Register("wifi_router_openwrt_has_expected_image", assertOpenWrtRouterHasExpectedImageExec)
	execs.Register("wifi_router_openwrt_update_to_expected_image", updateOpenWrtRouterToExpectedImageExec)
}
