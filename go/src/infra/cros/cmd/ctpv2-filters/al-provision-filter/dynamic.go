// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates"
	dynamic_builders "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/generators"

	androidapi "infra/cros/cmd/common_lib/android_api"
	"infra/cros/cmd/common_lib/common"
)

// GenerateDynamicProvisionUpdates generates and updates the provision components of the request
func GenerateDynamicProvisionUpdates(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	modifyProvisionRequest(req, updater, log)
	updateProvisionInstallPath(req, updater, log)
	return nil
}

func modifyProvisionRequest(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) {
	log.Printf("Adding AL provisioning dynamic updates...")
	if updater.ProvisionPath == "" {
		return
	}

	servodId := dynamic_common.NewTaskIdentifier(common.ServoNexus).AddDeviceId(dynamic_common.NewPrimaryDeviceIdentifier())
	taskID := dynamic_common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(dynamic_common.NewPrimaryDeviceIdentifier())
	generator := generators.NewModifyGenerator(
		dynamic_common.FindByDynamicIdentifier(
			taskID.Id))

	servodContainerBuilder := dynamic_builders.NewContainerBuilder(
		servodId.Id, common.ServoNexus, updater.ServoPath,
		"/tmp/servod", "cros-servod server -server_port 0",
	)
	provisionContainerBuilder := dynamic_builders.NewContainerBuilder(
		taskID.Id, common.CrosProvision, updater.ProvisionPath,
		"/tmp/provisionservice", "foil-provision server -port 0")

	containers := []*api.ContainerRequest{}
	servodContainer := servodContainerBuilder.Build()
	servodContainer.Network = "adb-network"
	containers = append(containers, servodContainer)
	container := provisionContainerBuilder.Build()
	container.Network = "adb-network"
	containers = append(containers, container)
	err := generator.AddModification(
		&api.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: containers,
		},
		map[string]string{
			"orderedContainerRequests": "orderedContainerRequests",
		},
	)
	if err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	err = dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying provision request, %s", err)
	}
}

func updateProvisionInstallPath(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) {
	log.Printf("Updating provision install path...")

	suiteInfo := req.GetSuiteInfo()
	if suiteInfo == nil {
		log.Printf("suite Info found nil")
	}
	suiteMetadata := suiteInfo.GetSuiteMetadata()
	if suiteMetadata == nil {
		log.Printf("suite Metadata found nil")
	}
	schedulingUnits := suiteMetadata.GetSchedulingUnits()
	if schedulingUnits == nil {
		log.Printf("scheduling Units found nil")
	}
	for _, su := range schedulingUnits {
		gcsPath := su.GetPrimaryTarget().GetSwReq().GetGcsPath()
		if strings.HasPrefix(gcsPath, "android-build") {
			su.DynamicUpdateLookupTable["installPath"] = gcsPath
			continue
		}

		// Look up latest for board as not provided in gcs path.
		board, ok := su.GetDynamicUpdateLookupTable()["board"]
		if !ok {
			log.Printf("board not found")
		}
		var latestGreenBuild int
		var err error
		if latestGreenBuild, ok = updater.LatestBuildsByBoard[board]; !ok {
			latestGreenBuild, err = androidapi.GetLatestGreenBuildNumber(androidapi.CONTAINER_GCE, buildGetReq(board))
			if err != nil {
				log.Printf("Error getting latest green build number: %v", err)
				continue
			}
		}
		su.DynamicUpdateLookupTable["buildNumber"] = fmt.Sprint(latestGreenBuild)
		log.Printf("Latest green build number: %d\n", latestGreenBuild)

		log.Println("Setting build target and latest green build number")
		boardTarget := board + "-trunk_staging-userdebug"
		installPath := fmt.Sprintf(
			"android-build/build_explorer/artifacts_list/%s/%s/%s-ota-%s.zip",
			strconv.Itoa(latestGreenBuild), boardTarget, board, strconv.Itoa(latestGreenBuild))
		log.Printf("InstallPath value: %s", installPath)
		su.DynamicUpdateLookupTable["installPath"] = installPath
	}

}

func buildGetReq(board string) androidapi.BuildGetRequest {
	return androidapi.BuildGetRequest{
		BuildType:   "submitted",
		Board:       board,
		MaxResults:  "1",
		Branch:      "git_main-al-dev",
		SortingType: "creationTimestamp",
		Successful:  "true",
	}
}
