// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	tq "go.chromium.org/luci/gae/service/taskqueue"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common"
	"go.chromium.org/infra/tricium/appengine/common/track"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

const (
	project   = "playground-gerrit-tricium"
	host      = "chromium-review.googlesource.com"
	okACLUser = "user:ok@example.com"
	changeID  = "playground/gerrit-tricium~master~I17e97e23ecf2890bf6b72ffd1d7a3167ed1b0a11"
	revision  = "refs/changes/97/97/1"
)

// mockConfigProvider mocks the common.ConfigProvider interface.
type mockConfigProvider struct {
}

func (*mockConfigProvider) GetServiceConfig(c context.Context) (*tricium.ServiceConfig, error) {
	return nil, nil // not used in this test
}
func (*mockConfigProvider) GetProjectConfig(c context.Context, p string) (*tricium.ProjectConfig, error) {
	return &tricium.ProjectConfig{
		Repos: []*tricium.RepoDetails{
			{
				Source: &tricium.RepoDetails_GitRepo{
					GitRepo: &tricium.GitRepo{
						Url: "https://chromium.googlesource.com/playground/gerrit-tricium",
					},
				},
			},
		},
		Acls: []*tricium.Acl{
			{
				Role:     tricium.Acl_READER,
				Identity: okACLUser,
			},
			{
				Role:     tricium.Acl_REQUESTER,
				Identity: okACLUser,
			},
		},
	}, nil
}

func (cp *mockConfigProvider) GetAllProjectConfigs(c context.Context) (map[string]*tricium.ProjectConfig, error) {
	pc, _ := cp.GetProjectConfig(c, project)
	return map[string]*tricium.ProjectConfig{project: pc}, nil
}

func TestAnalyze(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		t.Run("Basic request", func(t *ftt.Test) {
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: identity.Identity(okACLUser),
			})

			_, err := analyzeWithAuth(ctx, &tricium.AnalyzeRequest{
				Project: project,
				Files: []*tricium.Data_File{
					{
						Path: "README.md",
					},
				},
				Source: &tricium.AnalyzeRequest_GitCommit{
					GitCommit: &tricium.GitCommit{
						Url: "https://chromium.googlesource.com/playground/gerrit-tricium",
						Ref: "refs/heads/master",
					},
				},
			}, &mockConfigProvider{})
			assert.Loosely(t, err, should.BeNil)

			t.Run("Enqueues launch request", func(t *ftt.Test) {
				assert.Loosely(t, len(tq.GetTestable(ctx).GetScheduledTasks()[common.LauncherQueue]), should.Equal(1))
			})

			t.Run("Adds tracking of run", func(t *ftt.Test) {
				r, err := track.FetchRecentRequests(ctx, &mockConfigProvider{})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, len(r), should.Equal(1))
			})
		})
	})
}

func TestValidateAnalyzeRequest(t *testing.T) {
	ctx := triciumtest.Context()
	files := []*tricium.Data_File{
		{
			Path: "README.md",
		},
	}
	url := "https://example.com/notimportant.git"

	ftt.Run("A request for a Gerrit change with no host is invalid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GerritRevision{
				GerritRevision: &tricium.GerritRevision{
					// Host field is missing.
					Project: project,
					Change:  changeID,
					GitUrl:  url,
					GitRef:  revision,
				},
			},
		})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("A request with all Gerrit details is valid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GerritRevision{
				GerritRevision: &tricium.GerritRevision{
					Host:    host,
					Project: project,
					Change:  changeID,
					GitUrl:  url,
					GitRef:  revision,
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("A request with an invalid Change ID format is invalid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GerritRevision{
				GerritRevision: &tricium.GerritRevision{
					Host:    host,
					Project: project,
					Change:  "bogus change ID",
					GitUrl:  url,
					GitRef:  revision,
				},
			},
		})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("A request for a Gerrit change with no git URL is invalid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GerritRevision{
				GerritRevision: &tricium.GerritRevision{
					Host:    host,
					Project: project,
					Change:  changeID,
					GitRef:  revision,
				},
			},
		})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("A request for a git commit with all fields is valid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GitCommit{
				GitCommit: &tricium.GitCommit{
					Url: "https://example.com/repo.git",
					Ref: "refs/heads/master",
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("A request for a git commit with no URL is invalid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GitCommit{
				GitCommit: &tricium.GitCommit{
					Ref: "refs/heads/master",
				},
			},
		})
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("A request for a git commit with no ref is invalid", t, func(t *ftt.Test) {
		err := validateAnalyzeRequest(ctx, &tricium.AnalyzeRequest{
			Project: project,
			Files:   files,
			Source: &tricium.AnalyzeRequest_GitCommit{
				GitCommit: &tricium.GitCommit{
					Url: "https://example.com/repo.git",
				},
			},
		})
		assert.Loosely(t, err, should.NotBeNil)
	})
}
