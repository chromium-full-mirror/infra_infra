// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"

	"google.golang.org/grpc/metadata"

	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// SetupContext sets up context with namespace
func SetupContext(ctx context.Context, namespace string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, namespace)
	return metadata.NewOutgoingContext(ctx, md)
}

// ReadContextNamespace read namespace value from the context.
func ReadContextNamespace(ctx context.Context, defaultValue string) string {
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for _, v := range md.Get(ufsUtil.Namespace) {
			if v != "" {
				return v
			}
		}
	}
	return defaultValue
}
