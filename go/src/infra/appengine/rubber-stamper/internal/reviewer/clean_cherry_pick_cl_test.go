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

	"infra/appengine/rubber-stamper/config"
	"infra/appengine/rubber-stamper/tasks/taskspb"
)

func TestReviewCleanCherryPick(t *testing.T) {
	ftt.Run("review clean cherry pick", t, func(t *ftt.Test) {
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
			Host:               "test-host",
			Number:             12345,
			Revision:           "123abc",
			Repo:               "dummy",
			AutoSubmit:         false,
			RevisionsCount:     1,
			CherryPickOfChange: 12121,
			Hashtags:           []string{"Random", "Example"},
			OwnerEmail:         "userA@example.com",
			Created:            timestamppb.New(time.Now().Add(-time.Minute)),
		}

		t.Run("approve", func(t *ftt.Test) {
			t.Run("approves when the change is valid", func(t *ftt.Test) {
				tsk.RevisionsCount = 2
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Base:       "1",
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"/COMMIT_MSG": nil,
					},
				}, nil)
				gerritMock.EXPECT().GetMergeable(gomock.Any(), proto.MatcherEqual(&gerritpb.GetMergeableRequest{
					Number:     tsk.Number,
					Project:    tsk.Repo,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.MergeableInfo{
					Mergeable: true,
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.BeEmpty)
			})
			t.Run("has invalid files, but can be bypassed", func(t *ftt.Test) {
				tsk.RevisionsCount = 2
				cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
					CleanCherryPickPattern: &config.CleanCherryPickPattern{
						FileCheckBypassRule: &config.CleanCherryPickPattern_FileCheckBypassRule{
							IncludedPaths: []string{"dir_valid/**/*.txt"},
							Hashtag:       "Example",
							AllowedOwners: []string{"userA@example.com"},
						},
					},
				}
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Base:       "1",
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"/COMMIT_MSG":         nil,
						"dir_valid/valid.txt": nil,
					},
				}, nil)
				gerritMock.EXPECT().GetMergeable(gomock.Any(), proto.MatcherEqual(&gerritpb.GetMergeableRequest{
					Number:     tsk.Number,
					Project:    tsk.Repo,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.MergeableInfo{
					Mergeable: true,
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.BeEmpty)
			})
		})

		t.Run("decline when the current revision made any file changes compared with the initial version", func(t *ftt.Test) {
			tsk.RevisionsCount = 2
			gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
				Number:     tsk.Number,
				RevisionId: tsk.Revision,
				Base:       "1",
			})).Return(&gerritpb.ListFilesResponse{
				Files: map[string]*gerritpb.FileInfo{
					"/COMMIT_MSG": nil,
					"no.txt":      nil,
				},
			}, nil)

			msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msg, should.Equal("The current revision changed the following files compared with the initial revision: no.txt."))
		})
		t.Run("decline when out of configured time window", func(t *ftt.Test) {
			t.Run("global time window works", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-8 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review cherry-picks within 7 day(s)."))
			})
			t.Run("host-level time window works", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].CleanCherryPickTimeWindow = "5d"
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-6 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review cherry-picks within 5 day(s)."))
			})
			t.Run("repo-level time window works", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].CleanCherryPickTimeWindow = "5d"
				cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
					CleanCherryPickPattern: &config.CleanCherryPickPattern{
						TimeWindow: "58m",
					},
				}
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review cherry-picks within 58 minute(s)."))
			})
			t.Run("repo-level time window from repo_regexp_configs works", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].CleanCherryPickTimeWindow = "5d"
				cfg.HostConfigs["test-host"].RepoRegexpConfigs = []*config.HostConfig_RepoRegexpConfigPair{
					{
						Key: "^.*mmy$",
						Value: &config.RepoConfig{
							CleanCherryPickPattern: &config.CleanCherryPickPattern{
								TimeWindow: "10m",
							},
						},
					},
				}
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not in the configured time window. Rubber Stamper is only allowed to review cherry-picks within 10 minute(s)."))
			})
		})
		t.Run("decline when the change wasn't cherry-picked after the original CL has been merged.", func(t *ftt.Test) {
			t.Run("decline when the original CL hasn't been merged", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_NEW,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not cherry-picked after the original CL has been merged."))
			})
			t.Run("decline when cherry-picked before the original CL has been merged", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-24 * time.Hour)),
						},
					},
				}, nil)
				tsk.Created = timestamppb.New(time.Now().Add(-24*time.Hour - time.Minute))
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, msg, should.Equal("The change is not cherry-picked after the original CL has been merged."))
			})
		})
		t.Run("decline when alters any excluded file", func(t *ftt.Test) {
			cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
				CleanCherryPickPattern: &config.CleanCherryPickPattern{
					ExcludedPaths: []string{"p/q/**", "**.c"},
				},
			}
			gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
				Number:  tsk.CherryPickOfChange,
				Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
			})).Return(&gerritpb.ChangeInfo{
				Status:          gerritpb.ChangeStatus_MERGED,
				CurrentRevision: "456def",
				Revisions: map[string]*gerritpb.RevisionInfo{
					"456def": {
						Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
					},
					"789aaa": {
						Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
					},
				},
			}, nil)
			gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
				Number:     tsk.Number,
				RevisionId: tsk.Revision,
			})).Return(&gerritpb.ListFilesResponse{
				Files: map[string]*gerritpb.FileInfo{
					"p/q/o/0.txt": nil,
					"valid.md":    nil,
					"a/invalid.c": nil,
				},
			}, nil)
			msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msg, should.Equal("The change contains the following files which require a human reviewer: a/invalid.c, p/q/o/0.txt."))
		})
		t.Run("decline when not mergeable", func(t *ftt.Test) {
			gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
				Number:  tsk.CherryPickOfChange,
				Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
			})).Return(&gerritpb.ChangeInfo{
				Status:          gerritpb.ChangeStatus_MERGED,
				CurrentRevision: "456def",
				Revisions: map[string]*gerritpb.RevisionInfo{
					"456def": {
						Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
					},
					"789aaa": {
						Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
					},
				},
			}, nil)
			gerritMock.EXPECT().GetMergeable(gomock.Any(), proto.MatcherEqual(&gerritpb.GetMergeableRequest{
				Number:     tsk.Number,
				Project:    tsk.Repo,
				RevisionId: tsk.Revision,
			})).Return(&gerritpb.MergeableInfo{
				Mergeable: false,
			}, nil)
			msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, msg, should.Equal("The change is not mergeable."))
		})
		t.Run("return error works", func(t *ftt.Test) {
			t.Run("Gerrit GetChange API returns error", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("gerrit GetChange rpc call failed with error"))
			})
			t.Run("Gerrit ListFiles API returns error", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].RepoConfigs["dummy"] = &config.RepoConfig{
					CleanCherryPickPattern: &config.CleanCherryPickPattern{
						ExcludedPaths: []string{"p/q/**", "**.c"},
					},
				}
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("gerrit ListFiles rpc call failed with error"))
			})
			t.Run("Gerrit GetMergeable API returns error", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				gerritMock.EXPECT().GetMergeable(gomock.Any(), proto.MatcherEqual(&gerritpb.GetMergeableRequest{
					Number:     tsk.Number,
					Project:    tsk.Repo,
					RevisionId: tsk.Revision,
				})).Return(nil, grpc.Errorf(codes.NotFound, "not found"))
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("gerrit GetMergeable rpc call failed with error"))
			})
			t.Run("time window config error", func(t *ftt.Test) {
				cfg.HostConfigs["test-host"].CleanCherryPickTimeWindow = "112-1d"
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * 24 * time.Hour)),
						},
						"789aaa": {
							Created: timestamppb.New(time.Now().Add(-9 * 24 * time.Hour)),
						},
					},
				}, nil)
				msg, err := reviewCleanCherryPick(ctx, cfg, gerritMock, tsk)
				assert.Loosely(t, msg, should.BeEmpty)
				assert.Loosely(t, err, should.ErrLike("invalid time_window config 112-1d"))
			})
		})
	})
}

