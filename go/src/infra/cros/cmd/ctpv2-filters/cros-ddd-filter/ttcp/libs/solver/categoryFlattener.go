// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The functions FlattenCategoryExpression provides a
// mechanism to transform any CategoryExpression into a EnumeratedCategory.
// The flattening mechanism replaces any named expression or category with its
// value recursively.

package solver

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
)

func FlattenCategoryExpression(
	exp *ttcpSyntax.CategoryExpression,
	collection *ttcpSyntax.Collection,
	previouslyEncounteredNamedCategories []string) (*ttcpSyntax.EnumeratedCategory, error) {
	return flattenCategoryExpression(exp, collection, []string{})
}

// flattenCategoryExpression flattens an ttcp.CategoryExpression by transforming
// category referenced by name into their values are flattening recursively.
func flattenCategoryExpression(
	exp *ttcpSyntax.CategoryExpression,
	collection *ttcpSyntax.Collection,
	previouslyEncounteredNamedCategories []string) (*ttcpSyntax.EnumeratedCategory, error) {
	var val *ttcpSyntax.EnumeratedCategory
	var err error = nil
	switch typedBody := exp.GetBody().(type) {
	case *ttcpSyntax.CategoryExpression_Name:
		cat, ok := collection.Categories[typedBody.Name]
		if !ok {
			return nil, errors.NewErrorf("Unknown category:%s", typedBody.Name)
		}
		val, err = flattenCategory(cat, collection, append(previouslyEncounteredNamedCategories, typedBody.Name))
	case *ttcpSyntax.CategoryExpression_Value:
		val, err = flattenCategory(typedBody.Value, collection, previouslyEncounteredNamedCategories)
	default:
		return &ttcpSyntax.EnumeratedCategory{}, errors.NewErrorf("The evaluator does not implement the evaluation for %T", exp.GetBody())
	}
	if err != nil {
		return &ttcpSyntax.EnumeratedCategory{}, errors.JoinError(fmt.Sprintf("Could not flatten category expression:\n%s", protojson.Format(exp)), err)
	}
	return val, nil
}

// flattenCategory flattens ttcp.Category objects into ttcp.EnumerateCategory and recursively flatten the contents.
func flattenCategory(category *ttcpSyntax.Category, collection *ttcpSyntax.Collection, previouslyEncounteredNamedCategories []string) (*ttcpSyntax.EnumeratedCategory, error) {
	if category == nil {
		return nil, errors.NewError("The category argument is nil.")
	}
	if category.Category == nil {
		return nil, errors.NewError("The ttcpSyntax.Category's category field is nil")
	}
	switch typedExp := category.Category.(type) {
	case *ttcpSyntax.Category_Combinatorial:
		if typedExp.Combinatorial == nil {
			return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("Category_Combinatorial is missing the field Combinatorial.")
		}
		return flattenCombinatorialCategory(typedExp.Combinatorial, collection, []string{})
	case *ttcpSyntax.Category_Enumerated:
		if typedExp.Enumerated == nil {
			return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("Category_Enumerated is missing the field Enumerated.")
		}
		return flattenEnumeratedCategory(typedExp.Enumerated, collection, []string{})
	case *ttcpSyntax.Category_Union:
		if typedExp.Union == nil {
			return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("Category_Union is missing the field Enumerated.")
		}
		return flattenUnionCategory(typedExp.Union, collection, []string{})
	default:
		return &ttcpSyntax.EnumeratedCategory{}, errors.NewErrorf("The evaluator does not implement the evaluation for %T", typedExp)
	}
}

