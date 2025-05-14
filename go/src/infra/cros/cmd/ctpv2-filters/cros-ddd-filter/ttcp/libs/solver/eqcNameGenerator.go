// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The functions GenerateEqcName provides a
// mechanism to transform a solved class solution to a readable string
// A readable string can only be generated if reportCategory data is provided in
// the applicable class expression in the collection data (ie datasets/static/*.json) .

package solver

import (
	"log"
	"regexp"
	"strings"

	ttcpSolver "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/solver"
	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/deviceinfo"
)

// GenerateEqcName is the entry function to generated the eqc readable name string
func GenerateEqcName(
	classSolution map[deviceinfo.TargetId]*ExtendedSolvedDevice,
	expression *ttcpSyntax.CategoryExpression,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) (string, []*ttcpSolver.EqcCategory, error) {
	return generateEqcName(classSolution, expression, devicesInfo, collection)
}

type EqcCategory struct {
	name  string
	value string
}

// generateEqcName generates the readable eqc named string by iterating through an input class solution slice
// and joining the reportable properties as specified by the collection metadata
func generateEqcName(
	classSolution map[deviceinfo.TargetId]*ExtendedSolvedDevice,
	expression *ttcpSyntax.CategoryExpression,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) (string, []*ttcpSolver.EqcCategory, error) {
	eqcName := ""
	equivClasses := []*ttcpSolver.EqcCategory{}
	for _, value := range classSolution {
		targetId := value.solvedDevice.DeviceId + "|" + value.solvedDevice.ImageId
		eqcClasses, err := getEqcReportVals(targetId, expression, devicesInfo, collection)
		if err != nil {
			return "", []*ttcpSolver.EqcCategory{}, err
		}
		equivClasses = append(equivClasses, eqcClasses...)
		eqcVals := []string{}
		for _, eqc := range equivClasses {
			eqcVals = append(eqcVals, eqc.GetValue())
		}
		if len(eqcVals) > 0 {
			eqcName = strings.Join(eqcVals, "__")
		}
		// Just use the first element in the solution map as the proxy for the readable EqC name
		break
	}
	return eqcName, equivClasses, nil
}

// getEqcReportVals returns a slice containing the values of the reportable properties specified by the
// collections metadata for a user-defined category
func getEqcReportVals(
	targetId string,
	expression *ttcpSyntax.CategoryExpression,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) ([]*ttcpSolver.EqcCategory, error) {
	eqcCategories := []*ttcpSolver.EqcCategory{}
	switch typedBody := expression.GetBody().(type) {
	// If the expression body contains a name field, then check the user-defined categories for reportable fields
	case *ttcpSyntax.CategoryExpression_Name:
		cat, ok := collection.Categories[typedBody.Name]
		if !ok {
			return nil, errors.NewErrorf("Unknown category:%s", typedBody.Name)
		}
		reportCategories, ok := collection.ReportCategories[cat.ReportCategory]
		if !ok {
			log.Printf("Warning: Reporting %s category does not exist, will not provide eqc values", cat.ReportCategory)
		} else {
			if len(reportCategories.Categories) > 0 {
				//flatten report categories
				reportCategoriesList := flattenReportCategories(reportCategories, collection)
				eqcCategories = getEqcReportCategories(reportCategoriesList, targetId, devicesInfo)
			}
		}
	// If the expression body contains a value field, combinational or union categories must be evaluated
	// This enters recursive logic to determine if a user-defined category with reportable categories exists
	case *ttcpSyntax.CategoryExpression_Value:
		eqcCat, err := getEqcFromCategory(typedBody.Value, targetId, devicesInfo, collection)
		if err != nil {
			return eqcCategories, err
		}
		eqcCategories = append(eqcCategories, eqcCat...)
	}
	return eqcCategories, nil
}

// getEqcFromeCategory returns a slice containing the values of the reportable properties specified by the
// collections metadata from Combinational and Union categories
func getEqcFromCategory(
	category *ttcpSyntax.Category,
	targetId string,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) ([]*ttcpSolver.EqcCategory, error) {
	eqcCategories := []*ttcpSolver.EqcCategory{}
	switch typedExp := category.Category.(type) {
	case *ttcpSyntax.Category_Combinatorial:
		if typedExp.Combinatorial == nil {
			return []*ttcpSolver.EqcCategory{}, errors.NewError("Category_Combinatorial is missing the field Combinatorial.")
		}
		eqcCat, err := getEqcFromCombinatorialCategory(typedExp.Combinatorial, targetId, devicesInfo, collection)
		if err != nil {
			return eqcCategories, err
		}
		eqcCategories = append(eqcCategories, eqcCat...)
	case *ttcpSyntax.Category_Union:
		if typedExp.Union == nil {
			return []*ttcpSolver.EqcCategory{}, errors.NewError("Category_Union is missing the field Union.")
		}
		eqcCat, err := getEqcFromUnionCategory(typedExp.Union, targetId, devicesInfo, collection)
		if err != nil {
			return eqcCategories, err
		}
		eqcCategories = append(eqcCategories, eqcCat...)
	}
	return eqcCategories, nil
}

// getEqcFromCombinatorialCategory returns a slice containing the values of the reportable properties specified by the
// collections metadata from Combinational categories
func getEqcFromCombinatorialCategory(
	exp *ttcpSyntax.CombinatorialCategory,
	targetId string,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) ([]*ttcpSolver.EqcCategory, error) {
	return getEqcFromSubCategories(exp.Subcategories, targetId, devicesInfo, collection)
}

