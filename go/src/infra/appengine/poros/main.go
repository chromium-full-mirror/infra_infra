// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"os"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/server/cfgmodule"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/encryptedcookies"
	_ "go.chromium.org/luci/server/encryptedcookies/session/datastore"
	"go.chromium.org/luci/server/gaeemulation"
	"go.chromium.org/luci/server/mailer"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/secrets"
	"go.chromium.org/luci/server/templates"
	"go.chromium.org/luci/server/tq"

	"go.chromium.org/infra/appengine/poros/api/handlers"
	"go.chromium.org/infra/appengine/poros/api/proto"
	"go.chromium.org/infra/appengine/poros/api/service"
	"go.chromium.org/infra/appengine/poros/taskspb"
)

// authGroup is the name of the LUCI Auth group that controls whether the user
// should have access to Poros.
const authGroup = "project-poros-access"

// prepareTemplates configures templates.Bundle used by all UI handlers.
func prepareTemplates(opts *server.Options) *templates.Bundle {
	return &templates.Bundle{
		Loader: templates.FileSystemLoader(os.DirFS("templates")),
		// Controls whether templates are cached.
		DebugMode: func(context.Context) bool { return !opts.Prod },
		DefaultArgs: func(ctx context.Context, e *templates.Extra) (templates.Args, error) {
			logoutURL, err := auth.LogoutURL(ctx, e.Request.URL.RequestURI())
			if err != nil {
				return nil, err
			}

			return templates.Args{
				"AuthGroup":       authGroup,
				"AuthServiceHost": opts.AuthServiceHost,
				"User":            auth.CurrentUser(ctx).Email,
				"LogoutURL":       logoutURL,
			}, nil
		},
	}
}

// requireAuth is middleware that forces the user to login and checks the
// user is authorised to use Weetbix before handling any request.
// If the user is not authorised, a standard "access is denied" page is
// displayed that allows the user to logout and login again with new
// credentials.
func requireAuth(ctx *router.Context, next router.Handler) {
	user := auth.CurrentIdentity(ctx.Request.Context())
	if user.Kind() == identity.Anonymous {
		// User is not logged in.
		url, err := auth.LoginURL(ctx.Request.Context(), ctx.Request.URL.RequestURI())
		if err != nil {
			logging.Errorf(ctx.Request.Context(), "Fetching LoginURL: %s", err.Error())
			http.Error(ctx.Writer, "Internal server error while fetching Login URL.", http.StatusInternalServerError)
		} else {
			http.Redirect(ctx.Writer, ctx.Request, url, http.StatusFound)
		}
		return
	}

	isAuthorized, err := auth.IsMember(ctx.Request.Context(), authGroup)
	switch {
	case err != nil:
		logging.Errorf(ctx.Request.Context(), "Checking Auth Membership: %s", err.Error())
		http.Error(ctx.Writer, "Internal server error while checking authorization.", http.StatusInternalServerError)
	case !isAuthorized:
		ctx.Writer.WriteHeader(http.StatusForbidden)
		templates.MustRender(ctx.Request.Context(), ctx.Writer, "pages/access-denied.html", nil)
	default:
		next(ctx)
	}
}

func pageBase(srv *server.Server) router.MiddlewareChain {
	return router.NewMiddlewareChain(
		auth.Authenticate(srv.CookieAuth),
		templates.WithTemplates(prepareTemplates(&srv.Options)),
		requireAuth,
	)
}

func init() {
	// RegisterTaskClass tells the TQ module how to serialize, route and execute
	// a task of a particular proto type (*taskspb.CreateAssetTask in this case).
	//
	// It can be called any time before the serving loop (e.g. in an init,
	// in main, in server.Main callback, etc).
	tq.RegisterTaskClass(tq.TaskClass{
		// This is a stable ID that identifies this particular kind of tasks.
		// Changing it will essentially "break" all inflight tasks.
		ID: "spin-enterprise-resources-task",
		// This is used for deserialization and also for discovery of what ID to use
		// when submitting tasks. Changing it is safe as long as the JSONPB
		// representation of in-flight tasks still matches the new proto.
		Prototype: (*taskspb.AssetAdditionOrDeletionTask)(nil),
		// This controls how AddTask calls behave with respect to transactions.
		// FollowsContext means "enqueue transactionally if the context is
		// transactional, or non-transactionally otherwise". Other possibilities are
		// Transactional (always require a transaction) and NonTransactional
		// (fail if called from a transaction).
		Kind: tq.FollowsContext,
		// What Cloud Tasks queue to use for these tasks. See queue.yaml.
		Queue: "spin-enterprise-resources-task",
		// Handler will be called to handle a previously submitted task. It can also
		// be attached later (perhaps even in from a different package) via
		// AttachHandler.
		Handler: taskspb.CreateAssetHandler,
	})
}

func main() {
	modules := []module.Module{
		cfgmodule.NewModuleFromFlags(),
		encryptedcookies.NewModuleFromFlags(), // Required for auth sessions.
		gaeemulation.NewModuleFromFlags(),     // Needed by cfgmodule.
		secrets.NewModuleFromFlags(),          // Needed by encryptedcookies.
		tq.NewModuleFromFlags(),               // transactionally submit Cloud Tasks
		mailer.NewModuleFromFlags(),           // Needed for sending emails
	}

	server.Main(nil, modules, func(srv *server.Server) error {
		mw := pageBase(srv)

		handler := handlers.NewHandlers(srv.Options.Prod)
		handler.RegisterRoutes(srv.Routes, mw)

		srv.Routes.Static("/static/", mw, http.Dir("./static"))

		// Register pPRC servers.
		srv.ConfigurePRPC(func(p *prpc.Server) {
			p.AccessControl = prpc.AllowOriginAll
			// TODO(crbug/1082369): Remove this workaround once field masks can be decoded.
			p.EnableNonStandardFieldMasks = true
		})
		proto.RegisterAssetServer(srv, &service.AssetHandler{})
		proto.RegisterResourceServer(srv, &service.ResourceHandler{})
		proto.RegisterAssetResourceServer(srv, &service.AssetResourceHandler{})
		proto.RegisterAssetInstanceServer(srv, &service.AssetInstanceHandler{})

		return nil
	})
}
