// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/libs/fleet/boxster/swarming"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// fs is a temporary file system that holds the TleSources mapping file.
//
//go:embed tle_sources.jsonproto
var fs embed.FS

// Convert converts one DutAttribute label to multiple Swarming labels.
//
// For all TleSource labels needed to be converted for UFS, the implementation
// is handled in this file. All other labels uses the Boxster Swarming lib for
// conversion.
func Convert(ctx context.Context, dutAttr *api.DutAttribute, flatConfig *payload.FlatConfig, lse *ufspb.MachineLSE, dutState *chromeosLab.DutState) (swarming.Dimensions, error) {
	if dutAttr == nil {
		return nil, errors.New("DutAttribute cannot be nil")
	}
	if dutAttr.GetTleSource() != nil {
		logging.Debugf(ctx, "Convert: convert TleSource")
		return convertTleSource(ctx, dutAttr, lse, dutState)
	}

	// TODO (b/263153839): Disabling FlatConfig conversion for now. Pending
	// redesign of FlatConfig conversion.
	logging.Debugf(ctx, "Convert: convert FlatConfigSource")
	return nil, errors.New("convert FlatConfigSource is disabled; skipping")
}

// convertFlatConfigSource handles the label conversion of FlatConfig.
func convertFlatConfigSource(ctx context.Context, dutAttr *api.DutAttribute, flatConfig *payload.FlatConfig) (swarming.Dimensions, error) {
	if flatConfig == nil {
		return nil, errors.New("FlatConfig cannot be nil")
	}
	dims, err := swarming.ConvertAll(dutAttr, flatConfig)
	if err != nil {
		return nil, err
	}
	return dims, nil
}

// convertTleSource handles the label conversion of MachineLSE and DutState.
func convertTleSource(ctx context.Context, dutAttr *api.DutAttribute, lse *ufspb.MachineLSE, dutState *chromeosLab.DutState) (swarming.Dimensions, error) {
	labelAliases, err := swarming.GetLabelNames(dutAttr)
	if err != nil {
		return nil, err
	}
	labelName := dutAttr.GetId().GetValue()

	tleSource, err := getTleLabelMapping(labelName)
	if err != nil {
		logging.Warningf(ctx, "fail to find TLE label mapping: %s", err.Error())
		return nil, nil
	}

	switch tleSource.GetSourceType() {
	case ufspb.TleSourceType_TLE_SOURCE_TYPE_DUT_STATE:
		return constructTleLabels(tleSource, labelAliases, dutState)
	case ufspb.TleSourceType_TLE_SOURCE_TYPE_LAB_CONFIG:
		return constructTleLabels(tleSource, labelAliases, lse)
	default:
		return nil, fmt.Errorf("%s is not a valid label source", tleSource.GetSourceType())
	}
}

// getTleLabelMapping gets the predefined label mapping based on a label name.
func getTleLabelMapping(labelName string) (*ufspb.TleSource, error) {
	mapFile, err := fs.ReadFile("tle_sources.jsonproto")
	if err != nil {
		return nil, err
	}

	var tleMappings ufspb.TleSources
	err = jsonpb.Unmarshal(bytes.NewBuffer(mapFile), &tleMappings)
	if err != nil {
		return nil, err
	}

	for _, tleSource := range tleMappings.GetTleSources() {
		if tleSource.GetLabelName() == labelName {
			return tleSource, nil
		}
	}

	return nil, fmt.Errorf("no TLE label mapping found for %s", labelName)
}

// constructTleLabels returns label values of a set of label names.
//
// constructTleLabels retrieves label values from a proto message based on a
// given path. For each given label name, a full label in the form of
// `${name}:val1,val2` is constructed and returned as part of an array.
func constructTleLabels(tleSource *ufspb.TleSource, labelAliases []string, pm proto.Message) (swarming.Dimensions, error) {
	switch tleSource.GetConverterType() {
	case ufspb.TleConverterType_TLE_CONVERTER_TYPE_STANDARD:
		return standardConvert(tleSource, labelAliases, pm)
	case ufspb.TleConverterType_TLE_CONVERTER_TYPE_EXISTENCE:
		return existenceConvert(tleSource, labelAliases, pm)
	default:
		return nil, fmt.Errorf("converter type not valid: %s", tleSource.GetConverterType())
	}
}

