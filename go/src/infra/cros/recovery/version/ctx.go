// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"context"

	"google.golang.org/grpc"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cros/recovery/scopes"
	"infra/cros/stableversion/keys"
)

// versionKeyType is a unique type for a context key.
type keyType string

const (
	// contextKey is key to access to logger from context.
	ctxKey keyType = "recovery_version"
)

// VersionClient is a client that provide access to read recovery versions by RPC call.
type VersionClient interface {
	GetRecoveryVersion(ctx context.Context, in *fleet.GetRecoveryVersionRequest, opts ...grpc.CallOption) (*fleet.GetRecoveryVersionResponse, error)
}

// WithClient sets VersionClient to the context.
// If Client is not provided process will be finished with panic.
func WithClient(ctx context.Context, c VersionClient) context.Context {
	if c == nil {
		panic("client is not provided")
	}
	return context.WithValue(ctx, ctxKey, c)
}

// GetClient VersionClient from context.
func GetClient(ctx context.Context) VersionClient {
	if c, ok := ctx.Value(ctxKey).(VersionClient); ok {
		return c
	}
	return nil
}

func cacheKey(deviceType, devicename, board, model string, pools []string) string {
	kb := keys.NewBuilder()
	_ = kb.Add("dt", deviceType)
	_ = kb.Add("board", board)
	_ = kb.Add("model", model)
	for _, pool := range pools {
		_ = kb.Add("pool", pool)
	}
	return "version_" + kb.String()
}

func setVersionToCache(ctx context.Context, key string, c Data) {
	scopes.PutConfigParam(ctx, key, c)
}

// GetClient VersionClient from context.
func getVersionFromCache(ctx context.Context, key string) Data {
	if c, ok := ctx.Value(key).(Data); ok {
		return c
	}
	return nil
}
