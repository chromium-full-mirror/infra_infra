// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package track

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ds "go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common/triciumtest"
)

const (
	project    = "playground/gerrit-tricium"
	okACLUser  = "user:ok@example.com"
	okACLGroup = "tricium-playground-requesters"
)

// mockConfigProvider mocks the common.ConfigProvider interface.
type mockConfigProvider struct{}

func (*mockConfigProvider) GetServiceConfig(c context.Context) (*tricium.ServiceConfig, error) {
	return &tricium.ServiceConfig{}, nil
}

func (*mockConfigProvider) GetProjectConfig(c context.Context, p string) (*tricium.ProjectConfig, error) {
	if p == project {
		return &tricium.ProjectConfig{
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
	return &tricium.ProjectConfig{}, nil
}

func (*mockConfigProvider) GetAllProjectConfigs(c context.Context) (map[string]*tricium.ProjectConfig, error) {
	return nil, nil // not used in this test
}

func TestFetchRecentRequests(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {

		ctx := triciumtest.Context()

		request := &AnalyzeRequest{Project: project}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)

		t.Run("FetchRecentRequests ok user", func(t *ftt.Test) {
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity:       okACLUser,
				IdentityGroups: []string{okACLGroup},
			})
			rs, err := FetchRecentRequests(ctx, &mockConfigProvider{})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, rs, should.Resemble([]*AnalyzeRequest{request}))
		})

		t.Run("FetchRecentRequests other user", func(t *ftt.Test) {
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:other@example.com",
			})
			rs, err := FetchRecentRequests(ctx, &mockConfigProvider{})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(rs), should.BeZero)
		})
	})
}

func TestTrackHelperFunctions(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {

		ctx := triciumtest.Context()

		// Add completed request.
		request := &AnalyzeRequest{}
		assert.Loosely(t, ds.Put(ctx, request), should.BeNil)
		requestKey := ds.KeyForObj(ctx, request)
		assert.Loosely(t, ds.Put(ctx, &AnalyzeRequestResult{
			ID:     1,
			Parent: requestKey,
			State:  tricium.State_SUCCESS,
		}), should.BeNil)
		functionName := "Hello"
		run := &WorkflowRun{
			ID:        1,
			Parent:    requestKey,
			Functions: []string{functionName},
		}
		assert.Loosely(t, ds.Put(ctx, run), should.BeNil)
		runKey := ds.KeyForObj(ctx, run)
		assert.Loosely(t, ds.Put(ctx, &WorkflowRunResult{
			ID:     1,
			Parent: runKey,
			State:  tricium.State_SUCCESS,
		}), should.BeNil)
		platform := tricium.Platform_UBUNTU
		functionKey := ds.NewKey(ctx, "FunctionRun", functionName, 0, runKey)
		workerName := functionName + "_UBUNTU"
		assert.Loosely(t, ds.Put(ctx, &FunctionRun{
			ID:      functionName,
			Parent:  runKey,
			Workers: []string{workerName},
		}), should.BeNil)
		functionRunResult := &FunctionRunResult{
			ID:     1,
			Parent: functionKey,
			State:  tricium.State_SUCCESS,
		}
		assert.Loosely(t, ds.Put(ctx, functionRunResult), should.BeNil)
		workerKey := ds.NewKey(ctx, "WorkerRun", workerName, 0, functionKey)
		assert.Loosely(t, ds.Put(ctx, &WorkerRun{
			ID:       workerName,
			Parent:   functionKey,
			Platform: platform,
		}), should.BeNil)
		assert.Loosely(t, ds.Put(ctx, &Comment{
			Parent:  workerKey,
			Comment: []byte("Hello comment"),
		}), should.BeNil)

		t.Run("FetchFunctionRuns with results", func(t *ftt.Test) {
			functionRuns, err := FetchFunctionRuns(ctx, request.ID)
			assert.Loosely(t, len(functionRuns), should.Equal(1))
			assert.Loosely(t, functionRuns[0].ID, should.Equal("Hello"))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("FetchFunctionRuns without results", func(t *ftt.Test) {
			functionRuns, err := FetchFunctionRuns(ctx, request.ID+1)
			assert.Loosely(t, len(functionRuns), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("FetchComments with results", func(t *ftt.Test) {
			comments, err := FetchComments(ctx, request.ID)
			assert.Loosely(t, len(comments), should.Equal(1))
			assert.Loosely(t, string(comments[0].Comment), should.Equal("Hello comment"))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("FetchComments without results", func(t *ftt.Test) {
			comments, err := FetchComments(ctx, request.ID+1)
			assert.Loosely(t, len(comments), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("FetchWorkerRuns with results", func(t *ftt.Test) {
			workerRuns, err := FetchWorkerRuns(ctx, request.ID)
			assert.Loosely(t, len(workerRuns), should.Equal(1))
			assert.Loosely(t, workerRuns[0].ID, should.Equal("Hello_UBUNTU"))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("FetchWorkerRuns without results", func(t *ftt.Test) {
			workerRuns, err := FetchWorkerRuns(ctx, request.ID+1)
			assert.Loosely(t, len(workerRuns), should.BeZero)
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
