// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testplan_test

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	configpb "go.chromium.org/chromiumos/config/go/api"
	buildpb "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/payload"
	testpb "go.chromium.org/chromiumos/config/go/test/api"
	test_api_v1 "go.chromium.org/chromiumos/config/go/test/api/v1"
	"go.chromium.org/chromiumos/config/go/test/plan"

	"go.chromium.org/infra/cros/internal/testplan"
)

// buildMetadata is a convenience to reduce boilerplate when creating
// SystemImage_BuildMetadata in test cases.
func buildMetadata(overlay, kernelVersion, chipsetOverlay, arcVersion string) *buildpb.SystemImage_BuildMetadata {
	return &buildpb.SystemImage_BuildMetadata{
		BuildTarget: &buildpb.SystemImage_BuildTarget{
			PortageBuildTarget: &buildpb.Portage_BuildTarget{
				OverlayName: overlay,
			},
		},
		PackageSummary: &buildpb.SystemImage_BuildMetadata_PackageSummary{
			Kernel: &buildpb.SystemImage_BuildMetadata_Kernel{
				Version: kernelVersion,
			},
			Chipset: &buildpb.SystemImage_BuildMetadata_Chipset{
				Overlay: chipsetOverlay,
			},
			Arc: &buildpb.SystemImage_BuildMetadata_Arc{
				Version: arcVersion,
			},
		},
	}
}

var buildMetadataList = &buildpb.SystemImage_BuildMetadataList{
	Values: []*buildpb.SystemImage_BuildMetadata{
		buildMetadata("project1", "4.14", "chipsetA", "P"),
		buildMetadata("project2", "4.14", "chipsetB", "R"),
		buildMetadata("project3", "5.4", "chipsetA", ""),
	},
}

var dutAttributeList = &testpb.DutAttributeList{
	DutAttributes: []*testpb.DutAttribute{
		{
			Id: &testpb.DutAttribute_Id{Value: "fingerprint_location"},
			DataSource: &testpb.DutAttribute_FlatConfigSource_{
				FlatConfigSource: &testpb.DutAttribute_FlatConfigSource{
					Fields: []*testpb.DutAttribute_FieldSpec{
						{
							Path: "design_list.configs.hardware_features.fingerprint.location",
						},
					},
				},
			},
		},
		{
			Id: &testpb.DutAttribute_Id{Value: "system_build_target"},
			DataSource: &testpb.DutAttribute_FlatConfigSource_{
				FlatConfigSource: &testpb.DutAttribute_FlatConfigSource{
					Fields: []*testpb.DutAttribute_FieldSpec{
						{
							Path: "software_configs.system_build_target.portage_build_target.overlay_name",
						},
					},
				},
			},
		},
	},
}

var configBundleList = &payload.ConfigBundleList{
	Values: []*payload.ConfigBundle{
		{
			ProgramList: []*configpb.Program{
				{
					Id: &configpb.ProgramId{
						Value: "ProgA",
					},
				},
			},
		},
		{
			ProgramList: []*configpb.Program{
				{
					Id: &configpb.ProgramId{
						Value: "ProgB",
					},
				},
			},
		},
		{
			DesignList: []*configpb.Design{
				{
					Id: &configpb.DesignId{
						Value: "Design1",
					},
				},
				{
					Id: &configpb.DesignId{
						Value: "Design2",
					},
				},
			},
		},
	},
}

// writeTempStarlarkFile writes starlarkSource to a temp file created under
// a t.TempDir().
func writeTempStarlarkFile(t *testing.T, starlarkSource string) string {
	testDir := t.TempDir()
	planFilename := path.Join(testDir, "test.star")

	if err := os.WriteFile(
		planFilename,
		[]byte(starlarkSource),
		os.ModePerm,
	); err != nil {
		t.Fatal(err)
	}

	return planFilename
}

