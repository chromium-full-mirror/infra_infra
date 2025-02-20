// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package starlark_test

import (
	"context"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	configpb "go.chromium.org/chromiumos/config/go/api"
	buildpb "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/payload"
	test_api "go.chromium.org/chromiumos/config/go/test/api"
	test_api_v1 "go.chromium.org/chromiumos/config/go/test/api/v1"
	"go.chromium.org/chromiumos/config/go/test/plan"

	"go.chromium.org/infra/cros/internal/testplan/starlark"
)

var buildMetadataList = &buildpb.SystemImage_BuildMetadataList{
	Values: []*buildpb.SystemImage_BuildMetadata{
		{
			BuildTarget: &buildpb.SystemImage_BuildTarget{
				PortageBuildTarget: &buildpb.Portage_BuildTarget{OverlayName: "overlay1"},
			},
		},
		{
			BuildTarget: &buildpb.SystemImage_BuildTarget{
				PortageBuildTarget: &buildpb.Portage_BuildTarget{OverlayName: "overlay2"},
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

func TestExecTestPlan(t *testing.T) {
	ctx := context.Background()
	starlarkSource := `
load("@proto//chromiumos/test/api/v1/plan.proto", plan_pb = "chromiumos.test.api.v1")
load("@proto//chromiumos/test/api/coverage_rule.proto", coverage_rule_pb = "chromiumos.test.api")
load("@proto//chromiumos/test/api/dut_attribute.proto", dut_attribute_pb = "chromiumos.test.api")
load("@proto//chromiumos/test/api/test_suite.proto", test_suite_pb = "chromiumos.test.api")
load("@proto//lab/license.proto", licence_pb = "lab")

build_metadata = testplan.get_build_metadata()
config_bundles = testplan.get_config_bundle_list()
print('Got {} BuildMetadatas'.format(len(build_metadata.values)))
print('Got {} ConfigBundles'.format(len(config_bundles.values)))
coverage_rule_a = coverage_rule_pb.CoverageRule(
	name='ruleA',
	test_suites=[
		test_suite_pb.TestSuite(
			name=testplan.get_suite_name(),
			test_case_tag_criteria=test_suite_pb.TestSuite.TestCaseTagCriteria(
				tags=testplan.get_tag_criteria().tags
			)
		)
	],
	dut_targets = [
		dut_attribute_pb.DutTarget(
			criteria = [
				dut_attribute_pb.DutCriterion(
					attribute_id = dut_attribute_pb.DutAttribute.Id(
						value = "attr-program",
					),
					values = [testplan.get_program()],
				),
			],
		),
	]
)
coverage_rule_b = coverage_rule_pb.CoverageRule(name='ruleB')
test_licence = licence_pb.LICENSE_TYPE_WINDOWS_10_PRO
testplan.add_hw_test_plan(
	plan_pb.HWTestPlan(
		id=plan_pb.HWTestPlan.TestPlanId(value='plan1'),
		coverage_rules=[coverage_rule_a],
	),
)
testplan.add_vm_test_plan(
	plan_pb.VMTestPlan(
		id=plan_pb.VMTestPlan.TestPlanId(value='vm_plan2'),
		coverage_rules=[coverage_rule_b],
	)
)

s1 = struct(a=1, b=2)
print('Got struct {}'.format(s1))
`
	planFilename := writeTempStarlarkFile(
		t, starlarkSource,
	)

	hwTestPlans, vmTestPlans, err := starlark.ExecTestPlan(
		ctx,
		planFilename,
		buildMetadataList,
		configBundleList,
		&plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
			TagCriteria: &test_api.TestSuite_TestCaseTagCriteria{
				Tags: []string{"group:mainline", "group:mycustomgroup"},
			},
			SuiteName: "mycustomgroup",
			Program:   "programA",
		},
	)

	if err != nil {
		t.Fatalf("ExecTestPlan failed: %s", err)
	}

	expectedHwTestPlans := []*test_api_v1.HWTestPlan{
		{
			Id: &test_api_v1.HWTestPlan_TestPlanId{Value: "plan1"},
			CoverageRules: []*test_api.CoverageRule{
				{
					Name: "ruleA",
					TestSuites: []*test_api.TestSuite{
						{
							Name: "mycustomgroup",
							Spec: &test_api.TestSuite_TestCaseTagCriteria_{
								TestCaseTagCriteria: &test_api.TestSuite_TestCaseTagCriteria{
									Tags: []string{"group:mainline", "group:mycustomgroup"},
								},
							},
						},
					},
					DutTargets: []*test_api.DutTarget{
						{
							Criteria: []*test_api.DutCriterion{
								{
									AttributeId: &test_api.DutAttribute_Id{Value: "attr-program"},
									Values:      []string{"programA"},
								},
							},
						},
					},
				},
			},
		},
	}

	expectedVMTestPlans := []*test_api_v1.VMTestPlan{
		{
			Id: &test_api_v1.VMTestPlan_TestPlanId{Value: "vm_plan2"},
			CoverageRules: []*test_api.CoverageRule{
				{
					Name: "ruleB",
				},
			},
		},
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

func TestExecTestPlanErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		starlarkSource string
		err            string
		templateParams *plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters
	}{
		{
			name:           "invalid positional args",
			starlarkSource: "testplan.get_build_metadata(1, 2)",
			err:            "get_build_metadata: got 2 arguments, want at most 0",
			templateParams: nil,
		},
		{
			name:           "invalid named args",
			starlarkSource: "testplan.get_config_bundle_list(somearg='abc')",
			err:            "get_config_bundle_list: unexpected keyword argument \"somearg\"",
			templateParams: nil,
		},
		{
			name:           "invalid named args ctor HW",
			starlarkSource: "testplan.add_hw_test_plan(somearg='abc')",
			err:            "add_hw_test_plan: unexpected keyword argument \"somearg\"",
			templateParams: nil,
		},
		{
			name:           "invalid named args ctor VM",
			starlarkSource: "testplan.add_vm_test_plan(somearg='abc')",
			err:            "add_vm_test_plan: unexpected keyword argument \"somearg\"",
			templateParams: nil,
		},
		{
			name:           "invalid type ctor HW",
			starlarkSource: "testplan.add_hw_test_plan(hw_test_plan='abc')",
			err:            "add_hw_test_plan: arg must be a chromiumos.test.api.v1.HWTestPlan, got \"\\\"abc\\\"\"",
			templateParams: nil,
		},
		{
			name:           "invalid type ctor VM",
			starlarkSource: "testplan.add_vm_test_plan(vm_test_plan='abc')",
			err:            "add_vm_test_plan: arg must be a chromiumos.test.api.v1.VMTestPlan, got \"\\\"abc\\\"\"",
		},
		{
			name: "invalid proto ctor",
			starlarkSource: `
load("@proto//chromiumos/test/api/v1/plan.proto", plan_pb = "chromiumos.test.api.v1")
testplan.add_hw_test_plan(hw_test_plan=plan_pb.HWTestPlan.TestPlanId(value='abc'))
			`,
			err:            "add_hw_test_plan: arg must be a chromiumos.test.api.v1.HWTestPlan, got \"value:\\\"abc\\\"\"",
			templateParams: nil,
		},
		{
			name:           "no tag criteria available",
			starlarkSource: "testplan.get_tag_criteria()",
			err:            "get_tag_criteria: no TestCaseTagCriteria available in this Starlark execution, was it specified on the interpreter command line?",
			templateParams: nil,
		},
		{
			name:           "no suite name available",
			starlarkSource: "testplan.get_suite_name()",
			err:            "get_suite_name: no test suite name available in this Starlark execution, was it specified on the interpreter command line?",
			templateParams: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			planFilename := writeTempStarlarkFile(
				t, tc.starlarkSource,
			)

			_, _, err := starlark.ExecTestPlan(
				ctx, planFilename, buildMetadataList, configBundleList, tc.templateParams,
			)

			if err == nil {
				t.Errorf("Expected error from ExecTestPlan")
			}

			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("Expected error message %q, got %q", tc.err, err.Error())
			}
		})
	}
}
