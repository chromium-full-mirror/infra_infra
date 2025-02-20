// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"strconv"
	"testing"

	"google.golang.org/grpc/codes"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	tricium "go.chromium.org/infra/tricium/api/v1"
	"go.chromium.org/infra/tricium/appengine/common/track"
	"go.chromium.org/infra/tricium/appengine/common/triciumtest"
)

func TestProgress(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		var runID int64 = 22

		// Add completed request.
		request := &track.AnalyzeRequest{
			ID: runID,
		}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		assert.Loosely(t, ds.Put(ctx, &track.AnalyzeRequestResult{
			ID:     1,
			Parent: requestKey,
			State:  tricium.State_SUCCESS,
		}), should.BeNil)
		functionName := "Hello"
		run := &track.WorkflowRun{
			ID:        1,
			Parent:    requestKey,
			Functions: []string{functionName},
		}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &track.WorkflowRunResult{
			ID:     1,
			Parent: runKey,
			State:  tricium.State_SUCCESS,
		}), should.BeNil)
		platform := tricium.Platform_UBUNTU
		functionKey := ds.NewKey(ctx, "FunctionRun", functionName, 0, runKey)
		workerName := functionName + "_UBUNTU"
		assert.Loosely(t, ds.Put(ctx, &track.FunctionRun{
			ID:      functionName,
			Parent:  runKey,
			Workers: []string{workerName},
		}), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &track.FunctionRunResult{
			ID:     1,
			Parent: functionKey,
			State:  tricium.State_SUCCESS,
		}), should.BeNil)
		workerKey := ds.NewKey(ctx, "WorkerRun", workerName, 0, functionKey)
		worker := &track.WorkerRun{
			ID:       workerName,
			Parent:   functionKey,
			Platform: platform,
		}
		assert.Loosely(t, ds.Put(ctx, worker), should.BeNil)
		workerKey = ds.KeyForObj(ctx, worker)
		assert.Loosely(t, ds.Put(ctx, &track.WorkerRunResult{
			ID:          1,
			Parent:      workerKey,
			Function:    functionName,
			Platform:    tricium.Platform_UBUNTU,
			State:       tricium.State_SUCCESS,
			NumComments: 1,
		}), should.BeNil)

		// Add mapping from Gerrit change to the run ID.
		host := "chromium-review.googlesource.com"
		project := "playground/gerrit-tricium"
		changeIDFooter := "I17e97e23ecf2890bf6b72ffd1d7a3167ed1b0a11"
		change := fmt.Sprintf("%s~master~%s", project, changeIDFooter)
		revision := "refs/changes/97/97/1"
		g := &GerritChangeToRunID{
			ID:    gerritMappingID(host, project, change, revision),
			RunID: runID,
		}
		assert.Loosely(t, ds.Put(ctx, g), should.BeNil)

		t.Run("Progress request handler", func(t *ftt.Test) {
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: identity.Identity(okACLUser),
			})
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_GerritRevision{
					GerritRevision: &tricium.GerritRevision{
						Host:    host,
						Project: project,
						Change:  fmt.Sprintf("%s~master~%s", project, changeIDFooter),
						GitRef:  revision,
					},
				},
			}
			response, err := server.Progress(ctx, request)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, response, should.Match(&tricium.ProgressResponse{
				RunId: strconv.FormatInt(runID, 10),
				State: tricium.State_SUCCESS,
				FunctionProgress: []*tricium.FunctionProgress{
					{
						Name:        "Hello",
						Platform:    tricium.Platform_UBUNTU,
						State:       tricium.State_SUCCESS,
						NumComments: 1,
					},
				},
			}))
		})

		t.Run("Progress request handler with Gerrit patch not found", func(t *ftt.Test) {
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: identity.Identity(okACLUser),
			})
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_GerritRevision{
					GerritRevision: &tricium.GerritRevision{
						Host:    host,
						Project: project,
						Change:  fmt.Sprintf("%s~master~%s", project, changeIDFooter),
						GitRef:  "refs/changes/97/97/2",
					},
				},
			}
			response, err := server.Progress(ctx, request)
			assert.Loosely(t, response, should.Match(&tricium.ProgressResponse{}))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Validate request with valid run ID", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_RunId{
					RunId: "12345",
				},
			}
			id, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, id, should.Equal(12345))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Validate request with invalid run ID", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_RunId{
					RunId: "not a valid run ID",
				},
			}
			_, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Validate request with no contents", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{}
			_, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("Validate request with valid Gerrit details", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_GerritRevision{
					GerritRevision: &tricium.GerritRevision{
						Host:    host,
						Project: project,
						Change:  fmt.Sprintf("%s~master~%s", project, changeIDFooter),
						GitRef:  revision,
					},
				},
			}
			id, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, id, should.Equal(runID))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Validate request with valid Gerrit details but no stored run", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_GerritRevision{
					GerritRevision: &tricium.GerritRevision{
						Host:    host,
						Project: project,
						Change:  fmt.Sprintf("%s~master~%s", project, changeIDFooter),
						GitRef:  "refs/changes/97/97/2",
					},
				},
			}
			id, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, id, should.BeZero)
			assert.Loosely(t, grpcutil.Code(err), should.Equal(codes.OK))
		})

		t.Run("Validate request with missing Gerrit change ID", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_GerritRevision{
					GerritRevision: &tricium.GerritRevision{
						Host:    host,
						Project: project,
						GitRef:  revision,
					},
				},
			}
			_, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)

		})
		t.Run("Validate request with invalid Gerrit change ID", func(t *ftt.Test) {
			request := &tricium.ProgressRequest{
				Source: &tricium.ProgressRequest_GerritRevision{
					GerritRevision: &tricium.GerritRevision{
						Host:    host,
						Project: project,
						Change:  "not a change ID",
						GitRef:  revision,
					},
				},
			}
			_, err := validateProgressRequest(ctx, request)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
