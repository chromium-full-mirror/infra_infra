// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/router"
)

var _ = fmt.Printf

func TestMain(t *testing.T) {
	t.Parallel()

	ftt.Run("main", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = gologger.StdConfig.Use(c)

		cl := testclock.New(testclock.TestRecentTimeUTC)
		c = clock.Set(c, cl)

		w := httptest.NewRecorder()

		t.Run("index", func(t *ftt.Test) {
			t.Run("pathless", func(t *ftt.Test) {
				(&SOMHandlers{}).indexPage(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c, "/"),
				})

				assert.Loosely(t, w.Code, should.Equal(302))
			})

			t.Run("anonymous", func(t *ftt.Test) {
				(&SOMHandlers{}).indexPage(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c, "/chromium"),
				})

				r, err := io.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, w.Code, should.Equal(500))
				assert.Loosely(t, body, should.NotContainSubstring("som-app"))
				assert.Loosely(t, body, should.ContainSubstring("login"))
			})

			authState := &authtest.FakeState{
				Identity: "user:user@example.com",
			}
			c = auth.WithState(c, authState)

			t.Run("No access", func(t *ftt.Test) {
				(&SOMHandlers{}).indexPage(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c, "/chromium"),
				})

				assert.Loosely(t, w.Code, should.Equal(200))
				r, err := io.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, body, should.NotContainSubstring("som-app"))
				assert.Loosely(t, body, should.ContainSubstring("Access denied"))
			})
			authState.IdentityGroups = []string{authGroup}

			t.Run("good path", func(t *ftt.Test) {
				(&SOMHandlers{}).indexPage(&router.Context{
					Writer:  w,
					Request: makeGetRequest(c, "/chromium"),
				})
				r, err := io.ReadAll(w.Body)
				assert.Loosely(t, err, should.BeNil)
				body := string(r)
				assert.Loosely(t, body, should.ContainSubstring("som-app"))
				assert.Loosely(t, w.Code, should.Equal(200))
			})
		})

		t.Run("noop", func(t *ftt.Test) {
			noopHandler(nil)
		})
	})
}

func makeGetRequest(ctx context.Context, path string) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, "GET", path, nil)
	return req
}