// flattenCombinatorialCategory flatens a ttcpSyntax.CombinatorialCategory into a ttcpSyntax.EnumeratedCategory
func flattenCombinatorialCategory(exp *ttcpSyntax.CombinatorialCategory, collection *ttcpSyntax.Collection, previouslyEncounteredNamedCategories []string) (*ttcpSyntax.EnumeratedCategory, error) {
	subCategories := []*ttcpSyntax.EnumeratedCategory{}
	for _, subItem := range exp.Subcategories {
		switch typedItem := subItem.Body.(type) {
		case *ttcpSyntax.CategoryExpression_Name:
			cat, ok := collection.Categories[typedItem.Name]
			if !ok {
				return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("Could not retreive value of named category: " + typedItem.Name)
			}
			flatItem, err := flattenCategory(cat, collection, append(previouslyEncounteredNamedCategories, typedItem.Name))
			if err != nil {
				return &ttcpSyntax.EnumeratedCategory{}, errors.NewErrorf("Could not flatten Combinatorial catergory item: %s err: %s", subItem, err)
			}
			subCategories = append(subCategories, flatItem)
		case *ttcpSyntax.CategoryExpression_Value:
			flatItem, err := flattenCategory(typedItem.Value, collection, previouslyEncounteredNamedCategories)
			if err != nil {
				return &ttcpSyntax.EnumeratedCategory{}, errors.NewErrorf("Could not flatten Combinatorial catergory item: %s err: %s", subItem, err)
			}
			subCategories = append(subCategories, flatItem)
		default:
			return &ttcpSyntax.EnumeratedCategory{}, errors.NewErrorf("The evaluator does not implement the flattening for %T", typedItem)
		}
	}
	if len(subCategories) == 0 {
		return &ttcpSyntax.EnumeratedCategory{}, nil
	}
	expressions := []*nameExpression{}
	for i, expItem := range subCategories[0].Classes {
		expressions = append(expressions,
			&nameExpression{
				name: "Comb_" + strconv.Itoa(i),
				exp:  expItem.GetValue().Expression,
			})
	}
	for _, subCat := range subCategories[1:] {
		combinedExpressions := []*nameExpression{}
		for _, currentExp := range expressions {
			for _, nextExpItem := range subCat.Classes {
				nextExp := nextExpItem.GetValue()
				combinedExpressions = append(combinedExpressions,
					&nameExpression{
						exp: &ttcpSyntax.Expression{
							Operator: &ttcpSyntax.Expression_And{
								And: &ttcpSyntax.And{
									SubExpressions: []*ttcpSyntax.Expression{currentExp.exp, nextExp.GetExpression()}}}}})
			}
		}
		expressions = combinedExpressions
	}
	res := []*ttcpSyntax.ClassExpression{}
	for _, x := range expressions {
		res = append(res, &ttcpSyntax.ClassExpression{
			Body: &ttcpSyntax.ClassExpression_Value{
				Value: &ttcpSyntax.Class{
					Expression: x.exp,
				}}})
	}
	return &ttcpSyntax.EnumeratedCategory{
		Classes: res,
	}, nil
}

type nameExpression struct {
	name string
	exp  *ttcpSyntax.Expression
}

func flattenEnumeratedCategory(cat *ttcpSyntax.EnumeratedCategory, collection *ttcpSyntax.Collection, previouslyEncounteredNamedCategories []string) (*ttcpSyntax.EnumeratedCategory, error) {
	if cat == nil {
		return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("The EnumeratedCategory cat is nil.")
	}
	if cat.Classes == nil {
		return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("The field Classes of the EnumeratedCategory cat is nil.")
	}
	res := &ttcpSyntax.EnumeratedCategory{}
	for _, subCat := range cat.Classes {
		switch typedItem := subCat.Body.(type) {
		case *ttcpSyntax.ClassExpression_Name:
			cat, ok := collection.Classes[typedItem.Name]
			if !ok {
				return nil, errors.NewErrorf("Class %s is not present in the collection.", typedItem.Name)
			}
			res.Classes = append(res.Classes,
				&ttcpSyntax.ClassExpression{
					Body: &ttcpSyntax.ClassExpression_Value{
						Value: cat}})
		case *ttcpSyntax.ClassExpression_Value:
			if err := isClassExpression_ValueComplet(typedItem); err != nil {
				return res, err
			}
			res.Classes = append(res.Classes,
				&ttcpSyntax.ClassExpression{
					Body: &ttcpSyntax.ClassExpression_Value{
						Value: typedItem.Value}})
		default:
			return res, errors.NewError("A ClassExpression is missing a body field.")
		}
	}
	return res, nil
}

