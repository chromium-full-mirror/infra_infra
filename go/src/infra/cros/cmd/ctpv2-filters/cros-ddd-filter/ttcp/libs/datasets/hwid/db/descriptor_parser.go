// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/bitarray"
)

// TODO(b:302139234) Create reliable integration with HWID DB.
var avlComponentTypes = []string{
	"audio_codec",
	"battery",
	"bluetooth",
	"camera",
	"cellular",
	"cpu",
	"dram",
	"display_panel",
	"ec_flash_chip",
	"embedded_controller",
	"ethernet",
	"fingerprint",
	"flash_chip",
	"storage",
	"sku",
	"stylus",
	"tpm",
	"touchpad",
	"touchscreen",
	"usb_hosts",
	"video",
	"wireless"}

// parseComponentReference checks if the the reference has an avl reference format.
// An AVL reference is of the form <type>_<component_id>[_<approval_id>[#<index>]]
// If the reference is an AVL reference the returned value is
//
//	("avl:"+component_id,component type,component approval_id,index)
//
// If the reference is not an AVL reference the returned value is
//
//	(reference,"","","")
func parseComponentReference(reference string) (string, string, string, string) {
	var compReference, compType, approvalId, index = "", "", "", ""
	for _, t := range avlComponentTypes {
		//check if the reference has a prefix of the form `<type>_`
		if strings.HasPrefix(strings.ToLower(reference), t) && (len(reference) > len(t)+1) && reference[len(t)] == '_' {
			// split the avl component type from the reference
			// comptype  = <type>
			// reference = <component_id>[_<approval_id>[#<index>]]
			compType = t
			reference = reference[len(t)+1:]
			// if the reference has an index, split it from the reference
			dash_index := strings.IndexAny(reference, "#")
			if dash_index != -1 {
				// index = <index>
				// reference = <component_id>[_<approval_id>]
				index = reference[dash_index+1:]
				reference = reference[:dash_index]
			}
			// if the refrence has an approval_id, split it from the reference.
			var last_underscore_index = strings.LastIndexAny(reference, "_")
			if last_underscore_index != -1 {
				// approval_id = <approval_id>
				// reference    = <component_id>
				approvalId = reference[last_underscore_index+1:]
				reference = reference[:last_underscore_index]
			}
			// We prefix the component reference with "avl" to make it easier to identify it as an avl component
			// reference in the TTCP expressions.
			reference = "avl:" + reference
			break
		}
	}
	compReference = reference

	return compReference, compType, approvalId, index
}

func ExtractComponentValues(data map[string]any) map[string]map[string]*ComponentInfo {
	result := map[string]map[string]*ComponentInfo{}
	for componentType, componentsData := range data {
		if componentType == "region" {
			val, ok := componentsData.(string)
			if ok && val == "" {
				result[componentType] = map[string]*ComponentInfo{}
			} else {
				components_map := map[string]*ComponentInfo{}
				regionsData := componentsData.(map[string]any)
				for key, value := range regionsData {
					status := ComponentStatus(key)
					for _, region := range castToStringArray(value) {
						components_map[region] = &ComponentInfo{
							ID:     region,
							Status: status,
						}
					}
				}
				result[componentType] = components_map
			}
		} else {
			rawComponentsMap := componentsData.(map[string]any)["items"].(map[string]any)
			componentsMap := map[string]*ComponentInfo{}
			for componentReference, rawCompData := range rawComponentsMap {

				compReference, componentType,
					componentApprovalId, componentIndex := parseComponentReference(componentReference)

				compData := rawCompData.(map[string]any)
				status, ok := compData["status"].(string)
				if !ok {
					status = string(Supported)
				}
				values := compData["values"]
				compInfo := &ComponentInfo{}
				switch values := values.(type) {
				case map[string]any:
					compInfo = &ComponentInfo{
						ID:         compReference,
						Type:       componentType,
						ApprovalId: componentApprovalId,
						Index:      componentIndex,
						Status:     ComponentStatus(status),
						Properties: values,
					}
				case nil:
					compInfo = &ComponentInfo{
						ID:         componentReference,
						Status:     ComponentStatus(status),
						Properties: map[string]any{},
					}
				}
				componentsMap[componentReference] = compInfo
			}
			result[componentType] = componentsMap
		}
	}
	return result
}

func stringIn(s string, strings ...string) bool {
	for _, str := range strings {
		if s == str {
			return true
		}
	}
	return false
}

