// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	gr "golang.org/x/build/gerrit"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	tq "go.chromium.org/luci/gae/service/taskqueue"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	admin "infra/tricium/api/admin/v1"
	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common"
	gc "infra/tricium/appengine/common/gerrit"
	"infra/tricium/appengine/common/track"
	"infra/tricium/appengine/common/triciumtest"
)

const (
	host = "https://chromium-review.googlesource.com"
)

// mockPollRestAPI allows for modification of change state returned by
// QueryChanges.
type mockPollRestAPI struct {
	sync.Mutex
	changes map[string][]gr.ChangeInfo
}

func (m *mockPollRestAPI) QueryChanges(c context.Context, host, project string, ts time.Time) ([]gr.ChangeInfo, bool, error) {
	m.Lock()
	defer m.Unlock()

	if m.changes == nil {
		m.changes = make(map[string][]gr.ChangeInfo)
	}
	id := gerritProjectID(host, project)
	changes := m.changes[id]
	return changes, false, nil
}

func (*mockPollRestAPI) PostRobotComments(c context.Context, host, change, revision string, runID int64, comments []*track.Comment) error {
	// Not used by the poller.
	return nil
}

func (m *mockPollRestAPI) GetChangedLines(c context.Context, host, change, revision string) (gc.ChangedLinesInfo, error) {
	// Not used by the poller.
	return gc.ChangedLinesInfo{}, nil
}

func (m *mockPollRestAPI) addChanges(host, project string, c []gr.ChangeInfo) {
	m.Lock()
	defer m.Unlock()

	if m.changes == nil {
		m.changes = make(map[string][]gr.ChangeInfo)
	}
	id := gerritProjectID(host, project)
	changes := m.changes[id]
	changes = append(changes, c...)
	m.changes[id] = changes
}

// mockConfigProvider stores and returns canned configs.
type mockConfigProvider struct {
	Projects      map[string]*tricium.ProjectConfig
	ServiceConfig *tricium.ServiceConfig
}

func (m *mockConfigProvider) GetServiceConfig(c context.Context) (*tricium.ServiceConfig, error) {
	return m.ServiceConfig, nil
}

func (m *mockConfigProvider) GetProjectConfig(c context.Context, p string) (*tricium.ProjectConfig, error) {
	pc, ok := m.Projects[p]
	if !ok {
		return nil, fmt.Errorf("nonexistent project")
	}
	return pc, nil
}

func (m *mockConfigProvider) GetAllProjectConfigs(c context.Context) (map[string]*tricium.ProjectConfig, error) {
	return m.Projects, nil
}

func numEnqueuedAnalyzeRequests(ctx context.Context) int {
	return len(tq.GetTestable(ctx).GetScheduledTasks()[common.AnalyzeQueue])
}

func TestPollAllProjectsBehavior(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		cp := &mockConfigProvider{
			Projects: map[string]*tricium.ProjectConfig{
				"a-project": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "infra",
									GitUrl:  "https://repo-host.com/infra",
								},
							},
						},
					},
				},
				"b-project": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "project/tricium-gerrit",
									GitUrl:  "https://repo-host.com/playground",
								},
							},
						},
					},
				},
				"chromium": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "chromium/src",
									GitUrl:  "https://chromium.googlesource.com/chromium/src",
								},
							},
						},
					},
				},
				"chromium-m87": {
					// This is a special chromium milestone project which has
					// the same project config as the chromium project, but
					// which should be skipped.
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "chromium/src",
									GitUrl:  "https://chromium.googlesource.com/chromium/src",
								},
							},
						},
					},
				},
			},
		}

		t.Run("Poll puts a poll project request in the task queue for each project", func(t *ftt.Test) {
			assert.Loosely(t, poll(ctx, cp), should.BeNil)
			tasks := tq.GetTestable(ctx).GetScheduledTasks()[common.PollProjectQueue]
			var projects []string
			for _, task := range tasks {
				request := &admin.PollProjectRequest{}
				assert.Loosely(t, proto.Unmarshal(task.Payload, request), should.BeNil)
				projects = append(projects, request.Project)
			}
			sort.Strings(projects)
			// Note that chromium-m87 is not polled.
			assert.Loosely(t, projects, should.Resemble([]string{"a-project", "b-project", "chromium"}))
		})
	})
}