func TestGenerate(t *testing.T) {
	ctx := context.Background()

	starlarkSource := `
load("@proto//chromiumos/test/api/v1/plan.proto", plan_pb = "chromiumos.test.api.v1")

build_metadata = testplan.get_build_metadata()
config_bundles = testplan.get_config_bundle_list()
print('Got {} BuildMetadatas'.format(len(build_metadata.values)))
print('Got {} ConfigBundles'.format(len(config_bundles.values)))
testplan.add_hw_test_plan(
	plan_pb.HWTestPlan(id=plan_pb.HWTestPlan.TestPlanId(value='plan1'))
)
testplan.add_vm_test_plan(
	plan_pb.VMTestPlan(id=plan_pb.VMTestPlan.TestPlanId(value='plan2'))
)
	`

	noPlansStarlarkSource := `
load("@proto//chromiumos/test/api/v1/plan.proto", plan_pb = "chromiumos.test.api.v1")

def pointless_fn():
	if 1 == 2:
		testplan.add_hw_test_plan(
			plan_pb.HWTestPlan(id=plan_pb.HWTestPlan.TestPlanId(value='plan1'))
		)

pointless_fn()
	`

	planFilename := writeTempStarlarkFile(
		t, starlarkSource,
	)

	noPlanFilename := writeTempStarlarkFile(
		t, noPlansStarlarkSource,
	)

	hwTestPlans, vmTestPlans, err := testplan.Generate(
		ctx, []string{planFilename, noPlanFilename}, buildMetadataList, dutAttributeList, configBundleList, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedHwTestPlans := []*test_api_v1.HWTestPlan{
		{Id: &test_api_v1.HWTestPlan_TestPlanId{Value: "plan1"}},
	}

	expectedVMTestPlans := []*test_api_v1.VMTestPlan{
		{Id: &test_api_v1.VMTestPlan_TestPlanId{Value: "plan2"}},
	}

	if len(expectedHwTestPlans) != len(hwTestPlans) {
		t.Errorf("expected %d test plans, got %d", len(expectedHwTestPlans), len(hwTestPlans))
	}

	for i, expected := range expectedHwTestPlans {
		if diff := cmp.Diff(expected, hwTestPlans[i], protocmp.Transform()); diff != "" {
			t.Errorf("returned unexpected diff in test plan %d (-want +got):\n%s", i, diff)
		}
	}

	if len(expectedVMTestPlans) != len(vmTestPlans) {
		t.Errorf("expected %d test plans, got %d", len(expectedVMTestPlans), len(vmTestPlans))
	}

	for i, expected := range expectedVMTestPlans {
		if diff := cmp.Diff(expected, vmTestPlans[i], protocmp.Transform()); diff != "" {
			t.Errorf("returned unexpected diff in test plan %d (-want +got):\n%s", i, diff)
		}
	}
}

func TestGenerateWithTemplateParameters(t *testing.T) {
	ctx := context.Background()

	starlarkSource := `
load("@proto//chromiumos/test/api/v1/plan.proto", plan_pb = "chromiumos.test.api.v1")
load("@proto//chromiumos/test/api/coverage_rule.proto", coverage_rule_pb = "chromiumos.test.api")
load("@proto//chromiumos/test/api/test_suite.proto", test_suite_pb = "chromiumos.test.api")

coverage_rule_a = coverage_rule_pb.CoverageRule(
	name='ruleA',
	test_suites=[
		test_suite_pb.TestSuite(
			name=testplan.get_suite_name(),
			test_case_tag_criteria=test_suite_pb.TestSuite.TestCaseTagCriteria(
				tags=testplan.get_tag_criteria().tags
			)
		)
	]
)
testplan.add_hw_test_plan(
	plan_pb.HWTestPlan(
		id=plan_pb.HWTestPlan.TestPlanId(value='plan1'),
		coverage_rules=[coverage_rule_a],
	),
)
`
	planFilename := writeTempStarlarkFile(t, starlarkSource)
	hwTestPlans, vmTestPlans, err := testplan.Generate(
		ctx, []string{planFilename}, buildMetadataList, dutAttributeList,
		configBundleList, map[string][]*plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
			planFilename: {
				{
					TagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
						Tags: []string{"group:customgroup1"},
					},
					SuiteName: "customsuite1",
				},
				{
					TagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
						Tags: []string{"group:customgroup2"},
					},
					SuiteName: "customsuite2",
				},
			},
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	expectedHwTestPlans := []*test_api_v1.HWTestPlan{
		{
			Id: &test_api_v1.HWTestPlan_TestPlanId{Value: "plan1"},
			CoverageRules: []*testpb.CoverageRule{
				{
					Name: "ruleA",
					TestSuites: []*testpb.TestSuite{
						{
							Name: "customsuite1",
							Spec: &testpb.TestSuite_TestCaseTagCriteria_{
								TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
									Tags: []string{"group:customgroup1"},
								},
							},
						},
					},
				},
			},
		},
		{
			Id: &test_api_v1.HWTestPlan_TestPlanId{Value: "plan1"},
			CoverageRules: []*testpb.CoverageRule{
				{
					Name: "ruleA",
					TestSuites: []*testpb.TestSuite{
						{
							Name: "customsuite2",
							Spec: &testpb.TestSuite_TestCaseTagCriteria_{
								TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
									Tags: []string{"group:customgroup2"},
								},
							},
						},
					},
				},
			},
		},
	}

	expectedVMTestPlans := []*test_api_v1.VMTestPlan{}

	if len(expectedHwTestPlans) != len(hwTestPlans) {
		t.Errorf("expected %d test plans, got %d", len(expectedHwTestPlans), len(hwTestPlans))
	}

	for i, expected := range expectedHwTestPlans {
		if diff := cmp.Diff(expected, hwTestPlans[i], protocmp.Transform()); diff != "" {
			t.Errorf("returned unexpected diff in test plan %d (-want +got):\n%s", i, diff)
		}
	}

	if len(expectedVMTestPlans) != len(vmTestPlans) {
		t.Errorf("expected %d test plans, got %d", len(expectedVMTestPlans), len(vmTestPlans))
	}

	for i, expected := range expectedVMTestPlans {
		if diff := cmp.Diff(expected, vmTestPlans[i], protocmp.Transform()); diff != "" {
			t.Errorf("returned unexpected diff in test plan %d (-want +got):\n%s", i, diff)
		}
	}
}

