// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/duration"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/cfgclient"
	cfgmem "go.chromium.org/luci/config/impl/memory"
	"go.chromium.org/luci/config/validation"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/server/caching"
	"go.chromium.org/luci/server/router"

	"infra/appengine/arquebus/app/util"
)

func createConfig(t testing.TB, id string) *Config {
	t.Helper()

	// returns an assigner with a given ID and all required fields.
	var cfg Assigner
	assert.Loosely(t, proto.UnmarshalText(util.SampleValidAssignerCfg, &cfg), should.BeNil, truth.LineContext())
	cfg.Id = id

	return &Config{
		AccessGroup:           "trooper",
		MonorailHostname:      "example.com",
		RotationProxyHostname: "example.net",
		Assigners:             []*Assigner{&cfg},
	}
}

func createRotationSource(rotation string) *UserSource_Rotation {
	return &UserSource_Rotation{
		Rotation: &Oncall{Rotation: rotation, Position: Oncall_PRIMARY},
	}
}

func TestMiddleware(t *testing.T) {
	t.Parallel()

	ftt.Run("loads config and updates context", t, func(t *ftt.Test) {
		c := memory.Use(context.Background())
		c = caching.WithEmptyProcessCache(c)
		c = cfgclient.Use(c, cfgmem.New(map[config.Set]cfgmem.Files{
			"services/${appid}": {
				cachedCfg.Path: createConfig(t, "assigner").String(),
			},
		}))

		Middleware(&router.Context{Request: (&http.Request{}).WithContext(c)}, func(c *router.Context) {
			assert.Loosely(t, Get(c.Request.Context()).AccessGroup, should.Equal("trooper"))
			assert.Loosely(t, GetConfigRevision(c.Request.Context()), should.NotEqual(""))
		})
	})
}

