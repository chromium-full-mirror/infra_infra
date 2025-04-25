// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"fmt"
	"log"
	"sort"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
)

type PropertyDetails struct {
	Source string
	// A property can have multiple values.
	Values map[string]interface{}
}

func (prop *PropertyDetails) GetSingleValue() (string, error) {
	if len(prop.Values) > 1 {
		return "", errors.NewErrorf("Property has more than a single value, %s", prop.Source)
	}
	var val string
	for k := range prop.Values {
		val = k
		break
	}
	return val, nil
}

type DeviceProperties struct {
	PropertiesDetails map[string]PropertyDetails
}

func (props *DeviceProperties) Print() {
	log.Println("Device properties")
	log.Println("=================")
	for propName, details := range props.PropertiesDetails {
		log.Println("  ", propName, " ", details.Source)
		for k := range details.Values {
			log.Println("    ", k)
		}
	}
}

func CreateHwidProperties() DeviceProperties {
	return DeviceProperties{
		PropertiesDetails: map[string]PropertyDetails{},
	}
}

func (properties *DeviceProperties) addPropertyValue(propertyname string, source string, propertyValue string) {
	details, ok := properties.PropertiesDetails[propertyname]
	if !ok {
		details = PropertyDetails{
			Source: source,
			Values: map[string]interface{}{propertyValue: nil},
		}
		properties.PropertiesDetails[propertyname] = details

	} else {

		details.Values[propertyValue] = nil
	}
}

func (properties *DeviceProperties) propertyTypes() []string {
	result := make([]string, len(properties.PropertiesDetails))
	index := 0
	for key := range properties.PropertiesDetails {
		result[index] = key
		index++
	}
	return result
}

func (properties *DeviceProperties) propertyTypesOrdered() []string {
	result := properties.propertyTypes()
	sort.Strings(result)
	return result
}

func (properties *DeviceProperties) removePropertyType(propertyType string) {
	delete(properties.PropertiesDetails, propertyType)
}

func (properties *DeviceProperties) getPropertyValues(propertyName string) ([]string, error) {
	details, ok := properties.PropertiesDetails[propertyName]
	if !ok {
		return nil, fmt.Errorf("Attempt to remove a property type that is not present.")
	}
	result := make([]string, len(details.Values))
	index := 0
	for value := range details.Values {
		result[index] = value
		index++
	}
	return result, nil
}

func (properties *DeviceProperties) exportCategories(collection *ttcpSyntax.Collection) {

	for propertyType, details := range properties.PropertiesDetails {
		// Create the class for all devices who do not bear this property
		propertyNullClass := &ttcpSyntax.Class{
			Name: "HWID:" + propertyType + ":null",
			Expression: &ttcpSyntax.Expression{
				Operator: &ttcpSyntax.Expression_Property{
					Property: &ttcpSyntax.Condition{
						PropertyPath: propertyType,
						Condition: &ttcpSyntax.Condition_Present{
							Present: false,
						},
					},
				},
			},
		}
		propertyNullClassItem := &ttcpSyntax.ClassExpression{
			Body: &ttcpSyntax.ClassExpression_Value{
				Value: propertyNullClass,
			},
		}

		// Create one class for each value of this property type
		classes := []*ttcpSyntax.ClassExpression{}
		for value := range details.Values {
			_, class := propertyValuesToClass(propertyType, value)
			classes = append(classes, &ttcpSyntax.ClassExpression{
				Body: &ttcpSyntax.ClassExpression_Value{Value: class},
			})
		}
		sort.Slice(classes, func(i, j int) bool {
			switch classes[i].Body.(type) {
			case *ttcpSyntax.ClassExpression_Value:
				_, ok := classes[j].Body.(*ttcpSyntax.ClassExpression_Value)
				if !ok {
					return true
				}
				return classes[i].Body.(*ttcpSyntax.ClassExpression_Value).Value.Name <= classes[j].Body.(*ttcpSyntax.ClassExpression_Value).Value.Name
			case *ttcpSyntax.ClassExpression_Name:
				_, ok := classes[j].Body.(*ttcpSyntax.ClassExpression_Value)
				return !ok
			}
			log.Fatal(errors.NewError("Unexpected State"))
			return false
		})

		// Create a category with a class of each distinct value the property can take.
		categoryAllValues := &ttcpSyntax.Category{
			Description: "Category for the device HWID property type " + propertyType +
				", each class matches one of the possible values of the property.",
			Category: &ttcpSyntax.Category_Enumerated{
				Enumerated: &ttcpSyntax.EnumeratedCategory{
					Classes: classes,
				},
			},
		}
		collection.Categories["HWID:"+propertyType+":distinct_values"] = categoryAllValues

		// Create a category with a class of each distinct value the property can take and one class for the device that
		// do not bear this property.
		classesWithNull := make([]*ttcpSyntax.ClassExpression, len(classes)+1)
		copy(classesWithNull, classes)
		classesWithNull[len(classes)] = propertyNullClassItem
		categoryAllValuesAndNull := &ttcpSyntax.Category{
			Description: "Category for the device HWID property type " + propertyType +
				", each class matches one of the possible values of the HWID property and one case for the device" +
				" not bearing this value.",
			Category: &ttcpSyntax.Category_Enumerated{
				Enumerated: &ttcpSyntax.EnumeratedCategory{
					Classes: classesWithNull,
				},
			},
		}
		collection.Categories["HWID:"+propertyType+":distinct_values_and_absent"] = categoryAllValuesAndNull

		// Create a category with only one class  for the device that do not bear this property.
		collection.Categories["HWID:"+propertyType+":absent"] = &ttcpSyntax.Category{
			Description: "Category with only a single class for the devices without the HWID property type " +
				propertyType,
			Category: &ttcpSyntax.Category_Enumerated{
				Enumerated: &ttcpSyntax.EnumeratedCategory{
					Classes: []*ttcpSyntax.ClassExpression{
						propertyNullClassItem,
					},
				},
			},
		}

	}
}

func propertyValuesToClass(propertyType string, value string) (string, *ttcpSyntax.Class) {
	class_name := propertyType + "_class:" + value
	class := &ttcpSyntax.Class{
		Name: "HWID:" + class_name,
		Expression: &ttcpSyntax.Expression{
			Operator: &ttcpSyntax.Expression_Property{
				Property: &ttcpSyntax.Condition{
					PropertyPath: propertyType,
					Condition: &ttcpSyntax.Condition_StrEqual{
						StrEqual: value,
					},
				},
			},
		},
	}
	return class_name, class
}
