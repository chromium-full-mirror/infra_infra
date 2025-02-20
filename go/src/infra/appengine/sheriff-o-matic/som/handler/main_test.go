// Copyright 2016 The Chromium Authors
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

	"github.com/julienschmidt/httprouter"

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

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model"
	"go.chromium.org/infra/monitoring/messages"
)

var _ = fmt.Printf

func TestMain(t *testing.T) {
	ftt.Run("main", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = authtest.MockAuthConfig(c)
		c = gologger.StdConfig.Use(c)

		cl := testclock.New(testclock.TestRecentTimeUTC)
		c = clock.Set(c, cl)

		w := httptest.NewRecorder()

		monorailMux := http.NewServeMux()
		monorailServer := httptest.NewServer(monorailMux)
		defer monorailServer.Close()
		tok, err := xsrf.Token(c)
		assert.Loosely(t, err, should.BeNil)
		t.Run("/api/v1", func(t *ftt.Test) {
			alertIdx := datastore.IndexDefinition{
				Kind:     "AlertJSONNonGrouping",
				Ancestor: true,
				SortBy: []datastore.IndexColumn{
					{
						Property: "Resolved",
					},
					{
						Property:   "Date",
						Descending: false,
					},
				},
			}
			revisionSummaryIdx := datastore.IndexDefinition{
				Kind:     "RevisionSummaryJSON",
				Ancestor: true,
				SortBy: []datastore.IndexColumn{
					{
						Property:   "Date",
						Descending: false,
					},
				},
			}
			indexes := []*datastore.IndexDefinition{&alertIdx, &revisionSummaryIdx}
			datastore.GetTestable(c).AddIndexes(indexes...)

			t.Run("GetTrees", func(t *ftt.Test) {
				t.Run("no trees yet", func(t *ftt.Test) {
					trees, err := GetTrees(c)

					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, string(trees), should.Equal("[]"))
				})

				tree := &model.Tree{
					Name:        "oak",
					DisplayName: "Oak",
				}
				assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				t.Run("basic tree", func(t *ftt.Test) {
					trees, err := GetTrees(c)

					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, string(trees), should.Equal(`[{"name":"oak","display_name":"Oak","bb_project_filter":""}]`))
				})
			})

			t.Run("/alerts", func(t *ftt.Test) {
				contents, _ := json.Marshal(&messages.Alert{
					Key: "test",
				})
				alertJSON := &model.AlertJSON{
					ID:       "test",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: false,
					Date:     time.Unix(1, 0).UTC(),
					Contents: []byte(contents),
				}
				contents2, _ := json.Marshal(&messages.Alert{
					Key: "test2",
				})
				oldResolvedJSON := &model.AlertJSON{
					ID:       "test2",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: true,
					Date:     time.Unix(1, 0).UTC(),
					Contents: []byte(contents2),
				}
				contents3, _ := json.Marshal(&messages.Alert{
					Key: "test3",
				})
				newResolvedJSON := &model.AlertJSON{
					ID:       "test3",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: true,
					Date:     clock.Now(c),
					Contents: []byte(contents3),
				}

				t.Run("GET", func(t *ftt.Test) {
					t.Run("no alerts yet", func(t *ftt.Test) {
						GetAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						_, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
					})

					assert.Loosely(t, datastorePutAlertJSON(c, alertJSON), should.BeNil)
					datastore.GetTestable(c).CatchupIndexes()

					t.Run("basic alerts", func(t *ftt.Test) {
						GetAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						r, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
						summary := &messages.AlertsSummary{}
						err = json.Unmarshal(r, &summary)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, summary.Alerts, should.HaveLength(1))
						assert.Loosely(t, summary.Alerts[0].Key, should.Equal("test"))
						assert.Loosely(t, summary.Resolved, should.HaveLength(0))
						// TODO(seanmccullough): Remove all of the POST /alerts handling
						// code and tests except for whatever chromeos needs.
					})

					assert.Loosely(t, datastorePutAlertJSON(c, oldResolvedJSON), should.BeNil)
					assert.Loosely(t, datastorePutAlertJSON(c, newResolvedJSON), should.BeNil)

					t.Run("resolved alerts", func(t *ftt.Test) {
						GetAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						r, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
						summary := &messages.AlertsSummary{}
						err = json.Unmarshal(r, &summary)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, summary.Alerts, should.HaveLength(1))
						assert.Loosely(t, summary.Alerts[0].Key, should.Equal("test"))
						assert.Loosely(t, summary.Resolved, should.HaveLength(1))
						assert.Loosely(t, summary.Resolved[0].Key, should.Equal("test3"))
						// TODO(seanmccullough): Remove all of the POST /alerts handling
						// code and tests except for whatever chromeos needs.
					})
				})
			})

			t.Run("/unresolved", func(t *ftt.Test) {
				contents, _ := json.Marshal(&messages.Alert{
					Key: "test",
				})
				alertJSON := &model.AlertJSON{
					ID:       "test",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: false,
					Date:     time.Unix(1, 0).UTC(),
					Contents: []byte(contents),
				}
				contents2, _ := json.Marshal(&messages.Alert{
					Key: "test2",
				})
				oldResolvedJSON := &model.AlertJSON{
					ID:       "test2",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: true,
					Date:     time.Unix(1, 0).UTC(),
					Contents: []byte(contents2),
				}
				contents3, _ := json.Marshal(&messages.Alert{
					Key: "test3",
				})
				newResolvedJSON := &model.AlertJSON{
					ID:       "test3",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: true,
					Date:     clock.Now(c),
					Contents: []byte(contents3),
				}

				t.Run("GET", func(t *ftt.Test) {
					t.Run("no alerts yet", func(t *ftt.Test) {
						GetUnresolvedAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						_, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
					})

					assert.Loosely(t, datastorePutAlertJSON(c, alertJSON), should.BeNil)
					assert.Loosely(t, datastorePutAlertJSON(c, oldResolvedJSON), should.BeNil)
					assert.Loosely(t, datastorePutAlertJSON(c, newResolvedJSON), should.BeNil)
					datastore.GetTestable(c).CatchupIndexes()

					t.Run("basic alerts", func(t *ftt.Test) {
						GetUnresolvedAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						r, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
						summary := &messages.AlertsSummary{}
						err = json.Unmarshal(r, &summary)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, summary.Alerts, should.HaveLength(1))
						assert.Loosely(t, summary.Alerts[0].Key, should.Equal("test"))
						assert.Loosely(t, summary.Resolved, should.BeNil)
					})
				})
			})

			t.Run("/resolved", func(t *ftt.Test) {
				contents, _ := json.Marshal(&messages.Alert{
					Key: "test",
				})
				alertJSON := &model.AlertJSON{
					ID:       "test",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: false,
					Date:     time.Unix(1, 0).UTC(),
					Contents: []byte(contents),
				}
				contents2, _ := json.Marshal(&messages.Alert{
					Key: "test2",
				})
				oldResolvedJSON := &model.AlertJSON{
					ID:       "test2",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: true,
					Date:     time.Unix(1, 0).UTC(),
					Contents: []byte(contents2),
				}
				contents3, _ := json.Marshal(&messages.Alert{
					Key: "test3",
				})
				newResolvedJSON := &model.AlertJSON{
					ID:       "test3",
					Tree:     datastore.MakeKey(c, "Tree", "chromeos"),
					Resolved: true,
					Date:     clock.Now(c),
					Contents: []byte(contents3),
				}

				t.Run("GET", func(t *ftt.Test) {
					t.Run("no alerts yet", func(t *ftt.Test) {
						GetResolvedAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						_, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
					})

					assert.Loosely(t, datastorePutAlertJSON(c, alertJSON), should.BeNil)
					assert.Loosely(t, datastorePutAlertJSON(c, oldResolvedJSON), should.BeNil)
					assert.Loosely(t, datastorePutAlertJSON(c, newResolvedJSON), should.BeNil)
					datastore.GetTestable(c).CatchupIndexes()

					t.Run("resolved alerts", func(t *ftt.Test) {
						GetResolvedAlertsHandler(&router.Context{
							Writer:  w,
							Request: makeGetRequest(c),
							Params:  makeParams("tree", "chromeos"),
						})

						r, err := ioutil.ReadAll(w.Body)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, w.Code, should.Equal(200))
						summary := &messages.AlertsSummary{}
						err = json.Unmarshal(r, &summary)
						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, summary.Alerts, should.BeNil)
						assert.Loosely(t, summary.Resolved, should.HaveLength(1))
						assert.Loosely(t, summary.Resolved[0].Key, should.Equal("test3"))
						// TODO(seanmccullough): Remove all of the POST /alerts handling
						// code and tests except for whatever chromeos needs.
					})
				})
			})
		})

		t.Run("cron", func(t *ftt.Test) {
			t.Run("flushOldAnnotations", func(t *ftt.Test) {
				getAllAnns := func() []*model.Annotation {
					anns := []*model.Annotation{}
					q := datastoreCreateAnnotationQuery()
					assert.Loosely(t, datastoreGetAnnotationsByQuery(c, &anns, q), should.BeNil)
					return anns
				}

				ann := &model.Annotation{
					KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
					Key:              "foobar",
					ModificationTime: datastore.RoundTime(cl.Now()),
				}
				assert.Loosely(t, datastorePutAnnotation(c, ann), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				t.Run("current not deleted", func(t *ftt.Test) {
					num, err := flushOldAnnotations(c)
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, num, should.BeZero)
					assert.Loosely(t, getAllAnns(), should.Match([]*model.Annotation{ann}))
				})

				ann.ModificationTime = cl.Now().Add(-(annotationExpiration + time.Hour))
				assert.Loosely(t, datastorePutAnnotation(c, ann), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				t.Run("old deleted", func(t *ftt.Test) {
					num, err := flushOldAnnotations(c)
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, num, should.Equal(1))
					assert.Loosely(t, getAllAnns(), should.Match([]*model.Annotation{}))
				})

				datastore.GetTestable(c).CatchupIndexes()
				q := datastoreCreateAnnotationQuery()
				anns := []*model.Annotation{}
				datastore.GetTestable(c).CatchupIndexes()
				datastoreGetAnnotationsByQuery(c, &anns, q)
				datastoreDeleteAnnotations(c, anns)
				anns = []*model.Annotation{
					{
						KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar2"))),
						Key:              "foobar2",
						ModificationTime: datastore.RoundTime(cl.Now()),
					},
					{
						KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
						Key:              "foobar",
						ModificationTime: datastore.RoundTime(cl.Now().Add(-(annotationExpiration + time.Hour))),
					},
				}
				assert.Loosely(t, datastorePutAnnotations(c, anns), should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				t.Run("only delete old", func(t *ftt.Test) {
					num, err := flushOldAnnotations(c)
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, num, should.Equal(1))
					assert.Loosely(t, getAllAnns(), should.Match(anns[:1]))
				})

				t.Run("handler", func(t *ftt.Test) {
					FlushOldAnnotationsHandler(c)
				})
			})

			t.Run("clientmon", func(t *ftt.Test) {
				body := &eCatcherReq{XSRFToken: tok}
				bodyBytes, err := json.Marshal(body)
				assert.Loosely(t, err, should.BeNil)
				ctx := &router.Context{
					Writer:  w,
					Request: makePostRequest(c, string(bodyBytes)),
					Params:  makeParams("xsrf_token", tok),
				}

				PostClientMonHandler(ctx)
				assert.Loosely(t, w.Code, should.Equal(200))
			})

			t.Run("treelogo", func(t *ftt.Test) {
				ctx := &router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
					Params:  makeParams("tree", "chromium"),
				}

				getTreeLogo(ctx, "", &noopSigner{})
				assert.Loosely(t, w.Code, should.Equal(302))
			})

			t.Run("treelogo fail", func(t *ftt.Test) {
				ctx := &router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
					Params:  makeParams("tree", "chromium"),
				}

				getTreeLogo(ctx, "", &noopSigner{fmt.Errorf("fail")})
				assert.Loosely(t, w.Code, should.Equal(500))
			})
		})
	})
}

type noopSigner struct {
	err error
}

func (n *noopSigner) SignBytes(c context.Context, b []byte) (string, []byte, error) {
	return string(b), b, n.err
}

func makeGetRequest(ctx context.Context, queryParams ...string) *http.Request {
	if len(queryParams)%2 != 0 {
		return nil
	}
	params := make([]string, len(queryParams)/2)
	for i := range params {
		params[i] = fmt.Sprintf("%s=%s", queryParams[2*i], queryParams[2*i+1])
	}
	paramsStr := strings.Join(params, "&")
	url := fmt.Sprintf("/doesntmatter?%s", paramsStr)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	return req
}

func makePostRequest(ctx context.Context, body string) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, "POST", "/doesntmatter", strings.NewReader(body))
	return req
}

func makeParams(items ...string) httprouter.Params {
	if len(items)%2 != 0 {
		return nil
	}

	params := make([]httprouter.Param, len(items)/2)
	for i := range params {
		params[i] = httprouter.Param{
			Key:   items[2*i],
			Value: items[2*i+1],
		}
	}

	return params
}
