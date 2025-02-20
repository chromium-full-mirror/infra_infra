// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cron implements handlers for appengine cron targets in this app.
//
// All actual logic related to fleet management should be implemented in the
// main fleet API. These handlers should only encapsulate the following bits of
// logic:
// - Calling other API as the appengine service account user.
// - Translating luci-config driven admin task parameters.
package cron

import (
	"fmt"
	"net/http"
	"strings"

	"go.chromium.org/luci/appengine/gaemiddleware"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/router"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend"
)

// InstallHandlers installs handlers for cron jobs that are part of this app.
//
// All handlers serve paths under /internal/cron/*
// These handlers can only be called by appengine's cron service.
func InstallHandlers(r *router.Router, mwBase router.MiddlewareChain) {
	mwCron := mwBase.Extend(gaemiddleware.RequireCron)

	// Import config.cfg from LUCI Config.
	r.GET("/internal/cron/import-service-config", mwCron, logAndSetHTTPErr(importServiceConfig))

	// Generate repair jobs for needs_repair CrOS DUTs.
	r.GET("/internal/cron/push-bots-for-admin-tasks", mwCron, logAndSetHTTPErr(pushBotsForAdminTasksHandler([]fleet.DutState{fleet.DutState_NeedsRepair}, fleet.PushBotsForAdminTasksRequest_DefaultSchedulingStrategy)))

	// Generate repair jobs for repair_failed CrOS DUTs.
	r.GET("/internal/cron/push-repair-failed-bots-for-admin-tasks-hourly", mwCron, logAndSetHTTPErr(pushBotsForAdminTasksHandler([]fleet.DutState{fleet.DutState_RepairFailed}, fleet.PushBotsForAdminTasksRequest_DefaultSchedulingStrategy)))

	// Generate repair jobs for needs_manual_repair CrOS DUTs.
	r.GET("/internal/cron/push-repair-failed-bots-for-admin-tasks-daily", mwCron, logAndSetHTTPErr(pushBotsForAdminTasksHandler([]fleet.DutState{fleet.DutState_NeedsManualRepair}, fleet.PushBotsForAdminTasksRequest_DefaultSchedulingStrategy)))

	// for repair jobs of labstation.
	r.GET("/internal/cron/push-repair-jobs-for-labstations", mwCron, logAndSetHTTPErr(pushRepairJobsForLabstationsCronHandler))

	// Generate audit-usbkey jobs for CrOS DUTs.
	r.GET("/internal/cron/push-admin-audit-usbkey-tasks-for-duts", mwCron, logAndSetHTTPErr(pushAdminAuditActionHandler(fleet.AuditTask_ServoUSBKey)))

	// Generate audit-storage jobs for CrOS DUTs.
	r.GET("/internal/cron/push-admin-audit-storage-tasks-for-duts", mwCron, logAndSetHTTPErr(pushAdminAuditActionHandler(fleet.AuditTask_DUTStorage)))

	// Generate audit-rpm jobs for CrOS DUTs.
	r.GET("/internal/cron/push-admin-audit-rpm-tasks-for-duts", mwCron, logAndSetHTTPErr(pushAdminAuditActionHandler(fleet.AuditTask_RPMConfig)))

	// Report Bot metrics.
	r.GET("/internal/cron/report-bots", mwCron, logAndSetHTTPErr(reportBotsCronHandler))

	// dump information from stable version file to datastore
	r.GET("/internal/cron/dump-stable-version-to-datastore", mwCron, logAndSetHTTPErr(dumpStableVersionToDatastoreHandler))
}

func importServiceConfig(c *router.Context) error {
	return config.Import(c.Request.Context())
}

// pushBotsForAdminTasksHandler high-order for pushBotsForAdminTasksCronHandler.
func pushBotsForAdminTasksHandler(dutStates []fleet.DutState, strategy fleet.PushBotsForAdminTasksRequest_SchedulingStrategy) (err func(c *router.Context) error) {
	return func(c *router.Context) error {
		return pushBotsForAdminTasksCronHandler(c, dutStates, strategy)
	}
}

