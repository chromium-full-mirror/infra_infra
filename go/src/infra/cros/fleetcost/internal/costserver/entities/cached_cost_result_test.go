// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities_test

import (
	"context"
	"testing"
	"time"

	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "infra/cros/fleetcost/api/models"
	"infra/cros/fleetcost/internal/costserver/entities"
	"infra/cros/fleetcost/internal/costserver/testsupport"
)

// TestCachedCostResultEntitySimple tests putting a cached DUT into datastore and extracting it back out.
func TestCachedCostResultEntitySimple(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)

	if err := datastore.Put(tf.Ctx, &entities.CachedCostResultEntity{
		Hostname: "hostname",
		CostResult: &fleetcostpb.CostResult{
			SharedCost: 34.00,
		},
		CostReport: &fleetcostpb.CostReport{
			Total: 34.00,
			Expr: &fleetcostpb.CostReportExpr{
				Operation: fleetcostpb.CostReportExpr_SUM,
				Number: []*fleetcostpb.CostReportScalar{{
					Value: 34.0,
				}},
			},
		},
		ExpirationTime: time.Unix(1, 4).UTC(),
	}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	var destination []*entities.CachedCostResultEntity
	if err := datastore.GetAll(tf.Ctx, datastore.NewQuery(entities.CachedCostResultKind), &destination); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}
