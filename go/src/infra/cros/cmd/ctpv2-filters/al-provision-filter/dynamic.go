// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	"go.chromium.org/chromiumos/config/go/test/api"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates"
	dynamic_builders "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/generators"
)

var (
	// TODO(b/406307693): Switch al-dev to throttled.
	DefaultBranch = "git_main-al-dev"
	PDKBranch     = "partner-brya-temp-main-throttled-fs"
)

// GenerateDynamicProvisionUpdates generates and updates the provision components of the request
func GenerateDynamicProvisionUpdates(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	if err := modifyProvisionRequest(req, updater, log); err != nil {
		return err
	}
	if err := updateProvisionInstallPath(req, updater, log); err != nil {
		return err
	}
	return nil
}

// modifyProvisionRequest adds dynamic updates to the InternalTestplan request
// to configure provisioning containers and associated metadata.
func modifyProvisionRequest(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	log.Printf("Adding AL provisioning dynamic updates...")
	if updater.ProvisionPath == "" {
		return nil
	}

	servodId := dynamic_common.NewTaskIdentifier(common.ServoNexus).AddDeviceId(dynamic_common.NewPrimaryDeviceIdentifier())
	taskID := dynamic_common.NewTaskIdentifier(common.CrosProvision).AddDeviceId(dynamic_common.NewPrimaryDeviceIdentifier())
	newTaskID := dynamic_common.NewTaskIdentifier(common.FoilProvision).AddDeviceId(dynamic_common.NewPrimaryDeviceIdentifier())
	generator := generators.NewModifyGenerator(
		dynamic_common.FindByDynamicIdentifier(
			taskID.Id))

	servodContainerBuilder := dynamic_builders.NewContainerBuilder(
		servodId.Id, common.ServoNexus, updater.ServoPath,
		"/tmp/servod", "cros-servod server -server_port 0",
	)
	provisionContainerBuilder := dynamic_builders.NewContainerBuilder(
		newTaskID.Id, common.FoilProvision, updater.ProvisionPath,
		"/tmp/provisionservice", "foil-provision server -port 0")

	containers := []*api.ContainerRequest{}
	servodContainer := servodContainerBuilder.Build()
	servodContainer.Network = "adb-network"
	// Required for Satlab's Docker TLS daemon.
	tlsVars := []string{"DOCKER_CERT_PATH", "DOCKER_HOST", "DOCKER_TLS_VERIFY"}
	envvars := servodContainer.GetContainer().GetGeneric().Env
	servodContainer.GetContainer().GetGeneric().Env = append(envvars, tlsVars...)

	containers = append(containers, servodContainer)
	container := provisionContainerBuilder.Build()
	container.Network = "adb-network"
	containers = append(containers, container)
	if err := generator.AddModification(
		&api.CrosTestRunnerDynamicRequest_Task{
			OrderedContainerRequests: containers,
		},
		map[string]string{
			"orderedContainerRequests": "orderedContainerRequests",
		},
	); err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	// Update dynamic identifier
	if err := generator.AddModification(
		structpb.NewStringValue(newTaskID.Id),
		map[string]string{
			"provision.dynamicIdentifier":   "value",
			"provision.dynamicDeps.0.value": "value",
		},
	); err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	// Update partnermetadata to the install request
	if err := generator.AddModification(
		&api.PartnerMetadata{},
		map[string]string{
			"provision.installRequest.partnerMetadata": "",
		},
	); err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	// Add kernel artifacts to the install request.
	if getTestType(req) == common.KernelTestType {
		kernelBranch, kernelBuildId, kernelTarget, err := getKernelBuildInfo(req)
		if err != nil {
			return fmt.Errorf("fetching kernel build: %+w", err)
		}
		// android15-6.6 produces a partition image named `system_dlkm.img`,
		// while other branches produce `system_dlkm.erofs.img`.
		// See http://b/417259537#comment7 for additional context.
		system_dlkm_img_name := "system_dlkm.erofs.img"
		if strings.Contains(kernelBranch, "android15-6.6") {
			system_dlkm_img_name = "system_dlkm.img"
		}
		if err := generator.AddModification(
			&api.KernelPrebuilts{
				PartitionImages: []*api.KernelPrebuilts_PartitionImage{
					{
						PartitionName: "boot_a",
						ImagePath:     common.GetABStoragePath(kernelBuildId, kernelTarget, "boot.img"),
					},
					{
						PartitionName: "system_dlkm_a",
						ImagePath:     common.GetABStoragePath(kernelBuildId, kernelTarget, system_dlkm_img_name),
					},
					{
						PartitionName: "vendor_dlkm_a",
						ImagePath:     common.GetABStoragePath(kernelBuildId, kernelTarget, "vendor_dlkm.img"),
					},
					{
						PartitionName: "vendor_boot_a",
						RamdiskName:   "dlkm",
						ImagePath:     common.GetABStoragePath(kernelBuildId, kernelTarget, "initramfs.img"),
					},
				},
			},
			map[string]string{
				"provision.installRequest.kernelPrebuilts": "",
			},
		); err != nil {
			log.Printf("Error while adding modification to provision request, %s", err)
			return fmt.Errorf("adding kernel prebuilts to provision request: %+w", err)
		}
	}

	// Update partner account ID information
	if err := generator.AddModification(
		&api.DynamicDep{
			Key:   "installRequest.partnerMetadata.accountId",
			Value: "account-id",
		},
		map[string]string{
			"provision.dynamicDeps": "",
		},
	); err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	// Update partner GCS bucket information
	if err := generator.AddModification(
		&api.DynamicDep{
			Key:   "installRequest.partnerMetadata.partnerGcsBucket",
			Value: "partner-gcs-bucket",
		},
		map[string]string{
			"provision.dynamicDeps": "",
		},
	); err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	if err := dynamic_updates.AppendUserDefinedDynamicUpdates(
		&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate,
	); err != nil {
		log.Printf("Error while modifying provision request, %s", err)
	}

	return nil
}

