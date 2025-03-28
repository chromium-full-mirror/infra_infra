// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates"
	dynamic_builders "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/builders"
	dynamic_common "go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/common"
	"go.chromium.org/chromiumos/test/ctpv2/common/dynamic_updates/generators"

	androidapi "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/commonbuilders"
)

var (
	DefaultBranch = "git_main-al-dev"
	PDKBranch     = "partner-brya-temp-main-al-dev-fs"
)

// GenerateDynamicProvisionUpdates generates and updates the provision components of the request
func GenerateDynamicProvisionUpdates(req *api.InternalTestplan, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	modifyProvisionRequest(req, updater, log)
	if err := updateProvisionInstallPath(req, updater, log); err != nil {
		return err
	}
	return nil
}

// modifyProvisionRequest adds dynamic updates to the InternalTestplan request
// to configure provisioning containers and associated metadata.
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
	// Required for Satlab's Docker TLS daemon.
	tlsVars := []string{"DOCKER_CERT_PATH", "DOCKER_HOST", "DOCKER_TLS_VERIFY"}
	envvars := servodContainer.GetContainer().GetGeneric().Env
	servodContainer.GetContainer().GetGeneric().Env = append(envvars, tlsVars...)

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

	// Update partnermetadata to the install request
	err = generator.AddModification(
		&api.PartnerMetadata{},
		map[string]string{
			"provision.installRequest.partnerMetadata": "",
		},
	)
	if err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	// Update partner account ID information
	err = generator.AddModification(
		&api.DynamicDep{
			Key:   "installRequest.partnerMetadata.accountId",
			Value: "account-id",
		},
		map[string]string{
			"provision.dynamicDeps": "",
		},
	)
	if err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	// Update partner GCS bucket information
	err = generator.AddModification(
		&api.DynamicDep{
			Key:   "installRequest.partnerMetadata.partnerGcsBucket",
			Value: "partner-gcs-bucket",
		},
		map[string]string{
			"provision.dynamicDeps": "",
		},
	)
	if err != nil {
		log.Printf("Error while adding modification to provision request, %s", err)
	}

	err = dynamic_updates.AppendUserDefinedDynamicUpdates(&req.SuiteInfo.SuiteMetadata.DynamicUpdates, generator.Generate)
	if err != nil {
		log.Printf("Error while modifying provision request, %s", err)
	}

	if getTestType(req) == common.KernelTestType {
		// TODO: b/396475422 - Update provisioning request to specify kernel artifacts and partitions.
		log.Print("This is a kernel test request!")
	}
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
		if err := updateSchedulingUnit(schedulingUnit, updater, log); err != nil {
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
func updateSchedulingUnit(su *api.SchedulingUnit, updater *ALProvisionRequestUpdater, log *log.Logger) error {
	var buildId, buildTarget, installPath string
	if gcsPath := su.GetPrimaryTarget().GetSwReq().GetGcsPath(); strings.HasPrefix(gcsPath, "android-build") {
		buildId, buildTarget = extractBuildInfoFromInstallPath(gcsPath)
		installPath = gcsPath
	} else {
		// Look up latest for board as not provided in gcs path.
		if su.GetDynamicUpdateLookupTable() == nil {
			log.Printf("dynamic lookup table is nil")
		}
		board, ok := su.GetDynamicUpdateLookupTable()["board"]
		if !ok {
			log.Printf("board not found")
		}
		branch := getBranch(su, log)
		var latestGreenBuild int
		var err error
		if latestGreenBuild, ok = updater.LatestBuildsByBoard[board]; !ok {
			latestGreenBuild, err = androidapi.GetLatestGreenBuildNumber(androidapi.ContainerGce, buildGetReq(board, branch))
			if err != nil {
				log.Printf("Error getting latest green build number: %v", err)
				return err
			}
		}
		log.Printf("Latest green build number: %d\n", latestGreenBuild)
		buildId = fmt.Sprint(latestGreenBuild)

		log.Println("Setting build target and latest green build number")
		buildTarget = board + "-trunk_staging-userdebug"
		installPath = getOTAPath(buildId, buildTarget, board)
		log.Printf("InstallPath value: %s", installPath)
	}
	// Make sure the buildId and installPath is consistent in all expected locations.
	su.DynamicUpdateLookupTable["buildNumber"] = buildId
	su.DynamicUpdateLookupTable["installPath"] = installPath
	applyBuildInfoToTarget(buildId, buildTarget, su.GetPrimaryTarget())
	su.GetPrimaryTarget().GetSwReq().GcsPath = installPath
	return nil
}

// extractBuildInfoFromInstallPath extracts the buildId and buildTarget from the
// provided installPath.
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

// applyBuildInfoToTarget sets the provided buildId and buildTarget in the
// target's software request key values.
func applyBuildInfoToTarget(buildId, buildTarget string, target *api.Target) {
	target.GetSwReq().KeyValues = append(target.GetSwReq().KeyValues, []*api.KeyValue{
		{
			Key:   "al_build_id",
			Value: buildId,
		},
		{
			Key:   "al_build_target",
			Value: buildTarget,
		},
	}...)
	return
}

// getOTAPath returns the Android Build path to the *-ota-*.zip artifact.
// buildTarget is the full target name in Android Build, such as
// brya-trunk_staging-userdebug, whereas board is the short name of the board,
// such as brya.
func getOTAPath(buildId, buildTarget, board string) string {
	return fmt.Sprintf(
		common.AndroidBuildPrefix+"%s/%s/%s-ota-%s.zip",
		buildId, buildTarget, board, buildId)
}

// buildGetReq constructs a BuildGetRequest for the board.
func buildGetReq(board string, branch string) androidapi.BuildGetRequest {
	return androidapi.BuildGetRequest{
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
