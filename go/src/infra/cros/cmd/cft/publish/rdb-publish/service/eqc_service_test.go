// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/bigquery"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/api/metadata"
	"go.chromium.org/chromiumos/config/go/test/artifact"
	. "go.chromium.org/luci/common/testing/assertions"
)

func TestNewEQCPublishService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	Convey("NewEQCPublishService", t, func() {
		Convey("Valid request", func() {
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
			So(err, ShouldBeNil)

			req := &api.PublishRequest{
				Metadata: validMetadataAny,
			}

			s, err := NewEQCPublishService(ctx, req)

			So(err, ShouldBeNil)
			So(s, ShouldNotBeNil)
			So(s.eqcInfos, ShouldResembleProto, wantEQCInfos)

			s.Close()
		})

		Convey("Nil request", func() {
			s, err := NewEQCPublishService(ctx, nil)

			So(err, ShouldNotBeNil)
			So(s, ShouldBeNil)
			So(err.Error(), ShouldContainSubstring, "request is nil")
		})

		Convey("Nil metadata", func() {
			req := &api.PublishRequest{}
			s, err := NewEQCPublishService(ctx, req)

			So(err, ShouldNotBeNil)
			So(s, ShouldBeNil)
			So(err.Error(), ShouldContainSubstring, "invalid nil source message")
		})

		Convey("Invalid metadata type", func() {
			req := &api.PublishRequest{
				Metadata: &anypb.Any{}, // Empty Any proto
			}

			s, err := NewEQCPublishService(ctx, req)

			So(err, ShouldNotBeNil)
			So(s, ShouldBeNil)
			So(err.Error(), ShouldContainSubstring, "unpacking the metadata")
		})
	})
}

func TestEQCRowSave(t *testing.T) {
	t.Parallel()

	eqcHash := "A310930190d11"
	eqcName := "IntelRaptorLakeKernelNext"

	Convey("EQCRow Save", t, func() {
		Convey("Valid entry", func() {
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

			So(err, ShouldBeNil)
			So(gotInsertID, ShouldEqual, wantValue["eqc_hash"])
			So(gotValue["eqc_hash"], ShouldEqual, wantValue["eqc_hash"])
			So(gotValue["eqc_name"], ShouldEqual, eqcName)

			// Compare maps, ignoring key order in eqc_category_expression.
			gotCategoryExpression := gotValue["eqc_category_expression"]
			wantCategoryExpression := wantValue["eqc_category_expression"]

			var gotCategoryExprMap, wantCategoryExprMap map[string]string
			err = json.Unmarshal([]byte(gotCategoryExpression.(string)), &gotCategoryExprMap)
			So(err, ShouldBeNil)
			err = json.Unmarshal([]byte(wantCategoryExpression.(string)), &wantCategoryExprMap)
			So(err, ShouldBeNil)
			So(gotCategoryExprMap, ShouldResemble, wantCategoryExprMap)

			// Compare maps, ignoring key order in eqc_dimensions.
			gotDimensions := gotValue["eqc_dimensions"]
			wantDimensions := wantValue["eqc_dimensions"]

			var gotDimMap, wantDimMap map[string]string
			err = json.Unmarshal([]byte(gotDimensions.(string)), &gotDimMap)
			So(err, ShouldBeNil)
			err = json.Unmarshal([]byte(wantDimensions.(string)), &wantDimMap)
			So(err, ShouldBeNil)
			So(gotDimMap, ShouldResemble, wantDimMap)

		})

		Convey("Empty dimensions", func() {
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

			So(err, ShouldBeNil)
			So(gotValue, ShouldResemble, wantValue)
		})

		Convey("Nil dimensions", func() {
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

			So(err, ShouldBeNil)
			So(gotValue, ShouldResemble, wantValue)
		})
	})
}

// Generate unit tests for the function eqcInfo(req *api.PublishRequest)
func TestEqcInfo(t *testing.T) {
	t.Parallel()

	Convey("Valid request with EQC info", t, func() {
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
		So(err, ShouldBeNil)

		req := &api.PublishRequest{
			Metadata: validMetadataAny,
		}
		gotEQCInfos, err := EqcInfos(req)
		So(err, ShouldBeNil)
		So(gotEQCInfos, ShouldResembleProto, wantEQCInfos)
	})

	Convey("Valid request without EQC info", t, func() {
		validMetadataAny, err := anypb.New(&metadata.PublishRdbMetadata{})
		So(err, ShouldBeNil)

		req := &api.PublishRequest{
			Metadata: validMetadataAny,
		}
		gotEQCInfo, err := EqcInfos(req)

		// Expect an empty EqcInfo
		So(err, ShouldBeNil)
		So(gotEQCInfo, ShouldResembleProto, []*artifact.EqcInfo{})
	})

	Convey("Nil request", t, func() {
		gotEQCInfo, err := EqcInfos(nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "unpacking the metadata")
		So(gotEQCInfo, ShouldBeNil)
	})

	Convey("Invalid metadata", t, func() {
		req := &api.PublishRequest{
			Metadata: &anypb.Any{}, // Empty Any proto
		}
		gotEQCInfo, err := EqcInfos(req)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "unpacking the metadata")
		So(gotEQCInfo, ShouldBeNil)

	})
}