func TestPollProjectBasicBehavior(t *testing.T) {

	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		now := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, tc := testclock.UseTime(ctx, now)
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity: identity.AnonymousIdentity,
		})

		cp := &mockConfigProvider{
			Projects: map[string]*tricium.ProjectConfig{
				"infra": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "infra",
									GitUrl:  "https://repo-host.com/infra",
								},
							},
						},
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "project/tricium-gerrit",
									GitUrl:  "https://repo-host.com/playground",
								},
							},
						},
					},
				},
			},
		}
		projects, err := cp.GetAllProjectConfigs(ctx)
		assert.Loosely(t, err, should.BeNil)

		gerritProjects := []*tricium.GerritProject{
			projects["infra"].Repos[0].GetGerritProject(),
			projects["infra"].Repos[1].GetGerritProject(),
		}

		t.Run("First poll, no changes", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Creates tracking entries for Gerrit projects", func(t *ftt.Test) {
				for _, gd := range gerritProjects {
					p := &Project{ID: gerritProjectID(gd.Host, gd.Project)}
					assert.Loosely(t, ds.Get(ctx, p), should.BeNil)
				}
			})
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("Second poll, no changes", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			// Store last poll timestamps from first poll.
			lastPolls := make(map[string]time.Time)
			for _, gd := range gerritProjects {
				p := &Project{ID: gerritProjectID(gd.Host, gd.Project)}
				assert.Loosely(t, ds.Get(ctx, p), should.BeNil)
				lastPolls[p.ID] = p.LastPoll
			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Does not update timestamp of last poll", func(t *ftt.Test) {
				for _, gd := range gerritProjects {
					p := &Project{ID: gerritProjectID(gd.Host, gd.Project)}
					assert.Loosely(t, ds.Get(ctx, p), should.BeNil)
					tm := lastPolls[p.ID]
					assert.Loosely(t, tm.Equal(p.LastPoll), should.BeTrue)
				}
			})
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("First poll, with changes", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := clock.Now(ctx)
			// Fill up with one change per project.
			for _, gd := range gerritProjects {
				api.addChanges(gd.Host, gd.Project, []gr.ChangeInfo{
					{
						Project: gd.Project,
						Updated: gr.TimeStamp(lastChangeTs),
						Owner:   &gr.AccountInfo{Email: "user@example.com"},
					},
				})
			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("Second poll, with new changes adding files", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			// Fill up with one change per project.
			for _, gd := range gerritProjects {
				revisions := map[string]gr.RevisionInfo{
					"abcdef": {
						Kind:  "REWORK",
						Files: map[string]*gr.FileInfo{"README.md": {}},
					},
				}
				api.addChanges(gd.Host, gd.Project, []gr.ChangeInfo{
					{
						ChangeID:        "Ideadc0de",
						Branch:          "branch",
						Project:         gd.Project,
						Status:          "NEW",
						CurrentRevision: "abcdef",
						Updated:         gr.TimeStamp(lastChangeTs),
						Revisions:       revisions,
						Owner:           &gr.AccountInfo{Email: "user@example.com"},
					},
				})
			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Updates last poll timestamp to last change timestamp", func(t *ftt.Test) {
				for _, gd := range gerritProjects {
					p := &Project{ID: gerritProjectID(gd.Host, gd.Project)}
					assert.Loosely(t, ds.Get(ctx, p), should.BeNil)
					assert.Loosely(t, lastChangeTs.Equal(p.LastPoll), should.BeTrue)
				}
			})
			t.Run("Enqueues analyze requests for each repo in the project", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(len(gerritProjects)))
				tasks := tq.GetTestable(ctx).GetScheduledTasks()[common.AnalyzeQueue]
				var projects []string
				var repos []string
				for _, task := range tasks {
					ar := &tricium.AnalyzeRequest{}
					assert.Loosely(t, proto.Unmarshal(task.Payload, ar), should.BeNil)
					projects = append(projects, ar.Project)
					repos = append(repos, ar.GetGerritRevision().GitUrl)
				}
				assert.Loosely(t, projects, should.Resemble([]string{"infra", "infra"}))
				sort.Strings(repos)
				assert.Loosely(t, repos, should.Resemble([]string{
					"https://repo-host.com/infra",
					"https://repo-host.com/playground",
				}))

			})
			t.Run("Adds change tracking entities", func(t *ftt.Test) {
				for _, gd := range gerritProjects {
					assert.Loosely(t, ds.Get(ctx, &Change{
						ID:     fmt.Sprintf("%s~branch~Ideadc0de", url.PathEscape(gd.Project)),
						Parent: ds.NewKey(ctx, "GerritProject", gerritProjectID(gd.Host, gd.Project), 0, nil),
					}), should.BeNil)
				}
			})
		})

		t.Run("Poll with changes that include deleted and binary files", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			// Fill up with one change per project.
			for _, gd := range gerritProjects {
				revisions := map[string]gr.RevisionInfo{
					"abcdef": {
						Kind: "REWORK",
						Files: map[string]*gr.FileInfo{
							"changed.txt": {},
							"deleted.txt": {Status: "D"},
							"binary.png":  {Binary: true},
						},
					},
				}
				api.addChanges(gd.Host, gd.Project, []gr.ChangeInfo{
					{
						ChangeID:        "Ideadc0de",
						Branch:          "branch",
						Project:         gd.Project,
						Status:          "NEW",
						CurrentRevision: "abcdef",
						Updated:         gr.TimeStamp(lastChangeTs),
						Revisions:       revisions,
						Owner:           &gr.AccountInfo{Email: "user@example.com"},
					},
				})
			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Enqueued analyze requests do not include deleted files", func(t *ftt.Test) {
				tasks := tq.GetTestable(ctx).GetScheduledTasks()[common.AnalyzeQueue]
				assert.Loosely(t, len(tasks), should.Equal(len(gerritProjects)))
				for _, task := range tasks {
					ar := &tricium.AnalyzeRequest{}
					err := proto.Unmarshal(task.Payload, ar)

					// Sorting files according to their paths to account for random
					// enumeration in go maps.
					sort.Slice(ar.Files, func(i, j int) bool {
						return ar.Files[i].Path < ar.Files[j].Path
					})
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, ar.Files, should.Resemble([]*tricium.Data_File{
						{
							Path:     "binary.png",
							IsBinary: true,
							Status:   tricium.Data_MODIFIED,
						},
						{
							Path:     "changed.txt",
							IsBinary: false,
							Status:   tricium.Data_MODIFIED,
						},
					}))
				}
			})
		})

		t.Run("Poll when there is a change with no files", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			// Fill up with one change per project.
			for _, gd := range gerritProjects {
				revisions := map[string]gr.RevisionInfo{
					"abcdef": {
						Kind:  "REWORK",
						Files: make(map[string]*gr.FileInfo),
					},
				}
				api.addChanges(gd.Host, gd.Project, []gr.ChangeInfo{
					{
						ChangeID:        "Ideadc0de",
						Branch:          "branch",
						Project:         gd.Project,
						Status:          "NEW",
						CurrentRevision: "abcdef",
						Updated:         gr.TimeStamp(lastChangeTs),
						Revisions:       revisions,
						Owner:           &gr.AccountInfo{Email: "user@example.com"},
					},
				})
			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("Poll when the current revision is has no code change.", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			// Fill up with one change per project.
			for _, gd := range gerritProjects {
				revisions := map[string]gr.RevisionInfo{
					"abcdef": {
						// Since Kind is not REWORK, the revision is considered
						// "trivial", and there is no need to analyze.
						Kind: "NO_CODE_CHANGE",
						Files: map[string]*gr.FileInfo{
							"changed.txt": {},
							"binary.png":  {Binary: true},
						},
					},
				}
				api.addChanges(gd.Host, gd.Project, []gr.ChangeInfo{
					{
						ChangeID:        "Ideadc0de",
						Branch:          "branch",
						Project:         gd.Project,
						Status:          "NEW",
						CurrentRevision: "abcdef",
						Updated:         gr.TimeStamp(lastChangeTs),
						Revisions:       revisions,
						Owner:           &gr.AccountInfo{Email: "user@example.com"},
					},
				})
			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("Poll with many changes, so paging is used", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			// The first poll stores the timestamp.
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)

			// Fill up each project with multiple changes.
			numChanges := 6
			revBase := "abcdef"
			branch := "master"
			changeIDFooter := "Ideadc0de"
			for _, gd := range gerritProjects {
				var changes []gr.ChangeInfo
				for i := 0; i < numChanges; i++ {
					tc.Add(time.Second)
					rev := fmt.Sprintf("%s%d", revBase, i)
					files := map[string]*gr.FileInfo{"README.md": {}}
					revisions := make(map[string]gr.RevisionInfo)
					revisions[rev] = gr.RevisionInfo{
						Kind:  "REWORK",
						Files: files,
					}
					changes = append(changes, gr.ChangeInfo{
						ChangeID:        fmt.Sprintf("%s%d", changeIDFooter, i),
						Branch:          branch,
						Project:         gd.Project,
						Status:          "NEW",
						CurrentRevision: rev,
						Updated:         gr.TimeStamp(tc.Now().UTC()),
						Revisions:       revisions,
						Owner:           &gr.AccountInfo{Email: "user@example.com"},
					})
				}
				api.addChanges(gd.Host, gd.Project, changes)

			}
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)

			t.Run("Enqueues analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(len(gerritProjects)*numChanges))
			})

			t.Run("Adds change tracking entities", func(t *ftt.Test) {
				for _, gd := range gerritProjects {
					for i := 0; i < numChanges; i++ {
						assert.Loosely(t, ds.Get(ctx, &Change{
							ID:     fmt.Sprintf("%s~%s~%s%d", url.PathEscape(gd.Project), branch, changeIDFooter, i),
							Parent: ds.NewKey(ctx, "GerritProject", gerritProjectID(gd.Host, gd.Project), 0, nil),
						}), should.BeNil)
					}
				}
			})
		})
	})
}

