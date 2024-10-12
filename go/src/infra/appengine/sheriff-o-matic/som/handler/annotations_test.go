// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package handler

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/auth/xsrf"
	"go.chromium.org/luci/server/router"

	"infra/appengine/sheriff-o-matic/som/model"
	monorailv3 "infra/monorailv2/api/v3/api_proto"
)

func TestFilterAnnotations(t *testing.T) {
	ftt.Run("Test filter annotation", t, func(t *ftt.Test) {
		activeKeys := map[string]interface{}{
			"alert_1": nil,
			"alert_2": nil,
			"alert_3": nil,
		}

		annotations := []*model.Annotation{
			{
				Key:     "alert_1",
				GroupID: "group_1",
			},
			{
				Key:     "alert_2",
				GroupID: "group_2",
			},
			{
				Key:     "group_2",
				GroupID: "",
			},
			{
				Key:     "group_3",
				GroupID: "",
			},
			{
				Key:     "group_1",
				GroupID: "",
			},
		}
		result := filterAnnotations(annotations, activeKeys)
		assert.Loosely(t, len(result), should.Equal(4))
		assert.Loosely(t, result[0].Key, should.Equal("alert_1"))
		assert.Loosely(t, result[1].Key, should.Equal("alert_2"))
		assert.Loosely(t, result[2].Key, should.Equal("group_2"))
		assert.Loosely(t, result[3].Key, should.Equal("group_1"))
	})
}

func TestFilterDuplicateBugs(t *testing.T) {
	ftt.Run("Test filter annotation", t, func(t *ftt.Test) {
		bugs := []model.MonorailBug{
			{
				BugID:     "bug_1",
				ProjectID: "project_1",
			},
			{
				BugID:     "bug_2",
				ProjectID: "project_2",
			},
			{
				BugID:     "bug_1",
				ProjectID: "project_1",
			},
			{
				BugID:     "bug_3",
				ProjectID: "project_3",
			},
		}

		result := filterDuplicateBugs(bugs)
		assert.Loosely(t, len(result), should.Equal(3))
		assert.Loosely(t, result[0].BugID, should.Equal("bug_1"))
		assert.Loosely(t, result[1].BugID, should.Equal("bug_2"))
		assert.Loosely(t, result[2].BugID, should.Equal("bug_3"))
	})
}

func TestCreateProjectChunksMapping(t *testing.T) {
	ftt.Run("Test create project chunk mapping", t, func(t *ftt.Test) {
		bugs := []model.MonorailBug{
			{
				BugID:     "bug_1",
				ProjectID: "project_1",
			},
			{
				BugID:     "bug_2",
				ProjectID: "project_2",
			},
			{
				BugID:     "bug_3",
				ProjectID: "project_1",
			},
			{
				BugID:     "bug_4",
				ProjectID: "project_3",
			},
			{
				BugID:     "bug_5",
				ProjectID: "project_1",
			},
			{
				BugID:     "bug_6",
				ProjectID: "b",
			},
		}

		result := createMonorailProjectChunksMapping(bugs, 100)
		assert.Loosely(t,
			result,
			should.Resemble(
				map[string][][]string{
					"project_1": {{"bug_1", "bug_3", "bug_5"}},
					"project_2": {{"bug_2"}},
					"project_3": {{"bug_4"}},
				},
			))

		result = createMonorailProjectChunksMapping(bugs, 2)
		assert.Loosely(t,
			result,
			should.Resemble(
				map[string][][]string{
					"project_1": {{"bug_1", "bug_3"}, {"bug_5"}},
					"project_2": {{"bug_2"}},
					"project_3": {{"bug_4"}},
				},
			))
	})
}

