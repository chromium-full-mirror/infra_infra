// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scopes

import (
	"context"

	"go.chromium.org/infra/cros/recovery/internal/log"
)

// WithConfigScope inits map to create scope for a configuration variables.
func WithConfigScope(ctx context.Context) context.Context {
	if getConfigMap(ctx) != nil {
		return ctx
	}
	newMap := map[string]interface{}{}
	return context.WithValue(ctx, ctxConfigurationScopeKey, newMap)
}

// ReadConfigParam read and returns value from config scope.
//
// Also provide a bool if key is found or not.
func ReadConfigParam(ctx context.Context, key string) (val any, ok bool) {
	m := getConfigMap(ctx)
	if m == nil {
		return nil, false
	}
	param, ok := m[key]
	return param, ok
}

// PutConfigParam sets value to config scope to be available in the config context.
func PutConfigParam(ctx context.Context, key string, val any) {
	m := getConfigMap(ctx)
	if m == nil {
		log.Debugf(ctx, "Config scope is not initilized! fail to put value for key %q", key)
	} else {
		m[key] = val
	}
}

func getConfigMap(ctx context.Context) ParamsMap {
	if params, ok := ctx.Value(ctxConfigurationScopeKey).(ParamsMap); ok {
		return params
	}
	return nil
}