func ExtractFieldValues(data map[string]any, components map[string]map[string]*ComponentInfo) map[string][]*FieldValue {
	result := map[string][]*FieldValue{}
	for field_name, field_data := range data {
		if stringIn(field_name, "region_field", "new_region_field", "legacy_region_field") {
			field_values := []*FieldValue{}
			regions := castToStringArray(field_data)
			for index, region := range regions {
				component_sets := []*ComponentSet{{
					PropertyType: field_name,
					ComponentsValues: []*ComponentInfo{{
						ID:     region,
						Status: Supported,
					}}}}
				field_values = append(field_values, &FieldValue{
					EncodedBitValue: index,
					ComponentSets:   component_sets,
				})
			}
			result[field_name] = field_values
		} else {
			field_data_map := field_data.(map[any]any)
			field_values := []*FieldValue{}
			for index, field_raw_value := range field_data_map {
				var propertyType string
				var propertyValues []*ComponentInfo
				raw_present_component := field_raw_value.(map[string]any)
				component_sets := []*ComponentSet{}
				for k, v := range raw_present_component {
					propertyType = k
					for _, component_name := range castToStringArray(v) {
						propertyValues = append(propertyValues, components[propertyType][component_name])
					}
					component_sets = append(component_sets, &ComponentSet{
						PropertyType:     propertyType,
						ComponentsValues: propertyValues,
					})
				}
				field_values = append(field_values, &FieldValue{
					EncodedBitValue: index.(int),
					ComponentSets:   component_sets,
				})
			}
			result[field_name] = field_values
		}
	}
	return result
}

func ExtractFields(data []any, field_values map[string][]*FieldValue) map[string]*Field {
	result := map[string]*Field{}

	start := 0
	for _, f := range data {
		fmap := f.(map[string]any)
		if len(fmap) != 1 {
			log.Fatal("A field in the pattern can only be a map a needs to have a single entry. Recieved value:", fmap)
		}
		for k, v := range fmap {
			values, ok := field_values[k]
			if !ok {
				log.Fatal("Could not retreive field values for field ", k)
			}
			length := v.(int)
			ff, ok := result[k]
			if ok {
				if length != 0 {
					if ff.BitMask.Ranges[len(ff.BitMask.Ranges)-1].Length == 0 {
						ff.BitMask.Ranges[len(ff.BitMask.Ranges)-1] = &bitarray.BitRange{Start: start, Length: length}
					} else {
						ff.BitMask.Ranges = append(ff.BitMask.Ranges, &bitarray.BitRange{Start: start, Length: length})
					}
				}
			} else {
				ff = &Field{
					Name:    k,
					BitMask: &bitarray.BitRangeSequence{Ranges: []*bitarray.BitRange{{Start: start, Length: length}}},
					Values:  values,
				}
			}
			result[k] = ff
			start += length
		}
	}

	return result
}

func ExtractPatterns(raw_patterns []any, field_values map[string][]*FieldValue) []*PatternStruct {
	result := []*PatternStruct{}
	for _, pp_raw := range raw_patterns {
		pp := pp_raw.(map[string]any)
		result = append(result,
			&PatternStruct{
				ImageIds:       castToArrayInt(pp["image_ids"].([]any)),
				EncodingScheme: pp["encoding_scheme"].(string),
				Fields:         ExtractFields(pp["fields"].([]any), field_values),
			})
	}
	return result
}

func ExtractDescriptor(data map[any]any) *HwidDescriptor {
	components_values := ExtractComponentValues(data["components"].(map[string]any))
	field_values := ExtractFieldValues(data["encoded_fields"].(map[string]any), components_values)
	project := cast_to_string(data["project"])
	brand := cast_to_string(data["brand"])

	return &HwidDescriptor{
		Project:          project,
		Brand:            brand,
		EncodingPatterns: castToMapIntString(data["encoding_patterns"].(map[any]any)),
		ImageIds:         castToMapIntString(data["image_id"].(map[any]any)),
		Pattern:          ExtractPatterns(data["pattern"].([]any), field_values),
		Rules:            data["rules"],
	}
}

func LoadDescriptor(descriptor_path string) *HwidDescriptor {
	data, err := os.ReadFile(descriptor_path)
	if err != nil {
		log.Fatal("could not read hwid descriptor:", descriptor_path)
	}

	parsed_data := map[any]any{}

	err = yaml.Unmarshal(data, &parsed_data)
	if err != nil {
		log.Fatal("Could not parse the file", descriptor_path, " as yaml:", err)
	}
	return ExtractDescriptor(parsed_data)
}
