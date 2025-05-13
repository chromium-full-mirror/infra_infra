// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package decoder

import (
	"fmt"
	"strconv"
	"strings"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/bitarray"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/targetproperties"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/pprinter/json"
)

type ComponentSets struct {
	HwidEncode    int
	componentType string
	components    []db.ComponentInfo
}

type HWID struct {
	Hwid                 string
	Project              string
	Brand                string
	Board                string
	Configless           string
	Encoded_bom_checksum string
	Encoded_bom          string
	Binary_bom           string
	Checksum             string
	Pattern              uint64
	Pattern_name         string
	Image                uint64
	Image_name           string
	Components           map[string][]db.ComponentSet
}

func DecodeHwid(hwid string, hwidDb db.HwidDb) (HWID, error) {
	hwid_parts := strings.Split(hwid, " ")
	project := ""
	brand := ""
	configless := ""
	encoded_bom_checksum := ""
	switch len(hwid_parts) {
	case 2:
		project = hwid_parts[0]
		encoded_bom_checksum = hwid_parts[1]
	case 3:
		project = hwid_parts[0]
		configless = hwid_parts[1]
		encoded_bom_checksum = hwid_parts[2]
	default:
		return HWID{}, fmt.Errorf("A HWID is composed of two or three parts seperated by a space. This HWID has parts:%s", hwid_parts)
	}
	if strings.Contains(project, "-") {
		parts := strings.Split(project, "-")
		project = parts[0]
		brand = parts[1]
	}
	encoded_bom_checksum = strings.ReplaceAll(encoded_bom_checksum, "-", "")
	if len(encoded_bom_checksum) < 3 {
		return HWID{}, fmt.Errorf("Error decoding bom, the bom must be at least 3 characters long:%s", encoded_bom_checksum)
	}
	encode_bom := encoded_bom_checksum[:len(encoded_bom_checksum)-2]
	checksum := encoded_bom_checksum[len(encoded_bom_checksum)-2:]

	board := hwidDb.GetBoardForProject(project)

	binary_bom, err := bitarray.BitArrayFromString535(encode_bom)
	if err != nil {
		return HWID{}, fmt.Errorf("Error decoding bom:%s", err)
	}
	binary_bom.RightTrim()
	// The format of the header of the encoded BOM is based on https://source.corp.google.com/chromeos_public/src/platform/factory/py/hwid/v3/identity.py;l=25
	var pattern_id uint64 = 0
	if binary_bom.GetBitAt(0) != 0 {
		return HWID{}, fmt.Errorf("only encoding patterrn 0 is implemented")
	}
	image_id, err := binary_bom.GetRange(bitarray.BitRange{
		Start:  1,
		Length: 4,
	})
	if err != nil {
		return HWID{}, err
	}
	bom_offset := 5
	descriptor, err := hwidDb.GetDescriptor(project)
	if err != nil {
		return HWID{}, fmt.Errorf("#1 Unable to decode HWID: %w", err)
	}
	pattern, err := descriptor.GetPattern(int(image_id))
	if err != nil {
		return HWID{}, fmt.Errorf("#2 Unable to decode HWID: %w", err)
	}
	components := map[string][]db.ComponentSet{}
	for field_name, field := range pattern.Fields {
		if field_name == "region_field" {
			// invalidated region decoding
			components[field_name] = []db.ComponentSet{
				{
					PropertyType:     "region",
					ComponentsValues: []db.ComponentInfo{{}},
				},
			}
			continue
		}
		field_value, err := binary_bom.GetUInt64WithOffset(field.BitMask, bom_offset)
		if err != nil {
			return HWID{}, fmt.Errorf("#3 Unable to decode HWID field %s: %w", field_name, err)
		}
		component_set, err := field.DecodeValue(int(field_value))
		if err != nil {
			return HWID{}, fmt.Errorf("Could not decode HWID value: %w", err)
		}

		components[field_name] = component_set
	}

	return HWID{
		Hwid:                 hwid,
		Project:              project,
		Brand:                brand,
		Board:                board,
		Configless:           configless,
		Encoded_bom_checksum: encoded_bom_checksum,
		Encoded_bom:          encode_bom,
		Binary_bom:           binary_bom.ToString(),
		Checksum:             checksum,
		Pattern:              pattern_id,
		Pattern_name:         descriptor.EncodingPatterns[int(pattern_id)],
		Image:                image_id,
		Image_name:           descriptor.ImageIds[int(image_id)],
		Components:           components,
	}, nil
}

func (hwid *HWID) ToPropertiesBag() (targetproperties.TargetPropertiesValues, error) {
	properties := targetproperties.CreateTargetPropertiesValues()

	if hwid.Brand != "" {
		properties.AddPropertyValue("brand", db.HwidSource, hwid.Brand)
	}
	properties.AddPropertyValue("project", db.HwidSource, hwid.Project)
	properties.AddPropertyValue("board", db.HwidSource, hwid.Board)
	properties.AddPropertyValue("image", db.HwidSource, hwid.Image_name)
	for k, c := range hwid.Components {
		err := collectProperties(c, k, &properties)
		if err != nil {
			return properties, errors.JoinError(fmt.Sprintf("Error in collecting properties of HWID:%s", hwid.Hwid), err)
		}
	}
	return properties, nil
}

// todo
const nbComponentsSuffix = "_nb_components"

func collectProperties(set []db.ComponentSet, fieldType string, props *targetproperties.TargetPropertiesValues) error {
	for _, cs := range set {
		props.AddPropertyValue(fieldType, db.HwidSource, cs.GetId())
		props.AddPropertyValue(fieldType+nbComponentsSuffix, db.HwidSource, strconv.Itoa(len(cs.ComponentsValues)))
		for _, comp := range cs.ComponentsValues {
			for propName, prop := range comp.Properties {
				err := collectPropertyValueProperties(props, fieldType+"_"+propName, db.HwidSource, prop)
				if err != nil {
					return errors.JoinError(fmt.Sprintf("Error in property %s", propName), err)
				}
			}
		}
	}
	return nil
}

func collectPropertyValueProperties(props *targetproperties.TargetPropertiesValues, prop_id string, source string, prop any) error {
	switch v := prop.(type) {
	case bool:
		props.AddPropertyValue(prop_id, source, strconv.FormatBool(v))
	case string:
		props.AddPropertyValue(prop_id, source, strings.ToValidUTF8(v, "x"))
	case int:
		props.AddPropertyValue(prop_id, source, strconv.FormatInt(int64(v), 10))
	case float64:
		props.AddPropertyValue(prop_id, source, strconv.FormatFloat(v, 'f', 2, 64))
	case map[string]any:
		for kk, vv := range v {
			collectPropertyValueProperties(props, prop_id+"_"+kk, source, vv)
		}
	case nil:
		// a field with nil is ignored. It is equivalent to the field not being
		// present.
		return nil
	default:
		json, err := json.FormatStructAsJson(prop)
		if err != nil {
			return errors.NewErrorf("Error:%s", err)
		}
		return errors.NewErrorf("Unflattened prop type:%T value:%v", prop, json)
	}
	return nil
}
