// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cron implements handlers for appengine cron targets in this app.
package cron

import (
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/luci/appengine/gaemiddleware"
	authclient "go.chromium.org/luci/auth"
	gitilesapi "go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/info"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/router"

	"infra/appengine/cros/lab_inventory/app/config"
	bqlib "infra/cros/lab_inventory/bq"
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

	r.GET("/internal/cron/dump-to-bq", mwCron, logAndSetHTTPErr(dumpToBQCronHandler))

	r.GET("/internal/cron/dump-other-configs-snapshot", mwCron, logAndSetHTTPErr(dumpOtherConfigsCronHandler))

	r.GET("/internal/cron/sync-dev-config", mwCron, logAndSetHTTPErr(syncDevConfigHandler))
}

const pageSize = 500

func importServiceConfig(c *router.Context) error {
	return config.Import(c.Request.Context())
}

func dumpToBQCronHandler(c *router.Context) (err error) {
	logging.Infof(c.Request.Context(), "not implemented yet")
	return nil
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

func dumpOtherConfigsCronHandler(c *router.Context) error {
	ctx := c.Request.Context()
	logging.Infof(ctx, "Start to dump related configs in inventory to bigquery")

	curTime := time.Now()
	curTimeStr := bqlib.GetPSTTimeStamp(curTime)
	client, err := bigquery.NewClient(ctx, info.AppID(ctx))
	if err != nil {
		return err
	}

	uploader := bqlib.InitBQUploaderWithClient(ctx, client, "inventory", fmt.Sprintf("deviceconfig$%s", curTimeStr))
	msgs := bqlib.GetDeviceConfigProtos(ctx)
	logging.Debugf(ctx, "Dumping %d records of device configs to bigquery", len(msgs))
	if err := uploader.Put(ctx, msgs...); err != nil {
		return err
	}

	logging.Debugf(ctx, "Dump is successfully finished")
	return nil
}

func logAndSetHTTPErr(f func(c *router.Context) error) func(*router.Context) {
	return func(c *router.Context) {
		if err := f(c); err != nil {
			logging.Errorf(c.Request.Context(), err.Error())
			http.Error(c.Writer, "Internal server error", http.StatusInternalServerError)
		}
	}
}
