// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	ctpApi "go.chromium.org/chromiumos/config/go/test/api"
	labApi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/solver"
)

var categoriesAndClassesCollection *ttcpSyntax.Collection

func GenerateFilterExecutor() servertemplate.Filter {
	return &Filter3D{}
}

type Filter3D struct {
	servertemplate.FilterBase

	// From argument flags.
	googleApiCredsPath string
}

func (ddd *Filter3D) Init(args []string) error {
	fs := flag.NewFlagSet("3D", flag.ExitOnError)
	fs.StringVar(&ddd.googleApiCredsPath, "creds", "",
		"Path to json file with credential for the Google cloud services. "+
			"If the option is not provided, the service will use the default google api credential finder features.")

	return fs.Parse(args)
}

func (ddd *Filter3D) Executor(req *ctpApi.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*ctpApi.InternalTestplan, error) {

	// Convert InternalTestplan to a list of requestTestCaseVariants
	requests := []*requestTestCaseVariants{}
	tcSkippedDeps := map[string][]string{}
	pool := req.SuiteInfo.SuiteMetadata.Pool
	for _, testCase := range req.TestCases {
		testCaseName := testCase.Name
		dddVariant, skippedDeps := getTestCaseVariantExpression(testCase, log)
		tcSkippedDeps[testCaseName] = skippedDeps
		requests = append(requests, &requestTestCaseVariants{
			requestId:   testCaseName,
			variantsStr: dddVariant,
			optInStr:    "",
			optOutStr:   "",
		})
	}
	// Compute the TTCP variants
	solutions, err := computeVariants(requests, pool, commonParams.AuthHelper, log)
	if err != nil {
		log.Println("Compute Varaints err", err)
		return req, err
	}

	foundCache := make(map[string]bool)
	possible := false

	oldProto := len(req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements()) > 0
	log.Println(fmt.Sprintf("OLD PROTO? %t.", oldProto))

	// Insert the computed variant into the original requests
	for _, testCase := range req.TestCases {
		// For each test case we find the related computed variants and merge the into the test case requirements.
		testCaseName := testCase.Name
		result, ok := solutions[testCaseName]

		if !ok {
			log.Println(fmt.Sprintf("Solution was not found for testCase %s. Skipping SchedulingUnitOption creation.", testCaseName))
			continue
		}

		ctpVariants := []*ctpApi.HWRequirements{}
		newCTPVariants := []*ctpApi.SchedulingUnitOptions{}

		for _, variant := range result.Classes {
			// For each variant we report the set if equivalent swarming definitions that can execute the variant.
			// An equivalent swarming definition if a tuple of device constraints and a build variant software image,
			ctpTargets := []*ctpApi.SwarmingDefinition{}
			for _, possibleTargetOfVariant := range variant.Targets {
				hwid := possibleTargetOfVariant.DeviceId

				// swarming expects board/model to be lowercase.
				board := strings.ToLower(possibleTargetOfVariant.Info.Board)
				model := strings.ToLower(possibleTargetOfVariant.Info.Model)
				imageVariant := possibleTargetOfVariant.Info.ImageVariant
				formattedLabels := []string{}
				for _, sl := range possibleTargetOfVariant.Info.SwarmingLabels {
					combined := fmt.Sprintf("%s:%s", sl.Label, sl.Value)
					formattedLabels = append(formattedLabels, combined)
				}
				mergedLabels := append(formattedLabels, tcSkippedDeps[testCaseName]...)

				// Check if the solution exists the possible HW dimensions for the suite.
				// If not, then do not add the hw as a target.
				possible, foundCache = isPossible(req, board, imageVariant, foundCache, oldProto)
				if !possible {
					continue
				}
				log.Println("Appending CTP targ.")

				ctpTargets = append(ctpTargets, &ctpApi.SwarmingDefinition{
					DutInfo: &labApi.Dut{
						DutType: &labApi.Dut_Chromeos{
							Chromeos: &labApi.Dut_ChromeOS{
								// This is the device to use for the current target
								Hwid: hwid,
								DutModel: &labApi.DutModel{
									BuildTarget: board,
									ModelName:   model,
								},
							},
						},
					},
					Variant: imageVariant,
					// NOTE: Since 3D is purely responsible for HW selection, it will *not* accept suite-level hardware deps.
					SwarmingLabels: removeDupLabelsAndSort(mergedLabels),
				})
			}
			if len(ctpTargets) < 1 {
				continue
			}

			if oldProto {
				log.Println("USING OLD PROTO")
				ctpVariants = append(ctpVariants, &ctpApi.HWRequirements{
					// Todo (b:285909154) add the variant info for Luci analysis.
					HwDefinition: ctpTargets,
					// This indicate that for this variant only one of the HwDefinitions needs to run.
					State: ctpApi.HWRequirements_ONEOF,
				})

			} else {
				log.Println("USING NEW PROTO")
				options := generateSchedulingUnitOptions(variant, testCase, ctpTargets, log)
				log.Println("Appending Options: ", options)
				newCTPVariants = append(newCTPVariants, options)
			}

		}
		if oldProto {
			testCase.HwRequirements = ctpVariants
		} else {
			testCase.SchedulingUnitOptions = newCTPVariants
		}

	}
	log.Println("TTCP Solver completed.")
	return req, nil
}

func preloadExpressions() {
	log.Println("Preloading the predefined TTCP/3D expressions.")
	catAndClassFiles, err := getClassAndCatDatasetFiles(".", []string{"json", "zip"})
	if err != nil {
		log.Fatal(err)
	}
	categoriesAndClassesCollection, err = solver.ParseTtcpCategoryAndClassCollection(catAndClassFiles, log.Default())
	if err != nil {
		log.Fatal(err)
	}
}

// This is the entry point for the dockerized version of TTCP.
func main() {
	log.Println("solver service v1.1.06 initializing.")

	preloadExpressions()

	err := servertemplate.Server(GenerateFilterExecutor, "solver service")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
