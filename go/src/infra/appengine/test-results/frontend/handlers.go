// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Program frontend implements the App Engine based HTTP server
// behind test-results.appspot.com.
package main

import (
	"context"
	"html/template"
	"net/http"
	"os"
	"time"

	"go.chromium.org/luci/appengine/gaemiddleware/standard"
	"go.chromium.org/luci/gae/service/info"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/templates"
)

func init() {
	r := router.New()

	baseMW := standard.Base()
	frontendMW := baseMW.Extend(timeoutMiddleware(2 * time.Minute))
	getMW := frontendMW.Extend(templatesMiddleware())

	standard.InstallHandlers(r)

	// Endpoints used by end users.
	r.GET("/", getMW, polymerHandler)
	r.GET("/home", getMW, polymerHandler)
	r.GET("/revision_range", frontendMW, revisionHandler)

	http.DefaultServeMux.Handle("/", r)
}

func timeoutMiddleware(timeoutMs time.Duration) func(*router.Context, router.Handler) {
	return func(c *router.Context, next router.Handler) {
		newCtx, cancelFunc := context.WithTimeout(c.Request.Context(), timeoutMs)
		defer cancelFunc()
		c.Request = c.Request.WithContext(newCtx)
		next(c)
	}
}

// paramsTimeFormat is the time format string in incoming GET
// /testfile requests.
const paramsTimeFormat = "2006-01-02T15:04:05Z" // RFC3339, but enforce Z for timezone.

// templatesMiddleware returns the templates middleware.
func templatesMiddleware() router.Middleware {
	return templates.WithTemplates(&templates.Bundle{
		Loader:    templates.FileSystemLoader(os.DirFS("templates")),
		DebugMode: info.IsDevAppServer,
		FuncMap: template.FuncMap{
			"timeParams": func(t time.Time) string {
				return t.Format(paramsTimeFormat)
			},
			"timeJS": func(t time.Time) int64 {
				return t.Unix() * 1000
			},
		},
	})
}
