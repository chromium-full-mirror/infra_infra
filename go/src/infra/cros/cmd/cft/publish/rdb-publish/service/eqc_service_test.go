// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/bigquery"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestNewEQCPublishService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("NewEQCPublishService", t, func(t *ftt.Test) {
		t.Run("Valid request", func(t *ftt.Test) {
			wantEQCInfos := []*artifact.EqcInfo{
				{
					EqcHash: "9073744604696850342",
					EqcName: "Cometlake-U__INTEL_HRP2_AX201__5.15",
					EqcCategoryExpression: map[string]string{
						"name": "WifiBtChipset_Soc_Kernel_Intel",
					},
					EqcDimensions: map[string]string{
						"dlm:soc":               "Cometlake-U",
						"image:_kernel_version": "5.15",
						"wireless_field":        "INTEL_HRP2_AX201",
					},
				},
			}
			eqcInfoMap := map[string]string{
				"eqcCategoryExpression": "{\"name\": \"WifiBtChipset_Soc_Kernel_Intel\"}",
				"eqcDimensions":         "{\"dlm:soc\":\"Cometlake-U\",\"image:_kernel_version\":\"5.15\",\"wireless_field\":\"INTEL_HRP2_AX201\"}",
				"eqcHash":               "9073744604696850342",
				"eqcName":               "Cometlake-U__INTEL_HRP2_AX201__5.15",
				"eqcTests":              "[\"tast.wifi.SetTXPower\",\"tast.wifi.SetTXPower.vpd\"]",
			}
			rdbMetadata := &metadata.PublishRdbMetadata{
				PublishKeys: []*api.PublishKey{
					{
						Subject:   EQCSubjectKey,
						KeyValues: eqcInfoMap,
					},
				},
			}

			validMetadataAny, err := anypb.New(rdbMetadata)
			assert.Loosely(t, err, should.BeNil)

			req := &api.PublishRequest{
				Metadata: validMetadataAny,
			}

			s, err := NewEQCPublishService(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s, should.NotBeNil)
			assert.Loosely(t, s.eqcInfos, should.Resemble(wantEQCInfos))

			s.Close()
		})

		t.Run("Nil request", func(t *ftt.Test) {
			s, err := NewEQCPublishService(ctx, nil)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, s, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("request is nil"))
		})

		t.Run("Nil metadata", func(t *ftt.Test) {
			req := &api.PublishRequest{}
			s, err := NewEQCPublishService(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, s, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("invalid nil source message"))
		})

		t.Run("Invalid metadata type", func(t *ftt.Test) {
			req := &api.PublishRequest{
				Metadata: &anypb.Any{}, // Empty Any proto
			}

			s, err := NewEQCPublishService(ctx, req)

			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, s, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unpacking the metadata"))
		})
	})
}

