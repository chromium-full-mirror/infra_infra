// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"

	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common/triciumtest"
)

func TestMainPageHandler(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		authState := &authtest.FakeState{
			Identity: "user:user@example.com",
		}
		ctx = auth.WithState(ctx, authState)
		w := httptest.NewRecorder()

		t.Run("Basic request to main page handler", func(t *ftt.Test) {
			mainPageHandler(&router.Context{
				Writer:  w,
				Request: triciumtest.MakeGetRequest(nil).WithContext(ctx),
				Params:  triciumtest.MakeParams(),
			})
			assert.Loosely(t, w.Code, should.Equal(200))
			r, err := ioutil.ReadAll(w.Body)
			assert.Loosely(t, err, should.BeNil)
			body := string(r)
			assert.Loosely(t, body, should.ContainSubstring("html"))
		})

		t.Run("Constructing template args", func(t *ftt.Test) {
			args, err := templateArgs(ctx, triciumtest.MakeGetRequest(nil))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, args, should.Match(map[string]interface{}{
				"AppVersion":  "testVersionID",
				"IsAnonymous": false,
				"LoginURL":    "http://fake.example.com/login?dest=%2Ftesting-path",
				"LogoutURL":   "http://fake.example.com/logout?dest=%2Ftesting-path",
				"User":        "user@example.com",
			}))
		})
	})
}

func TestAnalyzeQueueHandler(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {
		ctx := triciumtest.Context()
		w := httptest.NewRecorder()

		t.Run("Analyze queue handler checks for invalid requests", func(t *ftt.Test) {
			// A request with an empty paths list is not valid.
			ar := &tricium.AnalyzeRequest{
				Project: "some-project",
				Files:   nil,
				Source: &tricium.AnalyzeRequest_GitCommit{
					GitCommit: &tricium.GitCommit{
						Ref: "some/ref",
						Url: "https://example.com/repo.git",
					},
				},
			}
			bytes, err := proto.Marshal(ar)
			analyzeHandler(&router.Context{
				Writer:  w,
				Request: triciumtest.MakeGetRequest(bytes).WithContext(ctx),
				Params:  triciumtest.MakeParams(),
			})
			assert.Loosely(t, w.Code, should.Equal(400))
			r, err := ioutil.ReadAll(w.Body)
			assert.Loosely(t, err, should.BeNil)
			body := string(r)
			assert.Loosely(t, body, should.BeEmpty)
		})
	})
}
