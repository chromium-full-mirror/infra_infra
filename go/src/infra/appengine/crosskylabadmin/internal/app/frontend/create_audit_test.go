// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/frontend/routing"
)

// TestRouteAuditTaskImpl tests routing audit tasks.
func TestRouteAuditTaskImpl(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("no config", t, func(t *ftt.Test) {
		tt, r := routeAuditTaskImpl(ctx, nil, "", 0.0)
		assert.Loosely(t, tt, should.Equal(routing.Paris))
		assert.Loosely(t, r, should.Equal(routing.ParisNotEnabled))
	})
	ftt.Run("invalid random float", t, func(t *ftt.Test) {
		tt, r := routeAuditTaskImpl(ctx, &config.RolloutConfig{}, "", 12.0)
		assert.Loosely(t, tt, should.Equal(routing.Paris))
		assert.Loosely(t, r, should.Equal(routing.InvalidRangeArgument))
	})
	ftt.Run("bad permille info", t, func(t *ftt.Test) {
		pat := &config.RolloutConfig{
			Pattern: []*config.RolloutConfig_Pattern{
				{Pattern: "^", ProdPermille: 4},
			},
		}
		res := pat.ComputePermilleData(ctx, "hostname")
		assert.Loosely(t, res, should.BeNil)
	})
	ftt.Run("25-25 split", t, func(t *ftt.Test) {
		pd := &config.RolloutConfig{Enable: true, ProdPermille: 250, LatestPermille: 250}
		t.Run("0.24", func(t *ftt.Test) {
			tt, r := routeAuditTaskImpl(ctx, pd, "", 0.24)
			assert.Loosely(t, tt, should.Equal(routing.ParisLatest))
			assert.Loosely(t, r, should.Equal(routing.ScoreBelowThreshold))
		})
		t.Run("0.26", func(t *ftt.Test) {
			tt, r := routeAuditTaskImpl(ctx, pd, "", 0.26)
			assert.Loosely(t, tt, should.Equal(routing.Paris))
			assert.Loosely(t, r, should.Equal(routing.ScoreBelowThreshold))
		})
	})
	ftt.Run("Repair-only field", t, func(t *ftt.Test) {
		pd := &config.RolloutConfig{Enable: true, OptinAllDuts: true}
		tt, r := routeAuditTaskImpl(ctx, pd, "", 0.24)
		assert.Loosely(t, tt, should.Equal(routing.Paris))
		assert.Loosely(t, r, should.Equal(routing.RepairOnlyField))
	})
}
