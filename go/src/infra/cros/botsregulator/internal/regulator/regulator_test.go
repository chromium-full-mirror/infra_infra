// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package regulator

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	ufspb "infra/unifiedfleet/api/v1/models"
)

func TestConsolidateAvailableDUTs(t *testing.T) {
	t.Parallel()
	r := &regulator{}
	// In the context of this test, slices are equal if they have the same elements,
	// regardless of the elements order.
	trans := cmpopts.SortSlices(func(a, b string) bool {
		return a < b
	})

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		t.Run("Happy path", func(t *testing.T) {
			t.Parallel()
			sus := []*ufspb.SchedulingUnit{
				{
					Name:        "schedulingunits/su-1",
					MachineLSEs: []string{"dut-1"},
				},
				{
					Name:        "schedulingunits/su-2",
					MachineLSEs: []string{"dut-2", "dut-3"},
				},
				{
					Name:        "schedulingunits/su-3",
					MachineLSEs: []string{"dut-6", "dut-7"},
				},
			}
			lses := []*ufspb.MachineLSE{
				{
					Name: "machineLSEs/dut-1",
				},
				{
					Name: "machineLSEs/dut-2",
				},
				{
					Name: "machineLSEs/dut-3",
				},
				{
					Name: "machineLSEs/dut-4",
				},
				{
					Name: "machineLSEs/dut-5",
				},
			}
			dbs := []*apipb.BotInfo{
				{
					BotId: "crossk-dut-1",
					Dimensions: []*apipb.StringListPair{
						{
							Key:   "dut_name",
							Value: []string{"dut-1"},
						},
					},
				},
				{
					BotId: "cloudbots-e2-small-dut-5",
					Dimensions: []*apipb.StringListPair{
						{
							Key:   "dut_name",
							Value: []string{"dut-5"},
						},
					},
				},
			}
			ctx := context.Background()
			dutIDMap := r.DutMapFromBots(ctx, dbs)
			got := r.ConsolidateAvailableDUTs(ctx, "cloudbots-e2-small", dutIDMap, lses, sus)
			want := []string{
				"su-1",
				"su-2",
				"dut-4",
				"dut-5",
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("SchedulingUnit with at least 1 correct lse should be considered", func(t *testing.T) {
			t.Parallel()
			sus := []*ufspb.SchedulingUnit{
				{
					Name:        "schedulingunits/su-1",
					MachineLSEs: []string{"dut-1", "dut-2", "dut-3"},
				},
			}
			lses := []*ufspb.MachineLSE{
				{
					Name: "machineLSEs/dut-1",
				},
			}
			got := r.ConsolidateAvailableDUTs(context.Background(), "cloudbots", nil, lses, sus)
			want := []string{
				"su-1",
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("No schedulingUnits", func(t *testing.T) {
			t.Parallel()
			lses := []*ufspb.MachineLSE{
				{
					Name: "machineLSEs/dut-1",
				},
			}
			got := r.ConsolidateAvailableDUTs(context.Background(), "cloudbots", nil, lses, nil)
			want := []string{
				"dut-1",
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("DUTs running on Drone should not be considered", func(t *testing.T) {
			t.Parallel()
			lses := []*ufspb.MachineLSE{
				{
					Name: "machineLSEs/dut-1",
				},
				{
					Name: "machineLSEs/dut-3",
				},
			}
			dbs := []*apipb.BotInfo{
				{
					BotId: "crossk-dut-1",
					Dimensions: []*apipb.StringListPair{
						{
							Key:   "dut_name",
							Value: []string{"dut-1"},
						},
					},
				},
				{
					BotId: "crossk-dut-2",
					Dimensions: []*apipb.StringListPair{
						{
							Key:   "dut_name",
							Value: []string{"dut-2"},
						},
					},
				},
			}
			ctx := context.Background()
			dutIDMap := r.DutMapFromBots(ctx, dbs)
			got := r.ConsolidateAvailableDUTs(ctx, "cloudbots-e2-small", dutIDMap, lses, nil)
			want := []string{
				"dut-3",
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("SUs running on Drone should not be considered", func(t *testing.T) {
			t.Parallel()
			lses := []*ufspb.MachineLSE{
				{
					Name: "machineLSEs/dut-1",
				},
				{
					Name: "machineLSEs/dut-2",
				},
				{
					Name: "machineLSEs/dut-3",
				},
			}
			dbs := []*apipb.BotInfo{
				{
					BotId: "crossk-su-1",
					Dimensions: []*apipb.StringListPair{
						{
							Key:   "dut_name",
							Value: []string{"su-1"},
						},
					},
				},
			}
			sus := []*ufspb.SchedulingUnit{
				{
					Name:        "schedulingunits/su-1",
					MachineLSEs: []string{"dut-1", "dut-2"},
				},
			}
			ctx := context.Background()
			dutIDMap := r.DutMapFromBots(ctx, dbs)
			got := r.ConsolidateAvailableDUTs(ctx, "cloudbots-e2", dutIDMap, lses, sus)
			want := []string{
				"dut-3",
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	})
}

func TestConfigHive(t *testing.T) {
	t.Parallel()
	trans := cmpopts.SortMaps(func(a, b string) bool {
		return a < b
	})
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		t.Run("Happy path", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				CfID:      "cloudbots-regular",
				Hive:      "cloudbots",
				CfIDHives: "cloudbots-e2-small:cloudbots-small,cloudbots-e2-custom-2-6144:cloudbots-large",
			}
			want := map[string]string{
				"cloudbots-regular":          "cloudbots",
				"cloudbots-e2-small":         "cloudbots-small",
				"cloudbots-e2-custom-2-6144": "cloudbots-large",
			}
			got, err := configHive(opts)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("Cfid but no hive", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				CfID: "cloudbots-regular",
			}
			_, err := configHive(opts)
			if err == nil {
				t.Errorf("expected error but got nil")
			}
		})
		t.Run("Hive but no cfid", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				Hive: "cloudbots",
			}
			_, err := configHive(opts)
			if err == nil {
				t.Errorf("expected error but got nil")
			}
		})
		t.Run("Empty", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{}
			_, err := configHive(opts)
			if err == nil {
				t.Errorf("expected error but got nil")
			}
		})
		t.Run("CfIDHives", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				CfIDHives: "cloudbots-e2-small:cloudbots-small,cloudbots-e2-custom-2-6144:cloudbots-large",
			}
			want := map[string]string{
				"cloudbots-e2-small":         "cloudbots-small",
				"cloudbots-e2-custom-2-6144": "cloudbots-large",
			}
			got, err := configHive(opts)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(want, got, trans); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("Cfid, hive and CfIDHives", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				CfID:      "cloudbots-e2-small",
				Hive:      "cloudbots",
				CfIDHives: "cloudbots-e2-small:cloudbots-small,cloudbots-e2-custom-2-6144:cloudbots-large",
			}
			_, err := configHive(opts)
			if err == nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
		t.Run("CfIDHives missing hive", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				CfIDHives: "cloudbots-e2-small:,cloudbots-e2-custom-2-6144:cloudbots-large",
			}
			_, err := configHive(opts)
			if err == nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
		t.Run("CfIDHives missing cfid", func(t *testing.T) {
			t.Parallel()
			opts := &RegulatorOptions{
				CfIDHives: ":cloudbots-e2-small,cloudbots-e2-custom-2-6144:cloudbots-large",
			}
			_, err := configHive(opts)
			if err == nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	})
}
