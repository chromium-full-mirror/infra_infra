// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"net/http"

	"go.chromium.org/luci/appengine/gaemiddleware/standard"
	"go.chromium.org/luci/grpc/discovery"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common"
)

func init() {
	r := router.New()
	base := common.MiddlewareForUI()
	baseInternal := common.MiddlewareForInternal()

	// LUCI frameworks needs a bunch of routes exposed via default module.
	standard.InstallHandlers(r)

	// This is the analyze queue handler.
	r.POST("/internal/analyze", baseInternal, analyzeHandler)

	r.GET("/", base, mainPageHandler)
	r.GET("/run/*runID", base, mainPageHandler)
	r.GET("/feedback/*runID", base, mainPageHandler)

	// Configure pRPC server.
	s := common.NewRPCServer()
	tricium.RegisterTriciumServer(s, server)
	discovery.Enable(s)
	s.InstallHandlers(r, common.MiddlewareForRPC())

	http.DefaultServeMux.Handle("/", r)
}
