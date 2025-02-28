// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import "time"

// All common constants used throughout the service.
const (
	ServiceConnectionTimeout               = 30 * time.Minute
	CtrCipdPackage                         = "chromiumos/infra/cros-tool-runner/${platform}"
	ContainerDefaultNetwork                = "host"
	LabDockerKeyFileLocation               = "/creds/service_accounts/skylab-drone.json"
	VMLabDockerKeyFileLocation             = "/creds/service_accounts/service-account-chromeos.json"
	VMLabDutHostName                       = "vm"
	GceProject                             = "chromeos-gce-tests"
	GceNetwork                             = "global/networks/chromeos-gce-tests"
	GceMachineTypeN14                      = "n1-standard-4"
	GceMachineTypeN18                      = "n1-standard-8"
	GceMinCPUPlatform                      = "Intel Haswell"
	DockerImagePathShaFmt                  = "%s/%s/%s@%s"
	DockerImageCacheServer                 = "us-docker.pkg.dev/cros-registry/test-services/cacheserver:prod"
	DefaultDockerHost                      = "us-docker.pkg.dev"
	DefaultDockerProject                   = "cros-registry/test-services"
	PartnerDockerProject                   = "cros-registry/partner-test-services"
	LroTimeout                             = 1 * time.Minute
	GcsPublishTestArtifactsDir             = "/tmp/gcs-publish-test-artifacts/"
	TKOPublishTestArtifactsDir             = "/tmp/tko-publish-test-artifacts/"
	CpconPublishTestArtifactsDir           = "/tmp/cpcon-publish-test-artifacts/"
	RdbPublishTestArtifactDir              = "/tmp/rdb-publish-test-artifacts/"
	CrosTestCqLight                        = "cros-test-cq-light"
	TesthausURLPrefix                      = "https://tests.chromeos.goog/p/chromeos/logs/unified/"
	GcsURLPrefix                           = "https://pantheon.corp.google.com/storage/browser/"
	HwTestCtrInputPropertyName             = "$chromeos/cros_tool_runner"
	HwTestTrInputPropertyName              = "$chromeos/cros_test_runner"
	HwTestCtpv2InputPropertyName           = "$chromeos/ctpv2"
	CftServiceMetadataFileName             = ".cftmeta"
	CftServiceMetadataLineContentSeparator = "="
	CftServiceMetadataServicePortKey       = "SERVICE_PORT"
	TestDidNotRunErr                       = "Test did not run"
	CtrCancelingCmdErrString               = "canceling Cmd"
	UfsServiceURL                          = "ufs.api.cr.dev"
	TkoParseScriptPath                     = "/usr/local/autotest/tko/parse"
	DutConnectionPort                      = 22
	VMLeaserExperimentStr                  = "chromeos.cros_infra_config.vmleaser.launch"
	VMLabMachineTypeExperiment             = "chromeos.cros_infra_config.vmlab.machine_type_n1"
	SwarmingBasePath                       = "https://chromeos-swarming.appspot.com/_ah/api/swarming/v1/"
	SwarmingMaxLimitForEachQuery           = 1000
	ContainerMetadataPath                  = "/metadata/containers.jsonpb"
	TestPlatformDataProjectID              = "chromeos-test-platform-data"
	TestPlatformFireStore                  = "test-platform-store"
	PartnerTestPlatformFireStore           = "partner-test-platform-store"
	FireStoreContainersStagingCollection   = "containers-staging"
	FireStoreContainersProdCollection      = "containers-prod"
	LabelStaging                           = "staging"
	LabelProd                              = "prod"
	LabelPool                              = "label-pool"
	LabelSuite                             = "label-suite"
	Suite                                  = "suite"
	AnalyticsName                          = "analytics_name"
	BotParamsRejectedErrKey                = "Bot Params Rejected"
	EnumerationErrKey                      = "Enumeration Error"
	SuiteLimitsErrKey                      = "Suite Limits Cancellation"
	OtherErrKey                            = "Other Error"
	CTPBucket                              = "testplatform"
	CTPBucketShadow                        = "testplatform.shadow"
	AndroidBuildPrefix                     = "android-build/build_explorer/artifacts_list/"
	AndroidBuildPathFormat                 = AndroidBuildPrefix + "%s/%s/%s-ota-%s.zip"
	InvocationDataFlag                     = "invocation-data"
	CbIngestionValue                       = "invocation-property=crystalball_ingest:yes"
	AncestorsPropName                      = "ancestor_buildbucket_ids"
	CbPropName                             = "crystalball_ingest"
	CbMetricsPropName                      = "crystalball_has_data"
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

	// Experiments
	DynamicExperiment = "chromeos.cros_infra_config.dynamic_trv2"
)

