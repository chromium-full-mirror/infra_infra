// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package handler

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"

	"infra/appengine/sheriff-o-matic/som/model"
	"infra/monorail"
	monorailv3 "infra/monorailv2/api/v3/api_proto"
)

type FakeIssueClient struct{}

func (ic FakeIssueClient) SearchIssues(context.Context, *monorailv3.SearchIssuesRequest, ...grpc.CallOption) (*monorailv3.SearchIssuesResponse, error) {
	res := &monorailv3.SearchIssuesResponse{
		Issues: []*monorailv3.Issue{},
	}
	return res, nil
}

func TestBugQueue(t *testing.T) {
	ftt.Run("/bugqueue", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = authtest.MockAuthConfig(c)
		c = gologger.StdConfig.Use(c)

		cl := testclock.New(testclock.TestRecentTimeUTC)
		c = clock.Set(c, cl)

		w := httptest.NewRecorder()

		monorailMux := http.NewServeMux()
		monorailResponse := func(w http.ResponseWriter, r *http.Request) {
			logging.Errorf(c, "got monorailMux request")
			res := &monorail.IssuesListResponse{
				Items:        []*monorail.Issue{},
				TotalResults: 0,
			}
			bytes, err := json.Marshal(res)
			if err != nil {
				logging.Errorf(c, "error marshaling response: %v", err)
			}
			w.Write(bytes)
		}
		monorailMux.HandleFunc("/", monorailResponse)

		monorailServer := httptest.NewServer(monorailMux)
		defer monorailServer.Close()

		issueClient := FakeIssueClient{}

		bqh := &BugQueueHandler{
			MonorailIssueClient:    issueClient,
			DefaultMonorailProject: "",
		}

		t.Run("mock getBugsFromMonorail", func(t *ftt.Test) {
			t.Run("get bug queue handler", func(t *ftt.Test) {
				bqh.GetBugQueueHandler(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
				})

				b, err := ioutil.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, w.Code, should.Equal(200))
				assert.Loosely(t, string(b), should.Equal(`{"extras":{"priority_field":"projects/chromium/fieldDefs/11"}}`))
			})

			t.Run("refresh bug queue handler", func(t *ftt.Test) {
				err := bqh.RefreshBugQueueHandler(c)
				assert.Loosely(t, err, should.BeNil)
			})

			t.Run("refresh bug queue", func(t *ftt.Test) {
				// HACK:
				oldOAClient := getOAuthClient
				getOAuthClient = func(c context.Context) (*http.Client, error) {
					return &http.Client{}, nil
				}

				_, err := bqh.refreshBugQueue(c, "sheriff-chromium", "chromium")
				assert.Loosely(t, err, should.BeNil)
				getOAuthClient = oldOAClient
			})

			t.Run("get uncached bugs", func(t *ftt.Test) {
				bqh.GetUncachedBugsHandler(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
					Params:  makeParams("label", "infra-troopers"),
				})

				b, err := ioutil.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, w.Code, should.Equal(200))
				assert.Loosely(t, string(b), should.Equal(`{"extras":{"priority_field":"projects/chromium/fieldDefs/11"}}`))
			})

			t.Run("get alternate email", func(t *ftt.Test) {
				e := getAlternateEmail("test@chromium.org")
				assert.Loosely(t, e, should.Equal("test@google.com"))

				e = getAlternateEmail("test@google.com")
				assert.Loosely(t, e, should.Equal("test@chromium.org"))
			})
		})

		t.Run("GetMonorailProjectNameFromLabel", func(t *ftt.Test) {
			t.Run("label match", func(t *ftt.Test) {
				tree := &model.Tree{
					Name:                       "oak",
					DisplayName:                "Oak",
					BugQueueLabel:              "sheriff-oak",
					DefaultMonorailProjectName: "oak-project",
				}
				assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()
				assert.Loosely(t, bqh.GetMonorailProjectNameFromLabel(c, "sheriff-oak"), should.Equal("oak-project"))
			})

			t.Run("label not match", func(t *ftt.Test) {
				tree := &model.Tree{
					Name:                       "oak2",
					DisplayName:                "Oak2",
					BugQueueLabel:              "sheriff-oak-2",
					DefaultMonorailProjectName: "oak-project",
				}
				assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()
				assert.Loosely(t, bqh.GetMonorailProjectNameFromLabel(c, "sheriff-oak"), should.Equal("chromium"))
			})

			t.Run("no label", func(t *ftt.Test) {
				tree := &model.Tree{
					Name:          "oak3",
					DisplayName:   "Oak3",
					BugQueueLabel: "sheriff-oak-3",
				}
				assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()
				assert.Loosely(t, bqh.GetMonorailProjectNameFromLabel(c, "sheriff-oak"), should.Equal("chromium"))
			})
		})
	})
}