func TestPollProjectDescriptionFlagBehavior(t *testing.T) {

	// mkRevInfo generate a RevisionInfo with the given commit message.
	mkRevInfo := func(commitMessage string) *gr.RevisionInfo {
		return &gr.RevisionInfo{
			Files:  map[string]*gr.FileInfo{"README.md": {}},
			Commit: &gr.CommitInfo{Message: commitMessage},
			Kind:   "REWORK",
		}
	}

	// mkRevInfoMap generates a map of revisionID to RevisionInfo.
	//
	// It takes commitMessage as input and adds that to the "curRev" revision.
	mkRevInfoMap := func(commitMessage string) map[string]gr.RevisionInfo {
		return map[string]gr.RevisionInfo{
			"curRev": *mkRevInfo(commitMessage),
			"olderRev": {
				Files: map[string]*gr.FileInfo{"another1.go": {}},
				Kind:  "REWORK",
			},
		}
	}

	// mkChangeInfo returns a one-item slice of ChangeInfo
	// for use in the tests below, where "curRev" is the current revision.
	mkChangeInfo := func(project string, lastChangeTs time.Time,
		revisions map[string]gr.RevisionInfo) []gr.ChangeInfo {
		return []gr.ChangeInfo{
			{
				ChangeID:        "Ideadc0de",
				Branch:          "branch",
				Project:         project,
				Status:          "NEW",
				CurrentRevision: "curRev",
				Updated:         gr.TimeStamp(lastChangeTs),
				Revisions:       revisions,
				Owner:           &gr.AccountInfo{Email: "user@example.com"},
			},
		}
	}

	ftt.Run("Private helper functions behave as expected", t, func(t *ftt.Test) {

		t.Run("A summary-only message with a colon is not a footer", func(t *ftt.Test) {
			assert.Loosely(t, len(extractFooterFlags("Tag: something\n")), should.BeZero)
		})

		t.Run("Footer keys are converted to title-case, values are unmodified", func(t *ftt.Test) {
			assert.Loosely(t, extractFooterFlags("summary\n\nkey-name: yEs\n"),
				should.Resemble(map[string]string{"Key-Name": "yEs"}))
		})

		t.Run("There can be non-flag lines in the footer paragraph", func(t *ftt.Test) {
			assert.Loosely(t, extractFooterFlags("summary\n\nkey-name: yEs\nnot a flag\n"),
				should.Resemble(map[string]string{"Key-Name": "yEs"}))
		})

		t.Run("http and https are not used as keys", func(t *ftt.Test) {
			assert.Loosely(t, extractFooterFlags("summary\n\nkey-name: yEs\nhttps://example.com\n"),
				should.Resemble(map[string]string{"Key-Name": "yEs"}))
			assert.Loosely(t, extractFooterFlags("summary\n\nkey-name: yEs\nhttp://example.com\n"),
				should.Resemble(map[string]string{"Key-Name": "yEs"}))
			// Only some URL schemas are treated specially; others are treated as keys.
			assert.Loosely(t, extractFooterFlags("summary\n\nkey-name: yEs\nfoo://example.com\n"),
				should.Resemble(map[string]string{"Key-Name": "yEs", "Foo": "//example.com"}))
		})

		t.Run("Footer flags can be extracted with newline at end", func(t *ftt.Test) {
			assert.Loosely(t, extractFooterFlags("Summary\n\none: A\nTWO: bee\nThree: sea\n"),
				should.Resemble(map[string]string{
					"One":   "A",
					"Two":   "bee",
					"Three": "sea",
				}))
		})

		t.Run("Footer flags can be extracted with no newline at end", func(t *ftt.Test) {
			assert.Loosely(t, extractFooterFlags("Summary\n\none: A\nTWO: bee\nThree: sea"),
				should.Resemble(map[string]string{
					"One":   "A",
					"Two":   "bee",
					"Three": "sea",
				}))
		})

		t.Run("Commit message with no flags has no skip command", func(t *ftt.Test) {
			assert.Loosely(t, hasSkipCommand(&gr.RevisionInfo{
				Commit: &gr.CommitInfo{Message: "one two three"},
			}), should.BeFalse)
		})

		t.Run("Commit message with skip flag has skip command", func(t *ftt.Test) {
			assert.Loosely(t, hasSkipCommand(&gr.RevisionInfo{
				Commit: &gr.CommitInfo{Message: "Summary line\n\nTricium: Skip\nChange-Id: I01234\n"},
			}), should.BeTrue)
		})

		t.Run("no, none, skip, disable and false are all 'skip' values", func(t *ftt.Test) {
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: no")), should.BeTrue)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: none")), should.BeTrue)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: skip")), should.BeTrue)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: disable")), should.BeTrue)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: false")), should.BeTrue)
		})

		t.Run("Other values are not 'skip' values", func(t *ftt.Test) {
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: foo")), should.BeFalse)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: yes")), should.BeFalse)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: affirmative")), should.BeFalse)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: indeed")), should.BeFalse)
			assert.Loosely(t, hasSkipCommand(mkRevInfo("Summary\n\nTricium: enable")), should.BeFalse)
		})
	})

	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		now := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, tc := testclock.UseTime(ctx, now)
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity: identity.AnonymousIdentity,
		})

		cp := &mockConfigProvider{
			Projects: map[string]*tricium.ProjectConfig{
				"infra": {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "infra",
									GitUrl:  "https://repo-host.com/infra",
								},
							},
						},
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: "project/tricium-gerrit",
									GitUrl:  "https://repo-host.com/playground",
								},
							},
						},
					},
				},
			},
		}
		projects, err := cp.GetAllProjectConfigs(ctx)
		assert.Loosely(t, err, should.BeNil)

		gerritProjects := []*tricium.GerritProject{
			projects["infra"].Repos[0].GetGerritProject(),
			projects["infra"].Repos[1].GetGerritProject(),
		}

		t.Run("Poll when changes have Tricium: disable description flag", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := clock.Now(ctx)

			for _, gd := range gerritProjects {
				api.addChanges(
					gd.Host, gd.Project, mkChangeInfo(
						gd.Project, lastChangeTs, mkRevInfoMap("Summary\n\nTricium: skip")))
			}

			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("No analyze requests are queued, all are skipped", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("Poll when only one of the two changes have Tricium: disable flag", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := clock.Now(ctx)
			// Add a skipped change and non-skipped change in each project.
			for _, gd := range gerritProjects {
				api.addChanges(
					gd.Host, gd.Project,
					mkChangeInfo(gd.Project, lastChangeTs, mkRevInfoMap("Summary\n\nFoo: bar\n")))
				api.addChanges(
					gd.Host, gd.Project,
					mkChangeInfo(gd.Project, lastChangeTs, mkRevInfoMap("Summary:\n\nTricium: disable\n")))
			}

			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, "infra", api, cp), should.BeNil)
			t.Run("Keeps non-skipped changes, one per project", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(len(gerritProjects)))
			})
		})
	})
}

