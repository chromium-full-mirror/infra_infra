// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reviewer

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/common/proto"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"

	"go.chromium.org/infra/appengine/rubber-stamper/config"
	"go.chromium.org/infra/appengine/rubber-stamper/tasks/taskspb"
)

func TestReviewBenignFileChange(t *testing.T) {
	ftt.Run("review benign file change", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())

		ctl := gomock.NewController(t)
		defer ctl.Finish()
		gerritMock := gerritpb.NewMockGerritClient(ctl)

		tsk := &taskspb.ChangeReviewTask{
			Host:       "test-host",
			Number:     12345,
			Revision:   "123abc",
			Repo:       "dummy",
			AutoSubmit: false,
		}

		t.Run("BenignFilePattern in RepoConfig works", func(t *ftt.Test) {
			hostCfg := &config.HostConfig{
				RepoConfigs: map[string]*config.RepoConfig{
					"dummy": {
						BenignFilePattern: &config.BenignFilePattern{
							Paths: []string{
								"test/a/*",
								"test/b/c.txt",
								"test/c/**",
								"test/override/**",
								"!test/override/**",
								"test/override/a/**",
								"!test/override/a/b/*",
							},
						},
					},
				},
			}

			t.Run("valid files with gitignore style patterns", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"/COMMIT_MSG":   nil,
						"test/a/b.xtb":  nil,
						"test/b/c.txt":  nil,
						"test/c/pp.xtb": nil,
						"test/c/i/a.md": nil,
					},
				}, nil)
				invalidFiles, err := reviewBenignFileChange(ctx, hostCfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(invalidFiles), should.BeZero)
			})
			t.Run("gitigore style patterns' order matters", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"/COMMIT_MSG":             nil,
						"test/override/1.txt":     nil,
						"test/override/a/2.txt":   nil,
						"test/override/a/b/3.txt": nil,
						"test/override/a/c/4.txt": nil,
						"test/override/ab/5.txt":  nil,
					},
				}, nil)
				invalidFiles, err := reviewBenignFileChange(ctx, hostCfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, invalidFiles, should.Resemble([]string{"test/override/1.txt", "test/override/a/b/3.txt", "test/override/ab/5.txt"}))
			})
			t.Run("gerrit ListFiles API returns error", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				invalidFiles, err := reviewBenignFileChange(ctx, hostCfg, gerritMock, tsk)
				assert.Loosely(t, err, should.ErrLike("gerrit ListFiles rpc call failed with error"))
				assert.Loosely(t, len(invalidFiles), should.BeZero)
			})
		})

		t.Run("BenignFilePattern in RepoRegexpConfig works", func(t *ftt.Test) {
			hostCfg := &config.HostConfig{
				RepoRegexpConfigs: []*config.HostConfig_RepoRegexpConfigPair{
					{
						Key: "^.*mmy$",
						Value: &config.RepoConfig{
							BenignFilePattern: &config.BenignFilePattern{
								Paths: []string{
									"test/a/*",
									"test/b/c.txt",
									"test/c/**",
									"test/override/**",
									"!test/override/**",
									"test/override/a/**",
									"!test/override/a/b/*",
								},
							},
						},
					},
				},
			}

			t.Run("valid files with gitignore style patterns", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"/COMMIT_MSG":   nil,
						"test/a/b.xtb":  nil,
						"test/b/c.txt":  nil,
						"test/c/pp.xtb": nil,
						"test/c/i/a.md": nil,
					},
				}, nil)
				invalidFiles, err := reviewBenignFileChange(ctx, hostCfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(invalidFiles), should.BeZero)
			})
			t.Run("gitigore style patterns' order matters", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"/COMMIT_MSG":             nil,
						"test/override/1.txt":     nil,
						"test/override/a/2.txt":   nil,
						"test/override/a/b/3.txt": nil,
						"test/override/a/c/4.txt": nil,
						"test/override/ab/5.txt":  nil,
					},
				}, nil)
				invalidFiles, err := reviewBenignFileChange(ctx, hostCfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, invalidFiles, should.Resemble([]string{"test/override/1.txt", "test/override/a/b/3.txt", "test/override/ab/5.txt"}))
			})
			t.Run("gerrit ListFiles API returns error", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				invalidFiles, err := reviewBenignFileChange(ctx, hostCfg, gerritMock, tsk)
				assert.Loosely(t, err, should.ErrLike("gerrit ListFiles rpc call failed with error"))
				assert.Loosely(t, len(invalidFiles), should.BeZero)
			})
		})
	})
}

