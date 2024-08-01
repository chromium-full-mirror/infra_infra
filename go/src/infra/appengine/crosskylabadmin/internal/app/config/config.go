// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"net/http"
	"strings"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/config/server/cfgcache"
	"go.chromium.org/luci/server/router"

	"infra/libs/skylab/common/heuristics"
	ufsUtil "infra/unifiedfleet/app/util"
)

// unique type to prevent assignment.
type contextKeyType struct{}

// unique key used to store and retrieve context.
var contextKey = contextKeyType{}

// defines how to fetch and cache the config.
var cachedCfg = cfgcache.Register(&cfgcache.Entry{
	Path: "config.cfg",
	Type: (*Config)(nil),
})

// Import fetches the most recent config and stores it in the datastore.
//
// Must be called periodically to make sure Get and Middleware use the freshest
// config.
func Import(c context.Context) error {
	_, err := cachedCfg.Update(c, nil)
	return err
}

// Get returns the config in c, or panics.
// See also Use and Middleware.
func Get(c context.Context) *Config {
	return c.Value(contextKey).(*Config)
}

// Middleware loads the service config and installs it into the context.
func Middleware(c *router.Context, next router.Handler) {
	msg, err := cachedCfg.Get(c.Request.Context(), nil)
	if err != nil {
		logging.WithError(err).Errorf(c.Request.Context(), "could not load application config")
		http.Error(c.Writer, "Internal server error", http.StatusInternalServerError)
	} else {
		c.Request = c.Request.WithContext(Use(c.Request.Context(), msg.(*Config)))
		next(c)
	}
}

// Use installs cfg into c.
func Use(c context.Context, cfg *Config) context.Context {
	return context.WithValue(c, contextKey, cfg)
}

// BotIDToDUTName converts botID to DUT-name populated on swarming.
func (x *Swarming_PoolCfg) BotIDToDUTName(botId string) string {
	if x != nil && x.BotPrefix != "" {
		return strings.TrimPrefix(botId, x.BotPrefix)
	}
	return heuristics.NormalizeBotNameToDeviceName(botId)
}

// UFSCtxNamespace provides namespace value to connect with UFS for device in particular swarming-pool.
func (x *Swarming_PoolCfg) UFSCtxNamespace() string {
	if x != nil && x.GetUfsNamespace() != "" {
		return x.GetUfsNamespace()
	}
	return ufsUtil.OSNamespace
}