// updateProvisionInstallPath sets the install path that will be used for scheduling.
func updateProvisionInstallPath(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	log.Printf("Updating provision install path...")

	suiteInfo := req.GetSuiteInfo()
	if suiteInfo == nil {
		log.Printf("suite Info found nil")
	}
	suiteMetadata := suiteInfo.GetSuiteMetadata()
	if suiteMetadata == nil {
		log.Printf("suite Metadata found nil")
	}

	for _, schedulingUnit := range getAllSchedulingUnits(suiteMetadata) {
		if err := updateSchedulingUnit(schedulingUnit, req, updater, log); err != nil {
			return err
		}
	}
	return nil
}

// getAllSchedulingUnits returns a list of all the SchedulingUnits in the SuiteMetadata, including those found in its SchedulingUnits and in its SchedulingUnitOptions.
func getAllSchedulingUnits(metadata *api.SuiteMetadata) []*api.SchedulingUnit {
	combined := []*api.SchedulingUnit{}
	if units := metadata.GetSchedulingUnits(); units != nil {
		combined = append(combined, units...)
	}
	if options := metadata.GetSchedulingUnitOptions(); options != nil {
		for _, option := range options {
			if units := option.GetSchedulingUnits(); units != nil {
				combined = append(combined, units...)
			}
		}
	}
	return combined
}

