// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"testing"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/config/validation"
)

var sampleConfigStr = `
	default_time_window: "7d"
	host_configs {
		key: "test-host"
		value: {
			repo_configs {
				key: "dummy"
				value: {
					benign_file_pattern {
						paths: "a/b.txt"
						paths: "a/*/c.txt"
						paths: "d/*.txt"
						paths: "z/*"
					}
					clean_revert_pattern {
						time_window: "7d"
						excluded_paths: "a/b/*"
					}
					clean_cherry_pick_pattern {
						time_window: "12m"
						excluded_paths: "*.xtb"
					}
				}
			}
			repo_regexp_configs {
				key: "dummy-*-123"
				value: {
					clean_cherry_pick_pattern {
						time_window: "10d"
					}
				}
			}
		}
  	}
`

func createConfig(t testing.TB) *Config {
	t.Helper()

	var cfg Config
	assert.Loosely(t, proto.UnmarshalText(sampleConfigStr, &cfg), should.BeNil, truth.LineContext())
	return &cfg
}

func TestConfigValidator(t *testing.T) {
	validate := func(cfg *Config) error {
		c := validation.Context{Context: context.Background()}
		validateConfig(&c, cfg)
		return c.Finalize()
	}

	ftt.Run("sampleConfigStr is valid", t, func(t *ftt.Test) {
		cfg := createConfig(t)
		assert.Loosely(t, validate(cfg), should.BeNil)
	})

	ftt.Run("validateConfig catches errors", t, func(t *ftt.Test) {
		cfg := createConfig(t)
		t.Run("empty default_time_window", func(t *ftt.Test) {
			cfg.DefaultTimeWindow = ""
			assert.Loosely(t, validate(cfg), should.ErrLike("empty default_time_window"))
		})
		t.Run("validateCleanRevertPattern catches errors", func(t *ftt.Test) {
			crp := cfg.HostConfigs["test-host"].RepoConfigs["dummy"].CleanRevertPattern
			t.Run("invalid time window value", func(t *ftt.Test) {
				crp.TimeWindow = "a1s"
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid time_window a1s"))
			})
			t.Run("invalid time window unit", func(t *ftt.Test) {
				crp.TimeWindow = "12t"
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid time_window 12t"))
			})
		})
		t.Run("validateCleanCherryPickPattern catches errors", func(t *ftt.Test) {
			ccpp := cfg.HostConfigs["test-host"].RepoConfigs["dummy"].CleanCherryPickPattern
			t.Run("invalid time window value", func(t *ftt.Test) {
				ccpp.TimeWindow = "a1s"
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid time_window a1s"))
			})
			t.Run("invalid time window unit", func(t *ftt.Test) {
				ccpp.TimeWindow = "12t"
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid time_window 12t"))
			})
		})
		t.Run("validateRepoRegexpConfig catches errors", func(t *ftt.Test) {
			rrcfgs := cfg.HostConfigs["test-host"].GetRepoRegexpConfigs()
			t.Run("invalid repo regexp", func(t *ftt.Test) {
				rrcfgs[0].Key = `dummy-(ac`
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid repo_regexp dummy-(ac"))
			})
			t.Run("invalid repo config", func(t *ftt.Test) {
				rrcfgs[0].Value.CleanCherryPickPattern.TimeWindow = "abc"
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid time_window abc"))
			})
		})
	})
}
