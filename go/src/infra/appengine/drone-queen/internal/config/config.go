// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package config contains the service configuration protos.
package config

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/server/cfgcache"
	"go.chromium.org/luci/server/cron"
)

//go:generate cproto

type key struct{}

// Defines how to fetch and cache the config.
var cachedCfg = cfgcache.Register(&cfgcache.Entry{
	Path: "config.cfg",
	Type: (*Config)(nil),
})

// Import fetches the most recent config and stores it in the datastore.
//
// Must be called periodically to make sure Get and Middleware use the freshest
// config.
func Import(ctx context.Context) error {
	_, err := cachedCfg.Update(ctx, nil)
	return err
}

// Get gets the config in the context.  If the context does not have a
// config, return a nil config.
//
// See also Use and Middleware.
func Get(ctx context.Context) *Config {
	switch v := ctx.Value(key{}); v := v.(type) {
	case *Config:
		return v
	case nil:
		return nil
	default:
		panic(v)
	}
}

// Use installs the config into ctx.
func Use(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, key{}, c)
}

// UnaryConfig is a gRPC interceptor for adding LUCI config to the
// request context.
func UnaryConfig(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	// TODO(ayatane): Move this to the middleware package, if that package still exists.
	msg, err := cachedCfg.Get(ctx, nil)
	if err != nil {
		logging.WithError(err).Errorf(ctx, "could not load application config")
		return nil, status.Errorf(codes.Unavailable, "load config: %s", err)
	} else {
		ctx = Use(ctx, msg.(*Config))
		return handler(ctx, req)
	}
}

// CronConfig is a wrapper to add LUCI config to the request context
// for cron handlers
func CronConfig(h cron.Handler) cron.Handler {
	// TODO(ayatane): Move this to the middleware package, if that package still exists.
	return func(ctx context.Context) error {
		msg, err := cachedCfg.Get(ctx, nil)
		if err != nil {
			logging.WithError(err).Errorf(ctx, "could not load application config")
			return status.Errorf(codes.Unavailable, "load config: %s", err)
		} else {
			ctx = Use(ctx, msg.(*Config))
			return h(ctx)
		}
	}
}

// Instance returns the configured instance of the service.
func Instance(ctx context.Context) string {
	n := Get(ctx).GetInstance()
	if n == "" {
		return "unknown"
	}
	return n
}

// AssignmentDuration returns the configured drone assignment duration.
func AssignmentDuration(ctx context.Context) time.Duration {
	pd := Get(ctx).GetAssignmentDuration()
	if pd == nil {
		const defaultDuration = 10 * time.Minute
		return defaultDuration
	}
	gd, err := ptypes.Duration(pd)
	if err != nil {
		panic(err)
	}
	return gd
}
