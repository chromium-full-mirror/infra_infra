// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cron implements handlers for appengine cron targets in this app.
package cron

import (
	"net/http"

	"go.chromium.org/luci/appengine/gaemiddleware"
	authclient "go.chromium.org/luci/auth"
	gitilesapi "go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/router"

	"infra/appengine/cros/lab_inventory/app/config"
	"infra/cros/lab_inventory/cfg2datastore"
	"infra/cros/lab_inventory/deviceconfig"
	"infra/libs/git"
)

// InstallHandlers installs handlers for cron jobs that are part of this app.
//
// All handlers serve paths under /internal/cron/*
// These handlers can only be called by appengine's cron service.
func InstallHandlers(r *router.Router, mwBase router.MiddlewareChain) {
	mwCron := mwBase.Extend(gaemiddleware.RequireCron, config.Middleware)

	r.GET("/internal/cron/import-service-config", mwCron, logAndSetHTTPErr(importServiceConfig))

	r.GET("/internal/cron/sync-dev-config", mwCron, logAndSetHTTPErr(syncDevConfigHandler))
}

const pageSize = 500

func importServiceConfig(c *router.Context) error {
	return config.Import(c.Request.Context())
}

func syncDevConfigHandler(c *router.Context) error {
	logging.Infof(c.Request.Context(), "Start syncing device_config repo")
	cfg := config.Get(c.Request.Context())
	dCcfg := cfg.GetDeviceConfigSource()
	cli, err := cfg2datastore.NewGitilesClient(c.Request.Context(), dCcfg.GetHost())
	if err != nil {
		return err
	}
	if cfg.GetProjectConfigSource().GetEnableProjectConfig() {
		t, err := auth.GetRPCTransport(c.Request.Context(), auth.AsSelf, auth.WithScopes(authclient.OAuthScopeEmail, gitilesapi.OAuthScope))
		if err != nil {
			return err
		}
		bsCfg := cfg.GetProjectConfigSource()
		logging.Infof(c.Request.Context(), "boxster configs: %q, %q, %q", bsCfg.GetGitilesHost(), bsCfg.GetProject(), bsCfg.GetBranch())
		gitClient, err := git.NewClient(c.Request.Context(), &http.Client{Transport: t}, "", bsCfg.GetGitilesHost(), bsCfg.GetProject(), bsCfg.GetBranch())
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}
		return deviceconfig.UpdateDatastoreFromBoxster(c.Request.Context(), gitClient, bsCfg.GetJoinedConfigPath(), cli, dCcfg.GetProject(), dCcfg.GetCommittish(), dCcfg.GetPath())
	}
	return deviceconfig.UpdateDatastore(c.Request.Context(), cli, dCcfg.GetProject(), dCcfg.GetCommittish(), dCcfg.GetPath())
}

func logAndSetHTTPErr(f func(c *router.Context) error) func(*router.Context) {
	return func(c *router.Context) {
		if err := f(c); err != nil {
			logging.Errorf(c.Request.Context(), err.Error())
			http.Error(c.Writer, "Internal server error", http.StatusInternalServerError)
		}
	}
}