// AL related constants
const (
	ATPSupportedTimeFormat         = "2006-01-02T15:04:05.000000"
	ATPSwitcherProjectIDAlpha      = "google.com:atp-switcher-alpha"
	ATPSwitcherProjectIDStaging    = "google.com:atp-switcher-staging"
	ATPSwitcherProjectIDProd       = "google.com:atp-switcher"
	ATPSwitcherTestJobEventTopicID = "test_job_event"

	TaskCanceledState  = "CANCELED"
	TaskCompletedState = "COMPLETED"
	TaskErrorState     = "ERROR"
	TaskFatalState     = "FATAL"
	TaskQueuedState    = "QUEUED"
	TaskRunningState   = "RUNNING"
	TaskUnknownState   = "UNKNOWN"
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
	ServoNexus       = "servo-nexus"

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
	TestDynamicDeps                     = "test.dynamicDeps"
	HostIP                              = "host-ip"
	PcqQsAccount                        = "pcq"

	ATILink           = "https://android-build.corp.google.com/test_investigate/invocation"
	PoolConfigsDirURL = "https://chrome-internal.googlesource.com/chromeos/infra/config/+/refs/heads/main/testingconfig/"
	BlockedPoolsURL   = PoolConfigsDirURL + "blocked_pools.txt?format=text"
	DmPoolsURL        = PoolConfigsDirURL + "dm_pools.txt?format=text"
	SchedukePoolsURL  = PoolConfigsDirURL + "ctp2_pools.txt?format=text"

	// Build Experiments
	EnableXTSArchiverExperiment = "chromeos.cros_infra_config.enable_xts_archiver"

	// Injectables
	XTSArchiverResultsGCS = "xts-archiver-results-gcs"
	XTSArchiverAPFEGCS    = "xts-archiver-apfe-gcs"
)

var (
	PrimaryDevice            = NewPrimaryDeviceIdentifier().GetDevice()
	CompanionDevices         = NewCompanionDeviceIdentifier("all").GetDevice()
	CompanionDevicesMetadata = NewCompanionDeviceIdentifier("all").GetDeviceMetadata()
)

// DockerEnvVarsToPreserve gets all env vars that are required
// for hw test execution configs.
func DockerEnvVarsToPreserve() []string {
	return []string{
		"ADB_CONNECTION_PORT",
		"LUCI_CONTEXT",
		"GCE_METADATA_HOST",
		"GCE_METADATA_IP",
		"GCE_METADATA_ROOT",
		"CONTAINER_CACHE_SERVICE_PORT",
		"CONTAINER_CACHE_SERVICE_HOST",
		"DRONE_AGENT_BOT_BLKIO_READ_BPS",
		"DRONE_AGENT_BOT_BLKIO_WRITE_BPS",
		"SWARMING_TASK_ID",
		"SWARMING_BOT_ID",
		"LOGDOG_STREAM_PREFIX",
		"DOCKER_CONFIG",
		"CLOUDBOTS_LAB_DOMAIN",
		"CLOUDBOTS_CA_CERTIFICATE",
		"CLOUDBOTS_PROXY_ADDRESS",
		"DOCKER_CERT_PATH",
		"DRONE_AGENT_HIVE",
		"DOCKER_HOST",
		"DOCKER_TLS_VERIFY"}
}
