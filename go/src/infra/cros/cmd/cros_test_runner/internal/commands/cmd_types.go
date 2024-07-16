// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"infra/cros/cmd/common_lib/interfaces"
)

// All supported command types.
const (
	// Server related commands
	CommandsServerCmdType interfaces.CommandType = "CommandsServer"

	// Build/env related commands
	BuildInputValidationCmdType interfaces.CommandType = "BuildInputValidation"
	ParseEnvInfoCmdType         interfaces.CommandType = "ParseEnvInfoCmd"
	ProcessResultsCmdType       interfaces.CommandType = "ProcessResultsCmd"
	ParseArgsCmdType            interfaces.CommandType = "ParseArgs"

	// Container related commands
	UpdateContainerImagesLocallyCmdType interfaces.CommandType = "UpdateContainerImagesLocally"
	FetchContainerMetadataCmdType       interfaces.CommandType = "FetchContainerMetadata"

	// Inventory service related commands
	InvServiceStartCmdType  interfaces.CommandType = "InvServiceStart"
	InvServiceStopCmdType   interfaces.CommandType = "InvServiceStop"
	LoadDutTopologyCmdType  interfaces.CommandType = "LoadDutTopology"
	BuildDutTopologyCmdType interfaces.CommandType = "BuildDutTopology"
	ParseDutTopologyCmdType interfaces.CommandType = "ParseDutTopology"

	// SSH service related commands
	SshStartTunnelCmdType        interfaces.CommandType = "SshTunnelStart"
	SshStartReverseTunnelCmdType interfaces.CommandType = "SshReverseTunnelStart"
	SshStopTunnelsCmdType        interfaces.CommandType = "SshTunnelsStop"

	// Cache server related commands
	CacheServerStartCmdType interfaces.CommandType = "CacheServerStart"

	// Dut service related commands
	DutServiceStartCmdType                 interfaces.CommandType = "DutServiceStart"
	AndroidCompanionDutServiceStartCmdType interfaces.CommandType = "AndroidCompanionDutServiceStart"

	// Dut VM test related commands
	DutVmGetImageCmdType         interfaces.CommandType = "DutVmGetImage"
	DutVmCacheServerStartCmdType interfaces.CommandType = "DutVmCacheServerStart"

	// Provision service related commands

	AndroidProvisionServiceStartCmdType interfaces.CommandType = "AndroidProvisionServiceStart"
	AndroidProvisionInstallCmdType      interfaces.CommandType = "AndroidProvisionInstall"
	ProvisionServiceStartCmdType        interfaces.CommandType = "ProvisionServiceStart"
	ProvisonInstallCmdType              interfaces.CommandType = "ProvisonInstall"
	VMProvisionServiceStartCmdType      interfaces.CommandType = "VMProvisionServiceStart"
	VMProvisionLeaseCmdType             interfaces.CommandType = "VMProvisionLease"
	VMProvisionReleaseCmdType           interfaces.CommandType = "VMProvisionRelease"
	GenericProvisionCmdType             interfaces.CommandType = "GenericProvision"

	// Test Finder service related commands
	TestFinderServiceStartCmdType interfaces.CommandType = "TestFinderServiceStart"
	TestFinderExecutionCmdType    interfaces.CommandType = "TestFinderExecution"

	// Test service related commands
	TestServiceStartCmdType interfaces.CommandType = "TestServiceStart"
	TestsExecutionCmdType   interfaces.CommandType = "TestsExecution"
	GenericTestsCmdType     interfaces.CommandType = "GenericTests"

	// Generic Service related commands
	GenericServiceCmdType interfaces.CommandType = "GenericService"

	// Post process service related commands
	GenericPostProcessCmdType interfaces.CommandType = "GenericPostProcess"

	// Publish service related commands
	GenericPublishCmdType interfaces.CommandType = "GenericPublish"

	GcsPublishStartCmdType  interfaces.CommandType = "GcsPublishStart"
	GcsPublishUploadCmdType interfaces.CommandType = "GcsPublishUpload"

	TkoPublishStartCmdType  interfaces.CommandType = "TkoPublishStart"
	TkoPublishUploadCmdType interfaces.CommandType = "TkoPublishUpload"
	TkoDirectUploadCmdType  interfaces.CommandType = "TkoDirectUpload"

	CpconPublishStartCmdType  interfaces.CommandType = "CpconPublishStart"
	CpconPublishUploadCmdType interfaces.CommandType = "CpconPublishUpload"

	RdbPublishStartCmdType  interfaces.CommandType = "RdbPublishStart"
	RdbPublishUploadCmdType interfaces.CommandType = "RdbPublishUpload"

	// Ufs related commands
	UpdateDutStateCmdType interfaces.CommandType = "UpdateDutState"

	// For testing purposes only
	UnSupportedCmdType interfaces.CommandType = "UnSupportedCmd"
)