func flattenUnionCategory(cat *ttcpSyntax.UnionCategory, collection *ttcpSyntax.Collection, previouslyEncounteredNamedCategories []string) (*ttcpSyntax.EnumeratedCategory, error) {
	if cat == nil {
		return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("The UnionCategory cat is nil.")
	}
	if cat.Subcategories == nil {
		return &ttcpSyntax.EnumeratedCategory{}, errors.NewError("The field Subcategories of the UnionCategory cat is nil.")
	}
	res := []*ttcpSyntax.ClassExpression{}
	for _, subCat := range cat.Subcategories {
		flattenCategory, error := flattenCategoryExpression(
			subCat,
			collection,
			previouslyEncounteredNamedCategories)
		if error != nil {
			return &ttcpSyntax.EnumeratedCategory{}, error
		}
		res = append(res, flattenCategory.Classes...)
	}

	return &ttcpSyntax.EnumeratedCategory{
		Classes: res,
	}, nil
}

func ParseTtcpCategoryExpression(expression string, logger *log.Logger) *ttcpSyntax.CategoryExpression {
	unmarshalOptions := protojson.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: false,
	}
	parsedExpression := &ttcpSyntax.CategoryExpression{}
	err := unmarshalOptions.Unmarshal([]byte(expression), parsedExpression)
	if err != nil {
		if logger != nil {
			// Verbosely note this for CTPv2 logs for now.
			logger.Println("ParseTtcpCategoryExpression FATAL ERROR CALLED ", err)
		}

		log.Fatal(errors.JoinError("Error parsing ttcp category expression.", errors.ApiError(err)))
	}
	return parsedExpression
}

func ParseTtcpClassExpression(expression string, logger *log.Logger) *ttcpSyntax.ClassExpression {
	if expression == "" {
		return nil
	}
	unmarshalOptions := protojson.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: false,
	}
	parsedExpression := &ttcpSyntax.ClassExpression{}
	err := unmarshalOptions.Unmarshal([]byte(expression), parsedExpression)
	if err != nil {
		if logger != nil {
			// Verbosely note this for CTPv2 logs for now.
			logger.Println("ParseTtcpClassExpression FATAL ERROR CALLED ", err)
		}
		log.Fatal(errors.JoinError("Error parsing ttcp class expression.", errors.ApiError(err)))
	}
	return parsedExpression
}

// decodeTtcpCategoryAndClassCollection parses a []byte into a  ttcpSyntax.Collection
func decodeTtcpCategoryAndClassCollection(data []byte) *ttcpSyntax.Collection {
	unmarshalOptions := protojson.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: false,
	}
	parsedExpression := &ttcpSyntax.Collection{}
	err := unmarshalOptions.Unmarshal(data, parsedExpression)
	if err != nil {
		log.Fatal(errors.JoinError("Error parsing categories and classes collection ", errors.ApiError(err)))
	}
	return parsedExpression
}

// loadTtcpCategoryAndClassCollection parses the file path into a  ttcpSyntax.Collection
func loadTtcpCategoryAndClassCollection(path string) *ttcpSyntax.Collection {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(errors.JoinError("Could not read file '"+path+"'", errors.ApiError(err)))
	}
	return decodeTtcpCategoryAndClassCollection(data)
}