func TestGenerateErrors(t *testing.T) {
	ctx := context.Background()

	badStarlarkFile := "testplan.invalidcall()"
	badPlanFilename := writeTempStarlarkFile(t, badStarlarkFile)

	tests := []struct {
		name               string
		planFilenames      []string
		buildMetadataList  *buildpb.SystemImage_BuildMetadataList
		dutAttributeList   *testpb.DutAttributeList
		configBundleList   *payload.ConfigBundleList
		templateParameters map[string][]*plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters
	}{
		{
			name:               "empty planFilenames",
			planFilenames:      []string{},
			buildMetadataList:  buildMetadataList,
			dutAttributeList:   dutAttributeList,
			configBundleList:   configBundleList,
			templateParameters: nil,
		},
		{
			name:               "nil buildMetadataList",
			planFilenames:      []string{"plan1.star"},
			buildMetadataList:  nil,
			dutAttributeList:   dutAttributeList,
			configBundleList:   configBundleList,
			templateParameters: nil,
		},
		{
			name:               "nil dutAttributeList",
			planFilenames:      []string{"plan1.star"},
			buildMetadataList:  buildMetadataList,
			dutAttributeList:   nil,
			configBundleList:   configBundleList,
			templateParameters: nil,
		},
		{
			name:               "nil ConfigBundleList",
			planFilenames:      []string{"plan1.star"},
			buildMetadataList:  buildMetadataList,
			dutAttributeList:   dutAttributeList,
			configBundleList:   nil,
			templateParameters: nil,
		},
		{
			name:               "bad Starlark file",
			planFilenames:      []string{badPlanFilename},
			buildMetadataList:  buildMetadataList,
			dutAttributeList:   dutAttributeList,
			configBundleList:   configBundleList,
			templateParameters: nil,
		},
		{
			name:              "invalid template parameters",
			planFilenames:     []string{"plan1.star"},
			buildMetadataList: buildMetadataList,
			dutAttributeList:  dutAttributeList,
			configBundleList:  configBundleList,
			templateParameters: map[string][]*plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
				"otherplan.star": {
					{
						TagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
							Tags: []string{"group:customgroup"},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := testplan.Generate(
				ctx, test.planFilenames, test.buildMetadataList, test.dutAttributeList, test.configBundleList, test.templateParameters,
			); err == nil {
				t.Error("Expected error from Generate")
			}
		})
	}
}
