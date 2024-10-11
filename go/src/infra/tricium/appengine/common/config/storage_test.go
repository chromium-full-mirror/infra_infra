// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common/triciumtest"
)

func TestConfigStorage(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		t.Run("Set and get single project config", func(t *ftt.Test) {
			config := &tricium.ProjectConfig{}
			assert.Loosely(t, setProjectConfig(ctx, "my-project", "version", config), should.BeNil)
			result, err := getProjectConfig(ctx, "my-project")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, result, should.Resemble(config))
		})

		t.Run("Set, get, delete multiple project configs", func(t *ftt.Test) {
			configs := map[string]*tricium.ProjectConfig{
				"infra": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GitRepo{
								GitRepo: &tricium.GitRepo{
									Url: "https://repo-host.com/infra",
								},
							},
						},
					},
				},
				"playground": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GitRepo{
								GitRepo: &tricium.GitRepo{
									Url: "https://repo-host.com/playground",
								},
							},
						},
					},
				},
			}
			assert.Loosely(t, setProjectConfig(ctx, "infra", "v", configs["infra"]), should.BeNil)
			assert.Loosely(t, setProjectConfig(ctx, "playground", "v", configs["playground"]), should.BeNil)
			result, err := getAllProjectConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(result), should.Equal(2))
			assert.Loosely(t, deleteProjectConfigs(ctx, []string{"playground"}), should.BeNil)
			result, err = getAllProjectConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(result), should.Equal(1))
			assert.Loosely(t, result["infra"], should.NotBeNil)
		})

		t.Run("Set and get service config", func(t *ftt.Test) {
			config := &tricium.ServiceConfig{
				Platforms: []*tricium.Platform_Details{
					{
						Name:       platform,
						Dimensions: []string{"pool:Chrome", "os:Ubuntu13.04"},
						HasRuntime: true,
					},
				},
				DataDetails: []*tricium.Data_TypeDetails{
					{
						Type:               tricium.Data_GIT_FILE_DETAILS,
						IsPlatformSpecific: false,
					},
				},
			}
			setServiceConfig(ctx, "version", config)
			result, err := getServiceConfig(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, result, should.NotBeNil)
		})
	})

	ftt.Run("Test Environment with nothing set", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		t.Run("Get service config when none is set", func(t *ftt.Test) {
			_, err := getServiceConfig(ctx)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Get project config when none is set", func(t *ftt.Test) {
			_, err := getProjectConfig(ctx, "project-name")
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Get all project configs when none are set", func(t *ftt.Test) {
			configs, err := getAllProjectConfigs(ctx)
			assert.Loosely(t, configs, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})

	})
}
