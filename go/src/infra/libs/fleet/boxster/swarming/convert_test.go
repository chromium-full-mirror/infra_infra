// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarming

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// TODO (justinsuen): jsonpb throws an error when working with the
// publicReplication field. "json: cannot unmarshal string into Go value of type
// map[string]json.RawMessage." The field is removed just for the unittests here
// but should not affect the functionality of the library as the protos will be
// directly unmarshaled from the datastore instead of a string representation.

func parseDutAttribute(t *ftt.Test, protoText string) api.DutAttribute {
	var da api.DutAttribute
	if err := jsonpb.UnmarshalString(protoText, &da); err != nil {
		t.Fatalf("Error unmarshalling example text: %s", err)
	}
	return da
}

func TestConvertAll(t *testing.T) {
	t.Parallel()

	b, err := ioutil.ReadFile("test_flat_config.cfg")
	if err != nil {
		t.Fatalf("Error reading test FlatConfig: %s", err)
	}

	var fc payload.FlatConfig
	unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: false}
	if err = unmarshaller.Unmarshal(bytes.NewBuffer(b), &fc); err != nil {
		t.Fatalf("Error unmarshalling test FlatConfig: %s", err)
	}

	t.Run("convert label with existing correct field path - single value", func(t *testing.T) {
		daText := `{
			"id": {
				"value": "attr-design"
			},
			"aliases": [
				"attr-model",
				"label-model"
			],
			"flatConfigSource": {
				"fields": [
					{
						"path": "hw_design.id.value"
					}
				]
			}
		}`
		da := parseDutAttribute(&ftt.Test{T: t}, daText)
		want := Dimensions{
			"attr-design": {"Test"},
			"attr-model":  {"Test"},
			"label-model": {"Test"},
		}
		got, err := ConvertAll(&da, &fc)
		if err != nil {
			t.Fatalf("ConvertAll failed: %s", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ConvertAll returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("convert label with existing correct field path - no matching values", func(t *testing.T) {
		daText := `{
      "id": {
        "value": "attr-ec-type"
      },
      "aliases": [
        "label-ec_type"
      ],
      "flatConfigSource": {
        "fields": [
          {
            "path": "hw_design_config.hardware_features.embedded_controller.ec_type"
          }
        ]
      }
    }`
		da := parseDutAttribute(&ftt.Test{T: t}, daText)
		got, err := ConvertAll(&da, &fc)
		if err == nil {
			t.Fatalf("ConvertAll passed without failures")
		}
		if diff := cmp.Diff(Dimensions(nil), got); diff != "" {
			t.Errorf("ConvertAll returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("convert label with existing correct field path - filter based on component", func(t *testing.T) {
		daText := `{
			"id": {
				"value": "hw-wireless"
			},
			"aliases": [
				"label-wifi_chip"
			],
			"hwidSource": {
				"componentType": "wifi",
				"fields": [
					{
						"path": "hwid_label"
					}
				]
			}
		}`
		da := parseDutAttribute(&ftt.Test{T: t}, daText)
		want := Dimensions{
			"hw-wireless":     {"wireless_test1"},
			"label-wifi_chip": {"wireless_test1"},
		}
		got, err := ConvertAll(&da, &fc)
		if err != nil {
			t.Fatalf("ConvertAll failed: %s", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ConvertAll returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("convert label with existing correct field path - filter based on component; array of values", func(t *testing.T) {
		daText := `{
			"id": {
				"value": "hw-storage"
			},
			"aliases": [
				"label-storage"
			],
			"hwidSource": {
				"componentType": "storage",
				"fields": [
					{
						"path": "hwid_label"
					}
				]
			}
		}`
		da := parseDutAttribute(&ftt.Test{T: t}, daText)
		want := Dimensions{
			"hw-storage":    {"storage_test1", "storage_test2", "storage_test3"},
			"label-storage": {"storage_test1", "storage_test2", "storage_test3"},
		}
		got, err := ConvertAll(&da, &fc)
		if err != nil {
			t.Fatalf("ConvertAll failed: %s", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ConvertAll returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("convert label with non-existent field path", func(t *testing.T) {
		daText := `{
			"id": {
				"value": "attr-test"
			},
			"aliases": [
				"label-test"
			],
			"flatConfigSource": {
				"fields": [
					{
						"path": "test.attr.id.value"
					}
				]
			}
		}`
		da := parseDutAttribute(&ftt.Test{T: t}, daText)
		got, err := ConvertAll(&da, &fc)
		if err == nil {
			t.Fatalf("ConvertAll passed without failures")
		}
		if got != nil {
			t.Errorf("The response is not nil: %s", got)
		}
	})
}

func TestGetLabelValues(t *testing.T) {
	t.Parallel()

	b, err := ioutil.ReadFile("test_flat_config.cfg")
	if err != nil {
		t.Fatalf("Error reading test FlatConfig: %s", err)
	}

	var fc payload.FlatConfig
	unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: false}
	if err = unmarshaller.Unmarshal(bytes.NewBuffer(b), &fc); err != nil {
		t.Fatalf("Error unmarshalling test FlatConfig: %s", err)
	}

	ftt.Run("TestGetLabelValues", t, func(t *ftt.Test) {
		t.Run("get label values from a null jsonpath", func(t *ftt.Test) {
			got, err := GetLabelValues("", &fc)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, got, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("jsonpath cannot be empty"))
		})

		t.Run("get label values from a null proto message", func(t *ftt.Test) {
			var nilConfig *payload.FlatConfig
			got, err := GetLabelValues("$.test-path", nilConfig)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, got, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("proto message cannot be empty"))
		})

		t.Run("get label values with a field path - single value", func(t *ftt.Test) {
			got, err := GetLabelValues("$.hw_design.id.value", &fc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{"Test"}))
		})

		t.Run("get label values with a field path - multiple values", func(t *ftt.Test) {
			got, err := GetLabelValues("$.hw_design.configs[:].hardware_features.camera.devices[:].ids[:]", &fc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{
				"test-camera-id-1",
				"test-camera-id-2",
				"test-camera-id-3",
			}))
		})

		t.Run("get label values with a field path - no matching value", func(t *ftt.Test) {
			got, err := GetLabelValues("$.hw_design_config.hardware_features.embedded_controller.ec_type", &fc)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unsupported value type"))
			assert.Loosely(t, got, should.BeNil)
		})
	})
}

func TestGetLabelNames(t *testing.T) {
	t.Parallel()

	daText := `{
		"id": {
			"value": "attr-design"
		},
		"aliases": [
			"attr-model",
			"label-model"
		],
		"flatConfigSource": {
			"fields": [
				{
					"path": "hw_design.id.value"
				}
			]
		}
	}`

	ftt.Run("TestGetLabelNames", t, func(t *ftt.Test) {
		t.Run("get label names from a normal DutAttribute", func(t *ftt.Test) {
			da := parseDutAttribute(t, daText)
			got, err := GetLabelNames(&da)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{"attr-design", "attr-model", "label-model"}))
		})

		t.Run("get label names from a DutAttribute with no ID", func(t *ftt.Test) {
			da := parseDutAttribute(t, daText)
			da.Id.Value = ""
			got, err := GetLabelNames(&da)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, got, should.BeNil)
		})
	})
}

func TestParseLabelValuesToArray(t *testing.T) {
	t.Parallel()

	ftt.Run("TestParseLabelValuesToArray", t, func(t *ftt.Test) {
		t.Run("get label names values from []interface{} - string castable", func(t *ftt.Test) {
			var labelVals []interface{}
			labelVals = append(labelVals, "label-1", "label-2")

			got, err := ParseLabelValuesToArray(labelVals)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{"label-1", "label-2"}))
		})

		t.Run("get label names values from []interface{} - string not castable", func(t *ftt.Test) {
			var labelVals []interface{}
			labelVals = append(labelVals, []string{"label-1"}, []string{"label-2"})

			got, err := ParseLabelValuesToArray(labelVals)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("cannot cast to string"))
			assert.Loosely(t, got, should.Match([]string(nil)))
		})

		t.Run("get label names values from struct - string not castable", func(t *ftt.Test) {
			type testStruct struct {
				val1 float64
				val2 string
			}
			labelVals := testStruct{10, "test"}

			got, err := ParseLabelValuesToArray(labelVals)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("cannot cast to string"))
			assert.Loosely(t, got, should.Match([]string(nil)))
		})

		t.Run("get label names values from []interface{} - boolean castable", func(t *ftt.Test) {
			var labelVals interface{} = true

			got, err := ParseLabelValuesToArray(labelVals)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{"true"}))
		})

		t.Run("get label names values from []interface{} - float64 castable", func(t *ftt.Test) {
			var labelVals interface{} = 1238764.987

			got, err := ParseLabelValuesToArray(labelVals)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{"1238764.987"}))
		})
	})
}

