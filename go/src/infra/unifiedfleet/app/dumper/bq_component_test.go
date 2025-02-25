// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	apibq "go.chromium.org/infra/unifiedfleet/api/v1/models/bigquery"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

func mockHwidData() *ufspb.HwidData {
	return &ufspb.HwidData{
		Sku:      "test-sku",
		Variant:  "test-variant",
		Hwid:     "test-hwid",
		DutLabel: mockDutLabel(),
	}
}

func mockDutLabel() *ufspb.DutLabel {
	return &ufspb.DutLabel{
		PossibleLabels: []string{
			"test-possible-1",
			"test-possible-2",
		},
		Labels: []*ufspb.DutLabel_Label{
			{
				Name:  "test-label-1",
				Value: "test-value-1",
			},
			{
				Name:  "Sku",
				Value: "test-sku",
			},
			{
				Name:  "variant",
				Value: "test-variant",
			},
		},
	}
}

// Tests for the getAllHwidData method.
func TestGetAllHwidData(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("dev~infra-unified-fleet-system")
	ctx = gologger.StdConfig.Use(ctx)
	ctx = logging.SetLevel(ctx, logging.Error)
	datastore.GetTestable(ctx).Consistent(true)

	bqMsgs := make([]proto.Message, 0, 4)
	for i := range 4 {
		hdId := fmt.Sprintf("test-hwid-%d", i)
		hd := mockHwidData()
		resp, err := configuration.UpdateHwidData(ctx, hd, hdId)
		if err != nil {
			t.Fatalf("UpdateHwidData failed: %s", err)
		}
		respProto, err := resp.GetProto()
		if err != nil {
			t.Fatalf("GetProto failed: %s", err)
		}
		bqMsgs = append(bqMsgs, &apibq.HwidDataRow{
			HwidData: respProto.(*ufspb.HwidData),
		})
	}

	ftt.Run("getAllHwidData", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := getAllHwidData(ctx)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(bqMsgs))
		})
	})
}