func TestReviewBypassFileCheck(t *testing.T) {
	ftt.Run("bypass file check", t, func(t *ftt.Test) {
		fr := &config.CleanCherryPickPattern_FileCheckBypassRule{
			IncludedPaths: []string{"dir_a/dir_b/**/*.json"},
			Hashtag:       "Example_Hashtag",
			AllowedOwners: []string{"userA@example.com", "userB@example.com"},
		}

		invalidFiles := []string{"dir_a/dir_b/test.json", "dir_a/dir_b/dir_c/ok.json"}
		hashtags := []string{"Random", "Example_Hashtag"}
		const owner = "userA@example.com"

		t.Run("approve", func(t *ftt.Test) {
			assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(true))
		})
		t.Run("decline when config is incomplete", func(t *ftt.Test) {
			t.Run("nil config", func(t *ftt.Test) {
				fr = nil
				assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(false))
			})
			t.Run("no includedPath", func(t *ftt.Test) {
				fr.IncludedPaths = nil
				assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(false))
			})
			t.Run("no hashtag", func(t *ftt.Test) {
				fr.Hashtag = ""
				assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(false))
			})
			t.Run("no allowedOwners", func(t *ftt.Test) {
				fr.AllowedOwners = nil
				assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(false))
			})
		})
		t.Run("decline when files are not included", func(t *ftt.Test) {
			invalidFiles = append(invalidFiles, "dir_c/ok.json")
			assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(false))
		})
		t.Run("decline when no hashtag matches", func(t *ftt.Test) {
			hashtags = []string{"Random1", "Random2"}
			assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, owner, fr), should.Equal(false))
		})
		t.Run("decline when owner is not allowed", func(t *ftt.Test) {
			assert.Loosely(t, bypassFileCheck(invalidFiles, hashtags, "userC@example.com", fr), should.Equal(false))
		})
	})
}
