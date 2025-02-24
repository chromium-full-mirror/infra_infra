// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/impl/memory"

	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

var exampleConfig = map[config.Set]memory.Files{
	"services/app": {
		"service.cfg": `
			platforms {
			  name: UBUNTU
			  dimensions: "pool:tricium"
			  dimensions: "os:Ubuntu"
			  has_runtime: true
			}
			data_details {
			  type: GIT_FILE_DETAILS
			  is_platform_specific: false
			}
			buildbucket_server_host: "cr-buildbucket-dev.appspot.com"
		`,
	},
	"projects/infra": {
		"app.cfg": `
			acls {
			  role: REQUESTER
			  group: "tricium-infra-requesters"
			}
			service_account: "tricium-dev@appspot.gserviceaccount.com"
		`,
	},
	"projects/playground": {
		"app.cfg": `
			acls {
			  role: REQUESTER
			  group: "tricium-playground-requesters"
			}
			service_account: "tricium-dev@appspot.gserviceaccount.com"
		`,
	},
}

// This is invalid because it includes now-unsupported fields.
var invalidConfig = map[config.Set]memory.Files{
	"services/app": {
		"service.cfg": `
			platforms {
			  name: UBUNTU
			  dimensions: "pool:tricium"
			  dimensions: "os:Ubuntu"
			  has_runtime: true
			}

			data_details {
			  type: GIT_FILE_DETAILS
			  is_platform_specific: false
			}
			functions {
			  type: ISOLATOR
			  name: "GitFileIsolator"
			  needs: GIT_FILE_DETAILS
			  provides: FILES
			  impls {
			    runtime_platform: UBUNTU
			    provides_for_platform: UBUNTU
			    cmd {
			      exec: "isolator"
			      args: "--output=${ISOLATED_OUTDIR}"
			    }
			    deadline: 900
			    cipd_packages {
			      package_name: "go.chromium.org/infra/tricium/function/git-file-isolator"
			      path: "."
			      version: "live"
			    }
			  }
			}
			buildbucket_server_host: "cr-buildbucket-dev.appspot.com"
		`,
	},
	"projects/infra": {
		"app.cfg": `
			acls {
			  role: REQUESTER
			  group: "tricium-infra-requesters"
			}
			service_account: "tricium-dev@appspot.gserviceaccount.com"
		`,
	},
	"projects/playground": {
		"app.cfg": `
			acls {
			  role: REQUESTER
			  group: "tricium-playground-requesters"
			}
			selections {
			  function: "GitFileIsolator"
			  platform: UBUNTU
			}
			service_account: "tricium-dev@appspot.gserviceaccount.com"
		`,
	},
}

func TestUpdateConfigs(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {

		ctx := triciumtest.Context()
		ctx = WithConfigService(ctx, memory.New(exampleConfig))

		assert.Loosely(t, common.AppID(ctx), should.Equal("app"))

		t.Run("Configs are not present before updating", func(t *ftt.Test) {
			configs, err := getAllProjectConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(configs), should.BeZero)

			revs, err := getStoredProjectConfigRevisions(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revs, should.BeEmpty)

			rev, err := getStoredServiceConfigRevision(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rev, should.BeEmpty)

			sc, err := getServiceConfig(ctx)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, sc, should.BeNil)
		})

		t.Run("Configs are updated, first time", func(t *ftt.Test) {
			assert.Loosely(t, UpdateAllConfigs(ctx), should.BeNil)
			configs, err := getAllProjectConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, len(configs), should.Match(2))
			assert.Loosely(t, configs["infra"], should.NotBeNil)
			assert.Loosely(t, configs["playground"], should.NotBeNil)

			revs, err := getStoredProjectConfigRevisions(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revs, should.Match(map[string]string{
				"infra":      "ac092d7b6d2c54346ac4ba0027580dbf31183abe",
				"playground": "59673f3521611145fd45e35a2d9e8e2051e7de53",
			}))

			rev, err := getStoredServiceConfigRevision(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rev, should.Equal("6100757dc507346393ee9bfa4c61ae1f2b6935b6"))

			sc, err := getServiceConfig(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, sc, should.NotBeNil)
		})

		t.Run("Configs are updated when some configs already set", func(t *ftt.Test) {
			assert.Loosely(t, setProjectConfig(ctx, "old-project", "abcd", &tricium.ProjectConfig{
				ServiceAccount: "foo@appspot.gserviceaccount.com",
			}), should.BeNil)
			assert.Loosely(t, setProjectConfig(ctx, "infra", "old-version", &tricium.ProjectConfig{
				ServiceAccount: "foo@appspot.gserviceaccount.com",
			}), should.BeNil)
			assert.Loosely(t, setServiceConfig(ctx, "old-version-service-config", &tricium.ServiceConfig{}), should.BeNil)

			revs, err := getStoredProjectConfigRevisions(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revs, should.Match(map[string]string{
				"infra":       "old-version",
				"old-project": "abcd",
			}))
			rev, err := getStoredServiceConfigRevision(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rev, should.Equal("old-version-service-config"))

			assert.Loosely(t, UpdateAllConfigs(ctx), should.BeNil)

			revs, err = getStoredProjectConfigRevisions(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, revs, should.Match(map[string]string{
				"infra":      "ac092d7b6d2c54346ac4ba0027580dbf31183abe",
				"playground": "59673f3521611145fd45e35a2d9e8e2051e7de53",
			}))
			rev, err = getStoredServiceConfigRevision(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rev, should.Equal("6100757dc507346393ee9bfa4c61ae1f2b6935b6"))
		})

		t.Run("Updating an invalid config", func(t *ftt.Test) {
			ctx := WithConfigService(triciumtest.Context(), memory.New(invalidConfig))
			assert.Loosely(t, common.AppID(ctx), should.Equal("app"))
			assert.Loosely(t, UpdateAllConfigs(ctx), should.NotBeNil)
		})
	})
}
