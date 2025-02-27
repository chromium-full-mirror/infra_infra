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

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/swarming/client/swarming"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
)

// dmClient creates a DeviceManager client using the command line method.
func dmClient(ctx context.Context, host string, authFlags authcli.Flags) (testapi.DeviceLeaseServiceClient, error) {
	httpClient, err := authenticatedClient(ctx, host, authFlags)
	if err != nil {
		return nil, errors.Annotate(err, "ping").Err()
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: host,
		Options: &prpc.Options{
			PerRPCTimeout: 30 * time.Second,
		},
	}
	return testapi.NewDeviceLeaseServiceClient(prpcClient), nil
}

// consoleClient creates a FleetConsoleClient pointing at a specific host.
//
// We always authenticate, even to  local server. However, we use the HTTP as the transport protocol
// if (and only if) we are talking to a local client.
func consoleClient(ctx context.Context, host string, authFlags authcli.Flags, useHTTP bool, timeout time.Duration) (fleetconsolerpc.FleetConsoleClient, error) {
	httpClient, err := authenticatedClient(ctx, host, authFlags)
	if err != nil {
		return nil, errors.Annotate(err, "ping").Err()
	}
	prpcClient := &prpc.Client{
		C:    httpClient,
		Host: host,
		Options: &prpc.Options{
			Insecure:      useHTTP,
			PerRPCTimeout: timeout,
		},
	}
	consoleClient := fleetconsolerpc.NewFleetConsoleClient(prpcClient)
	return consoleClient, nil
}

func swarmingClient(ctx context.Context, host string, authFlags authcli.Flags) (swarming.Client, error) {
	authOptions, err := authFlags.Options()
	if err != nil {
		return nil, err
	}
	return swarming.NewClient(ctx, swarming.ClientOptions{
		ServiceURL: host,
		Auth:       authOptions,
	})
}

func uiClient(ctx context.Context, host string, authFlags authcli.Flags) (*http.Client, error) {
	httpClient, err := authenticatedClient(ctx, host, authFlags)
	if err != nil {
		return nil, errors.Annotate(err, "ui client").Err()
	}
	return httpClient, nil
}

// authenticatedClient creates an authenticated HTTPS client.
func authenticatedClient(ctx context.Context, host string, authFlags authcli.Flags) (*http.Client, error) {
	authenticator, err := authenticator(ctx, host, authFlags)
	if err != nil {
		return nil, err
	}

	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, errors.Annotate(err, "creating authenticated client").Err()
	}
	return httpClient, nil
}

func authenticator(ctx context.Context, host string, authFlags authcli.Flags) (*auth.Authenticator, error) {
	authOptions, err := authFlags.Options()
	if err != nil {
		return nil, errors.Annotate(err, "creating authenticated client").Err()
	}
	authOptions.UseIDTokens = true
	if authOptions.Audience == "" {
		authOptions.Audience = "https://" + host
	}
	return auth.NewAuthenticator(ctx, auth.SilentLogin, authOptions), nil

}

// showProto writes a proto message as an indented object. Always adds a newline.
func showProto(dst io.Writer, message proto.Message) {
	if dst == nil {
		panic("dest cannot be nil")
	}
	bytes, err := (&protojson.MarshalOptions{
		Indent: "  ",
	}).Marshal(message)
	if err != nil {
		panic(err)
	}
	_, err = dst.Write(append(bytes, byte('\n')))
	if err != nil {
		panic(err)
	}
}
