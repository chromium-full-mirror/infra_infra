// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import "infra/cros/cmd/common_lib/interfaces"

// All supported container types.
const (
	// For testing purposes only
	UnsupportedContainerType               interfaces.ContainerType = "UnsupportedContainer"
	AndroidDutTemplatedContainerType       interfaces.ContainerType = "AndroidDutTemplatedContainer"
	AndroidProvisionTemplatedContainerType interfaces.ContainerType = "AndroidProvisionTemplatedContainer"
	CacheServerTemplatedContainerType      interfaces.ContainerType = "CacheServerTemplatedContainer"
	CrosProvisionTemplatedContainerType    interfaces.ContainerType = "CrosProvisionTemplatedContainer"
	CrosVMProvisionTemplatedContainerType  interfaces.ContainerType = "CrosVMProvisionTemplatedContainer"
	CrosDutTemplatedContainerType          interfaces.ContainerType = "CrosDutTemplatedContainer"
	CrosTestTemplatedContainerType         interfaces.ContainerType = "CrosTestTemplatedContainer"
	CrosTestFinderTemplatedContainerType   interfaces.ContainerType = "CrosTestFinderTemplatedContainer"
	CrosGcsPublishTemplatedContainerType   interfaces.ContainerType = "CrosGcsPublishTemplatedContainer"
	CrosTkoPublishTemplatedContainerType   interfaces.ContainerType = "CrosTkoPublishTemplatedContainer"
	CrosRdbPublishTemplatedContainerType   interfaces.ContainerType = "CrosRdbPublishTemplatedContainer"
	CrosPublishTemplatedContainerType      interfaces.ContainerType = "CrosPublishTemplatedContainer"
	GenericProvisionTemplatedContainerType interfaces.ContainerType = "GenericProvisionTemplatedContainer"
	PostProcessTemplatedContainerType      interfaces.ContainerType = "PostProcessTemplatedContainer"
)

// GetContainerImageKeyFromContainerType converts a ContainerType to its commonly known string representation.
func GetContainerImageKeyFromContainerType(containerType interfaces.ContainerType) string {
	switch containerType {
	case UnsupportedContainerType:
		return ""
	case AndroidProvisionTemplatedContainerType:
		return "android-provision"
	case AndroidDutTemplatedContainerType:
		return "cros-dut"
	case CacheServerTemplatedContainerType:
		return "cache-server"
	case CrosProvisionTemplatedContainerType:
		return "cros-provision"
	case CrosDutTemplatedContainerType:
		return "cros-dut"
	case CrosTestTemplatedContainerType:
		return "cros-test"
	case CrosTestFinderTemplatedContainerType:
		return "cros-test-finder"
	case CrosGcsPublishTemplatedContainerType, CrosTkoPublishTemplatedContainerType, CrosRdbPublishTemplatedContainerType, CrosPublishTemplatedContainerType:
		return "cros-publish"
	case CrosVMProvisionTemplatedContainerType:
		return "vm-provision"
	case PostProcessTemplatedContainerType:
		return "post-process"
	default:
		return ""
	}
}
