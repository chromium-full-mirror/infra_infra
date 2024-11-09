// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package namespace contains functions to work with namespace of the context.
package namespace

import (
	"context"

	"google.golang.org/grpc/metadata"

	ufsUtil "infra/unifiedfleet/app/util"
)

// Set sets up context with namespace.
func Set(ctx context.Context, ns string) context.Context {
	md := metadata.Pairs(ufsUtil.Namespace, ns)
	return metadata.NewOutgoingContext(ctx, md)
}

// IsPartner check if namespace set for partners.
func IsPartner(ctx context.Context) bool {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return false
	}
	for _, nsValue := range md.Get(ufsUtil.Namespace) {
		if nsValue == ufsUtil.OSPartnerNamespace {
			return true
		}
	}
	return false
}
