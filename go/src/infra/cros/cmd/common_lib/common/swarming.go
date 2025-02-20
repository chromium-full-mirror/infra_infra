// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"sort"
	"strings"
	"sync"

	"google.golang.org/api/option"

	"go.chromium.org/luci/auth/client/authcli"
	swarm_v1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cmdsupport/cmdlib"
)

var BotCountCache sync.Map

// CreateNewSwarmingService creates new swarming service.
func CreateNewSwarmingService(ctx context.Context) (*swarm_v1.Service, error) {
	authFlags := &authcli.Flags{}
	httpClient, err := cmdlib.NewHTTPClient(ctx, authFlags)
	if err != nil {
		logging.Infof(
			ctx,
			"error during creating new http client: %s",
			err.Error())
		return nil, err
	}
	swarmingService, err := swarm_v1.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		logging.Infof(
			ctx,
			"error during creating new swarming service: %s",
			err.Error())
		return nil, err
	}
	swarmingService.BasePath = SwarmingBasePath

	return swarmingService, nil
}

// GetBotCount gets total bot count for provided dims.
// dims example: {"label-board:zork", "label-model:morphius", "dut_state:ready"}
func GetBotCount(ctx context.Context, dims []string, swarmingService *swarm_v1.Service) (int64, error) {
	// Check cache.
	sort.Strings(dims)
	cacheKey := strings.Join(dims, "-")
	if botCount, found := BotCountCache.Load(cacheKey); found {
		logging.Infof(ctx, "Bot count cache hit : %s,%d", cacheKey, botCount)
		return botCount.(int64), nil
	}
	// Create new swarming service if not provided.
	if swarmingService == nil {
		newSwarmingService, err := CreateNewSwarmingService(ctx)
		if err != nil {
			logging.Infof(
				ctx,
				"error during creating swarming client: %s",
				err.Error())
			return -1, err
		}
		swarmingService = newSwarmingService
	}

	// Api call to get count.
	botReply, err := swarmingService.Bots.Count().Dimensions(dims...).Do()
	if err != nil {
		logging.Infof(
			ctx,
			"error during getting bot count: %s",
			err.Error())
		return -1, err
	}

	logging.Infof(
		ctx,
		"bot count: %d for dims: %v",
		botReply.Count, dims)

	// cache response
	BotCountCache.Store(cacheKey, botReply.Count)
	return botReply.Count, nil
}

// GetBotList gets bot list for provided dims and properties.
// dims example: {"label-board:zork", "label-model:morphius", "dut_state:ready"}
func GetBotList(ctx context.Context, dims []string, swarmingService *swarm_v1.Service, isBuysVal string, isDeadVal string, inMaintenanceVal string, quarantinedVal string) ([]*swarm_v1.SwarmingRpcsBotInfo, error) {
	// will be used to store all botItems received from swarming.
	bots := []*swarm_v1.SwarmingRpcsBotInfo{}

	// Create new swarming service if not provided.
	if swarmingService == nil {
		newSwarmingService, err := CreateNewSwarmingService(ctx)
		if err != nil {
			logging.Infof(
				ctx,
				"error during creating swarming client: %s",
				err.Error())
			return bots, err
		}
		swarmingService = newSwarmingService
	}

	// Create bots list call
	botsListCall := swarmingService.Bots.List().Context(ctx).Dimensions(dims...)

	if isBuysVal != "" {
		botsListCall = botsListCall.IsBusy(isBuysVal)
	}
	if isDeadVal != "" {
		botsListCall = botsListCall.IsDead(isDeadVal)
	}
	if inMaintenanceVal != "" {
		botsListCall = botsListCall.InMaintenance(inMaintenanceVal)
	}
	if quarantinedVal != "" {
		botsListCall = botsListCall.Quarantined(quarantinedVal)
	}

	// If the query has more results than maxLimit, a cursor will be provided to
	// continue iterating on the query results in subsequent pRPC calls.
	cursor := ""
	for {
		// Query swarming for bots with the provided dimensions via pRPC. NOTE: an
		// empty cursor on the first call acts as a noop.
		botReply, err := botsListCall.Cursor(cursor).Limit(SwarmingMaxLimitForEachQuery).Do()
		if err != nil {
			logging.Infof(
				ctx,
				"error during getting bots list: %s",
				err.Error())
			return bots, err
		}

		// If there are no elements in the return then exit.
		if len(botReply.Items) == 0 {
			break
		}

		// Add received bots to the list.
		bots = append(bots, botReply.Items...)

		// Query is <= maxLimit so a cursor wasn't needed. This will always occur on the
		// last iteration.
		if botReply.Cursor == "" {
			break
		}

		// Update the cursor.
		cursor = botReply.Cursor
	}

	// Return the raw list of all bots recognized in the lab.
	return bots, nil
}
