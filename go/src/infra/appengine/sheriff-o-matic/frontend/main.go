// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/xsrf"
	"go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/encryptedcookies"
	_ "go.chromium.org/luci/server/encryptedcookies/session/datastore"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/secrets"
	_ "go.chromium.org/luci/server/tq/txn/datastore"

	sompb "go.chromium.org/infra/appengine/sheriff-o-matic/proto/v1"
	"go.chromium.org/infra/appengine/sheriff-o-matic/rpc"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/handler"
)

const (
	authGroup             = "sheriff-o-matic-access"
	settingsKey           = "tree"
	productionAnalyticsID = "UA-55762617-1"
	stagingAnalyticsID    = "UA-55762617-22"
	prodAppID             = "sheriff-o-matic"
)

var (
	mainPage         = template.Must(template.ParseFiles("./index.html"))
	accessDeniedPage = template.Must(template.ParseFiles("./access-denied.html"))
)

var errStatus = func(c context.Context, w http.ResponseWriter, status int, msg string) {
	logging.Errorf(c, "Status %d msg %s", status, msg)
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

type SOMHandlers struct {
	// IsStaging is true if this is either a local dev server or on the staging GAE server
	IsStaging bool
	// IsDevAppServer is true if this is running locally instead of on GAE
	IsDevAppServer bool
	// CloudProject is the cloud project this is running as
	CloudProject string
}

func (s *SOMHandlers) indexPage(ctx *router.Context) {
	c, w, r, p := ctx.Request.Context(), ctx.Writer, ctx.Request, ctx.Request.URL.Path
	if p == "/" {
		http.Redirect(w, r, "/chromium", http.StatusFound)
		return
	}

	user := auth.CurrentIdentity(c)

	if user.Kind() == identity.Anonymous {
		url, err := auth.LoginURL(c, p)
		if err != nil {
			errStatus(c, w, http.StatusInternalServerError, fmt.Sprintf(
				"You must login. Additionally, an error was encountered while serving this request: %s", err.Error()))
		} else {
			http.Redirect(w, r, url, http.StatusFound)
		}

		return
	}

	isGoogler, err := auth.IsMember(c, authGroup)

	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	logoutURL, err := auth.LogoutURL(c, "/")

	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	if !isGoogler {
		err = accessDeniedPage.Execute(w, map[string]interface{}{
			"Group":     authGroup,
			"LogoutURL": logoutURL,
		})
		if err != nil {
			logging.Errorf(c, "while rendering index: %s", err)
		}
		return
	}

	tok, err := xsrf.Token(c)
	if err != nil {
		logging.Errorf(c, "while getting xsrf token: %s", err)
	}

	AnalyticsID := stagingAnalyticsID
	if !s.IsStaging {
		logging.Debugf(c, "Using production GA ID for app %s", s.CloudProject)
		AnalyticsID = productionAnalyticsID
	}

	trees, err := handler.GetTrees(c)
	if err != nil {
		logging.Errorf(c, "while getting trees: %s", err)
	}

	data := map[string]interface{}{
		"User":           user.Email(),
		"LogoutUrl":      logoutURL,
		"IsDevAppServer": s.IsDevAppServer,
		"IsStaging":      s.IsStaging,
		"XsrfToken":      tok,
		"AnalyticsID":    AnalyticsID,
		"Trees":          string(trees),
	}

	err = mainPage.Execute(w, data)
	if err != nil {
		logging.Errorf(c, "while rendering index: %s", err)
	}
}

func requireGoogler(c *router.Context, next router.Handler) {
	isGoogler, err := auth.IsMember(c.Request.Context(), authGroup)
	switch {
	case err != nil:
		errStatus(c.Request.Context(), c.Writer, http.StatusInternalServerError, err.Error())
	case !isGoogler:
		errStatus(c.Request.Context(), c.Writer, http.StatusForbidden, "Access denied")
	default:
		next(c)
	}
}

func noopHandler(ctx *router.Context) {}

func getXSRFToken(ctx *router.Context) {
	c, w := ctx.Request.Context(), ctx.Writer

	tok, err := xsrf.Token(c)
	if err != nil {
		logging.Errorf(c, "while getting xsrf token: %s", err)
	}

	data := map[string]string{
		"token": tok,
	}
	txt, err := json.Marshal(data)
	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(txt)
}

func newAnnotationHandler() *handler.AnnotationHandler {
	return &handler.AnnotationHandler{}
}

func (s *SOMHandlers) getAnnotationsHandler(ctx *router.Context) {
	ah := newAnnotationHandler()
	activeKeys := map[string]interface{}{}
	activeAlerts := handler.GetAlertsCommonHandler(ctx, true, false)
	for _, alrt := range activeAlerts.Alerts {
		activeKeys[alrt.Key] = nil
	}
	ah.GetAnnotationsHandler(ctx, activeKeys)
}

func (s *SOMHandlers) postAnnotationsHandler(ctx *router.Context) {
	ah := newAnnotationHandler()
	ah.PostAnnotationsHandler(ctx)
}

func main() {
	// Additional modules that extend the server functionality.
	modules := []module.Module{
		cron.NewModuleFromFlags(),
		encryptedcookies.NewModuleFromFlags(),
		gaeemulation.NewModuleFromFlags(),
		secrets.NewModuleFromFlags(),
	}

	server.Main(nil, modules, func(srv *server.Server) error {
		// When running locally, serve static files ourself.
		if !srv.Options.Prod {
			srv.Routes.Static("/bower_components", nil, http.Dir("./bower_components"))
			srv.Routes.Static("/images", nil, http.Dir("./images"))
			srv.Routes.Static("/elements", nil, http.Dir("./elements"))
			srv.Routes.Static("/scripts", nil, http.Dir("./scripts"))
			srv.Routes.Static("/test", nil, http.Dir("./test"))
		}

		somHandlers := &SOMHandlers{
			IsStaging:      !srv.Options.Prod || srv.Options.CloudProject != prodAppID,
			IsDevAppServer: !srv.Options.Prod,
			CloudProject:   srv.Options.CloudProject,
		}

		basemw := router.NewMiddlewareChain(
			auth.Authenticate(srv.CookieAuth),
		)
		protected := router.NewMiddlewareChain(
			auth.Authenticate(srv.CookieAuth),
			requireGoogler,
		)
		// Register pPRC servers.
		srv.ConfigurePRPC(func(s *prpc.Server) {
			s.AccessControl = prpc.AllowOriginAll
			// TODO(crbug/1082369): Remove this workaround once field masks can be decoded.
			s.EnableNonStandardFieldMasks = true
		})

		sompb.RegisterAlertsServer(srv, rpc.NewAlertsServer())

		srv.Routes.GET("/api/v1/alerts/:tree", protected, handler.GetAlertsHandler)
		srv.Routes.GET("/api/v1/unresolved/:tree", protected, handler.GetUnresolvedAlertsHandler)
		srv.Routes.GET("/api/v1/resolved/:tree", protected, handler.GetResolvedAlertsHandler)
		srv.Routes.GET("/api/v1/xsrf_token", protected, getXSRFToken)
		srv.Routes.GET("/api/v1/annotations/:tree", protected, somHandlers.getAnnotationsHandler)
		srv.Routes.POST("/api/v1/annotations/:tree/:action", protected, somHandlers.postAnnotationsHandler)
		srv.Routes.GET("/api/v1/revrange/:host/:repo", basemw, handler.GetRevRangeHandler)
		srv.Routes.GET("/api/v1/testexpectations", protected, handler.GetLayoutTestsHandler)
		srv.Routes.POST("/api/v1/testexpectation", protected, handler.PostLayoutTestExpectationChangeHandler)
		srv.Routes.GET("/logos/:tree", protected, handler.GetTreeLogoHandler)
		srv.Routes.GET("/_/autocomplete/:query", protected, handler.GetUserAutocompleteHandler)
		srv.Routes.POST("/_/clientmon", basemw, handler.PostClientMonHandler)
		// Non-public endpoints.
		cron.RegisterHandler("annotations_flush_old", handler.FlushOldAnnotationsHandler)
		cron.RegisterHandler("alerts_flush_old", handler.FlushOldAlertsHandler)
		// Ignore reqeuests from builder-alerts rather than 404.
		srv.Routes.GET("/alerts", nil, noopHandler)
		srv.Routes.POST("/alerts", nil, noopHandler)

		srv.Routes.NotFound(basemw, somHandlers.indexPage)

		return nil
	})
}
