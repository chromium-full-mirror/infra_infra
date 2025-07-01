// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"
	swarmingapi "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	"go.chromium.org/infra/cmdsupport/cmdlib"
	ufsutil "go.chromium.org/infra/unifiedfleet/app/util"
)

// newSwarmingBotClient returns a new bot client service to interact with.
func newSwarmingBotsClient(ctx context.Context, swarmingHost string, authFlags *authcli.Flags) (swarmingapi.BotsClient, error) {
	httpClient, err := cmdlib.NewHTTPClient(ctx, authFlags)
	if err != nil {
		return nil, err
	}
	return swarmingapi.NewBotsClient(&prpc.Client{
		C:       httpClient,
		Options: site.DefaultPRPCOptions,
		Host:    swarmingHost,
	},
	), nil
}

// hostnameToBotID returns the bot ID for a given DUT hostname.
func hostnameToBotID(ctx context.Context, swarmingBotsClient swarmingapi.BotsClient, hostname string) (string, error) {
	botsListReply, err := swarmingBotsClient.ListBots(ctx, &swarmingapi.BotsRequest{
		Limit: 1,
		Dimensions: []*swarmingapi.StringPair{
			{
				Key:   "dut_name",
				Value: hostname,
			},
		},
	})
	if err != nil {
		return "", err
	}
	bots := botsListReply.GetItems()
	if len(bots) == 0 {
		return "", errors.New(fmt.Sprintf("Invalid host %s: no associated Swarming bots found", hostname))
	}
	return bots[0].BotId, nil
}

// hostnameToPool returns the pool for a given DUT hostname.
func hostnameToPool(ctx context.Context, swarmingBotsClient swarmingapi.BotsClient, hostname string) (string, string, error) {
	botsListReply, err := swarmingBotsClient.ListBots(ctx, &swarmingapi.BotsRequest{
		Limit: 1,
		Dimensions: []*swarmingapi.StringPair{
			{
				Key:   "dut_name",
				Value: hostname,
			},
		},
	})
	if err != nil {
		return "", "", err
	}
	bots := botsListReply.GetItems()
	if len(bots) == 0 {
		return "", "", errors.New(fmt.Sprintf("Invalid host %s: no associated Swarming bots found", hostname))
	}
	dims := bots[0].GetDimensions()
	pool := []string{}
	dutID := []string{}
	for _, dim := range dims {
		if dim.GetKey() == poolLabelName {
			pool = dim.GetValue()
			if len(pool) == 0 {
				break
			}
		}
		if dim.GetKey() == dutIDLabelName {
			dutID = dim.GetValue()
			if len(dutID) == 0 {
				break
			}
		}
		if len(pool) > 0 && len(dutID) > 0 {
			return pool[0], dutID[0], nil
		}

	}
	return "", "", fmt.Errorf("no pool found for %s", hostname)
}

// countBotsWithDims returns the number of Swarming bots satisfying the given
// Swarming dimensions.
func countBotsWithDims(ctx context.Context, s swarmingapi.BotsClient, dimsMap map[string]string) (*swarmingapi.BotsCount, error) {
	var dims []*swarmingapi.StringPair
	for key, val := range dimsMap {
		dims = append(dims, &swarmingapi.StringPair{Key: key, Value: val})
	}
	return s.CountBots(ctx, &swarmingapi.BotsCountRequest{
		Dimensions: dims,
	})
}

// contextWithOSNamespace adds an "os" namespace to the given context, which
// is required for API calls to UFS.
func contextWithOSNamespace(ctx context.Context) context.Context {
	osMetadata := metadata.Pairs(ufsutil.Namespace, ufsutil.OSNamespace)
	return metadata.NewOutgoingContext(ctx, osMetadata)
}
