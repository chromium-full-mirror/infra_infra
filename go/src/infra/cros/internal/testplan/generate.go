// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testplan contains the main application code for the testplan tool.
package testplan

import (
	"context"
	"errors"
	"fmt"

	buildpb "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/payload"
	testpb "go.chromium.org/chromiumos/config/go/test/api"
	test_api_v1 "go.chromium.org/chromiumos/config/go/test/api/v1"
	"go.chromium.org/chromiumos/config/go/test/plan"
	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/testplan/starlark"
)

// validateTemplateParameters validates that all the plans used as keys in
// planToTemplateParametersList are in planFilenames.
func validateTemplateParameters(
	planFilenames []string,
	planToTemplateParametersList map[string][]*plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters,
) error {
	planFilenamesSet := stringset.NewFromSlice(planFilenames...)
	for plan := range planToTemplateParametersList {
		if !planFilenamesSet.Has(plan) {
			return fmt.Errorf("TemplateParameters passed for a plan that wasn't passed: %q", plan)
		}
	}
	return nil
}

// Generate evals the Starlark files in planFilenames to produce a list of
// HWTestPlans and VMTestPlans.
//
// planFilenames must be non-empty. buildMetadataList, dutAttributeList, and
// configBundleList must be non-nil.
//
// All keys in planToTemplateParametersList must be in planFilenames. If a plan
// does have a list of TemplateParameters, the plan will be evaluated once for
// each of the TemplateParameters.
func Generate(
	ctx context.Context,
	planFilenames []string,
	buildMetadataList *buildpb.SystemImage_BuildMetadataList,
	dutAttributeList *testpb.DutAttributeList,
	configBundleList *payload.ConfigBundleList,
	planToTemplateParametersList map[string][]*plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters,
) ([]*test_api_v1.HWTestPlan, []*test_api_v1.VMTestPlan, error) {
	if len(planFilenames) == 0 {
		return nil, nil, errors.New("planFilenames must be non-empty")
	}

	if buildMetadataList == nil {
		return nil, nil, errors.New("buildMetadataList must be non-nil")
	}

	if dutAttributeList == nil {
		return nil, nil, errors.New("dutAttributeList must be non-nil")
	}

	if configBundleList == nil {
		return nil, nil, errors.New("configBundleList must be non-nil")
	}

	if err := validateTemplateParameters(planFilenames, planToTemplateParametersList); err != nil {
		return nil, nil, err
	}

	var allHwTestPlans []*test_api_v1.HWTestPlan
	var allVMTestPlans []*test_api_v1.VMTestPlan
	for _, planFilename := range planFilenames {
		// Get the TemplateParameters for plan if they were passed. If not, just
		// use a single nil value, which means no TemplateParameters are
		// available when the plan is executed.
		templateParametersList := planToTemplateParametersList[planFilename]
		if len(templateParametersList) == 0 {
			templateParametersList = append(templateParametersList, nil)
		}

		for _, templateParameters := range templateParametersList {
			if templateParameters == nil {
				logging.Infof(ctx, "executing %q with no TemplateParameters", planFilename)
			} else {
				logging.Infof(ctx, "executing %q with TemplateParameters %q", planFilename, templateParameters)
			}

			hwTestPlans, vmTestPlans, err := starlark.ExecTestPlan(
				ctx, planFilename, buildMetadataList, configBundleList, templateParameters,
			)
			if err != nil {
				return nil, nil, err
			}

			if len(hwTestPlans) == 0 && len(vmTestPlans) == 0 {
				logging.Warningf(ctx, "starlark file %q returned no TestPlans", planFilename)
			}

			allHwTestPlans = append(allHwTestPlans, hwTestPlans...)
			allVMTestPlans = append(allVMTestPlans, vmTestPlans...)
		}
	}

	return allHwTestPlans, allVMTestPlans, nil
}
