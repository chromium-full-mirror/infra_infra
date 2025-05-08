// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package swarming

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/PaesslerAG/jsonpath"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/chromiumos/config/go/test/api"
)

// LabelMarshaler contains the json marshaling params.
var LabelMarshaler = jsonpb.Marshaler{
	EnumsAsInts:  false,
	EmitDefaults: true,
	Indent:       "  ",
	OrigName:     true,
}

// Dimensions is the type for Swarming dimensions.
type Dimensions map[string][]string

// ConvertAll converts one DutAttribute label to multiple Swarming labels.
//
// The converted labels are returned in a map where keys are label names and
// values are string arrays of the label values. Label labelNames are the
// DutAttribute ID and the aliases listed.
func ConvertAll(dutAttr *api.DutAttribute, flatConfig *payload.FlatConfig) (Dimensions, error) {
	labelNames, err := GetLabelNames(dutAttr)
	if err != nil {
		return nil, err
	}

	// Construct and try each path defined in DutAttribute. Tried in order. First
	// path to return a value will be used.
	jsonPaths, err := ConstructJsonPaths(dutAttr)
	if err != nil {
		return nil, err
	}

	for _, p := range jsonPaths {
		valsArr, err := GetLabelValues(p, flatConfig)
		if err != nil {
			return nil, err
		}
		if len(valsArr) != 0 {
			return FormLabels(labelNames, valsArr)
		}
	}
	return nil, errors.New("no supported config source found")
}

// FormLabels pairs label names with the label values.
//
// FormLabels takes each label name from labelNames as a key. The values of the
// label are stored as an array of string. FormLabels returns a key-value map.
func FormLabels(labelNames []string, values []string) (Dimensions, error) {
	// Exhausted all possible paths defined in DutAttribute. If valuesStr is empty,
	// then no values found.
	if len(values) == 0 {
		return nil, errors.New("no label values found in config source found")
	}

	if len(labelNames) == 0 {
		return nil, errors.New("no labels can be generated")
	}

	dims := make(Dimensions)
	for _, label := range labelNames {
		dims[label] = values
	}
	return dims, nil
}

// GetLabelNames extracts all possible label names from a DutAttribute.
//
// For each DutAttribute, the main label name is defined by its ID value. In
// addition, users can define other aliases. GetLabelNames will return all as
// valid label names. The first label is always the main label as defined by the
// ID value.
func GetLabelNames(dutAttr *api.DutAttribute) ([]string, error) {
	name := dutAttr.GetId().GetValue()
	if name == "" {
		return nil, errors.New("DutAttribute has no ID")
	}
	return append([]string{name}, dutAttr.GetAliases()...), nil
}

// GetLabelValues takes a path and returns the proto values.
//
// It uses a jsonpath string to try to find corresponding values in a proto. It
// returns a string array of the values found.
func GetLabelValues(jsonGetPath string, pm proto.Message) ([]string, error) {
	if jsonGetPath == "" {
		return nil, errors.New("jsonpath cannot be empty")
	}

	pmJson, err := parseProtoIntoJson(pm)
	if err != nil {
		return nil, err
	}

	labelVals, err := jsonpath.Get(jsonGetPath, pmJson)
	if err != nil {
		return nil, err
	}
	return ParseLabelValuesToArray(labelVals)
}

// GetProtoExistence takes a path and returns the whether the proto exists.
//
// It uses a jsonpath string to try to find corresponding values in a proto. If
// the destination proto is not nil, then it exists and the method returns true.
func GetProtoExistence(jsonGetPath string, pm proto.Message) (bool, error) {
	if jsonGetPath == "" {
		return false, errors.New("jsonpath cannot be empty")
	}

	pmJson, err := parseProtoIntoJson(pm)
	if err != nil {
		return false, err
	}

	// Get only succeeds if the path is valid
	res, err := jsonpath.Get(jsonGetPath, pmJson)
	if err != nil {
		return false, err
	}
	return !isNil(res), nil
}

// parseProtoIntoJson parses a proto message into a JSON interface.
func parseProtoIntoJson(pm proto.Message) (any, error) {
	if reflect.ValueOf(pm).IsNil() {
		return nil, errors.New("proto message cannot be empty")
	}

	js, err := LabelMarshaler.MarshalToString(pm)
	if err != nil {
		return nil, err
	}

	pmJson := any(nil)
	err = json.Unmarshal([]byte(js), &pmJson)
	if err != nil {
		return nil, err
	}
	return pmJson, nil
}

// ParseLabelValuesToArray takes label values and returns a string array.
//
// It takes an interface of label values parsed from a json object and returns a
// an array of the values casted to string. The interfaces supported are
// primitive types and iterable interfaces.
func ParseLabelValuesToArray(labelVals any) ([]string, error) {
	var rsp []string
	switch x := labelVals.(type) {
	case []any:
		for _, i := range x {
			i, ok := i.(string)
			if !ok {
				return nil, fmt.Errorf("cannot cast to string: %s", i)
			}
			rsp = append(rsp, i)
		}
	case bool:
		rsp = append(rsp, strconv.FormatBool(labelVals.(bool)))
	case float64:
		rsp = append(rsp, strconv.FormatFloat(labelVals.(float64), 'f', -1, 64))
	default:
		var ok bool
		val, ok := labelVals.(string)
		if !ok {
			return nil, fmt.Errorf("cannot cast to string: %s", val)
		}
		rsp = append(rsp, val)
	}
	return rsp, nil
}

// ConstructJsonPaths returns config paths defined by a DutAttribute.
//
// It takes a DutAttribute and returns an array of field paths defined in
// jsonpath syntax. The sources that are currently supported are:
//  1. FlatConfigSource
//  2. HwidSource
func ConstructJsonPaths(dutAttr *api.DutAttribute) ([]string, error) {
	if dutAttr.GetFlatConfigSource() != nil {
		return generateFlatConfigSourcePaths(dutAttr), nil
	} else if dutAttr.GetHwidSource() != nil {
		return generateHwidSourcePaths(dutAttr), nil
	}
	return nil, errors.New("no supported config source found")
}

// generateFlatConfigSourcePaths returns config paths defined by a DutAttribute.
//
// It takes a DutAttribute and returns an array of FlatConfigSource field paths
// strings defined in jsonpath syntax.
func generateFlatConfigSourcePaths(dutAttr *api.DutAttribute) []string {
	var rsp []string
	for _, f := range dutAttr.GetFlatConfigSource().GetFields() {
		rsp = append(rsp, fmt.Sprintf("$.%s", f.GetPath()))
	}
	return rsp
}

// generateHwidSourcePaths returns config paths defined by a DutAttribute.
//
// It takes a DutAttribute and returns an array of HwidSource field paths
// strings defined in jsonpath syntax.
func generateHwidSourcePaths(dutAttr *api.DutAttribute) []string {
	var rsp []string
	componentType := dutAttr.GetHwidSource().GetComponentType()
	for _, f := range dutAttr.GetHwidSource().GetFields() {
		rsp = append(rsp, fmt.Sprintf(`$.hw_components[?(@.%s != null)].%s`, componentType, f.GetPath()))
	}
	return rsp
}

// isNil takes an unknown interface and determines the nil-ness of it.
func isNil(i any) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil() || reflect.ValueOf(i).Len() == 0
	}
	return false
}

// CombineDims takes two maps of dimensions and combines them.
//
// CombineDims will overwrite the first map with the second map when a key
// collides. A Dimensions map is returned.
func CombineDims(d1 Dimensions, d2 Dimensions) Dimensions {
	for k, v := range d2 {
		d1[k] = v
	}
	return d1
}