func TestBreakToChunk(t *testing.T) {
	ftt.Run("Test break bug ids to chunk", t, func(t *ftt.Test) {
		bugIDs := []string{"bug1", "bug2", "bug3", "bug4", "bug5"}
		chunks := breakToChunks(bugIDs, 1)
		assert.Loosely(t, chunks, should.Resemble([][]string{{"bug1"}, {"bug2"}, {"bug3"}, {"bug4"}, {"bug5"}}))
		chunks = breakToChunks(bugIDs, 3)
		assert.Loosely(t, chunks, should.Resemble([][]string{{"bug1", "bug2", "bug3"}, {"bug4", "bug5"}}))
		chunks = breakToChunks(bugIDs, 5)
		assert.Loosely(t, chunks, should.Resemble([][]string{{"bug1", "bug2", "bug3", "bug4", "bug5"}}))
		chunks = breakToChunks(bugIDs, 6)
		assert.Loosely(t, chunks, should.Resemble([][]string{{"bug1", "bug2", "bug3", "bug4", "bug5"}}))
	})
}

func TestMakeAnnotationResponse(t *testing.T) {
	ftt.Run("Test make annotation response successful", t, func(t *ftt.Test) {
		annotations := &model.Annotation{
			Bugs: []model.MonorailBug{
				{BugID: "123", ProjectID: "chromium"},
				{BugID: "456", ProjectID: "chromium"},
			},
		}
		meta := []*MonorailBugData{
			{
				ProjectID: "chromium",
				BugID:     "123",
				Summary:   "Sum1",
				Status:    "Assigned",
			},
			{
				ProjectID: "chromium",
				BugID:     "456",
				Summary:   "Sum2",
				Status:    "Fixed",
			},
		}
		expected := &AnnotationResponse{
			Annotation: *annotations,
			BugData: map[string]MonorailBugData{
				"123": {
					BugID:     "123",
					ProjectID: "chromium",
					Summary:   "Sum1",
					Status:    "Assigned",
				},
				"456": {
					BugID:     "456",
					ProjectID: "chromium",
					Summary:   "Sum2",
					Status:    "Fixed",
				},
			},
		}
		actual := makeAnnotationResponse(annotations, meta)
		assert.Loosely(t, actual, should.Resemble(expected))
	})
}

type FakeIC struct{}

func (ic FakeIC) SearchIssues(c context.Context, req *monorailv3.SearchIssuesRequest, ops ...grpc.CallOption) (*monorailv3.SearchIssuesResponse, error) {
	if req.Projects[0] == "projects/chromium" {
		return &monorailv3.SearchIssuesResponse{
			Issues: []*monorailv3.Issue{
				{
					Name: "projects/chromium/issues/333",
					Status: &monorailv3.Issue_StatusValue{
						Status: "Untriaged",
					},
				},
				{
					Name: "projects/chromium/issues/444",
					Status: &monorailv3.Issue_StatusValue{
						Status: "Untriaged",
					},
				},
			},
		}, nil
	}
	if req.Projects[0] == "projects/fuchsia" {
		return &monorailv3.SearchIssuesResponse{
			Issues: []*monorailv3.Issue{
				{
					Name: "projects/fuchsia/issues/555",
					Status: &monorailv3.Issue_StatusValue{
						Status: "Untriaged",
					},
				},
				{
					Name: "projects/fuchsia/issues/666",
					Status: &monorailv3.Issue_StatusValue{
						Status: "Untriaged",
					},
				},
			},
		}, nil
	}
	return nil, nil
}

func (ic FakeIC) MakeIssue(c context.Context, req *monorailv3.MakeIssueRequest, opts ...grpc.CallOption) (*monorailv3.Issue, error) {
	projectRes := req.Parent
	return &monorailv3.Issue{
		Name:    fmt.Sprintf("%s/issues/123", projectRes),
		Summary: req.Issue.Summary,
		Status:  req.Issue.Status,
		Labels:  req.Issue.Labels,
		CcUsers: req.Issue.CcUsers,
	}, nil
}

