// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestValidateVmLeaserBackend(t *testing.T) {
	ftt.Run("CreateVmInstanceRequest Validate", t, func(t *ftt.Test) {
		t.Run("Valid request - successful path", func(t *ftt.Test) {
			d, err := time.ParseDuration("60s")
			assert.Loosely(t, err, should.BeNil)

			req := &CreateVmInstanceRequest{
				Config: &Config{
					Backend: &Config_VmLeaserBackend_{
						VmLeaserBackend: &Config_VmLeaserBackend{
							Env: Config_VmLeaserBackend_ENV_LOCAL,
							VmRequirements: &api.VMRequirements{
								GceImage:       "test-image",
								GceRegion:      "test-region",
								GceProject:     "test-project",
								GceMachineType: "test-machine-type",
								GceDiskSize:    100,
							},
							LeaseDuration: durationpb.New(d),
						},
					},
				},
			}
			err = req.ValidateVmLeaserBackend()
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Invalid request - empty request", func(t *ftt.Test) {
			req := &CreateVmInstanceRequest{}
			err := req.ValidateVmLeaserBackend()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: no config found"))
		})
		t.Run("Invalid request - empty VmLeaserBackend", func(t *ftt.Test) {
			req := &CreateVmInstanceRequest{
				Config: &Config{},
			}
			err := req.ValidateVmLeaserBackend()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: bad backend: want vmleaser"))
		})
		t.Run("Invalid request - wrong backend", func(t *ftt.Test) {
			req := &CreateVmInstanceRequest{
				Config: &Config{
					Backend: &Config_GcloudBackend{
						GcloudBackend: &Config_GCloudBackend{},
					},
				},
			}
			err := req.ValidateVmLeaserBackend()
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.ErrLike("invalid argument: bad backend: want vmleaser"))
		})
	})
}
