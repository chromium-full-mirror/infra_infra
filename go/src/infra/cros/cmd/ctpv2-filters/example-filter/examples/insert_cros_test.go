// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package examples

import (
	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	dynamiccommon "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
)

func InsertCrosTest(req *api.InternalTestplan) error {
	containerBuilder := builders.NewContainerBuilder(
		"inserted_cros_test_container",      //  ContainerID
		"cros-test",                         //  ContainerImageKey
		"",                                  //  Container ImagePath
		"/tmp/inserted_cros_test_container", //  ContainerArtifactDir
		"cros-test server -port 0",          // Cmd
	)

	crosTestTask := &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{
			containerBuilder.Build(),
		},
		Task: &api.CrosTestRunnerDynamicRequest_Task_Test{
			Test: &api.TestTask{
				DynamicIdentifier: "inserted_cros_test",
				TestRequest: &api.CrosTestRequest{
					TestSuites: []*api.TestSuite{
						{
							Name: "inserted_suite",
							Spec: &api.TestSuite_TestCaseIds{
								TestCaseIds: &api.TestCaseIdList{
									TestCaseIds: []*api.TestCase_Id{
										{
											Value: "inserted_test_case",
										},
									},
								},
							},
							ExecutionMetadata: req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata(),
						},
					},
				},
				DynamicDeps: []*api.DynamicDep{
					{
						Key:   common.ServiceAddress,
						Value: "inserted_cros_test_container",
					},
					{
						Key:   common.TestRequestPrimary,
						Value: common.PrimaryDevice,
					},
					{
						Key:   common.TestRequestCompanions,
						Value: common.CompanionDevices,
					},
					{
						Key:   common.TestRequestPrimary + ".dutServer",
						Value: common.NewPrimaryDeviceIdentifier().GetCrosDutServer(),
					},
				},
			},
		},
	}

	insertAt := dynamiccommon.AppendTaskWrapper(
		dynamiccommon.FindFirst(api.FocalTaskFinder_TEST))

	generator := generators.NewInsertGenerator()
	generator.AddInsertion(crosTestTask, insertAt)

	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}