// updateSchedulingUnit sets the SchedulingUnit's install path and associated metadata.
func updateSchedulingUnit(su *api.SchedulingUnit, req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	var buildId, buildTarget, installPath, branch string
	var err error
	// Look up board information. Exit early if not found.
	board, ok := su.GetDynamicUpdateLookupTable()["board"]
	if !ok {
		return fmt.Errorf("board information not found")
	}
	buildApi := androidapi.AndroidBuildFactory()
	gcsPath := su.GetPrimaryTarget().GetSwReq().GetGcsPath()
	if strings.HasPrefix(gcsPath, "android-build") {
		buildId, buildTarget = extractBuildInfoFromInstallPath(gcsPath)
		installPath = gcsPath
		branch, err = buildApi.GetBranchFromBuildID(updater.AndroidAuthHandler, buildGetReq(board, "", buildId))
		if err != nil {
			log.Printf("Error getting branch for build number %s: target: %s %v", buildId, buildTarget, err)
		}
		log.Printf("Got branch %s from android api for build number: %s\n", branch, buildId)
	} else {
		branch = getBranch(su, log)
		var latestGreenBuild int
		var err error
		if latestGreenBuild, ok = updater.LatestBuildsByBoard[board]; !ok {
			latestGreenBuild, err = buildApi.GetLatestGreenBuildNumber(updater.AndroidAuthHandler, buildGetReq(board, branch, ""))
			if err != nil {
				log.Printf("Error getting latest green build number: %v", err)
				return err
			}
		}
		log.Printf("Latest green build number: %d\n", latestGreenBuild)
		buildId = fmt.Sprint(latestGreenBuild)

		log.Println("Setting build target and latest green build number")
		buildTarget = board + "-trunk_staging-userdebug"
		installPath = common.GetABOTAPath(buildId, buildTarget, board)
		log.Printf("InstallPath value: %s", installPath)
	}
	if getTestType(req) == common.KernelTestType {
		var err error
		installPath, err = fixInstallPathForKernelTest(installPath, req, board)
		if err != nil {
			return fmt.Errorf("fixing install path for kernel test: %+w", err)
		}
	}
	// Make sure the buildId and installPath are consistent in all expected locations.
	su.DynamicUpdateLookupTable["buildNumber"] = buildId
	su.DynamicUpdateLookupTable["installPath"] = installPath
	applyBuildInfoToTarget(buildId, buildTarget, branch, su.GetPrimaryTarget())
	su.GetPrimaryTarget().GetSwReq().GcsPath = installPath
	return nil
}

// fixInstallPathForKernelTest finds the installPath to use for kernel tests.
// When the test request comes into al-provision-filter, the installPath points
// to an *-ota-*.zip artifact found in the primary build, which is normally an
// AL OS build. However, for tests on the kernel tree, the primary build is
// actually the kernel build, so this artifact doesn't exist. We must replace
// it with an artifact from the OS build, which gets passed in via Args.
func fixInstallPathForKernelTest(installPath string, req *api.InternalTestplan, board string) (string, error) {
	var osBuildId, osTarget string
	args := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
	for _, arg := range args {
		switch arg.GetFlag() {
		case "build_id":
			osBuildId = arg.GetValue()
		case "build_target":
			osTarget = arg.GetValue()
		}
	}
	if osBuildId == "" || osTarget == "" {
		return "", fmt.Errorf(
			"testplan args did not contain all required info for kernel tests: build_id=%s, build_target=%s, args=%+v",
			osBuildId, osTarget, args)
	}
	return common.GetABOTAPath(osBuildId, osTarget, board), nil
}

// extractBuildInfoFromInstallPath parses metadata out of the provided installPath.
// A typical installPath is expected to look like: {common.AndroidBuildPrefix}/{buildId}/{buildTarget}/{board}-ota-{buildId}.zip
// For example: android-build/build_explorer/artifacts_list/123456789/brya-trunk_staging-userdebug/brya-ota-123456789.zip
func extractBuildInfoFromInstallPath(installPath string) (buildId, buildTarget string) {
	trimmedPath := strings.TrimPrefix(installPath, common.AndroidBuildPrefix)
	splitPath := strings.Split(trimmedPath, "/")
	if len(splitPath) < 2 {
		log.Printf("Warning: could not extract buildId and buildTarget from installPath")
		return
	}
	// Indexes 0 and 1 correspond to buildId and buildTarget.
	buildId, buildTarget = splitPath[0], splitPath[1]
	return
}