func TestPollProjectWhitelistBehavior(t *testing.T) {

	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		var (
			noWhitelistProject = "no-whitelist-project"
			whitelistProject   = "whitelist-group-project"
			whitelistGroup     = "whitelist-group-name"
		)

		now := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, tc := testclock.UseTime(ctx, now)
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity: identity.AnonymousIdentity,
			FakeDB: authtest.NewFakeDB(
				authtest.MockMembership("user:whitelisteduser@example.com", whitelistGroup),
			),
		})

		cp := &mockConfigProvider{
			Projects: map[string]*tricium.ProjectConfig{
				noWhitelistProject: {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: noWhitelistProject,
									GitUrl:  "https://repo-host.com/no-whitelist",
								},
							},
						},
					},
				},
				whitelistProject: {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: whitelistProject,
									GitUrl:  "https://repo-host.com/whitelist",
								},
							},
							WhitelistedGroup: []string{whitelistGroup},
						},
					},
				},
			},
		}

		projects, err := cp.GetAllProjectConfigs(ctx)
		assert.Loosely(t, err, should.BeNil)

		var gerritProjects []*tricium.GerritProject
		for _, pc := range projects {
			for _, repo := range pc.Repos {
				if gd := repo.GetGerritProject(); gd != nil {
					gerritProjects = append(gerritProjects, gd)
				}
			}
		}

		t.Run("No whitelisted groups means all changes are analyzed", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Kind:  "REWORK",
					Files: map[string]*gr.FileInfo{"README.md": {}}},
			}
			api.addChanges(host, noWhitelistProject, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         noWhitelistProject,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTs),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "whitelisteduser@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, noWhitelistProject, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, noWhitelistProject, api, cp), should.BeNil)
			t.Run("Enqueues an analyze request", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(1))
			})
		})

		t.Run("Poll with a change by a whitelisted user", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Kind:  "REWORK",
					Files: map[string]*gr.FileInfo{"README.md": {}},
				},
			}
			api.addChanges(host, whitelistProject, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         whitelistProject,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTs),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "whitelisteduser@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, whitelistProject, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, whitelistProject, api, cp), should.BeNil)
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(1))
			})
		})

		t.Run("Poll with a change by an unwhitelisted user", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTs := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Files: map[string]*gr.FileInfo{"README.md": {}},
					Kind:  "REWORK",
				},
			}
			api.addChanges(host, whitelistProject, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         whitelistProject,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTs),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "somebody-else@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, whitelistProject, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, whitelistProject, api, cp), should.BeNil)
			t.Run("Does not enqueue analyze requests", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})
	})
}