func TestRetrieveBenignFilePattern(t *testing.T) {
	ftt.Run("retrieveBenignFilePattern works", t, func(t *ftt.Test) {
		sampleBenignFilePattern := &config.BenignFilePattern{
			Paths: []string{
				"test/a/*",
				"test/b/c.txt",
			},
		}
		t.Run("returns nil when hostConfig is nil", func(t *ftt.Test) {
			assert.Loosely(t, retrieveBenignFilePattern(context.Background(), nil, "dummy"), should.BeNil)
		})
		t.Run("when repoConfig exists", func(t *ftt.Test) {
			hostCfg := &config.HostConfig{
				RepoRegexpConfigs: []*config.HostConfig_RepoRegexpConfigPair{
					{
						Key:   "^.*mmy$",
						Value: nil,
					},
				},
			}
			t.Run("BenignFilePattern exists", func(t *ftt.Test) {
				hostCfg.RepoConfigs = map[string]*config.RepoConfig{
					"dummy": {
						BenignFilePattern: sampleBenignFilePattern,
					},
				}
				assert.Loosely(t, retrieveBenignFilePattern(context.Background(), hostCfg, "dummy"), should.Equal(sampleBenignFilePattern))
			})
			t.Run("BenignFilePattern is nil", func(t *ftt.Test) {
				hostCfg.RepoConfigs = map[string]*config.RepoConfig{
					"dummy": {
						BenignFilePattern: nil,
					},
				}
				assert.Loosely(t, retrieveBenignFilePattern(context.Background(), hostCfg, "dummy"), should.BeNil)
			})
		})
		t.Run("when repoConfig doesn't exist and repoRegexpConfig exists", func(t *ftt.Test) {
			t.Run("BenignFilePattern exists", func(t *ftt.Test) {
				hostCfg := &config.HostConfig{
					RepoRegexpConfigs: []*config.HostConfig_RepoRegexpConfigPair{
						{
							Key: "^.*mmy$",
							Value: &config.RepoConfig{
								BenignFilePattern: sampleBenignFilePattern,
							},
						},
					},
				}
				assert.Loosely(t, retrieveBenignFilePattern(context.Background(), hostCfg, "dummy"), should.Equal(sampleBenignFilePattern))
			})
			t.Run("BenignFilePattern is nil", func(t *ftt.Test) {
				hostCfg := &config.HostConfig{
					RepoRegexpConfigs: []*config.HostConfig_RepoRegexpConfigPair{
						{
							Key: "^.*mmy$",
							Value: &config.RepoConfig{
								BenignFilePattern: nil,
							},
						},
					},
				}
				assert.Loosely(t, retrieveBenignFilePattern(context.Background(), hostCfg, "dummy"), should.BeNil)
			})
		})
		t.Run("returns nil when no repo config can be found", func(t *ftt.Test) {
			hostCfg := &config.HostConfig{
				RepoConfigs: map[string]*config.RepoConfig{
					"dummy": {
						BenignFilePattern: sampleBenignFilePattern,
					},
				},
				RepoRegexpConfigs: []*config.HostConfig_RepoRegexpConfigPair{
					{
						Key: "^.*mmy$",
						Value: &config.RepoConfig{
							BenignFilePattern: nil,
						},
					},
				},
			}
			assert.Loosely(t, retrieveBenignFilePattern(context.Background(), hostCfg, "invalid"), should.BeNil)
		})
	})
}