// applyBuildInfoToTarget sets the provided buildId, buildTarget, buildBranch in the
// target's software request key values.
func applyBuildInfoToTarget(buildId, buildTarget, buildBranch string, target *api.Target) {
	target.GetSwReq().KeyValues = append(target.GetSwReq().KeyValues, []*api.KeyValue{
		{
			Key:   "al_build_id",
			Value: buildId,
		},
		{
			Key:   "al_build_target",
			Value: buildTarget,
		},
		{
			Key:   "al_build_branch",
			Value: buildBranch,
		},
	}...)
}

// buildGetReq constructs a BuildGetRequest for the board.
func buildGetReq(board string, branch string, buildID string) androidapi.BuildGetRequest {
	return androidapi.BuildGetRequest{
		BuildID:            buildID,
		BuildType:          "submitted",
		Board:              board,
		MaxResults:         "1",
		Branch:             branch,
		SortingType:        "creationTimestamp",
		Successful:         "true",
		BuildAttemptStatus: "complete",
	}
}

// getBranch returns the branch used by a SchedulingUnit.
func getBranch(su *api.SchedulingUnit, log *log.Logger) string {
	kvs := su.GetPrimaryTarget().GetSwReq().GetKeyValues()
	if kvs == nil {
		log.Printf("KeyValues found nil")
	}
	for _, kv := range kvs {
		if kv.Key == commonbuilders.ChromeosBuildGcsBucket && kv.Value != commonbuilders.DefaultChromeosBuildGcsBucket {
			return PDKBranch
		}
	}
	return DefaultBranch
}

// getTestType returns the test-type argument from the test request's ExecutionMetadata.
// If no test-type could be found, assume it's an OS test by default.
func getTestType(req *api.InternalTestplan) common.TestType {
	const defaultTestType = common.OSTestType
	args := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
	if args == nil {
		log.Printf("InternalTestplan did not contain args: %+v. Defaulting to test-type %s.", req, defaultTestType)
		return defaultTestType
	}
	for _, arg := range args {
		if arg.GetFlag() != "test-type" {
			continue
		}
		switch arg.GetValue() {
		case string(common.OSTestType):
			return common.OSTestType
		case string(common.KernelTestType):
			return common.KernelTestType
		default:
			log.Printf("Unsure how to parse test-type arg: %s. Defaulting to %s.", arg.GetValue(), defaultTestType)
			return defaultTestType
		}
	}
	log.Printf("No test-type argument found in args: %+v. Defaulting to %s.", args, defaultTestType)
	return defaultTestType
}

// getKernelBuildInfo returns the kernel branch, build ID and build target used for a kernel test.
func getKernelBuildInfo(req *api.InternalTestplan) (kernelBranch, kernelBuildId, kernelTarget string, err error) {
	args := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
	if args == nil {
		return "", "", "", fmt.Errorf("InternalTestplan for kernel test did not contain args: %+v", req)
	}
	for _, arg := range args {
		switch arg.GetFlag() {
		case "kernel_branch":
			kernelBranch = arg.GetValue()
		case "kernel_build":
			kernelBuildId = arg.GetValue()
		case "kernel_target":
			kernelTarget = arg.GetValue()
		}
	}
	if kernelBranch == "" {
		return "", "", "", fmt.Errorf("ExecutionMetadata.Args for kernel test did not contain kernel_branch: %+v", args)
	}
	if kernelBuildId == "" {
		return "", "", "", fmt.Errorf("ExecutionMetadata.Args for kernel test did not contain kernel_build: %+v", args)
	}
	if kernelTarget == "" {
		return "", "", "", fmt.Errorf("ExecutionMetadata.Args for kernel test did not contain kernel_target: %+v", args)
	}
	return
}