// standardConvert takes a field path and retrieves the value from a proto.
//
// standardConvert will do one of 3 things:
//  1. Return the value retrieved directly
//  2. Truncate the value based on a prefix
//  3. Append the value with a predetermined prefix
func standardConvert(tleSource *ufspb.TleSource, labelAliases []string, pm proto.Message) (swarming.Dimensions, error) {
	valsArr, err := swarming.GetLabelValues(fmt.Sprintf("$.%s", tleSource.GetFieldPath()), pm)
	if err != nil {
		return nil, err
	}
	if tleSource.GetStandardConverter().GetAppendPrefix() && tleSource.GetStandardConverter().GetPrefix() != "" {
		valsArr = appendPrefixForLabelValues(tleSource.GetStandardConverter().GetPrefix(), valsArr)
	} else {
		valsArr = truncatePrefixForLabelValues(tleSource.GetStandardConverter().GetPrefix(), valsArr)
	}
	return swarming.FormLabels(labelAliases, valsArr)
}

// truncatePrefixForLabelValues returns label values with prefix truncated.
func truncatePrefixForLabelValues(prefix string, valsArr []string) []string {
	var processed []string
	for _, v := range valsArr {
		if v != "" {
			processed = append(processed, strings.TrimPrefix(v, prefix))
		}
	}
	return processed
}

// appendPrefixForLabelValues returns label values with prefix appended.
func appendPrefixForLabelValues(prefix string, valsArr []string) []string {
	var processed []string
	for _, v := range valsArr {
		if v != "" {
			processed = append(processed, fmt.Sprintf("%s%s", prefix, v))
		}
	}
	return processed
}

// existenceConvert determines the existence of an entity and returns a boolean.
//
// existenceConvert has two usages. Both checks existence based on proto values.
// One checks the existence of an entity by checking the state config. If
// the state of the entity is in an invalid state, then the entity is deemed to
// not exist for the sake of scheduling labels. The other checks if the
// destination of a field path exists or not.
func existenceConvert(tleSource *ufspb.TleSource, labelAliases []string, pm proto.Message) (swarming.Dimensions, error) {
	var exists bool
	var err error
	if !reflect.ValueOf(tleSource.GetExistenceConverter().GetStateExistence()).IsNil() {
		exists = true
		valsArr, err := swarming.GetLabelValues(fmt.Sprintf("$.%s", tleSource.GetFieldPath()), pm)
		if err != nil {
			return nil, err
		}
		// Set to not exist if any state value is invalid
		for _, v := range valsArr {
			for _, invalidState := range tleSource.GetExistenceConverter().GetStateExistence().GetInvalidStates() {
				if v == invalidState {
					exists = false
					break
				}
			}
			if !exists {
				break
			}
		}
	} else {
		exists, err = swarming.GetProtoExistence(fmt.Sprintf("$.%s", tleSource.GetFieldPath()), pm)
		if err != nil {
			return nil, err
		}
	}
	return swarming.FormLabels(labelAliases, []string{strconv.FormatBool(exists)})
}

// ConvertHwidDataLabels converts HwidData hwid_components into test labels.
func ConvertHwidDataLabels(ctx context.Context, hwidData *ufspb.HwidData) (swarming.Dimensions, error) {
	dims := make(swarming.Dimensions)
	for _, label := range hwidData.GetDutLabel().GetLabels() {
		if label.GetName() == "hwid_component" {
			val := strings.Split(label.GetValue(), "/")
			if len(val) != 2 {
				logging.Warningf(ctx, "hwid label value is invalid: %s", label.GetValue())
				continue
			}
			labelName := fmt.Sprintf("hw-%s", strings.Join(strings.Split(val[0], "_"), "-"))
			dims[labelName] = append(dims[labelName], val[1])
		}
	}
	return dims, nil
}