// pushBotsForAdminTasksCronHandler pushes bots that require admin tasks to bot queue.
func pushBotsForAdminTasksCronHandler(c *router.Context, dutStates []fleet.DutState, strategy fleet.PushBotsForAdminTasksRequest_SchedulingStrategy) (err error) {
	defer func() {
		pushBotsForAdminTasksCronHandlerTick.Add(c.Request.Context(), 1, err == nil)
	}()

	cfg := config.Get(c.Request.Context())

	if cfg.GetTasker().GetEnableSmartScheduling() {
		logging.Infof(c.Request.Context(), "smart scheduling enabled by config")
	} else {
		logging.Infof(c.Request.Context(), "smart scheduling disabled by config")
		strategy = fleet.PushBotsForAdminTasksRequest_DefaultSchedulingStrategy
	}

	if cfg.RpcControl != nil && cfg.RpcControl.GetDisablePushBotsForAdminTasks() {
		logging.Infof(c.Request.Context(), "PushBotsForAdminTasks is disabled via config.")
		return nil
	}
	dutStateNames := make([]string, len(dutStates))
	for ind, dutState := range dutStates {
		dutStateNames[ind] = dutState.String()
	}
	logging.Infof(c.Request.Context(), fmt.Sprintf("PushBotsForAdminTasks strategy %q for states: %v", strategy.String(), strings.Join(dutStateNames, ", ")))

	tsi := frontend.TrackerServerImpl{}

	for _, dutState := range dutStates {
		logging.Infof(c.Request.Context(), fmt.Sprintf("Started push AdminTasks for state %#v", dutState.String()))
		if _, err := tsi.PushBotsForAdminTasks(c.Request.Context(), &fleet.PushBotsForAdminTasksRequest{
			TargetDutState:     dutState,
			SchedulingStrategy: strategy,
		}); err != nil {
			return err
		}
		logging.Infof(c.Request.Context(), fmt.Sprintf("Finished push AdminTasks for state %#v", dutState.String()))
	}
	logging.Infof(c.Request.Context(), "Successfully finished")
	return nil
}

// pushLabstationsForRepairCronHandler pushes bots that require admin tasks to bot queue.
func pushRepairJobsForLabstationsCronHandler(c *router.Context) (err error) {
	defer func() {
		pushRepairJobsForLabstationsCronHandlerTick.Add(c.Request.Context(), 1, err == nil)
	}()

	cfg := config.Get(c.Request.Context())
	if cfg.RpcControl != nil && cfg.RpcControl.GetDisablePushLabstationsForRepair() {
		logging.Infof(c.Request.Context(), "PushLabstationsForRepair is disabled via config.")
		return nil
	}

	tsi := frontend.TrackerServerImpl{}
	if _, err := tsi.PushRepairJobsForLabstations(c.Request.Context(), &fleet.PushRepairJobsForLabstationsRequest{}); err != nil {
		return err
	}
	logging.Infof(c.Request.Context(), "Successfully finished")
	return nil
}

// pushAdminAuditActionHandler high-order for pushAdminAuditJobsCronHandler.
func pushAdminAuditActionHandler(auditTask fleet.AuditTask) (err func(c *router.Context) error) {
	return func(c *router.Context) error {
		return pushAdminAuditJobsCronHandler(c, auditTask)
	}
}

// pushAdminAuditJobsCronHandler pushes bots that will run audit tasks to bot queue.
func pushAdminAuditJobsCronHandler(c *router.Context, auditTask fleet.AuditTask) (err error) {
	defer func() {
		pushAdminAuditTasksCronHandlerTick.Add(c.Request.Context(), 1, err == nil)
	}()

	cfg := config.Get(c.Request.Context())
	if cfg.RpcControl != nil && cfg.RpcControl.GetDisablePushDutsForAdminAudit() {
		logging.Infof(c.Request.Context(), "PushDutsForAdminAudit is disabled via config.")
		return nil
	}

	tsi := frontend.TrackerServerImpl{}
	if _, err := tsi.PushBotsForAdminAuditTasks(c.Request.Context(), &fleet.PushBotsForAdminAuditTasksRequest{
		Task: auditTask,
	}); err != nil {
		return err
	}
	logging.Infof(c.Request.Context(), "Successfully finished")
	return nil
}

func reportBotsCronHandler(c *router.Context) (err error) {
	defer func() {
		reportBotsCronHandlerTick.Add(c.Request.Context(), 1, err == nil)
	}()

	tsi := frontend.TrackerServerImpl{}
	if _, err := tsi.ReportBots(c.Request.Context(), &fleet.ReportBotsRequest{}); err != nil {
		return err
	}
	logging.Infof(c.Request.Context(), "Successfully report bot metrics")
	return nil
}

func logAndSetHTTPErr(f func(c *router.Context) error) func(*router.Context) {
	return func(c *router.Context) {
		if err := f(c); err != nil {
			http.Error(c.Writer, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func dumpStableVersionToDatastoreHandler(c *router.Context) error {
	cfg := config.Get(c.Request.Context())
	if cfg.RpcControl != nil && cfg.RpcControl.GetDisableDumpStableVersionToDatastore() {
		logging.Infof(c.Request.Context(), "Dump StableVersion aborted as it is disabled!")
		return nil
	}
	inv := &frontend.ServerImpl{}
	req := &fleet.DumpStableVersionToDatastoreRequest{}
	if _, err := inv.DumpStableVersionToDatastore(c.Request.Context(), req); err != nil {
		logging.Infof(c.Request.Context(), "Dump StableVersion failed with error: %s", err)
		return err
	}
	logging.Infof(c.Request.Context(), "Dump StableVersion finished successfully!")
	return nil
}
