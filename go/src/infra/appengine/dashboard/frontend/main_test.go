// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/julienschmidt/httprouter"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/templates"
)

func TestDashboard(t *testing.T) {
	t.Parallel()

	ftt.Run("dashboard", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = templates.Use(
			c, prepareTemplates(&server.Options{}), &templates.Extra{
				Request: &http.Request{
					URL: &url.URL{Path: "/"}}})
		c = gologger.StdConfig.Use(c)

		w := httptest.NewRecorder()

		t.Run("template params", func(t *ftt.Test) {
			t.Run("anonymous", func(t *ftt.Test) {
				c = auth.WithState(c, &authtest.FakeState{})
				dashboard(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
					Params:  makeParams("path", "/"),
				})
				r, err := io.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, body, should.ContainSubstring("chopsdash-app"))
				assert.Loosely(t, w.Code, should.Equal(200))
				assert.Loosely(t, body, should.NotContainSubstring("is-googler"))
				assert.Loosely(t, body, should.ContainSubstring("user=\"\""))
			})

			authState := &authtest.FakeState{
				Identity: "user:user@example.com",
			}
			t.Run("not-googler", func(t *ftt.Test) {
				c = auth.WithState(c, authState)
				dashboard(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
					Params:  makeParams("path", "/"),
				})

				r, err := io.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, body, should.ContainSubstring("chopsdash-app"))
				assert.Loosely(t, w.Code, should.Equal(200))
				assert.Loosely(t, body, should.NotContainSubstring("is-googler"))
				assert.Loosely(t, body, should.ContainSubstring("user=\"user@example.com\""))
			})

			t.Run("googler", func(t *ftt.Test) {
				authState.IdentityGroups = []string{authGroup}
				c = auth.WithState(c, authState)
				dashboard(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c),
					Params:  makeParams("path", "/"),
				})

				r, err := io.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, body, should.ContainSubstring("chopsdash-app"))
				assert.Loosely(t, w.Code, should.Equal(200))
				assert.Loosely(t, body, should.ContainSubstring("is-googler"))
				assert.Loosely(t, body, should.ContainSubstring("user=\"user@example.com\""))
			})
		})
	})
}

func makeGetRequest(ctx context.Context) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, "GET", "/doesntmatter", nil)
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
