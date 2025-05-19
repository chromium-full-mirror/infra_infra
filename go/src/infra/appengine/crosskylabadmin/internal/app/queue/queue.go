// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package queue implements handlers for taskqueue jobs in this app.
//
// All actual logic are implemented in tasker layer.
package queue

import (
	"context"
	"math/rand"
	"net/http"

	"go.chromium.org/luci/appengine/gaemiddleware"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/ufs"
	"go.chromium.org/infra/libs/fleet/device"
)

// InstallHandlers installs handlers for queue jobs that are part of this app.
//
// All handlers serve paths under /internal/queue/*
func InstallHandlers(r *router.Router, mwBase router.MiddlewareChain) {
	r.POST(
		"/internal/task/cros_repair/*ignored",
		mwBase.Extend(gaemiddleware.RequireTaskQueue("repair-bots")),
		logAndSetHTTPErr(runRepairQueueHandler),
	)
	r.POST(
		"/internal/task/labstation_repair/*ignored",
		mwBase.Extend(gaemiddleware.RequireTaskQueue("repair-labstations")),
		logAndSetHTTPErr(runRepairQueueHandler),
	)
	r.POST(
		"/internal/task/audit/*ignored",
		mwBase.Extend(gaemiddleware.RequireTaskQueue("audit-bots")),
		logAndSetHTTPErr(runAuditQueueHandler),
	)
}

func runRepairQueueHandler(c *router.Context) (err error) {
	ctx := c.Request.Context()
	defer func() {
		runRepairTick.Add(ctx, 1, err == nil)
	}()
	// Create a UFS client at the beginning of repair and log the result, but do NOT stop execution
	// because of problems. We are not yet ready to make UFS a hard dependency of CSA, so at this point
	// it is a soft dependency. The UFS client can be nil.
	//
	// We are going to use the pools associated with a device as an input to decide which implementation
	// of repair to use.
	cfg := config.Get(ctx)
	botID := c.Request.FormValue("botID")
	swarmingPool := c.Request.FormValue("swarmingPool")
	expectedState := c.Request.FormValue("expectedState")
	poolCfg := frontend.GetPoolCfg(ctx, swarmingPool)
	if poolCfg == nil {
		return errors.Reason("run repair queue handler: fail to find pool config for bot: %q, by pool: %q", botID, swarmingPool).Err()
	}
	dutName := poolCfg.BotIDToDUTName(botID)
	namespace := poolCfg.UFSCtxNamespace()
	logging.Infof(ctx, "Using namespace %q for %q", namespace, dutName)
	ufsCtx := ufs.ContextWithNamespace(ctx, namespace)
	ufsClient, err := createUFSClient(ctx, cfg.GetUFS().GetHost())
	if err != nil {
		logging.Errorf(ctx, "Fail to create UFS client for bot %q: %w", botID, err)
		return errors.Annotate(err, "run repair queue handler").Err()
	}
	logging.Infof(ctx, "run repair queue handler: UFS client created successfully")
	di, err := getDeviceInfo(ufsCtx, ufsClient, dutName)
	if err != nil {
		logging.Errorf(ufsCtx, "Fail to get device info for bot %q: %w", botID, err)
		return errors.Annotate(err, "run repair queue handler").Err()
	}
	logging.Infof(ufsCtx, "run repair queue handler: found pools for bot %s: %s", botID, di.Pools)
	// RandFloat is guaranteed to be in the half-open interval [0,1).
	randFloat := rand.Float64()
	taskURL, err := frontend.CreateRepairTask(ufsCtx, dutName, expectedState, di.Pools, randFloat, poolCfg)
	if err != nil {
		logging.Errorf(ufsCtx, "Fail to create repair task for %s in swarming pool %q: %s", swarmingPool, err.Error())
		return err
	}
	logging.Infof(ufsCtx, "Successfully run repair job for %q: %s", botID, taskURL)
	return nil
}

func runAuditQueueHandler(c *router.Context) (err error) {
	ctx := c.Request.Context()

	defer func() {
		runAuditTick.Add(ctx, 1, err == nil)
	}()

	botID := c.Request.FormValue("botID")
	swarmingPool := c.Request.FormValue("swarmingPool")
	cfg := config.Get(ctx)
	poolCfg := frontend.GetPoolCfg(ctx, swarmingPool)
	if poolCfg == nil {
		return errors.Reason("run audit queue handler: fail to find pool config for bot: %q, by pool: %q", botID, swarmingPool).Err()
	}
	dutName := poolCfg.BotIDToDUTName(botID)
	namespace := poolCfg.UFSCtxNamespace()
	logging.Infof(ctx, "Using namespace %q for %q", namespace, dutName)
	ufsCtx := ufs.ContextWithNamespace(ctx, namespace)
	ufsClient, err := createUFSClient(ufsCtx, cfg.GetUFS().GetHost())
	if err != nil {
		logging.Errorf(ufsCtx, "Fail to create UFS client for bot %q: %w", dutName, err)
		return errors.Annotate(err, "run audit queue handler").Err()
	}

	di, err := getDeviceInfo(ufsCtx, ufsClient, dutName)
	if err != nil {
		logging.Errorf(ufsCtx, "Fail to get device info for bot %q: %w", botID, err)
		return errors.Annotate(err, "run audit queue handler").Err()
	}
	logging.Infof(ufsCtx, "run audit queue handler: found pools for bot %s: %s", botID, di.Pools)
	actions := c.Request.FormValue("actions")
	taskname := c.Request.FormValue("taskname")
	randFloat := rand.Float64()
	taskURL, err := frontend.CreateAuditTask(ufsCtx, dutName, di.Pools[0], taskname, actions, randFloat)
	if err != nil {
		logging.Errorf(ufsCtx, "Fail to create repair task for %q: %w", botID, err)
		return err
	}
	logging.Infof(ufsCtx, "Successfully run audit job for %s: %s", botID, taskURL)
	return nil
}

func logAndSetHTTPErr(f func(c *router.Context) error) func(*router.Context) {
	return func(c *router.Context) {
		if err := f(c); err != nil {
			http.Error(c.Writer, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func createUFSClient(ctx context.Context, ufsHost string) (ufs.Client, error) {
	hc, err := ufs.NewHTTPClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "creating HTTP client").Err()
	}
	c, err := ufs.NewClient(ctx, hc, ufsHost)
	if err != nil {
		return nil, errors.Annotate(err, "creating UFS client").Err()
	}
	return c, nil
}

func getDeviceInfo(ctx context.Context, c ufs.Client, hostname string) (*device.DeviceInfo, error) {
	di, err := device.GetDeviceInfo(ctx, c, hostname)
	if err != nil {
		return nil, errors.Annotate(err, "getting device info for hostname %s", hostname).Err()
	}
	if len(di.Pools) == 0 {
		return nil, errors.Reason("found no pools for hostname %s", hostname).Err()
	}
	return di, nil
}
