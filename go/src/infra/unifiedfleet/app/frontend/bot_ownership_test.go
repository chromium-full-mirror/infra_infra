// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package frontend

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	api "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/controller"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
)

var branchNumber uint32 = 0

// encTestingContext creates a testing context which mocks the logging and datastore services.
// Also loads a custom config, which will allow the loading of a dummy bot config file
func encTestingContext() context.Context {
	c := gaetesting.TestingContextWithAppID("dev~infra-unified-fleet-system")
	c = gologger.StdConfig.Use(c)
	c = logging.SetLevel(c, logging.Error)
	c = config.Use(c, &config.Config{
		OwnershipConfig: &config.OwnershipConfig{
			GitilesHost: "test_gitiles",
			Project:     "test_project",
			Branch:      fmt.Sprintf("test_branch_%d", atomic.AddUint32(&branchNumber, 1)),
			EncConfig: []*config.OwnershipConfig_ConfigFile{
				{
					Name:       "test_name",
					RemotePath: "test_enc_git_path",
				},
			},
			SecurityConfig: []*config.OwnershipConfig_ConfigFile{
				{
					Name:       "test_name",
					RemotePath: "test_security_git_path",
				},
			},
		},
	})
	c = external.WithTestingContext(c)
	datastore.GetTestable(c).Consistent(true)
	return c
}

// Tests the RPC for getting ownership data
func TestGetOwnershipData(t *testing.T) {
	t.Parallel()
	ctx := encTestingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Get Ownership Data for Bots", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "testing-1"})
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = controller.ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			req := &api.GetOwnershipDataRequest{
				Hostname: "testing-1",
			}

			res, err := tf.Fleet.GetOwnershipData(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, res.Pools, should.Contain("test"))
			assert.Loosely(t, res.SwarmingInstance, should.Equal("testSwarming"))
		})
		t.Run("Missing host - returns error", func(t *ftt.Test) {
			req := &api.GetOwnershipDataRequest{
				Hostname: "blah-1",
			}
			res, err := tf.Fleet.GetOwnershipData(ctx, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("not found"))
		})
	})
}

// Tests the RPC for listing ownership data
func TestListOwnershipData(t *testing.T) {
	t.Parallel()
	ctx := encTestingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("List Ownership Data for Bots", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			err := controller.ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			req := &api.ListOwnershipDataRequest{
				PageSize: 10,
			}

			res, err := tf.Fleet.ListOwnershipData(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, len(res.OwnershipData), should.Equal(10))
			assert.Loosely(t, res.NextPageToken, should.NotBeBlank)

			// Get next set of entities
			req = &api.ListOwnershipDataRequest{
				PageSize:  10,
				PageToken: res.NextPageToken,
			}

			res, err = tf.Fleet.ListOwnershipData(ctx, req)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, len(res.OwnershipData), should.Equal(4))
			assert.Loosely(t, res.NextPageToken, should.Equal(""))
		})
	})
}
