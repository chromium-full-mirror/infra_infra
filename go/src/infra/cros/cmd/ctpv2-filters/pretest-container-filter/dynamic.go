// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/test_plan"
)

// InsertContainer appends the task to launch the container.
func InsertContainer(req *api.InternalTestplan, gcu *PreTestContainerUpdater, log *log.Logger) error {
	containerBuilder := builders.NewContainerBuilder(
		gcu.ContainerName,         //  ContainerID
		gcu.ContainerName,         //  ContainerImageKey
		gcu.ContainerPath,         //  Container ImagePath
		"/tmp/"+gcu.ContainerName, //  ContainerArtifactDir
		gcu.ContainerRunCmd,       // Cmd
	)

	// Mount any additional volumes.
	if gcu.Volumes != "" {
		containerBuilder.SetAdditionalVolumes(strings.Split(gcu.Volumes, ","))
	}

	// If using AL, then switch network to adb.
	if test_plan.IsAlRun(req) {
		log.Printf("AL run detected")
		containerBuilder.Network = "adb-network"
	}

	task := &api.CrosTestRunnerDynamicRequest_Task{
		OrderedContainerRequests: []*api.ContainerRequest{
			containerBuilder.Build(),
		},
		Task: &api.CrosTestRunnerDynamicRequest_Task_PreTest{
			PreTest: &api.PreTestTask{
				DynamicIdentifier: gcu.ContainerName,
				ServiceAddress:    &labapi.IpEndpoint{},
				DynamicDeps: []*api.DynamicDep{
					{
						Key:   dynamic_common.ServiceAddress,
						Value: gcu.ContainerName,
					},
				},
			},
		},
		Required: true,
	}

	log.Printf("appending task: %v", task)
	generator := generators.NewInsertGenerator()
	generator.AddInsertion(
		task,
		dynamic_common.PrependTaskWrapper(dynamic_common.FindFirst(api.FocalTaskFinder_TEST)))

	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}

func AddContainerArg(req *api.InternalTestplan, gcu *PreTestContainerUpdater, log *log.Logger) error {
	cliArgName := gcu.ContainerName + ".address"
	if gcu.TestCLIArg != "" {
		cliArgName = gcu.TestCLIArg
	}

	generator := generators.NewModifyGenerator(dynamic_common.FindByDynamicIdentifier(common.CrosTest))

	if test_plan.IsAlRun(req) {
		// If using AL, then we need to pass the argument as a "param"
		// We also need to pass host/port separately as args are expected to be
		// delimited by :.
		paramName := fmt.Sprintf("extra-test-params_%s", gcu.ContainerName)
		if gcu.TestParamName != "" {
			paramName = gcu.TestParamName
		}

		log.Printf("AL run detected")
		err := generator.AddModification(
			&api.DynamicDep{
				Key: "testRequest.testSuites.0.executionMetadata.args",
				Value: fmt.Sprintf(`JSON={"flag":"%s","value":"%s_host:${%s.address},%s_port:${%s.port}"}`,
					paramName, cliArgName, gcu.ContainerName, cliArgName, gcu.ContainerName),
			},
			map[string]string{
				"test.dynamicDeps": "",
			},
		)
		if err != nil {
			return fmt.Errorf("error while adding modification to test request, %w", err)
		}
	} else {
		err := generator.AddModification(
			&api.DynamicDep{
				Key: "testRequest.testSuites.0.executionMetadata.args",
				Value: fmt.Sprintf(`JSON={"flag":"%s","value":"${%s.address}:${%s.port}"}`,
					cliArgName, gcu.ContainerName, gcu.ContainerName),
			},
			map[string]string{
				"test.dynamicDeps": "",
			},
		)
		if err != nil {
			return fmt.Errorf("error while adding modification to test request, %w", err)
		}
	}
	return dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
}