func TestGetFlatConfigLabelValuesStr(t *testing.T) {
	t.Parallel()

	b, err := ioutil.ReadFile("test_flat_config.cfg")
	if err != nil {
		t.Fatalf("Error reading test FlatConfig: %s", err)
	}

	var fc payload.FlatConfig
	unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: false}
	if err = unmarshaller.Unmarshal(bytes.NewBuffer(b), &fc); err != nil {
		t.Fatalf("Error unmarshalling test FlatConfig: %s", err)
	}

	ftt.Run("GetLabelValues", t, func(t *ftt.Test) {
		t.Run("convert label with existing correct field path - single value", func(t *ftt.Test) {
			got, err := GetLabelValues("$.hw_design.id.value", &fc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.NotBeNil)
			assert.Loosely(t, got, should.Match([]string{"Test"}))
		})
	})
}

func TestGetProtoExistence(t *testing.T) {
	t.Parallel()

	b, err := ioutil.ReadFile("test_flat_config.cfg")
	if err != nil {
		t.Fatalf("Error reading test FlatConfig: %s", err)
	}

	var fc payload.FlatConfig
	unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: false}
	if err = unmarshaller.Unmarshal(bytes.NewBuffer(b), &fc); err != nil {
		t.Fatalf("Error unmarshalling test FlatConfig: %s", err)
	}

	ftt.Run("TestGetProtoExistence", t, func(t *ftt.Test) {
		t.Run("get proto existence using a null jsonpath", func(t *ftt.Test) {
			got, err := GetProtoExistence("", &fc)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, got, should.BeFalse)
			assert.Loosely(t, err.Error(), should.ContainSubstring("jsonpath cannot be empty"))
		})

		t.Run("get proto existence using a null proto message", func(t *ftt.Test) {
			var nilConfig *payload.FlatConfig
			got, err := GetProtoExistence("test-path", nilConfig)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, got, should.BeFalse)
			assert.Loosely(t, err.Error(), should.ContainSubstring("proto message cannot be empty"))
		})

		t.Run("get proto existence with a field path - single value", func(t *ftt.Test) {
			got, err := GetProtoExistence("hw_design.id.value", &fc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.BeTrue)
		})

		t.Run("get proto existence with a non-terminal field path", func(t *ftt.Test) {
			// $.hw_components[?(@.soc) != null] specifies all objects inside
			// hw_components that contains the field "soc". This exists in the cfg.
			got, err := GetProtoExistence("$.hw_components[?(@.soc != null)]", &fc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.BeTrue)
		})

		t.Run("get proto existence with a non-terminal field path with no match", func(t *ftt.Test) {
			// $.hw_components[?(@.soc) != null] specifies all objects inside
			// hw_components that contains the field "society". This does not exist
			// and should evaluate to an empty slice.
			got, err := GetProtoExistence("$.hw_components[?(@.society != null)]", &fc)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, got, should.BeFalse)
		})

		t.Run("get proto existence with a field path - no matching value", func(t *ftt.Test) {
			got, err := GetProtoExistence("hw_design_config.hardware_features.embedded_controller.ec_type", &fc)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("unknown parameter"))
			assert.Loosely(t, got, should.BeFalse)
		})
	})
}

