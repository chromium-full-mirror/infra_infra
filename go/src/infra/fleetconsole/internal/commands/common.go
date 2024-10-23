// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"io"
	"net/http"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"infra/fleetconsole/api/fleetconsolerpc"
)

// consoleClient creates a FleetConsoleClient pointing at a specific host.
func consoleClient(ctx context.Context, host string, authFlags authcli.Flags, useHTTP bool) (fleetconsolerpc.FleetConsoleClient, error) {
	var httpClient *http.Client
	if !useHTTP {
		var err error
		httpClient, err = authenticatedClient(ctx, host, authFlags)
		if err != nil {
			return nil, errors.Annotate(err, "ping").Err()
		}
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: host,
		Options: &prpc.Options{
			Insecure:      useHTTP,
			PerRPCTimeout: 30 * time.Second,
		},
	}
	consoleClient := fleetconsolerpc.NewFleetConsoleClient(prpcClient)
	return consoleClient, nil
}

// authenticatedClient creates an authenticated HTTP client.
func authenticatedClient(ctx context.Context, host string, authFlags authcli.Flags) (*http.Client, error) {
	authOptions, err := authFlags.Options()
	if err != nil {
		return nil, errors.Annotate(err, "creating authenticated client").Err()
	}
	authOptions.UseIDTokens = true
	if authOptions.Audience == "" {
		authOptions.Audience = "https://" + host
	}
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, authOptions)
	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, errors.Annotate(err, "creating authenticated client").Err()
	}
	return httpClient, nil
}

// showProto writes a proto message as an indentend object. Always adds a newline.
func showProto(dst io.Writer, message proto.Message) (int, error) {
	if dst == nil {
		return 0, errors.New("dest cannot be nil")
	}
	bytes, err := (&protojson.MarshalOptions{
		Indent: "  ",
	}).Marshal(message)
	if err != nil {
		return 0, errors.Annotate(err, "show proto").Err()
	}
	return dst.Write(append(bytes, byte('\n')))
}
