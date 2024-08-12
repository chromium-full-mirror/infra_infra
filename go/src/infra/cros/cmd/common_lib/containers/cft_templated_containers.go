// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package containers

import (
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func NewCrosDutTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(CrosDutTemplatedContainerType, "cros-dut", "host", containerImage, ctr, true)
}

func NewAndroidDutTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(AndroidDutTemplatedContainerType, "android-dut", "host", containerImage, ctr, true)
}

func NewCrosProvisionTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(CrosProvisionTemplatedContainerType, "cros-provision", "host", containerImage, ctr, true)
}

func NewCrosVMProvisionTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(CrosVMProvisionTemplatedContainerType, "vm-provision", "host", containerImage, ctr, true)
}

func NewCrosTestTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(CrosTestTemplatedContainerType, "cros-test", "host", containerImage, ctr, true)
}

func NewCrosTestFinderTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(CrosTestFinderTemplatedContainerType, "cros-test-finder", "host", containerImage, ctr, true)
}

func NewCacheServerTemplatedContainer(
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(CacheServerTemplatedContainerType, "cache-server", "host", containerImage, ctr, true)
}

func NewCrosPublishTemplatedContainer(
	contType interfaces.ContainerType,
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	if contType != CrosGcsPublishTemplatedContainerType && contType != CrosTkoPublishTemplatedContainerType && contType != CrosPublishTemplatedContainerType && contType != CrosRdbPublishTemplatedContainerType {
		return nil
	}
	return NewContainer(contType, "cros-publish", "host", containerImage, ctr, true)
}

// NewPostProcessTemplatedContainer creates a new templated post-process container.
func NewPostProcessTemplatedContainer(
	contType interfaces.ContainerType,
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {
	if contType != PostProcessTemplatedContainerType {
		return nil
	}
	return NewContainer(contType, "post-process", "host", containerImage, ctr, true)
}

func NewGenericTemplatedContainer(
	contType interfaces.ContainerType,
	containerImage string,
	namePrefix string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(contType, namePrefix, "host", containerImage, ctr, true)
}

func NewGenericProvisionTemplatedContainer(
	namePrefix string,
	containerImage string,
	ctr *crostoolrunner.CrosToolRunner) interfaces.ContainerInterface {

	return NewContainer(GenericProvisionTemplatedContainerType, namePrefix, "host", containerImage, ctr, true)
}