func TestCombineDims(t *testing.T) {
	t.Parallel()

	ftt.Run("TestCombineDims", t, func(t *ftt.Test) {
		t.Run("happy path - combine dimensions", func(t *ftt.Test) {
			d1 := Dimensions{
				"attr-design": {"Test"},
				"attr-model":  {"Test"},
			}

			d2 := Dimensions{
				"label-model": {"Test"},
			}

			got := CombineDims(d1, d2)
			assert.Loosely(t, got, should.Match(Dimensions{
				"attr-design": {"Test"},
				"attr-model":  {"Test"},
				"label-model": {"Test"},
			}))
		})

		t.Run("one is empty", func(t *ftt.Test) {
			d1 := Dimensions{}

			d2 := Dimensions{
				"label-model": {"Test"},
			}

			got := CombineDims(d1, d2)
			assert.Loosely(t, got, should.Match(Dimensions{
				"label-model": {"Test"},
			}))
		})

		t.Run("overwrite with second list of dimensions", func(t *ftt.Test) {
			d1 := Dimensions{
				"attr-model":  {"Test"},
				"label-model": {"Test"},
			}

			d2 := Dimensions{
				"label-model": {"Test2"},
			}

			got := CombineDims(d1, d2)
			assert.Loosely(t, got, should.Match(Dimensions{
				"attr-model":  {"Test"},
				"label-model": {"Test2"},
			}))
		})
	})
}
