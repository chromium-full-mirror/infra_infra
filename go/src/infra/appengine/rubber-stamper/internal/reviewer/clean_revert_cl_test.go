// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reviewer

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/proto"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"

	"go.chromium.org/infra/appengine/rubber-stamper/config"
	"go.chromium.org/infra/appengine/rubber-stamper/tasks/taskspb"
)

func TestReviewCleanRevert(t *testing.T) {
	ftt.Run("review clean revert", t, func(t *ftt.Test) {
		ctx := memory.Use(context.Background())

		ctl := gomock.NewController(t)
		defer ctl.Finish()
		gerritMock := gerritpb.NewMockGerritClient(ctl)

		cfg := &config.Config{
			DefaultTimeWindow: "7d",
			HostConfigs: map[string]*config.HostConfig{
				"test-host": {
					RepoConfigs: map[string]*config.RepoConfig{},
				},
			},
		}

		tsk := &taskspb.ChangeReviewTask{
			Host:       "test-host",
			Number:     12345,
			Revision:   "123abc",
			Repo:       "dummy",
			AutoSubmit: false,
			RevertOf:   45678,
		}

		t.Run("clean revert with no repo config is valid", func(t *ftt.Test) {
			gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
				Number:  tsk.Number,
				Project: tsk.Repo,
			})).Return(&gerritpb.PureRevertInfo{
				IsPureRevert: true,
			}, nil)
			gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
				Number:  tsk.RevertOf,
				Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
			})).Return(&gerritpb.ChangeInfo{
				CurrentRevision: "456def",
				Revisions: map[string]*gerritpb.RevisionInfo{
					"456def": {
						Created: timestamppb.Now(),
					},
				},
			}, nil)
			msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msg, should.BeEmpty)
		})
		t.Run("clean revert with repo config is valid", func(t *ftt.Test) {
			cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
				CleanRevertPattern: &config.CleanRevertPattern{
					TimeWindow:    "5m",
					ExcludedPaths: []string{"a/b/c.txt", "a/**/*.md"},
				},
			}
			gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
				Number:  tsk.Number,
				Project: tsk.Repo,
			})).Return(&gerritpb.PureRevertInfo{
				IsPureRevert: true,
			}, nil)
			gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
				Number:  tsk.RevertOf,
				Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
			})).Return(&gerritpb.ChangeInfo{
				CurrentRevision: "456def",
				Revisions: map[string]*gerritpb.RevisionInfo{
					"456def": {
						Created: timestamppb.New(time.Now().Add(-time.Minute)),
					},
				},
			}, nil)
			gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
				Number:     tsk.Number,
				RevisionId: tsk.Revision,
			})).Return(&gerritpb.ListFilesResponse{
				Files: map[string]*gerritpb.FileInfo{
					"a/d/c.txt": nil,
					"a/valid.c": nil,
				},
			}, nil)
			msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, msg, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("clean revert with repo exp config is valid", func(t *ftt.Test) {
			cfg.HostConfigs["test-host"].RepoRegexpConfigs = []*config.HostConfig_RepoRegexpConfigPair{
				{
					Key: "^.*my$",
					Value: &config.RepoConfig{
						CleanRevertPattern: &config.CleanRevertPattern{
							TimeWindow:    "5m",
							ExcludedPaths: []string{"a/b/c.txt", "a/**/*.md"},
						},
					},
				},
			}
			gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
				Number:  tsk.Number,
				Project: tsk.Repo,
			})).Return(&gerritpb.PureRevertInfo{
				IsPureRevert: true,
			}, nil)
			gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
				Number:  tsk.RevertOf,
				Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
			})).Return(&gerritpb.ChangeInfo{
				CurrentRevision: "456def",
				Revisions: map[string]*gerritpb.RevisionInfo{
					"456def": {
						Created: timestamppb.New(time.Now().Add(-time.Minute)),
					},
				},
			}, nil)
			gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
				Number:     tsk.Number,
				RevisionId: tsk.Revision,
			})).Return(&gerritpb.ListFilesResponse{
				Files: map[string]*gerritpb.FileInfo{
					"a/d/c.txt": nil,
					"a/valid.c": nil,
				},
			}, nil)
			msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, msg, should.BeEmpty)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("invalid when gerrit GetPureRevert api returns false", func(t *ftt.Test) {
			gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
				Number:  tsk.Number,
				Project: tsk.Repo,
			})).Return(&gerritpb.PureRevertInfo{
				IsPureRevert: false,
			}, nil)
			msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msg, should.Equal("Gerrit GetPureRevert API does not mark this CL as a pure revert."))
		})
		t.Run("invalid when out of time window", func(t *ftt.Test) {
			commonMock := func() {
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: true,
				}, nil)
			}
			t.Run("global time window works", func(t *ftt.Test) {
				commonMock()
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-8 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review reverts within 7 day(s)."))
			})
			t.Run("host-level time window works", func(t *ftt.Test) {
				commonMock()
				cfg.HostConfigs["test-host"].CleanRevertTimeWindow = "5d"
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-6 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review reverts within 5 day(s)."))
			})
			t.Run("repo-level time window works", func(t *ftt.Test) {
				commonMock()
				cfg.HostConfigs["test-host"].CleanRevertTimeWindow = "5d"
				cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
					CleanRevertPattern: &config.CleanRevertPattern{
						TimeWindow: "5m",
					},
				}
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review reverts within 5 minute(s)."))
			})
			t.Run("repo-level time window from regexp config works", func(t *ftt.Test) {
				commonMock()
				cfg.HostConfigs["test-host"].CleanRevertTimeWindow = "5d"
				cfg.HostConfigs["test-host"].RepoRegexpConfigs = []*config.HostConfig_RepoRegexpConfigPair{
					{
						Key: "^.*ummy$",
						Value: &config.RepoConfig{
							CleanRevertPattern: &config.CleanRevertPattern{
								TimeWindow: "12m",
							},
						},
					},
				}
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review reverts within 12 minute(s)."))
			})
		})
		t.Run("invalid when contains excluded files", func(t *ftt.Test) {
			t.Run("repo-level excluded files works", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
					CleanRevertPattern: &config.CleanRevertPattern{
						ExcludedPaths: []string{"a/b/c.txt", "a/**/*.md"},
					},
				}
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: true,
				}, nil)
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-2 * 24 * time.Hour)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/b/c.txt":  nil,
						"a/a/c/a.md": nil,
						"a/valid.c":  nil,
					},
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change contains the following files which require a human reviewer: a/a/c/a.md, a/b/c.txt."))
			})
			t.Run("repo-level excluded files from regexp config works", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].RepoRegexpConfigs = []*config.HostConfig_RepoRegexpConfigPair{
					{
						Key: "^.*ummy$",
						Value: &config.RepoConfig{
							CleanRevertPattern: &config.CleanRevertPattern{
								ExcludedPaths: []string{"well.txt"},
							},
						},
					},
				}
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: true,
				}, nil)
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-2 * 24 * time.Hour)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"well.txt":   nil,
						"a/a/c/a.md": nil,
						"a/valid.c":  nil,
					},
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change contains the following files which require a human reviewer: well.txt."))
			})
		})
		t.Run("returns error", func(t *ftt.Test) {
			t.Run("GetPureRevert API error", func(t *ftt.Test) {
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("gerrit GetPureRevert rpc call failed with error"))
			})
			t.Run("GetChange API error", func(t *ftt.Test) {
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: true,
				}, nil)
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("gerrit GetChange rpc call failed with error"))
			})
			t.Run("time window config error", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].CleanRevertTimeWindow = "1.2d"
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: true,
				}, nil)
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("invalid time_window config 1.2d"))
			})
			t.Run("ListFiles API error", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
					CleanRevertPattern: &config.CleanRevertPattern{
						ExcludedPaths: []string{"a/b/c.txt", "a/**/*.md"},
					},
				}
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: true,
				}, nil)
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-time.Minute)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				msg, err := reviewCleanRevert(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("gerrit ListFiles rpc call failed with error"))
			})
		})
	})
}