func TestConfigValidator(t *testing.T) {
	t.Parallel()

	validate := func(cfg *Config) error {
		c := validation.Context{Context: context.Background()}
		validateConfig(&c, cfg)
		return c.Finalize()
	}

	ftt.Run("devcfg template is valid", t, func(t *ftt.Test) {
		content, err := ioutil.ReadFile(
			"../devcfg/services/dev/config-template.cfg",
		)
		assert.Loosely(t, err, should.BeNil)
		cfg := &Config{}
		assert.Loosely(t, proto.UnmarshalText(string(content), cfg), should.BeNil)
		assert.Loosely(t, validate(cfg), should.BeNil)
	})

	ftt.Run("empty monorail_hostname is not valid", t, func(t *ftt.Test) {
		cfg := createConfig(t, "my-assigner")
		cfg.MonorailHostname = ""
		assert.Loosely(t, validate(cfg), should.ErrLike("empty value is not allowed"))
	})

	ftt.Run("validateConfig catches errors", t, func(t *ftt.Test) {
		t.Run("For duplicate IDs", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners = append(cfg.Assigners, cfg.Assigners[0])
			assert.Loosely(t, validate(cfg), should.ErrLike("duplicate id"))
		})

		t.Run("for invalid IDs", func(t *ftt.Test) {
			msg := "invalid id"
			assert.Loosely(t, validate(createConfig(t, "a-")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "a-")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "-a")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "-")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "a--b")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "a@!3")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "12=56")), should.ErrLike(msg))
			assert.Loosely(t, validate(createConfig(t, "A-cfg")), should.ErrLike(msg))
		})

		t.Run("for invalid owners", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners[0].Owners = []string{"example.com"}
			assert.Loosely(t, validate(cfg), should.ErrLike("invalid email address"))
		})

		t.Run("for missing interval", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners[0].Interval = nil
			assert.Loosely(t, validate(cfg), should.ErrLike("missing interval"))
		})

		t.Run("for an interval shoter than 1 minute", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners[0].Interval = &duration.Duration{Seconds: 59}
			assert.Loosely(t, validate(cfg), should.ErrLike("interval should be at least one minute"))
		})

		t.Run("for missing assignees", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners[0].Assignees = []*UserSource{}
			t.Run("with ccs", func(t *ftt.Test) {
				// If ccs[] is given, assignees[] can be omitted.
				assert.Loosely(t, cfg.Assigners[0].Ccs, should.NotBeNil)
				assert.Loosely(t, validate(cfg), should.BeNil)
			})

			t.Run("Without ccs", func(t *ftt.Test) {
				cfg.Assigners[0].Ccs = []*UserSource{}
				assert.Loosely(t, validate(cfg), should.ErrLike("at least one of assignees or ccs must be given"))
			})
		})

		t.Run("for missing ccs", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners[0].Ccs = []*UserSource{}
			t.Run("with assignees", func(t *ftt.Test) {
				// If assignees[] is given, ccs[] can be omitted.
				assert.Loosely(t, cfg.Assigners[0].Ccs, should.NotBeNil)
				assert.Loosely(t, validate(cfg), should.BeNil)
			})

			t.Run("Without assignees", func(t *ftt.Test) {
				cfg.Assigners[0].Assignees = []*UserSource{}
				assert.Loosely(t, validate(cfg), should.ErrLike("at least one of assignees or ccs must be given"))
			})
		})

		t.Run("for missing issue_query", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			cfg.Assigners[0].IssueQuery = nil
			assert.Loosely(t, validate(cfg), should.ErrLike("missing issue_query"))
			cfg.Assigners[0].IssueQuery = &IssueQuery{ProjectNames: []string{}}
			assert.Loosely(t, validate(cfg), should.ErrLike("missing q"))
			cfg.Assigners[0].IssueQuery = &IssueQuery{Q: "text"}
			assert.Loosely(t, validate(cfg), should.ErrLike("missing project_names"))
		})

		t.Run("for valid UserResource", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			assigner := cfg.Assigners[0]
			source := &UserSource{}
			assigner.Assignees[0] = source

			t.Run("with valid rotation names", func(t *ftt.Test) {
				source.From = createRotationSource("oncallator:foo-bar")
				assert.Loosely(t, validate(cfg), should.BeNil)
				source.From = createRotationSource("grotation:foo-bar")
				assert.Loosely(t, validate(cfg), should.BeNil)
			})
		})

		t.Run("for invalid UserResource", func(t *ftt.Test) {
			cfg := createConfig(t, "my-assigner")
			assigner := cfg.Assigners[0]
			source := &UserSource{}
			assigner.Assignees[0] = source

			t.Run("with missing value", func(t *ftt.Test) {
				source.Reset()
				assert.Loosely(t, validate(cfg), should.ErrLike("missing or unknown user source"))
			})

			t.Run("with invalid rotation names", func(t *ftt.Test) {
				invalidID := "invalid id"
				source.From = createRotationSource("")
				assert.Loosely(t, validate(cfg), should.ErrLike("either name or rotation must be specified"))
				source.From = createRotationSource("foo-bar")
				assert.Loosely(t, validate(cfg), should.ErrLike(invalidID))
				source.From = createRotationSource("oncallator: foo-bar")
				assert.Loosely(t, validate(cfg), should.ErrLike(invalidID))
				source.From = createRotationSource("oncallator:foo:bar")
				assert.Loosely(t, validate(cfg), should.ErrLike(invalidID))
				source.From = createRotationSource("oncallator:[foo-bar]")
				assert.Loosely(t, validate(cfg), should.ErrLike(invalidID))
			})

			t.Run("with invalid user", func(t *ftt.Test) {
				source.From = &UserSource_Email{Email: "example"}
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid email"))
				source.From = &UserSource_Email{Email: "example.org"}
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid email"))
				source.From = &UserSource_Email{Email: "http://foo@example.org"}
				assert.Loosely(t, validate(cfg), should.ErrLike("invalid email"))
			})
		})
	})
}
