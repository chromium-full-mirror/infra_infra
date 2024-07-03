// Copyright 2019 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"infra/appengine/crosskylabadmin/internal/app/config"
	"infra/appengine/crosskylabadmin/internal/app/frontend"
	"infra/appengine/crosskylabadmin/internal/ufs"
	"infra/libs/skylab/common/heuristics"
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
	ufsClient, err := createUFSClient(ctx, cfg.GetUFS().GetHost())
	if err != nil {
		return errors.Annotate(err, "run repair queue handler").Err()
	}
	logging.Infof(ctx, "run repair queue handler: UFS client created successfully")
	botID := c.Request.FormValue("botID")
	expectedState := c.Request.FormValue("expectedState")
	swarmingPool := c.Request.FormValue("swarmingPool")
	// RandFloat is guaranteed to be in the half-open interval [0,1).
	randFloat := rand.Float64()
	pools, err := GetPoolsForHostname(ctx, ufsClient, botID)
	if err != nil {
		return errors.Annotate(err, "run repair queue handler").Err()
	}
	logging.Infof(ctx, "run repair queue handler: found pools for bot %s: %s", botID, pools)
	taskURL, err := frontend.CreateRepairTask(ctx, botID, expectedState, pools, randFloat, swarmingPool)
	if err != nil {
		logging.Infof(ctx, "fail to run repair job in queue for %s in swarming pool %q: %s", swarmingPool, err.Error())
		return err
	}

	logging.Infof(ctx, "Successfully run repair job for %s: %s", botID, taskURL)
	return nil
}

func runAuditQueueHandler(c *router.Context) (err error) {
	ctx := c.Request.Context()

	defer func() {
		runAuditTick.Add(ctx, 1, err == nil)
	}()

	botID := c.Request.FormValue("botID")
	cfg := config.Get(ctx)
	ufsClient, err := createUFSClient(ctx, cfg.GetUFS().GetHost())
	if err != nil {
		return errors.Annotate(err, "run audit queue handler").Err()
	}
	pools, err := GetPoolsForHostname(ctx, ufsClient, botID)
	if err != nil {
		return errors.Annotate(err, "run audit queue handler").Err()
	}
	logging.Infof(ctx, "run audit queue handler: found pools for bot %s: %s", botID, pools)
	actions := c.Request.FormValue("actions")
	taskname := c.Request.FormValue("taskname")
	randFloat := rand.Float64()
	taskURL, err := frontend.CreateAuditTask(ctx, botID, pools[0], taskname, actions, randFloat)
	if err != nil {
		return err
	}
	logging.Infof(ctx, "Successfully run audit job for %s: %s", botID, taskURL)
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

func GetPoolsForHostname(ctx context.Context, c ufs.Client, hostname string) ([]string, error) {
	hostname = heuristics.NormalizeBotNameToDeviceName(hostname)
	pools, err := ufs.GetPools(ctx, c, hostname)
	if err != nil {
		return nil, errors.Annotate(err, "getting pools for hostname %s", hostname).Err()
	}
	if len(pools) == 0 {
		return nil, errors.Reason("found no pools for hostname %s", hostname).Err()
	}
	return pools, nil
}