// getEqcFromUnionCategory returns a slice containing the values of the reportable properties specified by the
// collections metadata from Union categories
func getEqcFromUnionCategory(
	exp *ttcpSyntax.UnionCategory,
	targetId string,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) ([]*ttcpSolver.EqcCategory, error) {
	return getEqcFromSubCategories(exp.Subcategories, targetId, devicesInfo, collection)
}

// getEqcFromSubCategory returns a slice containing the values of the reportable properties specified by the
// collections metadata from expression sub-categories
func getEqcFromSubCategories(
	subCategories []*ttcpSyntax.CategoryExpression,
	targetId string,
	devicesInfo []*deviceinfo.TargetVariant,
	collection *ttcpSyntax.Collection,
) ([]*ttcpSolver.EqcCategory, error) {
	eqcCategories := []*ttcpSolver.EqcCategory{}
	for _, subItem := range subCategories {
		// Recursive call to determine if the next level has reportable fields
		eqcCats, err := getEqcReportVals(targetId, subItem, devicesInfo, collection)
		if err != nil {
			return eqcCategories, err
		}
		eqcCategories = append(eqcCategories, eqcCats...)
	}
	return eqcCategories, nil
}

// flattenReportCategories recursively flattens the reportCategories metadata for a user-defined category
// The flattened slice of report categories will be used to retrieve the values specified in the collections metadata
func flattenReportCategories(reportCategoriesList *ttcpSyntax.ReportCategoryList, collection *ttcpSyntax.Collection) []*ttcpSyntax.ReportCategory {
	reportCategories := []*ttcpSyntax.ReportCategory{}

	for _, reportCategory := range reportCategoriesList.Categories {
		switch typedBody := reportCategory.GetBody().(type) {
		case *ttcpSyntax.ReportCategory_Name:
			nextReportCategories, ok := collection.ReportCategories[typedBody.Name]
			if !ok {
				log.Printf("Warning: Reporting %s category does not exist, will not provide eqc values", typedBody.Name)
			}
			reportCategories = append(reportCategories, flattenReportCategories(nextReportCategories, collection)...)
		case *ttcpSyntax.ReportCategory_Value:
			reportCategories = append(reportCategories, reportCategory)
		}
	}
	return reportCategories
}

// getEqcReportCategories takes an input slice of reportCategory objects and
// iterates through the inventory devices for a target device to extract the property values that will
// represent the report category values of a class. Property values may also be modified depending on the
// `override` rules specified in the collections metadata for a reportCategory.
// Returns a slice of EqcCategory objects
func getEqcReportCategories(
	reportCategories []*ttcpSyntax.ReportCategory,
	targetId string,
	devicesInfo []*deviceinfo.TargetVariant) []*ttcpSolver.EqcCategory {

	eqcCategories := []*ttcpSolver.EqcCategory{}
	for _, info := range devicesInfo {
		if targetId == string(info.Id()) {
			for _, reportCat := range reportCategories {
				eqcCategory, err := getEqcCategory(info, reportCat)
				if err != nil {
					log.Println(err.Error())
				} else {
					eqcCategories = append(eqcCategories, eqcCategory)
				}
			}
			break
		}
	}
	return eqcCategories
}

// getEqcCategory takes the target deviceInfo and report category information
// of a class to determine the eqcCategory for the target device. nil will be
// returned when there is no report category class information available.
func getEqcCategory(deviceInfo *deviceinfo.TargetVariant, reportCategory *ttcpSyntax.ReportCategory) (*ttcpSolver.EqcCategory, error) {
	if reportCategory == nil {
		return nil, errors.NewError("ReportCategory is empty.")
	}
	categoryVal, err := getEqcCategoryValue(deviceInfo, reportCategory)
	if err != nil {
		return nil, err
	}
	namedCategory := reportCategory.GetValue().Property
	return &ttcpSolver.EqcCategory{Name: namedCategory, Value: categoryVal}, nil
}

// getEqcCategoryValue takes the target deviceInfo and report category information
// of a class to determine the eqcCategory value for the target device. And error will be
// returned when a eqcCategory value can not be determined.
func getEqcCategoryValue(deviceInfo *deviceinfo.TargetVariant, reportCategory *ttcpSyntax.ReportCategory) (string, error) {
	namedCategory := reportCategory.GetValue().Property
	deviceProp, ok := deviceInfo.Properties.PropertiesDetails[namedCategory]
	if !ok {
		return "", errors.NewErrorf("Category %s does not exist in the class", namedCategory)
	}
	if len(deviceProp.Values) == 0 {
		return "", errors.NewErrorf("Category %s has empty property values", namedCategory)
	}
	categoryVal := deviceProp.Values[0].(string)
	// Modify the category value based on the match/replace rule
	nameOverrides := reportCategory.GetValue().GetOverrides()

	// If report category overrides exist, populate the categoryVal on the
	// match/replace rule from the class information
	for _, override := range nameOverrides {
		innerPropName := override.GetProperty()
		innerPropVal := categoryVal
		match := override.GetMatch()
		replace := override.GetReplace()
		if innerPropName != "" {
			innerProp, ok := deviceInfo.Properties.PropertiesDetails[innerPropName]
			if ok && len(innerProp.Values) > 0 {
				innerPropVal = innerProp.Values[0].(string)
			}
		}
		re := regexp.MustCompile(match)
		if re.MatchString(innerPropVal) {
			categoryVal = re.ReplaceAllString(innerPropVal, replace)
			return categoryVal, nil
		}
	}
	// Else return the value of the device property specified in the report
	// category
	return categoryVal, nil
}
