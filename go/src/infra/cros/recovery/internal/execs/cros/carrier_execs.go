// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
)

func init() {
	execs.Register("carrier_not_in", carrierNotInExec)
	execs.Register("carrier_is_in", carrierIsInExec)
}

// carrierIsInExec validates that the DUT carier is in the provided list.
func carrierIsInExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("carrier is in: cellular data is not present in dut info").Err()
	}

	if c.GetCarrier() == "" {
		return errors.Reason("carrier is in: DUT carrier label is empty").Err()
	}

	argsMap := info.GetActionArgs(ctx)
	carriers := argsMap.AsStringSlice(ctx, "carriers", []string{})
	for _, carrier := range carriers {
		if strings.EqualFold(carrier, c.GetCarrier()) {
			return nil
		}
	}

	return errors.Reason("carrier nis in: carrier %q is not the provided list", c.GetCarrier()).Err()
}

// carrierNotInExec validates that the DUT cellular network carrier is not in a provided list.
func carrierNotInExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("carrier not in: cellular data is not present in dut info").Err()
	}

	if c.GetCarrier() == "" {
		return errors.Reason("carrier not in: DUT carrier label is empty").Err()
	}

	argsMap := info.GetActionArgs(ctx)
	carriers := argsMap.AsStringSlice(ctx, "carriers", []string{})
	for _, carrier := range carriers {
		if strings.EqualFold(carrier, c.GetCarrier()) {
			return errors.Reason("carrier not in: carrier %q is in the provided list", c.Carrier).Err()
		}
	}

	return nil
}
