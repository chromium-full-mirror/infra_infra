// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"fmt"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
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
