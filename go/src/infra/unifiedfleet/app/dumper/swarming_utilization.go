// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This cron job is imported from CrosSkylabAdmin.
// It collects ChromeOSSkylab bot metrics from chromeos-swarming

package dumper

import (
	"context"
	"net/http"

	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	"infra/cros/lab_inventory/utilization"
	"infra/unifiedfleet/app/config"
)

const (
	// PoolDimensionKey identifies the swarming pool dimension.
	poolDimensionKey = "pool"

	// maxSwarmingIterations is a sensible maximum number of iterations for functions that call swarming (possibly with pagination) in a loop.
	maxSwarmingIterations = 3000

	// swarmingQueryLimit is a sensible maximum number of entities to query at a time.
	swarmingQueryLimit = 500
)

// swarmingClient exposes Swarming client API used by this package.
//
// In prod, a swarmingClient for interacting with the Swarming service will be
// used. Tests should use a fake.
type swarmingClient interface {
	listAliveBotsInPool(context.Context, string) ([]*swarmingv2.BotInfo, error)
}

type swarmingClientImpl struct {
	botsClient swarmingv2.BotsClient
}

// newSwarmingClient returns a swarmingClient for interaction with the Swarming
// service.
func newSwarmingClient(c context.Context, host string) (swarmingClient, error) {
	// The Swarming call to list bots requires special previliges (beyond task
	// trigger privilege) This app is authorized to make those API calls.
	t, err := auth.GetRPCTransport(c, auth.AsSelf)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get RPC transport for host %s", host).Err()
	}
	prpcClient := &prpc.Client{
		C:    &http.Client{Transport: t},
		Host: host,
		Options: &prpc.Options{
			UserAgent: "unifiedfleetsystem",
		},
	}
	botsClient := swarmingv2.NewBotsClient(prpcClient)
	return &swarmingClientImpl{
		botsClient: botsClient,
	}, nil
}

// listAliveBotsInPool lists the Swarming bots in the given pool.
func (sc *swarmingClientImpl) listAliveBotsInPool(ctx context.Context, pool string) ([]*swarmingv2.BotInfo, error) {
	dims := strpair.Map{}
	dims.Set(poolDimensionKey, pool)
	getRequest := func(cursor string) *swarmingv2.BotsRequest {
		return &swarmingv2.BotsRequest{
			Cursor: cursor,
			IsDead: swarmingv2.NullableBool_FALSE,
			Limit:  swarmingQueryLimit,
		}
	}

	cursor := ""
	var out []*swarmingv2.BotInfo

	for i := 0; i < maxSwarmingIterations; i++ {
		resp, err := sc.botsClient.ListBots(ctx, getRequest(cursor))
		if err != nil {
			return nil, errors.Annotate(err, "failed to list alive and idle bots in pool %s", pool).Err()
		}
		out = append(out, resp.GetItems()...)
		cursor = resp.GetCursor()
		if cursor == "" {
			return out, nil
		}
	}

	return nil, errors.New("internal error in app/dumper/swarming_utilization.go: we iterated too much over the alive bots without encountering an error. Consider raising the limits.")
}

// reportSwarmingBotsCronHandler reports metrics of swarming bots.
func reportSwarmingBotsCronHandler(ctx context.Context) (err error) {
	defer func() {
		err = grpcutil.GRPCifyAndLogErr(ctx, err)
	}()
	cfg := config.Get(ctx)
	sc, err := newSwarmingClient(ctx, cfg.GetSwarming().GetHost())
	if err != nil {
		return errors.Annotate(err, "failed to obtain Swarming client").Err()
	}

	bots, err := sc.listAliveBotsInPool(ctx, cfg.GetSwarming().GetBotPool())
	utilization.ReportMetrics(ctx, flattenAndDeduplicateBots([][]*swarmingv2.BotInfo{bots}))
	return nil
}

func flattenAndDeduplicateBots(nb [][]*swarmingv2.BotInfo) []*swarmingv2.BotInfo {
	bm := make(map[string]*swarmingv2.BotInfo)
	for _, bs := range nb {
		for _, b := range bs {
			bm[b.BotId] = b
		}
	}
	bots := make([]*swarmingv2.BotInfo, 0, len(bm))
	for _, v := range bm {
		bots = append(bots, v)
	}
	return bots
}
