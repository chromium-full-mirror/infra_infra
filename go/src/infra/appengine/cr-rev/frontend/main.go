// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Frontend service handles home page, API and redirects. Static files are
// served by GAE directly (configured in app.yaml).
package main

import (
	"net/http"
	"os"
	"strings"

	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/templates"

	"infra/appengine/cr-rev/frontend/api/v1"
	"infra/appengine/cr-rev/frontend/redirect"
)

const templatePath = "templates"

// handleIndex serves homepage of cr-rev
func handleIndex(c *router.Context) {
	templates.MustRender(
		c.Request.Context(), c.Writer, "pages/index.html", templates.Args{})
}

// Assembles the URL to a CL on the Gerrit instance with given base URL. If no
// CL is given in the path, returns the URL to the Gerrit instance's dashboard.
func getGerritUrl(base string, path string) string {
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return base + "dashboard/self"
	}
	return base + "c/" + path
}

// handlePublicGerritRedirect redirects user to a CL on chromium-review
func handlePublicGerritRedirect(c *router.Context) {
	path := c.Params.ByName("path")
	url := getGerritUrl("https://chromium-review.googlesource.com/", path)
	http.Redirect(
		c.Writer, c.Request, url, http.StatusPermanentRedirect)
}

// handleInternalGerritRedirect redirects user to a CL on
// chrome-internal-review.
func handleInternalGerritRedirect(c *router.Context) {
	path := c.Params.ByName("path")
	url := getGerritUrl("https://chrome-internal-review.googlesource.com/", path)
	http.Redirect(
		c.Writer, c.Request, url, http.StatusPermanentRedirect)
}

// handleRedirect redirects user base on redirect rules. This is a catch-all
// redirect handler (e.g. crrev.com/3, crrev.com/{commit hash}). To add more
// rules, look at redirect package.
func handleRedirect(redirectRules *redirect.Rules, c *router.Context) {
	url, _, err := redirectRules.FindRedirectURL(c.Request.Context(), c.Request.RequestURI)
	switch err {
	case nil:
		http.Redirect(
			c.Writer, c.Request, url, http.StatusPermanentRedirect)
	case redirect.ErrNoMatch:
		http.NotFound(c.Writer, c.Request)
	default:
		http.Error(
			c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	mw := router.MiddlewareChain{}
	mw = mw.Extend(templates.WithTemplates(&templates.Bundle{
		Loader:          templates.FileSystemLoader(os.DirFS(templatePath)),
		DefaultTemplate: "base",
	}))

	modules := []module.Module{
		cfgmodule.NewModuleFromFlags(),
		gaeemulation.NewModuleFromFlags(),
	}

	server.Main(nil, modules, func(srv *server.Server) error {
		redirect := redirect.NewRules(redirect.NewGitilesRedirect())

		srv.Routes.Handle("GET", "/i/*path", mw, handleInternalGerritRedirect)
		srv.Routes.Handle("GET", "/c/*path", mw, handlePublicGerritRedirect)
		srv.Routes.GET("/", mw, handleIndex)

		server := api.NewServer(redirect)

		// Host pRPC servers.
		srv.ConfigurePRPC(func(s *prpc.Server) {
			// CORS requests are fine because
			// 1. we don't use any cookie/TLS based authentication for pRPC endpoints,
			//    and
			// 2. none of the pRPC endpoints trigger mutation anyway.
			s.AccessControl = prpc.AllowOriginAll
			// TODO(crbug/1082369): Remove this workaround once field masks can be decoded.
			s.HackFixFieldMasksForJSON = true
		})
		api.RegisterCrrevServer(srv, server)

		// Host HTTP servers.
		apiV1 := srv.Routes.Subrouter("/_ah/api/crrev/v1")
		api.NewRESTServer(apiV1, server)

		// NotFound is used as catch-all.
		srv.Routes.NotFound(mw, func(c *router.Context) {
			handleRedirect(redirect, c)
		})
		return nil
	})
}
