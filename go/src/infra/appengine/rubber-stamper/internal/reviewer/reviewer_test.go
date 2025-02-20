// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reviewer

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/proto"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"

	"go.chromium.org/infra/appengine/rubber-stamper/config"
	"go.chromium.org/infra/appengine/rubber-stamper/internal/util"
	"go.chromium.org/infra/appengine/rubber-stamper/tasks/taskspb"
)

func TestReviewChange(t *testing.T) {
	ftt.Run("review change", t, func(t *ftt.Test) {
		cfg := &config.Config{
			DefaultTimeWindow: "7d",
			HostConfigs: map[string]*config.HostConfig{
				"test-host": {
					RepoConfigs: map[string]*config.RepoConfig{
						"dummy": {
							BenignFilePattern: &config.BenignFilePattern{
								Paths: []string{"a/x", "a/b.txt", "a/c.txt", "a/e/*.txt", "a/f*.txt"},
							},
						},
					},
				},
			},
		}
		ctx := memory.Use(context.Background())
		ctx, gerritMock, _ := util.SetupTestingContext(ctx, cfg, "srv-account@example.com", "test-host", t)

		t.Run("BenignFileChange", func(t *ftt.Test) {
			tsk := &taskspb.ChangeReviewTask{
				Host:       "test-host",
				Number:     12345,
				Revision:   "123abc",
				Repo:       "dummy",
				AutoSubmit: false,
				Created:    timestamppb.New(time.Now().Add(-time.Minute)),
			}
			t.Run("valid BenignFileChange", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/x": nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Labels:     map[string]int32{"Bot-Commit": 1},
				})).Return(&gerritpb.ReviewResult{}, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("valid BenignFileChange with Auto-Submit", func(t *ftt.Test) {
				tsk.AutoSubmit = true
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/x": nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Labels:     map[string]int32{"Bot-Commit": 1, "Commit-Queue": 2},
				})).Return(&gerritpb.ReviewResult{}, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("invalid BenignFileChange", func(t *ftt.Test) {
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/d.txt":     nil,
						"a/p":         nil,
						"a/e/p/p.txt": nil,
						"a/f/z.txt":   nil,
						"a/fz.txt":    nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Message:    "The change cannot be auto-reviewed. The following files do not match the benign file configuration: a/d.txt, a/e/p/p.txt, a/f/z.txt, a/p. Learn more: go/rubber-stamper-user-guide.",
				})).Return(&gerritpb.ReviewResult{}, nil)
				gerritMock.EXPECT().DeleteReviewer(gomock.Any(), proto.MatcherEqual(&gerritpb.DeleteReviewerRequest{
					Number:    tsk.Number,
					AccountId: "srv-account@example.com",
				})).Return(nil, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
		})
		t.Run("CleanRevert", func(t *ftt.Test) {
			tsk := &taskspb.ChangeReviewTask{
				Host:       "test-host",
				Number:     12345,
				Revision:   "123abc",
				Repo:       "dummy",
				AutoSubmit: false,
				RevertOf:   45678,
				Created:    timestamppb.New(time.Now().Add(-time.Minute)),
			}
			t.Run("valid CleanRevert", func(t *ftt.Test) {
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
							Created: timestamppb.New(time.Now().Add(-5 * time.Minute)),
						},
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Labels:     map[string]int32{"Bot-Commit": 1},
				})).Return(&gerritpb.ReviewResult{}, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("invalid CleanRevert but can pass the BenignFilePattern", func(t *ftt.Test) {
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: false,
				}, nil)
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * time.Minute)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/x": nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Labels:     map[string]int32{"Bot-Commit": 1},
				})).Return(&gerritpb.ReviewResult{}, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("invalid CleanRevert", func(t *ftt.Test) {
				gerritMock.EXPECT().GetPureRevert(gomock.Any(), proto.MatcherEqual(&gerritpb.GetPureRevertRequest{
					Number:  tsk.Number,
					Project: tsk.Repo,
				})).Return(&gerritpb.PureRevertInfo{
					IsPureRevert: false,
				}, nil)
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.RevertOf,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * time.Minute)),
						},
					},
				}, nil)
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/d.txt":     nil,
						"a/p":         nil,
						"a/e/p/p.txt": nil,
						"a/f/z.txt":   nil,
						"a/fz.txt":    nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Message:    "Gerrit GetPureRevert API does not mark this CL as a pure revert. Learn more: go/rubber-stamper-user-guide.",
				})).Return(&gerritpb.ReviewResult{}, nil)
				gerritMock.EXPECT().DeleteReviewer(gomock.Any(), proto.MatcherEqual(&gerritpb.DeleteReviewerRequest{
					Number:    tsk.Number,
					AccountId: "srv-account@example.com",
				})).Return(nil, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
		})
		t.Run("CleanCherryPick", func(t *ftt.Test) {
			tsk := &taskspb.ChangeReviewTask{
				Host:               "test-host",
				Number:             12345,
				Revision:           "123abc",
				Repo:               "dummy",
				AutoSubmit:         false,
				RevisionsCount:     1,
				CherryPickOfChange: 45678,
				Created:            timestamppb.New(time.Now().Add(-time.Minute)),
			}
			t.Run("valid CherryPick", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * time.Minute)),
						},
					},
				}, nil)
				gerritMock.EXPECT().GetMergeable(gomock.Any(), proto.MatcherEqual(&gerritpb.GetMergeableRequest{
					Number:     tsk.Number,
					Project:    tsk.Repo,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.MergeableInfo{
					Mergeable: true,
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Labels:     map[string]int32{"Bot-Commit": 1},
				})).Return(&gerritpb.ReviewResult{}, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("invalid CherryPick but can pass the BenignFilePattern", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * time.Minute)),
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
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/faaa.txt": nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Labels:     map[string]int32{"Bot-Commit": 1},
				})).Return(&gerritpb.ReviewResult{}, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("invalid CherryPick", func(t *ftt.Test) {
				gerritMock.EXPECT().GetChange(gomock.Any(), proto.MatcherEqual(&gerritpb.GetChangeRequest{
					Number:  tsk.CherryPickOfChange,
					Options: []gerritpb.QueryOption{gerritpb.QueryOption_CURRENT_REVISION},
				})).Return(&gerritpb.ChangeInfo{
					Status:          gerritpb.ChangeStatus_MERGED,
					CurrentRevision: "456def",
					Revisions: map[string]*gerritpb.RevisionInfo{
						"456def": {
							Created: timestamppb.New(time.Now().Add(-5 * time.Minute)),
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
				gerritMock.EXPECT().ListFiles(gomock.Any(), proto.MatcherEqual(&gerritpb.ListFilesRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
				})).Return(&gerritpb.ListFilesResponse{
					Files: map[string]*gerritpb.FileInfo{
						"a/invalid.md": nil,
					},
				}, nil)
				gerritMock.EXPECT().SetReview(gomock.Any(), proto.MatcherEqual(&gerritpb.SetReviewRequest{
					Number:     tsk.Number,
					RevisionId: tsk.Revision,
					Message:    "The change is not mergeable. Learn more: go/rubber-stamper-user-guide.",
				})).Return(&gerritpb.ReviewResult{}, nil)
				gerritMock.EXPECT().DeleteReviewer(gomock.Any(), proto.MatcherEqual(&gerritpb.DeleteReviewerRequest{
					Number:    tsk.Number,
					AccountId: "srv-account@example.com",
				})).Return(nil, nil)

				err := ReviewChange(ctx, tsk)
				assert.Loosely(t, err, should.BeNil)
			})
		})
	})
}
