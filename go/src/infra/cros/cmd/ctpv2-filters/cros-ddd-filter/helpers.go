// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"

	ctpApi "go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	solver_proto "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/solver"
	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets"
	buildmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	dlmmetadata "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/dlmmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/deviceinfo"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/solver"
)

type requestTestCaseVariants struct {
	// requesId is an optional label that will be copied into the results so
	// external services can match the request with the results
	requestId string
	// variantsStr is the required 3D/TTCP expression from the test
	variantsStr string
	// optInStr is an optional TTCP class that the devices need to satisfy in order
	//          to be considered as targets of any variant.
	optInStr string
	// optOutStr is an optional TTCP class that the devices need to not satisfy in
	//          order to be considered as targets of any variant.
	optOutStr string
}

// computeVariants process the CliArgs in order to feed them to the TTCP solver.
// testCases is the list of test case to compute variants for.
// pool is the name of the swarming pool to which all the device that will be considered
//
//	for each variant need to belong to. If the pool is "" the pool will be defaulted
//	to the quota pool
func computeVariants(requests []*requestTestCaseVariants, pool string, authHelper common.FilterAuthInterface, logger *log.Logger) (map[string]*solver_proto.SolvedCategory, error) {
	if pool == "" {
		pool = "DUT_POOL_QUOTA"
	}

	// Retrieve the authentication for reaching database.
	tokenSource := authHelper.GetTokenSource(common.CTPv2DockerKeyFileLocations, common.BigqueryScope)

	// retrieve the set of devices in the inventory specified in the options.
	// This involves a large cost to retrieve the inventory from BQ swarming or
	// Databake.
	inventoryInfo := inventory.RetreiveInventoryProperties(true, "",
		&datasets.AllDatasetsResources{
			HwidDB: &db.HwidDbResources{
				DescriptorsPaths: getAllHwidEntries(),
				ProjectIndexPath: "HWID_DB/projects.yaml",
			},
			Buildmetadata: &buildmetadata.BuildMetadataResources{
				Path: "build_metadata.jsonproto",
			},
			Dlmmetadata: &dlmmetadata.DlmResources{
				Path: "dlm_devices.json",
			}},
		pool,
		logger,
		option.WithTokenSource(tokenSource))

	solutions := map[string]*solver_proto.SolvedCategory{}

	for _, request := range requests {
		solution, err := serviceRequest(logger, request, inventoryInfo, pool)
		if err != nil {
			logger.Println("Error during solving: ", err)
			return map[string]*solver_proto.SolvedCategory{}, err
		}
		if solution != nil {
			solutions[request.requestId] = solution
		}
	}

	return solutions, nil
}

var (
	// Type: map[string]*solver_proto.SolvedCategory
	solutionCache = sync.Map{}
	// Lock the solutionCache down by pool and variant.
	// Type: map[string]sync.Mutex{}
	solutionCacheLocksByPoolAndVariant = sync.Map{}
	// Track the expiration of the cache per pool.
	// Type: time.Time
	solutionCacheExpiration = sync.Map{}
)

