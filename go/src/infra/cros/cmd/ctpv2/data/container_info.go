// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"fmt"
	"sync"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
)

// ContainerInfo represents all container related info.
type ContainerInfo struct {
	ImageKey        string
	Request         *api.ContainerRequest
	ImageInfo       *buildapi.ContainerImageInfo
	ServiceEndpoint *labapi.IpEndpoint
}

// GetKey gets the image key.
func (contInfo *ContainerInfo) GetKey() string {
	return contInfo.ImageKey
}

// GetImagePath gets the container image path.
func (contInfo *ContainerInfo) GetImagePath() (string, error) {
	imagePath, err := common.CreateImagePath(contInfo.ImageInfo)
	if err != nil {
		return "", errors.Annotate(err, "error getting container image path: ").Err()
	}
	return imagePath, nil
}

// GetEndpointString gets the service endpoint in string where the container service is running.
func (contInfo *ContainerInfo) GetEndpointString() (string, error) {
	if contInfo.ServiceEndpoint == nil {
		return "", errors.Reason("cannot get endpoint string for nil service endpoint.").Err()
	}
	return fmt.Sprintf("%s:%d", contInfo.ServiceEndpoint.GetAddress(), contInfo.ServiceEndpoint.GetPort()), nil
}

type ContainerInfoMap struct {
	syncMap sync.Map
}

func NewContainerInfoMap() *ContainerInfoMap {
	return &ContainerInfoMap{syncMap: sync.Map{}}
}

func (c *ContainerInfoMap) Set(key string, contInfo *ContainerInfo) {
	c.syncMap.Store(key, contInfo)
}

func (c *ContainerInfoMap) Get(key string) (*ContainerInfo, error) {
	value, found := c.syncMap.Load(key)
	if found {
		// Convert interface to struct using type assertion
		contInfo, ok := value.(*ContainerInfo)
		if !ok {
			return nil, fmt.Errorf("conversion to container info failed for key %s", key)
		}
		return contInfo, nil
	}
	return nil, fmt.Errorf("key %s not found in sync map", key)
}