func TestPollProjectAllRevisionKindsBehavior(t *testing.T) {

	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()

		var (
			allRevisionsKinds   = "all-revision-kind"
			noAllRevisionsKinds = "no-all-revision-kinds"
			projectName         = "infra"
		)

		now := time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC)
		ctx, tc := testclock.UseTime(ctx, now)
		ctx = auth.WithState(ctx, &authtest.FakeState{
			Identity: identity.AnonymousIdentity,
		})

		cp := &mockConfigProvider{
			Projects: map[string]*tricium.ProjectConfig{
				allRevisionsKinds: {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: projectName,
									GitUrl:  "https://repo-host.com/all",
								},
							},
							CheckAllRevisionKinds: true,
						},
					},
				},
				noAllRevisionsKinds: {
					Repos: []*tricium.RepoDetails{
						{
							Source: &tricium.RepoDetails_GerritProject{
								GerritProject: &tricium.GerritProject{
									Host:    host,
									Project: projectName,
									GitUrl:  "https://repo-host.com/rework",
								},
							},
						},
					},
				},
			},
		}

		t.Run("A non-rework change in a rework-only project", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTS := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Kind:  "NO_CODE_CHANGE",
					Files: map[string]*gr.FileInfo{"README.md": {}}},
			}
			api.addChanges(host, projectName, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         projectName,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTS),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "user@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, noAllRevisionsKinds, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, noAllRevisionsKinds, api, cp), should.BeNil)
			t.Run("Enqueues an analyze request", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.BeZero)
			})
		})

		t.Run("A rework change in a rework-only project", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTS := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Kind:  "REWORK",
					Files: map[string]*gr.FileInfo{"README.md": {}}},
			}
			api.addChanges(host, projectName, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         projectName,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTS),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "user@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, noAllRevisionsKinds, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, noAllRevisionsKinds, api, cp), should.BeNil)
			t.Run("Enqueues an analyze request", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(1))
			})
		})

		t.Run("A non-rework change in an any kind project", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTS := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Kind:  "NO_CODE_CHANGE",
					Files: map[string]*gr.FileInfo{"README.md": {}}},
			}
			api.addChanges(host, projectName, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         projectName,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTS),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "user@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, allRevisionsKinds, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, allRevisionsKinds, api, cp), should.BeNil)
			t.Run("Enqueues an analyze request", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(1))
			})
		})

		t.Run("A rework change in an any kind project", func(t *ftt.Test) {
			api := &mockPollRestAPI{}
			lastChangeTS := tc.Now().UTC()
			revisions := map[string]gr.RevisionInfo{
				"abcdef": {
					Kind:  "REWORK",
					Files: map[string]*gr.FileInfo{"README.md": {}}},
			}
			api.addChanges(host, projectName, []gr.ChangeInfo{
				{
					ChangeID:        "Ideadc0de",
					Branch:          "branch",
					Project:         projectName,
					Status:          "NEW",
					CurrentRevision: "abcdef",
					Updated:         gr.TimeStamp(lastChangeTS),
					Revisions:       revisions,
					Owner:           &gr.AccountInfo{Email: "user@example.com"},
				},
			})
			assert.Loosely(t, pollProject(ctx, allRevisionsKinds, api, cp), should.BeNil)
			tc.Add(time.Second)
			assert.Loosely(t, pollProject(ctx, allRevisionsKinds, api, cp), should.BeNil)
			t.Run("Enqueues an analyze request", func(t *ftt.Test) {
				assert.Loosely(t, numEnqueuedAnalyzeRequests(ctx), should.Equal(1))
			})
		})
	})
}

func TestStatusCode(t *testing.T) {
	ctx := triciumtest.Context()

	ftt.Run("Valid codes", t, func(t *ftt.Test) {
		assert.Loosely(t, statusFromCode(ctx, "A"), should.Equal(tricium.Data_ADDED))
		assert.Loosely(t, statusFromCode(ctx, "D"), should.Equal(tricium.Data_DELETED))
		assert.Loosely(t, statusFromCode(ctx, "R"), should.Equal(tricium.Data_RENAMED))
		assert.Loosely(t, statusFromCode(ctx, "C"), should.Equal(tricium.Data_COPIED))
		assert.Loosely(t, statusFromCode(ctx, "W"), should.Equal(tricium.Data_REWRITTEN))
		assert.Loosely(t, statusFromCode(ctx, "M"), should.Equal(tricium.Data_MODIFIED))
	})

	ftt.Run("Unknown status means modified", t, func(t *ftt.Test) {
		assert.Loosely(t, statusFromCode(ctx, "X"), should.Equal(tricium.Data_MODIFIED))
	})
}