func TestEQCRowSave(t *testing.T) {
	t.Parallel()

	eqcHash := "A310930190d11"
	eqcName := "IntelRaptorLakeKernelNext"

	ftt.Run("EQCRow Save", t, func(t *ftt.Test) {
		t.Run("Valid entry", func(t *ftt.Test) {
			entry := &EQCRow{
				EQCHash: eqcHash,
				EQCName: eqcName,
				EQCCategoryExpression: map[string]string{
					"name": "WifiBtChipset_Soc_Kernel_Intel",
				},
				EQCDimensions: map[string]string{
					"soc":      "value1",
					"wifiChip": "value2",
				},
			}

			wantValue := map[string]bigquery.Value{
				"eqc_hash":                eqcHash,
				"eqc_name":                eqcName,
				"eqc_category_expression": `{"name": "WifiBtChipset_Soc_Kernel_Intel"}`,
				"eqc_dimensions":          `{"soc":"value1","wifiChip":"value2"}`, // Note: order may vary
			}

			gotValue, gotInsertID, err := entry.Save()

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotInsertID, should.Equal(wantValue["eqc_hash"]))
			assert.Loosely(t, gotValue["eqc_hash"], should.Equal(wantValue["eqc_hash"]))
			assert.Loosely(t, gotValue["eqc_name"], should.Equal(eqcName))

			// Compare maps, ignoring key order in eqc_category_expression.
			gotCategoryExpression := gotValue["eqc_category_expression"]
			wantCategoryExpression := wantValue["eqc_category_expression"]

			var gotCategoryExprMap, wantCategoryExprMap map[string]string
			err = json.Unmarshal([]byte(gotCategoryExpression.(string)), &gotCategoryExprMap)
			assert.Loosely(t, err, should.BeNil)
			err = json.Unmarshal([]byte(wantCategoryExpression.(string)), &wantCategoryExprMap)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotCategoryExprMap, should.Resemble(wantCategoryExprMap))

			// Compare maps, ignoring key order in eqc_dimensions.
			gotDimensions := gotValue["eqc_dimensions"]
			wantDimensions := wantValue["eqc_dimensions"]

			var gotDimMap, wantDimMap map[string]string
			err = json.Unmarshal([]byte(gotDimensions.(string)), &gotDimMap)
			assert.Loosely(t, err, should.BeNil)
			err = json.Unmarshal([]byte(wantDimensions.(string)), &wantDimMap)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotDimMap, should.Resemble(wantDimMap))

		})

		t.Run("Empty dimensions", func(t *ftt.Test) {
			entry := &EQCRow{
				EQCHash:               eqcHash,
				EQCName:               eqcName,
				EQCCategoryExpression: make(map[string]string), // Empty map
				EQCDimensions:         make(map[string]string), // Empty map
			}

			wantValue := map[string]bigquery.Value{
				"eqc_hash":                eqcHash,
				"eqc_name":                eqcName,
				"eqc_category_expression": `{}`,
				"eqc_dimensions":          `{}`,
			}

			gotValue, _, err := entry.Save()

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotValue, should.Resemble(wantValue))
		})

		t.Run("Nil dimensions", func(t *ftt.Test) {
			entry := &EQCRow{
				EQCHash:               eqcHash,
				EQCName:               eqcName,
				EQCCategoryExpression: nil,
				EQCDimensions:         nil,
			}

			wantValue := map[string]bigquery.Value{
				"eqc_hash": eqcHash,
				"eqc_name": eqcName,
			}

			gotValue, _, err := entry.Save()

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gotValue, should.Resemble(wantValue))
		})
	})
}

