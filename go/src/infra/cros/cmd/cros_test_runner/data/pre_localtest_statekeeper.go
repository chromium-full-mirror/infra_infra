// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package data

import (
	"go.chromium.org/chromiumos/config/go/build/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// PreLocalTestStateKeeper represents all the data pre local test execution flow requires.
type PreLocalTestStateKeeper struct {
	interfaces.StateKeeper

	Args                            *LocalArgs
	ContainerKeysRequestedForUpdate []string

	// Updates for localtest_statekeeper
	Tests           []string
	Tags            []string
	TagsExclude     []string
	DutAddress      *labapi.IpEndpoint
	DutCacheAddress *labapi.IpEndpoint
	CacheAddress    *labapi.IpEndpoint
	ContainerImages map[string]*api.ContainerImageInfo
	ImagePath       string
}
