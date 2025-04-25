// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"log"
	"strconv"
	"strings"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/pprinter/json"
)

func (descriptor *HwidDescriptor) collectDescriptorPropertyValues(props *DeviceProperties) {
	props.addPropertyValue("brand", HwidSource, descriptor.Brand)
	if descriptor.Project != "" {
		props.addPropertyValue("project", HwidSource, descriptor.Project)
	}
	for _, imageName := range descriptor.ImageIds {
		props.addPropertyValue("image", HwidSource, imageName)
	}
	for _, p := range descriptor.Pattern {
		p.collectPatternPropertyValues(props)
	}
	for _, propertyType := range props.propertyTypes() {
		if strings.HasSuffix(propertyType, nbComponentsSuffix) {
			values, _ := props.getPropertyValues(propertyType)
			if len(values) <= 1 {
				props.removePropertyType(propertyType)
			}
		}
	}
}

func (pattern *PatternStruct) collectPatternPropertyValues(props *DeviceProperties) {
	for fieldType, field := range pattern.Fields {
		for _, fieldValue := range field.Values {
			fieldValue.collectPatternPropertyValues(fieldType, props)
		}
	}
}

const nbComponentsSuffix = "_nb_components"

func (field *FieldValue) collectPatternPropertyValues(fieldType string, props *DeviceProperties) {
	props.addPropertyValue(fieldType, HwidSource, field.GetId())
	for _, set := range field.ComponentSets {
		if set.ComponentsValues == nil {
			continue
		}
		props.addPropertyValue(fieldType+nbComponentsSuffix, HwidSource, strconv.Itoa(len(set.ComponentsValues)))
		for _, comp := range set.ComponentsValues {
			for propName, prop := range comp.Properties {
				collectPropertyValueProperties(props, fieldType+"_"+propName, HwidSource, prop)
			}
		}
	}
}
func collectPropertyValueProperties(props *DeviceProperties, prop_id string, source string, prop interface{}) {
	switch v := prop.(type) {
	case bool:
		props.addPropertyValue(prop_id, source, strconv.FormatBool(v))
	case string:
		props.addPropertyValue(prop_id, source, strings.ToValidUTF8(v, "x"))
	case int:
		props.addPropertyValue(prop_id, source, strconv.FormatInt(int64(v), 10))
	case float64:
		props.addPropertyValue(prop_id, source, strconv.FormatFloat(v, 'f', 2, 64))
	case map[string]interface{}:
		for kk, vv := range v {
			collectPropertyValueProperties(props, prop_id+"_"+kk, source, vv)
		}
	case nil:
		// a field with nil is ignored. It is equivalent to the field not being
		// present.
		return
	default:
		// b/342453504: Field types of nested list/sequences are falling out
		// to the default block. Temporarily changed log.Fatal -> log.Println
		// until the fix has been determined
		json, err := json.FormatStructAsJson(prop)
		if err != nil {
			log.Println("Error:", err)
		}
		log.Printf("Unflattened prop type:%T value:%v", prop, json)
	}
}
