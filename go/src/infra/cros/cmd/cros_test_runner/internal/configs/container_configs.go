// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"fmt"

	"go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/interfaces"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
)

// CftContainerConfig represents Cft container configs.
type CftContainerConfig struct {
	Ctr                *crostoolrunner.CrosToolRunner
	ContainerImagesMap map[string]*api.ContainerImageInfo

	containersMap map[interfaces.ContainerType]interfaces.ContainerInterface
	cqRun         bool
}

func NewContainerConfig(
	ctr *crostoolrunner.CrosToolRunner,
	containerImagesMap map[string]*api.ContainerImageInfo, cqRun bool) interfaces.ContainerConfigInterface {

	contMap := make(map[interfaces.ContainerType]interfaces.ContainerInterface)
	return &CftContainerConfig{
		Ctr:                ctr,
		ContainerImagesMap: containerImagesMap,
		containersMap:      contMap,
		cqRun:              cqRun,
	}
}

// GetContainer returns the concrete container based on provided container type.
func (cfg *CftContainerConfig) GetContainer(contType interfaces.ContainerType) (interfaces.ContainerInterface, error) {
	// Return container if already created.
	if savedCont, ok := cfg.containersMap[contType]; ok {
		return savedCont, nil
	}

	if len(cfg.ContainerImagesMap) == 0 {
		return nil, fmt.Errorf("ContainerImagesMap is empty!")
	}
	if cfg.Ctr == nil {
		return nil, fmt.Errorf("CrosToolRunner is nil!")
	}

	var cont interfaces.ContainerInterface
	key := containers.GetContainerImageKeyFromContainerType(contType)
	// Get container based on container type.
	switch contType {
	case containers.CrosDutTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewCrosDutTemplatedContainer(containerImage, cfg.Ctr)

	case containers.CrosProvisionTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewCrosProvisionTemplatedContainer(containerImage, cfg.Ctr)
	case containers.AndroidDutTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewAndroidDutTemplatedContainer(containerImage, cfg.Ctr)
	case containers.AndroidProvisionTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewGenericProvisionTemplatedContainer(key, containerImage, cfg.Ctr)

	case containers.CrosTestTemplatedContainerType:
		platform := common.GetBotProvider()
		if platform == common.BotProviderGce && cfg.cqRun {
			key = "cros-test-cq-light"
		}
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewCrosTestTemplatedContainer(containerImage, cfg.Ctr)

	case containers.CrosTestFinderTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewCrosTestFinderTemplatedContainer(containerImage, cfg.Ctr)

	case containers.CrosGcsPublishTemplatedContainerType, containers.CrosTkoPublishTemplatedContainerType, containers.CrosRdbPublishTemplatedContainerType, containers.CrosPublishTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewCrosPublishTemplatedContainer(contType, containerImage, cfg.Ctr)

	case containers.PostProcessTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewPostProcessTemplatedContainer(contType, containerImage, cfg.Ctr)

	case containers.CacheServerTemplatedContainerType:
		containerImage := common.DockerImageCacheServer
		cont = containers.NewCacheServerTemplatedContainer(containerImage, cfg.Ctr)

	case containers.CrosVMProvisionTemplatedContainerType:
		containerImage, err := common.GetContainerImageFromMap(key, cfg.ContainerImagesMap)
		if err != nil {
			return nil, errors.Annotate(err, "error during getting container image from map for %s container type", contType).Err()
		}
		cont = containers.NewCrosVMProvisionTemplatedContainer(containerImage, cfg.Ctr)

	default:
		return nil, fmt.Errorf("Container type %s not supported in container configs!", contType)
	}

	cfg.containersMap[contType] = cont
	return cont, nil
}
