package config

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/cfgclient"
	"go.chromium.org/luci/config/impl/memory"
	gae "go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/server/caching"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	ftt.Run("With mocks", t, func(t *ftt.Test) {
		configs := map[config.Set]memory.Files{
			"services/${appid}": map[string]string{},
		}
		mockConfig := func(body string) {
			configs["services/${appid}"][cachedCfg.Path] = body
		}

		ctx := gae.Use(context.Background())
		ctx = cfgclient.Use(ctx, memory.New(configs))
		ctx = caching.WithEmptyProcessCache(ctx)

		t.Run("No config", func(t *ftt.Test) {
			assert.Loosely(t, Set(ctx), should.ErrLike("no such config"))

			cfg, err := Get(ctx)
			assert.Loosely(t, cfg, should.BeNil)
			assert.Loosely(t, err, should.ErrLike("failed to fetch cached config"))
		})

		t.Run("Broken config", func(t *ftt.Test) {
			mockConfig("broken")
			assert.Loosely(t, Set(ctx), should.ErrLike("validation errors"))
		})

		t.Run("Good config", func(t *ftt.Test) {
			mockConfig(`
        hosts {
          name: "chromium"
          repos {
            name: "chromium/src"
            priority: true
          }
          repos {
            name: "chromium/src/codesearch"
            do_not_index: true
          }
        }
        hosts {
          name: "webrtc"
          repos {
            name: "src"
            refs: "refs/heads/.*"
            refs: "refs/branch-heads/.*"
            exclude_refs: "refs/branch-heads/7.*"
          }
        }
			`)
			assert.Loosely(t, Set(ctx), should.BeNil)

			cfg, err := Get(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cfg, should.Resemble(&Config{
				Hosts: []*Host{
					{
						Name: "chromium",
						Repos: []*Repository{
							{
								Name: "chromium/src",
								Indexing: &Repository_Priority{
									Priority: true,
								},
							},
							{
								Name: "chromium/src/codesearch",
								Indexing: &Repository_DoNotIndex{
									DoNotIndex: true,
								},
							},
						},
					},
					{
						Name: "webrtc",
						Repos: []*Repository{
							{
								Name: "src",
								Refs: []string{
									"refs/heads/.*",
									"refs/branch-heads/.*",
								},
								ExcludeRefs: []string{"refs/branch-heads/7.*"},
							},
						},
					},
				},
			}))
		})
	})
}
