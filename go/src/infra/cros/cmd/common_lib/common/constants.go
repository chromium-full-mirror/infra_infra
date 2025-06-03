// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"time"
)

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
	Ctpv2WithFifoPropertyName              = "$chromeos/migration"
	Ctpv2WithFifoMapKey                    = "ctpv2_with_fifo"
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
	LabelPartner                           = "partner"
	LabelPool                              = "label-pool"
	LabelSuite                             = "label-suite"
	Suite                                  = "suite"
	Branch                                 = "branch"
	AnalyticsName                          = "analytics_name"
	BotParamsRejectedErrKey                = "Bot Params Rejected"
	EnumerationErrKey                      = "Enumeration Error"
	SuiteLimitsErrKey                      = "Suite Limits Cancellation"
	OtherErrKey                            = "Other Error"
	CTPBucket                              = "testplatform"
	CTPBucketShadow                        = "testplatform.shadow"
	AndroidBuildPrefix                     = "android-build/build_explorer/artifacts_list/"
	InvocationDataFlag                     = "invocation-data"
	CbIngestionValue                       = "invocation-property=crystalball_ingest:yes"
	AncestorsPropName                      = "ancestor_buildbucket_ids"
	CbPropName                             = "crystalball_ingest"
	CbMetricsPropName                      = "crystalball_has_data"
	GeminiApiKey                           = "gemini-api-key"
	GeminiApiKeyProject                    = "cros-registry"
	FilterCloudRunPort                     = "443"
	FilterCloudRunPortInt                  = 443
	TaggedFilterEndpointSuffix             = "-xxs6mpc42a-uc.a.run.app"
	StagingFilterEndpointSuffix            = "-114509166396.us-central1.run.app"
	ProdFilterEndpointSuffix               = "-771356732494.us-central1.run.app"
	PartnerFilterEndpointSuffix            = "-986313285412.us-central1.run.app"
	PartnerTaggedFilterEndpointSuffix      = "-2nx7zyup7q-uc.a.run.app"
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
	DynamicExperiment  = "chromeos.cros_infra_config.dynamic_trv2"
	CloudRunExperiment = "chromeos.cros_infra_config.cloudrun_enabled"

	// Data Sizes
	KB = 1024
	MB = KB * KB

	// MaxPublishMsgSize the maximum size of the publish request
	// message that the publish gRPC can receive is 4000MB.
	// TODO: Remove once streaming is implemented.
	MaxPublishMsgSize = 4000 * MB
	// MaxFilterMsgSize establishes the max size of the request
	// being sent to the CTP filters.
	MaxFilterMsgSize = 32 * MB
)

// Auth Scopes
const (
	AllPurposeCloudScope = "https://www.googleapis.com/auth/cloud-platform"
	DatastoreScope       = "https://www.googleapis.com/auth/datastore"
	BigqueryScope        = "https://www.googleapis.com/auth/bigquery"
	MoblabScope          = "https://www.googleapis.com/auth/moblabapi"
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
	AshChromeProvision = "ash-chrome-provision"
	CrosProvision      = "cros-provision"
	FoilProvision      = "foil-provision"
	AndroidProvision   = "android-provision"
	FwProvision        = "cros-fw-provision"
	VmProvision        = "vm-provision"
	CrosDut            = "cros-dut"
	CrosTest           = "cros-test"
	CrosPublish        = "cros-publish"
	RdbPublish         = "rdb-publish"
	GcsPublish         = "gcs-publish"
	CpconPublish       = "cpcon-publish"
	PostProcess        = "post-process"
	ServoNexus         = "servo-nexus"

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
		"DOCKER_TLS_VERIFY",
		"DRONE_AGENT_GCS_IMAGE_STORAGE_SERVER"}
}

var (
	CTPv2DockerKeyFileLocations = []string{
		VMLabDockerKeyFileLocation,
		LabDockerKeyFileLocation,
	}
)

var (
	// ctpv2WithFifo stores the pools which are expected to run inside CTPv2 but
	// not scheduled with Scheduke.
	ctpv2WithFifo []string
)

// SetCtpv2WithFifoList sets the value of ctpv2WithFifo. It will throw an error
// if called more than once making the variable like a "read-only" global var.
func SetCtpv2WithFifoList(list []string) error {
	if ctpv2WithFifo != nil {
		return fmt.Errorf("ctpv2WithFifoList list can only be set once")
	}

	ctpv2WithFifo = list
	return nil
}

// GetCtpv2WithFifoList returns a "read-only" copy of the ctpv2WithFifo list.
func GetCtpv2WithFifoList() []string {
	readOnly := make([]string, len(ctpv2WithFifo))
	copy(readOnly, ctpv2WithFifo)
	return readOnly
}

// TestType declares whether a test request is intended to test the kernel, OS, etc.
// It is generally used to communicate between ATP and CTPv2 for AL tests..
type TestType string

// DO NOT CHANGE THESE STRING VALUES
const (
	OSTestType     TestType = "OS"
	KernelTestType TestType = "KERNEL"
)

var (
	// Arguments common to each filter for cloud run deployment.
	FilterArgs = []string{"server", "-port", FilterCloudRunPort}
)
