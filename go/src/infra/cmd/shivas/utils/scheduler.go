// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains utility functions for Shivas.
package utils

import (
	"context"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/errors"

	"infra/libs/fleet/device"
	schedulingapi "infra/libs/fleet/scheduling/api"
	"infra/libs/fleet/scheduling/schedulers"
	"infra/libs/skylab/common/heuristics"
)

// SchedukeClient initializes a Scheduke client for CLI use.
func SchedukeClient(ctx context.Context, gpc device.GetPoolsClient, authOpts auth.Options, host string) (schedulingapi.TaskSchedulingAPI, error) {
	host = heuristics.NormalizeBotNameToDeviceName(host)
	pools, err := device.GetPools(ctx, gpc, host)
	if err != nil {
		return nil, errors.Annotate(err, "getting pools for device %s", host).Err()
	}
	if len(pools) == 0 {
		return nil, errors.Reason("found no pool for device %s", host).Err()
	}
	sc, err := schedulers.NewSchedukeClientForCLI(ctx, pools[0], authOpts)
	if err != nil {
		return nil, errors.Annotate(err, "initializing Scheduke client").Err()
	}
	return sc, nil
}
