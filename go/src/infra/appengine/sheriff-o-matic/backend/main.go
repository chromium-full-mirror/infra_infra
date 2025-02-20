// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements HTTP server that handles requests to the backend analyzer module.
package main

import (
	"net/http"

	"google.golang.org/appengine"

	"go.chromium.org/luci/appengine/gaeauth/server"
	"go.chromium.org/luci/appengine/gaemiddleware/standard"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/analyzer"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/handler"
)

// base is the root of the middleware chain.
func base() router.MiddlewareChain {
	a := auth.Authenticator{
		Methods: []auth.Method{
			&server.OAuth2Method{Scopes: []string{server.EmailScope}},
			&server.InboundAppIDAuthMethod{},
			server.CookieAuth,
		},
	}
	return standard.Base().Extend(a.GetMiddleware()).Extend(withServiceClients)
}

func withServiceClients(ctx *router.Context, next router.Handler) {
	a := analyzer.CreateAnalyzer(ctx.Request.Context())
	ctx.Request = ctx.Request.WithContext(handler.WithAnalyzer(ctx.Request.Context(), a))
	next(ctx)
}

// // Routes.
func init() {
	r := router.New()
	basemw := base()
	standard.InstallHandlers(r)

	r.GET("/_cron/analyze/:tree", basemw, handler.GetAnalyzeHandler)
	r.GET("/_cron/bq_query/:project", basemw, handler.GetBQQueryHandler)

	http.DefaultServeMux.Handle("/", r)
}

func main() {
	appengine.Main()
}