func TestAnnotations(t *testing.T) {
	newContext := func() (context.Context, testclock.TestClock) {
		c := gaetesting.TestingContext()
		c = authtest.MockAuthConfig(c)
		c = gologger.StdConfig.Use(c)

		cl := testclock.New(testclock.TestRecentTimeUTC)
		c = clock.Set(c, cl)
		return c, cl
	}
	ftt.Run("/annotations", t, func(t *ftt.Test) {

		w := httptest.NewRecorder()
		c, cl := newContext()
		tok, err := xsrf.Token(c)
		assert.Loosely(t, err, should.BeNil)

		ah := &AnnotationHandler{
			Bqh:                 &BugQueueHandler{},
			MonorailIssueClient: FakeIC{},
		}

		t.Run("GET", func(t *ftt.Test) {
			t.Run("no annotations yet", func(t *ftt.Test) {
				ah.GetAnnotationsHandler(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
				}, nil)

				r, err := ioutil.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, w.Code, should.Equal(200))
				assert.Loosely(t, body, should.Equal("[]"))
			})

			ann := &model.Annotation{
				KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
				Key:              "foobar",
				Bugs:             []model.MonorailBug{{BugID: "111", ProjectID: "fuchsia"}, {BugID: "222", ProjectID: "chromium"}},
				SnoozeTime:       123123,
				ModificationTime: datastore.RoundTime(clock.Now(c).Add(4 * time.Hour)),
			}

			assert.Loosely(t, datastorePutAnnotation(c, ann), should.BeNil)
			datastore.GetTestable(c).CatchupIndexes()

			t.Run("basic annotation", func(t *ftt.Test) {
				ah.GetAnnotationsHandler(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
				}, map[string]interface{}{ann.Key: nil})

				r, err := ioutil.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, w.Code, should.Equal(200))
				rslt := []*model.Annotation{}
				assert.Loosely(t, json.NewDecoder(strings.NewReader(body)).Decode(&rslt), should.BeNil)
				assert.Loosely(t, rslt, should.HaveLength(1))
				assert.Loosely(t, rslt[0], should.Resemble(ann))
			})

			t.Run("basic annotation, alert no longer active", func(t *ftt.Test) {
				ah.GetAnnotationsHandler(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
				}, nil)

				r, err := ioutil.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, w.Code, should.Equal(200))
				rslt := []*model.Annotation{}
				assert.Loosely(t, json.NewDecoder(strings.NewReader(body)).Decode(&rslt), should.BeNil)
				assert.Loosely(t, rslt, should.HaveLength(0))
			})
		})

		addXSRFToken := func(data map[string]interface{}, tok string) string {
			change, err := json.Marshal(map[string]interface{}{
				"xsrf_token": tok,
				"data":       data,
			})
			assert.Loosely(t, err, should.BeNil)
			return string(change)
		}

		t.Run("POST", func(t *ftt.Test) {
			t.Run("invalid action", func(t *ftt.Test) {
				ah.PostAnnotationsHandler(&router.Context{
					Writer:  w,
					Request: makePostRequest(c, ""),
					Params:  makeParams("action", "lolwut"),
				})

				assert.Loosely(t, w.Code, should.Equal(400))
			})

			t.Run("invalid json", func(t *ftt.Test) {
				ah.PostAnnotationsHandler(&router.Context{
					Writer:  w,
					Request: makePostRequest(c, "invalid json"),
					Params:  makeParams("annKey", "foobar", "action", "add"),
				})

				assert.Loosely(t, w.Code, should.Equal(http.StatusBadRequest))
			})

			ann := &model.Annotation{
				Tree:             datastore.MakeKey(c, "Tree", "tree.unknown"),
				Key:              "foobar",
				KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
				ModificationTime: datastore.RoundTime(clock.Now(c)),
			}
			cl.Add(time.Hour)

			t.Run("add, bad xsrf token", func(t *ftt.Test) {
				ah.PostAnnotationsHandler(&router.Context{
					Writer: w,
					Request: makePostRequest(c, addXSRFToken(map[string]interface{}{
						"snoozeTime": 123123,
					}, "no good token")),
					Params: makeParams("annKey", "foobar", "action", "add"),
				})

				assert.Loosely(t, w.Code, should.Equal(http.StatusForbidden))
			})

			t.Run("add", func(t *ftt.Test) {
				ann = &model.Annotation{
					Tree:             datastore.MakeKey(c, "Tree", "tree.unknown"),
					Key:              "foobar",
					KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
					ModificationTime: datastore.RoundTime(clock.Now(c)),
				}
				change := map[string]interface{}{}
				t.Run("snoozeTime", func(t *ftt.Test) {
					ah.PostAnnotationsHandler(&router.Context{
						Writer: w,
						Request: makePostRequest(c, addXSRFToken(map[string]interface{}{
							"snoozeTime": 123123,
							"key":        "foobar",
						}, tok)),
						Params: makeParams("action", "add", "tree", "tree.unknown"),
					})

					assert.Loosely(t, w.Code, should.Equal(200))
					assert.Loosely(t, datastoreGetAnnotation(c, ann), should.BeNil)
					assert.Loosely(t, ann.SnoozeTime, should.Equal(123123))
				})

				t.Run("bugs", func(t *ftt.Test) {
					change["bugs"] = []model.MonorailBug{{BugID: "123123", ProjectID: "chromium"}}
					change["key"] = "foobar"
					ah.PostAnnotationsHandler(&router.Context{
						Writer:  w,
						Request: makePostRequest(c, addXSRFToken(change, tok)),
						Params:  makeParams("action", "add", "tree", "tree.unknown"),
					})

					assert.Loosely(t, w.Code, should.Equal(200))

					assert.Loosely(t, datastoreGetAnnotation(c, ann), should.BeNil)
					assert.Loosely(t, ann.Bugs, should.Resemble([]model.MonorailBug{{BugID: "123123", ProjectID: "chromium"}}))
				})
			})

			t.Run("remove", func(t *ftt.Test) {
				t.Run("can't remove non-existent annotation", func(t *ftt.Test) {
					ah.PostAnnotationsHandler(&router.Context{
						Writer:  w,
						Request: makePostRequest(c, addXSRFToken(map[string]interface{}{"key": "foobar"}, tok)),
						Params:  makeParams("action", "remove", "tree", "tree.unknown"),
					})

					assert.Loosely(t, w.Code, should.Equal(404))
				})

				ann.SnoozeTime = 123
				assert.Loosely(t, datastorePutAnnotation(c, ann), should.BeNil)

				t.Run("basic", func(t *ftt.Test) {
					assert.Loosely(t, ann.SnoozeTime, should.Equal(123))

					ah.PostAnnotationsHandler(&router.Context{
						Writer: w,
						Request: makePostRequest(c, addXSRFToken(map[string]interface{}{
							"key":        "foobar",
							"snoozeTime": true,
						}, tok)),
						Params: makeParams("action", "remove", "tree", "tree.unknown"),
					})

					assert.Loosely(t, w.Code, should.Equal(200))
					assert.Loosely(t, datastoreGetAnnotation(c, ann), should.BeNil)
					assert.Loosely(t, ann.SnoozeTime, should.BeZero)
				})
			})
		})

		t.Run("refreshAnnotations", func(t *ftt.Test) {
			t.Run("handler", func(t *ftt.Test) {
				c, _ := newContext()
				err := ah.RefreshAnnotationsHandler(c)
				assert.Loosely(t, err, should.BeNil)
			})

			ann := &model.Annotation{
				KeyDigest: fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
				Key:       "foobar",
				Bugs:      []model.MonorailBug{{BugID: "333", ProjectID: "chromium"}, {BugID: "444", ProjectID: "chromium"}},
			}

			ann1 := &model.Annotation{
				KeyDigest: fmt.Sprintf("%x", sha1.Sum([]byte("foobar1"))),
				Key:       "foobar1",
				Bugs:      []model.MonorailBug{{BugID: "555", ProjectID: "fuchsia"}, {BugID: "666", ProjectID: "fuchsia"}},
			}

			assert.Loosely(t, datastorePutAnnotation(c, ann), should.BeNil)
			assert.Loosely(t, datastorePutAnnotation(c, ann1), should.BeNil)
			datastore.GetTestable(c).CatchupIndexes()

			t.Run("query alerts which have multiple bugs", func(t *ftt.Test) {
				err := ah.RefreshAnnotationsHandler(c)
				assert.Loosely(t, err, should.BeNil)
			})
		})
	})
}