func serviceRequest(logger *log.Logger, request *requestTestCaseVariants, inventoryInfo []*deviceinfo.TargetVariant, pool string) (*solver_proto.SolvedCategory, error) {
	logger.Println("SERVICING REQUEST: ", request)
	// Load and parse the TTCP request that specifies the dimension constraints on the desired variants
	if request.variantsStr == "" {
		logger.Println("TEST HAD NO TTCP EXPRESSION. WILL BE SKIPPED")
		return nil, nil
	}
	cacheKey := fmt.Sprint(pool, request.variantsStr)
	lock, _ := solutionCacheLocksByPoolAndVariant.LoadOrStore(cacheKey, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	v, solutionFound := solutionCache.Load(cacheKey)
	solutionExpiration, _ := solutionCacheExpiration.LoadOrStore(cacheKey, time.Now().Add(8*time.Hour))
	if solutionFound && time.Now().Before(solutionExpiration.(time.Time)) {
		return v.(*solver_proto.SolvedCategory), nil
	}

	variants := solver.ParseTtcpCategoryExpression(request.variantsStr, logger)
	optIn := solver.ParseTtcpClassExpression(request.optInStr, logger)
	optOut := solver.ParseTtcpClassExpression(request.optOutStr, logger)

	// Compute the solution (Set of requests variants)
	solution, err := solver.EvalExpression(
		variants,
		solver.ClassFilterOnlyTestable,
		inventoryInfo,
		optIn,
		optOut,
		categoriesAndClassesCollection,
		logger,
		true,
		pool)

	if err != nil {
		// This might to need to be handled better if we want to allow partial results
		logger.Println("Error during solving: ", err)
		return nil, err
	}
	solutionCache.Store(cacheKey, solution)
	solutionCacheExpiration.Store(cacheKey, time.Now().Add(8*time.Hour))
	return solution, nil
}

// getAllHwidEntries loads all the HWID db entries that are stored in the container.
func getAllHwidEntries() []string {
	entries, err := filepath.Glob("HWID_DB/*.internal")
	if err != nil {
		log.Fatal("err:", err)
	}
	return entries

}

func isPossible(in *ctpApi.InternalTestplan, board string, variant string, cache map[string]bool, oldProto bool) (bool, map[string]bool) {
	cacheShorthand := board
	if variant != "" {
		cacheShorthand = fmt.Sprintf("%s-%s", board, variant)
	}
	found, value := cache[cacheShorthand]
	if found {
		return value, cache
	}

	if oldProto {

		infos := in.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements()
		for _, HwRequirement := range infos {
			hwDefs := HwRequirement.GetHwRequirements().GetHwDefinition()
			if len(hwDefs) == 0 {
				continue
			}
			hwDef := hwDefs[0]
			inboard := hwDef.GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget()
			invar := hwDef.GetVariant()
			if board == inboard {
				if variant == invar {
					cache[cacheShorthand] = true
					return true, cache
				}
			}
		}
	} else {
		infos := in.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()
		for _, HwRequirement := range infos {
			hwDef := HwRequirement.GetPrimaryTarget().GetSwarmingDef()

			inboard := hwDef.GetDutInfo().GetChromeos().GetDutModel().GetBuildTarget()
			invar := hwDef.GetVariant()
			if board == inboard {
				if variant == invar {
					cache[cacheShorthand] = true
					return true, cache
				}
			}
		}
	}

	cache[cacheShorthand] = false
	return false, cache
}

// getClassAndCatDatasetFiles returns a list of filenames from the specified root and filtered by file extensions
func getClassAndCatDatasetFiles(root string, exts []string) ([]string, error) {
	var files []string
	dirFiles, err := os.ReadDir(root)
	if err != nil {
		return files, fmt.Errorf("failed to read directory %s: %v", root, err)
	}

	for _, file := range dirFiles {
		for _, ext := range exts {
			// NOTE: Temporary workaround to exclude raw DLM datasource which is needed for packaging into
			// the categories and classes zip file but gets picked up at the root level of the container
			if strings.HasSuffix(file.Name(), "."+ext) && file.Name() != "dlm_devices.json" {
				files = append(files, file.Name())
			}
		}
	}
	return files, nil
}

// getTestCaseVariantExpression returns the test case variant expression string with combined dependencies
func getTestCaseVariantExpression(testCase *ctpApi.CTPTestCase, logger *log.Logger) (string, []string) {
	testCaseVarCat := testCase.GetMetadata().GetTestCaseInfo().GetVariantCategory().GetValue()
	dependencies, skippedDeps := getTestCaseDependencies(testCase)
	// Return combine expressions if testcase variantCategory does not exist
	if testCaseVarCat == "" || len(dependencies) == 0 {
		return testCaseVarCat, []string{}
	}
	categoryExpressions := generateDependencyExpressions(dependencies)
	tcVarExp := solver.ParseTtcpCategoryExpression(testCaseVarCat, logger)
	categoryExpressions = append(categoryExpressions, tcVarExp)
	testCaseVarCat = comboCategoryExpressionStr(categoryExpressions)
	return testCaseVarCat, skippedDeps
}

// generateDependencyExpressions returns a sorted list of dependency category expressions from a list of test dependencies
func generateDependencyExpressions(dependencies []string) []*ttcpSyntax.CategoryExpression {
	categoryExpressions := []*ttcpSyntax.CategoryExpression{}
	// Order the dependencies to ensure maintained performance of cached solve expression logic
	sort.Strings(dependencies)
	for _, dep := range dependencies {
		depPropValue := strings.Split(dep, ":")
		if len(depPropValue) != 2 {
			log.Fatalf("err: Dependency Property and Value could not be parsed: %s", dep)
		}
		depProp := depPropValue[0]
		depValue := depPropValue[1]
		expression := &ttcpSyntax.CategoryExpression{
			Body: &ttcpSyntax.CategoryExpression_Value{
				Value: &ttcpSyntax.Category{
					Category: &ttcpSyntax.Category_Enumerated{
						Enumerated: &ttcpSyntax.EnumeratedCategory{
							Classes: []*ttcpSyntax.ClassExpression{
								{
									Body: &ttcpSyntax.ClassExpression_Value{
										Value: &ttcpSyntax.Class{
											Name: fmt.Sprintf("swarming:%s", dep),
											Expression: &ttcpSyntax.Expression{
												Operator: &ttcpSyntax.Expression_Property{
													Property: &ttcpSyntax.Condition{
														PropertyPath: fmt.Sprintf("swarming:%s", depProp),
														Condition: &ttcpSyntax.Condition_StrEqual{
															StrEqual: depValue,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		categoryExpressions = append(categoryExpressions, expression)
	}
	return categoryExpressions
}

// comboCategoryExpressionStr returns a string from the combinational expression of a list category expressions
func comboCategoryExpressionStr(catExpressions []*ttcpSyntax.CategoryExpression) string {
	comboExpression := &ttcpSyntax.CategoryExpression{
		Body: &ttcpSyntax.CategoryExpression_Value{
			Value: &ttcpSyntax.Category{
				Category: &ttcpSyntax.Category_Combinatorial{
					Combinatorial: &ttcpSyntax.CombinatorialCategory{
						Subcategories: catExpressions,
					},
				},
			},
		},
	}
	marshalOptions := protojson.MarshalOptions{
		AllowPartial: false,
	}
	comboExp, err := marshalOptions.Marshal(comboExpression)
	if err != nil {
		log.Fatalf("err: Combinational expression could not be generated: %v", err)
	}
	return string(comboExp)
}

// getTestCaseDependencies returns a list of string dependencies for a CTP test case
func getTestCaseDependencies(testCase *ctpApi.CTPTestCase) (dependencies []string, skippedDeps []string) {
	for _, dep := range testCase.GetMetadata().GetTestCase().GetDependencies() {
		label := dep.GetValue()
		if !strings.Contains(label, ":") {
			skippedDeps = append(skippedDeps, label)
			continue
		} else if strings.Contains(label, "label-") {
			dependencies = append(dependencies, label)
		} else if strings.HasPrefix(label, "dut_name") || strings.HasPrefix(label, "drone") {
			dependencies = append(dependencies, label)
		} else {
			dependencies = append(dependencies, fmt.Sprintf("label-%s", label))
		}
	}
	return dependencies, skippedDeps
}

func removeDupLabelsAndSort(labels []string) []string {
	visited := map[string]bool{}
	filteredLabels := []string{}

	for _, label := range labels {
		if !visited[label] {
			visited[label] = true
			filteredLabels = append(filteredLabels, label)
		}
	}
	sort.Strings(filteredLabels)
	return filteredLabels
}

func generateSchedulingUnitOptions(eqcVariant *solver_proto.SolvedClass,
	testCase *ctpApi.CTPTestCase,
	ctpTargets []*ctpApi.SwarmingDefinition,
	log *log.Logger) *ctpApi.SchedulingUnitOptions {
	eqcHash := solver.GetEqcExpressionHash(eqcVariant)
	dddVariantStr := testCase.GetMetadata().GetTestCaseInfo().GetVariantCategory().GetValue()
	options := &ctpApi.SchedulingUnitOptions{
		State:       ctpApi.SchedulingUnitOptions_ONEOF,
		PublishKeys: []*ctpApi.PublishKey{},
	}

	keyValues := map[string]string{
		"eqcCategoryExpression": dddVariantStr,
		"eqcHash":               fmt.Sprintf("%d", eqcHash),
		"eqcName":               eqcVariant.GetName(),
		"eqcDimensions":         serializeEqcDimensions(eqcVariant.GetDimensions()),
	}

	options.PublishKeys = append(
		options.PublishKeys,
		&ctpApi.PublishKey{
			Subject:   "3D",
			KeyValues: keyValues,
		},
	)
	log.Println("Appending CTP targ.", len(ctpTargets))

	for _, targ := range ctpTargets {
		su := &ctpApi.SchedulingUnit{
			PrimaryTarget: &ctpApi.Target{
				SwarmingDef: targ,
			},
		}
		options.SchedulingUnits = append(options.SchedulingUnits, su)

	}
	return options
}

func serializeEqcDimensions(dimensions []*solver_proto.EqcCategory) string {
	eqcDimensionsMap := map[string]string{}
	for _, dimension := range dimensions {
		eqcDimensionsMap[dimension.GetName()] = dimension.GetValue()
	}
	eqcDimensions, err := json.Marshal(eqcDimensionsMap)
	if err != nil {
		log.Fatal("Error marshalling eqc dimensions:", err)
	}
	return string(eqcDimensions)
}