// parseTtcpCategoryAndClassCollection parses a list of files references and merges them into
// single ttcpSyntax.Collection it accepts file that are a serialized ttcp.Collection or a zip file
// and attempts to parse all contained .json files are ttcp.Collection objects.
func ParseTtcpCategoryAndClassCollection(cpaths []string, logger *log.Logger) (*ttcpSyntax.Collection, error) {
	if logger != nil {
		logger.Println("parsing classes & categories:", cpaths)
	}
	AllCategoriesAndClasses := &ttcpSyntax.Collection{
		Name:             "All available Classes and Categories",
		Description:      ``,
		Categories:       map[string]*ttcpSyntax.Category{},
		Classes:          map[string]*ttcpSyntax.Class{},
		ReportCategories: map[string]*ttcpSyntax.ReportCategoryList{},
	}
	for _, path := range cpaths {
		switch extension := filepath.Ext(path); extension {
		case ".json":
			err := parseTtcpCategoryAndClassCollectionJson(path, AllCategoriesAndClasses)
			if err != nil {
				return &ttcpSyntax.Collection{}, err
			}
		case ".zip":
			err := parseTtcpCategoryAndClassCollectionZip(path, AllCategoriesAndClasses)
			if err != nil {
				return &ttcpSyntax.Collection{}, err
			}
		default:
			return &ttcpSyntax.Collection{}, errors.NewError("Unknow ttcp collection type:" + extension)
		}
	}

	if logger != nil {
		logger.Printf("nb loaded classes : %d", len(AllCategoriesAndClasses.Classes))
		logger.Printf("nb loaded cats    : %d", len(AllCategoriesAndClasses.Categories))
	}
	return AllCategoriesAndClasses, nil
}

// parseTtcpCategoryAndClassCollectionJson parse a json file that contains a ttcp.Collection object and merges its
// contents into the AllCategoriesAndClasses object.
func parseTtcpCategoryAndClassCollectionJson(path string, AllCategoriesAndClasses *ttcpSyntax.Collection) error {
	categoryAndClasses := loadTtcpCategoryAndClassCollection(path)
	for k, v := range categoryAndClasses.Categories {
		if _, ok := AllCategoriesAndClasses.Categories[k]; ok {
			return errors.NewError("The category " + k + " is defined multiple times.")
		}
		AllCategoriesAndClasses.Categories[k] = v
	}
	for k, v := range categoryAndClasses.Classes {
		if _, ok := AllCategoriesAndClasses.Classes[k]; ok {
			return errors.NewError("The class " + k + " is defined multiple times.")
		}
		AllCategoriesAndClasses.Classes[k] = v
	}
	for k, v := range categoryAndClasses.ReportCategories {
		if _, ok := AllCategoriesAndClasses.ReportCategories[k]; ok {
			return errors.NewError("The report category " + k + " is defined multiple times.")
		}
		AllCategoriesAndClasses.ReportCategories[k] = v
	}
	return nil
}

// parseTtcpCategoryAndClassCollectionZip attempts to parse all .json files in the zip file, parses each of them as
// ttcp.Collection and merges their content into AllCategoriesAndClasses.
func parseTtcpCategoryAndClassCollectionZip(path string, AllCategoriesAndClasses *ttcpSyntax.Collection) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return errors.ApiError(err)
	}
	defer r.Close()

	for _, f := range r.File {
		log.Printf("Parsing section %s", f.Name)
		if !strings.HasSuffix(f.Name, ".json") {
			log.Println("Skipping zip section " + f.Name)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return errors.ApiError(err)
		}
		buffer := bytes.NewBuffer([]byte{})
		_, err = io.Copy(buffer, rc)
		if err != nil {
			return errors.ApiError(err)
		}
		rc.Close()

		categoryAndClasses := decodeTtcpCategoryAndClassCollection(buffer.Bytes())

		for k, v := range categoryAndClasses.Categories {
			if _, ok := AllCategoriesAndClasses.Categories[k]; ok {
				return errors.NewError("The category " + k + " is defined multiple times.")
			}
			AllCategoriesAndClasses.Categories[k] = v
		}
		for k, v := range categoryAndClasses.Classes {
			if _, ok := AllCategoriesAndClasses.Classes[k]; ok {
				return errors.NewError("The class " + k + " is defined multiple times.")
			}
			AllCategoriesAndClasses.Classes[k] = v
		}
		for k, v := range categoryAndClasses.ReportCategories {
			if _, ok := AllCategoriesAndClasses.ReportCategories[k]; ok {
				return errors.NewError("The report category " + k + " is defined multiple times.")
			}
			AllCategoriesAndClasses.ReportCategories[k] = v
		}
	}
	return nil
}
