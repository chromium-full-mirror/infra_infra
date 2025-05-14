// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package targetproperties

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/pprinter/json"
)

type PropertyValue struct {
	Source string
	Values []any
}

func (pValue *PropertyValue) Clone() *PropertyValue {
	values := make([]any, len(pValue.Values))
	copy(values, pValue.Values)
	result := &PropertyValue{
		Source: pValue.Source,
		Values: values,
	}
	return result
}

func (prop *PropertyValue) GetSingleStringValue() (string, error) {
	if len(prop.Values) > 1 {
		return "", errors.NewErrorf("Property has more than a single value")
	}
	str, ok := prop.Values[0].(string)
	if ok {
		return str, nil
	}
	return "", errors.NewError("The value is not a string.")
}

type TargetPropertiesValues struct {
	PropertiesDetails map[string]*PropertyValue
}

func (properties *TargetPropertiesValues) ToString() string {
	str, err := json.FormatStructAsJson(properties)
	if err != nil {
		log.Fatal(errors.ApiError(err))
	}
	return str
}

func (properties *TargetPropertiesValues) Print() {
	log.Print(properties.ToString())
}

func (DeviceProps *TargetPropertiesValues) Clone() *TargetPropertiesValues {
	result := &TargetPropertiesValues{
		PropertiesDetails: map[string]*PropertyValue{},
	}
	for k, v := range DeviceProps.PropertiesDetails {
		result.PropertiesDetails[k] = v.Clone()
	}
	return result
}

func CreateTargetPropertiesValues() *TargetPropertiesValues {
	return &TargetPropertiesValues{
		PropertiesDetails: map[string]*PropertyValue{},
	}
}

func (properties *TargetPropertiesValues) AddPropertyValue(propertyname string, source string, propertyValue any) {
	properties.AddPropertyValues(propertyname, source, []any{propertyValue})
}

func (properties *TargetPropertiesValues) AddPropertyValues(propertyname string, source string, propertyValues []any) {
	details, ok := properties.PropertiesDetails[propertyname]
	if !ok {
		details = &PropertyValue{
			Source: source,
			Values: []any{},
		}
	}

	for _, val := range propertyValues {
		present := false
		for _, existingValue := range details.Values {
			if existingValue == val {
				present = true
				break
			}
		}
		if !present {
			details.Values = append(details.Values, val)
		}
	}
	properties.PropertiesDetails[propertyname] = details
}

func (properties *TargetPropertiesValues) Merge(src *TargetPropertiesValues) {
	for k, v := range src.PropertiesDetails {
		for _, innerv := range v.Values {
			properties.AddPropertyValue(k, v.Source, innerv)
		}
	}
}

type PropertiesAndValuesStore map[string]map[string]any

func (store *PropertiesAndValuesStore) AddPropertyValue(propertyPath string, value string) {
	property, ok := (*store)[propertyPath]
	if !ok {
		property = map[string]any{}
		(*store)[propertyPath] = property
	}
	property[value] = struct{}{}
}

func (store *PropertiesAndValuesStore) ExtractPropertiesAndValue(source any, prefix string) error {
	switch t := source.(type) {
	case string:
		store.AddPropertyValue(prefix, t)
	case int:
		store.AddPropertyValue(prefix, strconv.Itoa(t))
	case float64:
		store.AddPropertyValue(prefix, fmt.Sprintf("%f", t))
	case []string:
		for _, value := range t {
			store.ExtractPropertiesAndValue(value, prefix)
		}
	case map[string][]string:
		for k, v := range t {
			prefixProp := prefix
			if strings.Contains(prefix, "image:") {
				prefixProp += "_"
			}
			store.ExtractPropertiesAndValue(v, prefixProp+k)
		}
	case map[string]any:
		for k, v := range t {
			prefixProp := prefix
			if strings.Contains(prefix, "image:") {
				prefixProp += "_"
			}
			store.ExtractPropertiesAndValue(v, prefixProp+k)
		}
	case map[string]string:
		for k, v := range t {
			prefixProp := prefix
			if strings.Contains(prefix, "image:") {
				prefixProp += "_"
			}
			store.ExtractPropertiesAndValue(v, prefixProp+k)
		}
	default:
		return errors.NewErrorf("Unsuported type:%v", t)
	}
	return nil
}

func (store *PropertiesAndValuesStore) ExportCategories(categories *ttcpSyntax.Collection) {
	for name, values := range *store {
		classes := []*ttcpSyntax.ClassExpression{}
		for value := range values {
			classes = append(classes,
				&ttcpSyntax.ClassExpression{
					Body: &ttcpSyntax.ClassExpression_Value{
						Value: &ttcpSyntax.Class{
							Name: value,
							Expression: &ttcpSyntax.Expression{
								Operator: &ttcpSyntax.Expression_Property{
									Property: &ttcpSyntax.Condition{
										PropertyPath: name,
										Condition: &ttcpSyntax.Condition_StrEqual{
											StrEqual: value,
										},
									},
								},
							},
						},
					},
				},
			)
		}
		category := &ttcpSyntax.Category{
			Name: name,
			Category: &ttcpSyntax.Category_Enumerated{
				Enumerated: &ttcpSyntax.EnumeratedCategory{
					Classes: classes,
				},
			},
		}
		categories.Categories[name] = category
	}
}
