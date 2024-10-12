// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
)

func TestConfig(t *testing.T) {
	ftt.Run("loads config and updates context", t, func(t *ftt.Test) {
		sampleCfg := &Config{
			HostConfigs: map[string]*HostConfig{
				"test-host": {
					RepoConfigs: map[string]*RepoConfig{
						"dummy": {
							BenignFilePattern: &BenignFilePattern{
								Paths: []string{"whitespace.txt", "a/*.txt"},
							},
						},
					},
				},
			},
		}

		c := memory.Use(context.Background())
		SetTestConfig(c, sampleCfg)

		cfg, err := Get(c)
		assert.Loosely(t, err, should.BeNil)

		_, ok := cfg.HostConfigs["test-host"]
		assert.Loosely(t, ok, should.Equal(true))
	})
}

func TestIsRepoRegexpConfigMatch(t *testing.T) {
	ftt.Run("Works", t, func(t *ftt.Test) {
		sampleCfg := &RepoConfig{
			BenignFilePattern: &BenignFilePattern{
				Paths: []string{"whitespace.txt", "a/*.txt"},
			},
		}
		t.Run("Should return the matched config when there is one", func(t *ftt.Test) {
			rrcfgs := []*HostConfig_RepoRegexpConfigPair{
				{
					Key:   "^dummy/dummy-.*$",
					Value: sampleCfg,
				},
				{
					Key:   "^invalid/.*$",
					Value: nil,
				},
			}
			assert.Loosely(t, RetrieveRepoRegexpConfig(context.Background(), "dummy/dummy-valid", rrcfgs), should.Equal(sampleCfg))
		})
		t.Run("Should return nil when no matched config", func(t *ftt.Test) {
			rrcfgs := []*HostConfig_RepoRegexpConfigPair{
				{
					Key:   "^dummy/dummy-.*$",
					Value: sampleCfg,
				},
			}
			assert.Loosely(t, RetrieveRepoRegexpConfig(context.Background(), "dummy-valid", rrcfgs), should.BeNil)
		})
	})
}
