// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"
)

// NewAuthenticatedClient creates a new fleet console pRPC client.
func NewAuthenticatedClient(ctx context.Context, rpcAuthorityKind auth.RPCAuthorityKind, baseURL string, port int, insecure bool) (*prpc.Client, error) {
	var opts []auth.RPCOption
	switch rpcAuthorityKind {
	case auth.AsCredentialsForwarder:
		// do nothing
	case auth.AsSelf:
		// TODO(gregorynisbet): This logic is too complicated. Streamline it.
		audience := "https://" + ensureSingleTrailingSlash(baseURL)
		if insecure {
			audience = "http://" + ensureSingleTrailingSlash(baseURL)
		}
		opts = []auth.RPCOption{
			auth.WithIDTokenAudience(audience),
		}
	default:
		opts = []auth.RPCOption{auth.WithScopes(auth.CloudOAuthScopes...)}
	}
	t, err := auth.GetRPCTransport(ctx, rpcAuthorityKind, opts...)
	if err != nil {
		return nil, errors.Annotate(err, "setting up auth").Err()
	}
	httpClient := &http.Client{
		Transport: t,
	}
	prpcClient := &prpc.Client{
		C: httpClient,
		Options: &prpc.Options{
			Insecure: insecure,
		},
		Host: fmt.Sprintf("%s:%d", baseURL, port),
	}
	return prpcClient, nil
}

func ensureSingleTrailingSlash(content string) string {
	out := strings.TrimRight(content, "/")
	return out + "/"
}
