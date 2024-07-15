// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import "time"

// All common constants used throughout the service.
const (
	ServiceConnectionTimeout               = 5 * time.Minute
	CtrCipdPackage                         = "chromiumos/infra/cros-tool-runner/${platform}"
	ContainerDefaultNetwork                = "host"
	LabDockerKeyFileLocation               = "/creds/service_accounts/skylab-drone.json"
	VmLabDockerKeyFileLocation             = "/creds/service_accounts/service-account-chromeos.json"
	VmLabDutHostName                       = "vm"
	GceProject                             = "chromeos-gce-tests"
	GceNetwork                             = "global/networks/chromeos-gce-tests"
	GceMachineTypeN14                      = "n1-standard-4"
	GceMachineTypeN18                      = "n1-standard-8"
	GceMinCpuPlatform                      = "Intel Haswell"
	DockerImageCacheServer                 = "us-docker.pkg.dev/cros-registry/test-services/cacheserver:prod"
	DefaultDockerHost                      = "us-docker.pkg.dev"
	DefaultDockerProject                   = "cros-registry/test-services"
	LroTimeout                             = 1 * time.Minute
	GcsPublishTestArtifactsDir             = "/tmp/gcs-publish-test-artifacts/"
	TKOPublishTestArtifactsDir             = "/tmp/tko-publish-test-artifacts/"
	CpconPublishTestArtifactsDir           = "/tmp/cpcon-publish-test-artifacts/"
	RdbPublishTestArtifactDir              = "/tmp/rdb-publish-test-artifacts/"
	CrosTestCqLight                        = "cros-test-cq-light"
	TesthausURLPrefix                      = "https://tests.chromeos.goog/p/chromeos/logs/unified/"
	GcsURLPrefix                           = "https://pantheon.corp.google.com/storage/browser/"
	HwTestCtrInputPropertyName             = "$chromeos/cros_tool_runner"
	HwTestCtpv2InputPropertyName           = "$chromeos/ctpv2"
	CftServiceMetadataFileName             = ".cftmeta"
	CftServiceMetadataLineContentSeparator = "="
	CftServiceMetadataServicePortKey       = "SERVICE_PORT"
	TestDidNotRunErr                       = "Test did not run"
	CtrCancelingCmdErrString               = "canceling Cmd"
	UfsServiceUrl                          = "ufs.api.cr.dev"
	TkoParseScriptPath                     = "/usr/local/autotest/tko/parse"
	DutConnectionPort                      = 22
	VmLeaserExperimentStr                  = "chromeos.cros_infra_config.vmleaser.launch"
	VmLabMachineTypeExperiment             = "chromeos.cros_infra_config.vmlab.machine_type_n1"
	SwarmingBasePath                       = "https://chromeos-swarming.appspot.com/_ah/api/swarming/v1/"
	SwarmingMaxLimitForEachQuery           = 1000
	ContainerMetadataPath                  = "/metadata/containers.jsonpb"
	TestPlatformDataProjectID              = "chromeos-test-platform-data"
	TestPlatformFireStore                  = "test-platform-store"
	FireStoreContainersStagingCollection   = "containers-staging"
	FireStoreContainersProdCollection      = "containers-prod"
	LabelStaging                           = "staging"
	LabelProd                              = "prod"
	BotParamsRejectedErrKey                = "Bot Params Rejected"
	EnumerationErrKey                      = "Enumeration Error"
	OtherErrKey                            = "Other Error"
	// SourceMetadataPath is the path in the build output directory that
	// details the code sources compiled into the build. The path is
	// specified relative to the root of the build output directory.
	SourceMetadataPath = "/metadata/sources.jsonpb"
	// OS file constants
	// OWNER: Execute, Read, Write
	// GROUP: Execute, Read
	// OTHER: Execute, Read
	DirPermission = 0755
	// OWNER: Read, Write
	// GROUP: Read
	// OTHER: Read
	FilePermission = 0644
)

// Constants relating to dynamic dependency storage.
const (
	// Base task identifiers and image metadata keys.
	CrosProvision    = "cros-provision"
	AndroidProvision = "android-provision"
	FwProvision      = "cros-fw-provision"
	CrosDut          = "cros-dut"
	CrosTest         = "cros-test"
	CrosPublish      = "cros-publish"
	RdbPublish       = "rdb-publish"
	GcsPublish       = "gcs-publish"
	CpconPublish     = "cpcon-publish"
	PostProcess      = "post-process"

	// Device base identifiers.
	Primary   = "primary"
	Companion = "companion"

	// Commonly used Dynamic Dependecy keys.
	ServiceAddress                      = "serviceAddress"
	CrosDutCacheServer                  = "crosDut.cacheServer"
	CrosDutDutAddress                   = "crosDut.dutAddress"
	CrosProvisionMetadataUpdateFirmware = "installRequest.metadata.updateFirmware"
	ProvisionStartupDut                 = "startupRequest.dut"
	ProvisionStartupDutServer           = "startupRequest.dutServer"
	TestRequestTestSuites               = "testRequest.testSuites"
	TestRequestPrimary                  = "testRequest.primary"
	TestRequestCompanions               = "testRequest.companions"
	RequestTestSuites                   = "req.params.testSuites"
	CacheServer                         = "cache-server"
)

var (
	PrimaryDevice            = NewPrimaryDeviceIdentifier().GetDevice()
	CompanionDevices         = NewCompanionDeviceIdentifier("all").GetDevice()
	CompanionDevicesMetadata = NewCompanionDeviceIdentifier("all").GetDeviceMetadata()
)