// Generate unit tests for the function eqcInfo(req *api.PublishRequest)
func TestEqcInfo(t *testing.T) {
	t.Parallel()

	ftt.Run("Valid request with EQC info", t, func(t *ftt.Test) {
		wantEQCInfos := []*artifact.EqcInfo{
			{
				EqcHash: "9073744604696850342",
				EqcName: "Cometlake-U__INTEL_HRP2_AX201__5.15",
				EqcCategoryExpression: map[string]string{
					"value": "{\"combinatorial\":{\"subcategories\":[{\"value\":{\"enumerated\":{\"classes\":[{\"value\":{\"expression\":{\"property\":{\"propertyPath\":\"swarming:label-wifi_state\",\"strEqual\":\"NORMAL\"}},\"name\":\"swarming:label-wifi_state:NORMAL\"}}]}}},{\"name\":\"WifiBtChipset_Soc_Kernel\"}]}}",
				},
				EqcDimensions: map[string]string{
					"dlm:soc":               "Cometlake-U",
					"image:_kernel_version": "5.15",
					"wireless_field":        "INTEL_HRP2_AX201",
				},
			},
			{
				EqcHash: "8073744604696850123",
				EqcName: "Cometlake-U__INTEL_HRP2_AX201__6.26",
				EqcCategoryExpression: map[string]string{
					"value": "{\"combinatorial\":{\"subcategories\":[{\"value\":{\"enumerated\":{\"classes\":[{\"value\":{\"expression\":{\"property\":{\"propertyPath\":\"swarming:label-wifi_state\",\"strEqual\":\"NORMAL\"}},\"name\":\"swarming:label-wifi_state:NORMAL\"}}]}}},{\"name\":\"WifiBtChipset_Soc_Kernel\"}]}}",
				},
				EqcDimensions: map[string]string{
					"dlm:soc":               "Cometlake-X",
					"image:_kernel_version": "6.26",
					"wireless_field":        "INTEL_HRP2_AX123",
				},
			},
		}
		eqcInfoMap1 := map[string]string{
			"eqcCategoryExpression": "{\"value\":{\"combinatorial\":{\"subcategories\":[{\"value\":{\"enumerated\":{\"classes\":[{\"value\":{\"name\":\"swarming:label-wifi_state:NORMAL\",\"expression\":{\"property\":{\"propertyPath\":\"swarming:label-wifi_state\",\"strEqual\":\"NORMAL\"}}}}]}}},{\"name\":\"WifiBtChipset_Soc_Kernel\"}]}}}",
			"eqcDimensions":         "{\"dlm:soc\":\"Cometlake-U\",\"image:_kernel_version\":\"5.15\",\"wireless_field\":\"INTEL_HRP2_AX201\"}",
			"eqcHash":               "9073744604696850342",
			"eqcName":               "Cometlake-U__INTEL_HRP2_AX201__5.15",
			"eqcTests":              "[\"tast.wifi.SetTXPower\"]",
		}
		eqcInfoMap2 := map[string]string{
			"eqcCategoryExpression": "{\"value\":{\"combinatorial\":{\"subcategories\":[{\"value\":{\"enumerated\":{\"classes\":[{\"value\":{\"name\":\"swarming:label-wifi_state:NORMAL\",\"expression\":{\"property\":{\"propertyPath\":\"swarming:label-wifi_state\",\"strEqual\":\"NORMAL\"}}}}]}}},{\"name\":\"WifiBtChipset_Soc_Kernel\"}]}}}",
			"eqcDimensions":         "{\"dlm:soc\":\"Cometlake-X\",\"image:_kernel_version\":\"6.26\",\"wireless_field\":\"INTEL_HRP2_AX123\"}",
			"eqcHash":               "8073744604696850123",
			"eqcName":               "Cometlake-U__INTEL_HRP2_AX201__6.26",
			"eqcTests":              "[\"tast.wifi.SetTXPower.vpd\"]",
		}
		rdbMetadata := &metadata.PublishRdbMetadata{
			PublishKeys: []*api.PublishKey{
				{
					Subject:   EQCSubjectKey,
					KeyValues: eqcInfoMap1,
				},
				{
					Subject:   EQCSubjectKey,
					KeyValues: eqcInfoMap2,
				},
			},
		}

		validMetadataAny, err := anypb.New(rdbMetadata)
		assert.Loosely(t, err, should.BeNil)

		req := &api.PublishRequest{
			Metadata: validMetadataAny,
		}
		gotEQCInfos, err := EqcInfos(req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, gotEQCInfos, should.Resemble(wantEQCInfos))
	})

	ftt.Run("Valid request without EQC info", t, func(t *ftt.Test) {
		validMetadataAny, err := anypb.New(&metadata.PublishRdbMetadata{})
		assert.Loosely(t, err, should.BeNil)

		req := &api.PublishRequest{
			Metadata: validMetadataAny,
		}
		gotEQCInfo, err := EqcInfos(req)

		// Expect an empty EqcInfo
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, gotEQCInfo, should.Resemble([]*artifact.EqcInfo{}))
	})

	ftt.Run("Nil request", t, func(t *ftt.Test) {
		gotEQCInfo, err := EqcInfos(nil)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.ContainSubstring("unpacking the metadata"))
		assert.Loosely(t, gotEQCInfo, should.BeNil)
	})

	ftt.Run("Invalid metadata", t, func(t *ftt.Test) {
		req := &api.PublishRequest{
			Metadata: &anypb.Any{}, // Empty Any proto
		}
		gotEQCInfo, err := EqcInfos(req)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, err.Error(), should.ContainSubstring("unpacking the metadata"))
		assert.Loosely(t, gotEQCInfo, should.BeNil)

	})
}
