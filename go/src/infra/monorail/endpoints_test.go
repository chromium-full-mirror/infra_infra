// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package monorail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.chromium.org/luci/common/retry/transient"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestEndpointsInsertIssue(t *testing.T) {
	t.Parallel()

	ftt.Run("Endpoints client: InsertIssue", t, func(t *ftt.Test) {
		ctx := context.Background()

		t.Run("Insert issue request succeeds", func(t *ftt.Test) {
			req := &InsertIssueRequest{
				Issue: &Issue{
					Summary:     "Write tests for monorail client",
					Author:      &AtomPerson{Name: "seanmccullough@chromium.org"},
					Owner:       &AtomPerson{Name: "nodir@chromium.org"},
					Status:      StatusStarted,
					Cc:          []*AtomPerson{{Name: "agable@chromium.org"}},
					Description: "We should keep our code coverage high, so write tests",
					Components:  []string{"Infra"},
					Labels:      []string{"M-53"},
					ProjectId:   "chromium",
				},
			}

			res := &InsertIssueResponse{
				Issue: &Issue{},
			}
			*res.Issue = *req.Issue
			res.Issue.Id = 1

			var insertIssueServer *httptest.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Loosely(t, r.URL.String(), should.Equal("/projects/chromium/issues?sendEmail=false"))

				actualReq := &Issue{}
				err := json.NewDecoder(r.Body).Decode(actualReq)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, actualReq, should.Resemble(req.Issue))

				err = json.NewEncoder(w).Encode(res.Issue)
				assert.Loosely(t, err, should.BeNil)
			}))
			defer insertIssueServer.Close()

			httpClient := &http.Client{}
			client := NewEndpointsClient(httpClient, insertIssueServer.URL)
			actualRes, err := client.InsertIssue(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actualRes, should.Resemble(res))
		})

		t.Run("Insert issue with invalid request", func(t *ftt.Test) {
			req := &InsertIssueRequest{
				Issue: &Issue{
					Summary: "Write tests for monorail client",
					Author:  &AtomPerson{Name: "seanmccullough@chromium.org"},
					Owner:   &AtomPerson{Name: "nodir@chromium.org"},
					Status:  StatusStarted,
				},
			}

			httpClient := &http.Client{}
			client := NewEndpointsClient(httpClient, "https://example.com")
			_, err := client.InsertIssue(ctx, req)
			assert.Loosely(t, err, should.ErrLike("no projectId"))
		})

		t.Run("Insert comment request", func(t *ftt.Test) {
			req := &InsertCommentRequest{
				Issue: &IssueRef{
					ProjectId: "chromium",
					IssueId:   1,
				},
				Comment: &InsertCommentRequest_Comment{
					Content: "Done",
					Updates: &Update{
						Status: StatusFixed,
					},
				},
			}

			var handler http.HandlerFunc
			var server *httptest.Server
			actualURL := ""
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actualURL = r.URL.String()
				handler(w, r)
			}))
			defer server.Close()

			client := NewEndpointsClient(nil, server.URL)

			t.Run("Succeeds", func(t *ftt.Test) {
				handler = func(w http.ResponseWriter, r *http.Request) {
					actualReq := &InsertCommentRequest_Comment{}
					err := json.NewDecoder(r.Body).Decode(actualReq)
					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, actualReq, should.Resemble(req.Comment))

					fmt.Fprint(w, "{}")
				}

				_, err := client.InsertComment(ctx, req)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, actualURL, should.Equal("/projects/chromium/issues/1/comments?"))
			})

			t.Run("SendEmail", func(t *ftt.Test) {
				req.SendEmail = true
				handler = func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, "{}")
				}

				_, err := client.InsertComment(ctx, req)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, actualURL, should.Equal("/projects/chromium/issues/1/comments?sendEmail=true"))
			})

			t.Run("Transient error", func(t *ftt.Test) {
				test := func(status int) {
					handler = func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(status)
					}

					_, err := client.InsertComment(ctx, req)
					assert.Loosely(t, err, should.NotBeNil)
					assert.Loosely(t, transient.Tag.In(err), should.BeTrue)
				}
				t.Run("With HTTP 404", func(t *ftt.Test) {
					test(404)
				})
				t.Run("With HTTP 503", func(t *ftt.Test) {
					test(503)
				})
			})
		})
	})
}

func TestEndpointsListComments(t *testing.T) {
	t.Parallel()

	ftt.Run("Endpoints client: ListComments", t, func(t *ftt.Test) {
		ctx := context.Background()

		t.Run("succeeds", func(t *ftt.Test) {
			req := &ListCommentsRequest{Issue: &IssueRef{IssueId: 859707, ProjectId: "chromium"}}

			var srv *httptest.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Loosely(t, r.URL.String(), should.Equal("/projects/chromium/issues/859707/comments?startIndex=0"))
				_, err := w.Write([]byte(`{
					"items": [
						{
						 "canDelete": false,
						 "author": { "kind": "monorail#issuePerson", "name": "a@example.com" },
						 "is_description": true,
						 "content": "Actually, this is description\n",
						 "published": "2018-07-02T23:12:47",
						 "id": 0
						},
						{
						 "canDelete": false,
						 "author": { "kind": "monorail#issuePerson", "name": "b@example.com" },
						 "is_description": false,
						 "content": "lol",
						 "updates": {
							"status": "Started",
							"kind": "monorail#issueCommentUpdate",
							"labels": [
							 "-Type-Task",
							 "Type-Feature"
							]
						 },
						 "published": "2018-07-02T23:14:59",
						 "id": 1
						}
					 ] ,
					 "kind": "monorail#issueCommentList",
					 "totalResults": 2,
					 "etag": "\"se1Lh8IyiCDwsGaF9fqPeVscq_I/rYRAvt40qdVXvtOjNLeqW1ZMUjA\""
				}`))
				assert.Loosely(t, err, should.BeNil)
			}))
			defer srv.Close()

			client := NewEndpointsClient(&http.Client{}, srv.URL)
			res, err := client.ListComments(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Resemble(&ListCommentsResponse{
				TotalResults: 2,
				Items: []*Comment{
					{
						Author:        &AtomPerson{Name: "a@example.com"},
						IsDescription: true,
						Content:       "Actually, this is description\n",
						Published:     "2018-07-02T23:12:47",
						Id:            0,
					},
					{
						Author:        &AtomPerson{Name: "b@example.com"},
						IsDescription: false,
						Content:       "lol",
						Published:     "2018-07-02T23:14:59",
						Id:            1,
						Updates: &Update{
							Status: "Started",
							Labels: []string{"-Type-Task", "Type-Feature"},
						},
					},
				},
			}))
		})
	})
}
