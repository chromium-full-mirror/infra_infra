// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/urlfetch"
	"go.chromium.org/luci/server/router"
)

func TestRevisionHandler(t *testing.T) {
	t.Parallel()

	data := map[string][]byte{
		"500": []byte("invalid json"),
		"1300": []byte(
			`{
         "git_sha": "0dfc81bbe403cd98f4cd2d58e7817cdc8a881a5f",
         "repo": "chromium/src",
         "redirect_url": "https://chromium.googlesource.com/chromium/src/+/0dfc81bbe403cd98f4cd2d58e7817cdc8a881a5f",
         "project": "chromium",
         "redirect_type": "GIT_FROM_NUMBER",
         "repo_url": "https://chromium.googlesource.com/chromium/src/",
         "kind": "crrev#redirectItem",
         "etag": "\"kuKkspxlsT40mYsjSiqyueMe20E/qt8_sNqlQbK8xP9pkpc9EOsNyrE\""
       }`),
		"1350": []byte(
			`{
         "git_sha": "2a2d3d036c39043ef5f8232493252732a17f7e16",
         "repo": "chromium/src",
         "redirect_url": "https://chromium.googlesource.com/chromium/src/+/0dfc81bbe403cd98f4cd2d58e7817cdc8a881a5f",
         "project": "chromium",
         "redirect_type": "GIT_FROM_NUMBER",
         "repo_url": "https://chromium.googlesource.com/chromium/src/",
         "kind": "crrev#redirectItem",
         "etag": "\"kuKkspxlsT40mYsjSiqyueMe20E/qt8_sNqlQbK8xP9pkpc9EOsNyrE\""
       }`),
	}

	handler := func(c *router.Context) {
		b, ok := data[c.Params.ByName("pos")]
		if !ok {
			http.Error(c.Writer, "not found", http.StatusNotFound)
			return
		}
		c.Writer.Write(b)
	}

	ctx := memory.Use(context.Background())
	withTestingContext := func(c *router.Context, next router.Handler) {
		c.Request = c.Request.WithContext(ctx)
		next(c)
	}

	r := router.New()
	r.GET(
		"/revision_range", router.NewMiddlewareChain(withTestingContext),
		revisionHandler)
	r.GET("/commitHash/:pos", router.MiddlewareChain{}, handler)
	srv := httptest.NewServer(r)
	crRevURL = srv.URL + "/commitHash"

	ftt.Run("commitHash", t, func(t *ftt.Test) {
		client := crRevClient{
			HTTPClient: &http.Client{},
			BaseURL:    crRevURL,
		}

		t.Run("with existing position", func(t *ftt.Test) {
			hash, err := client.commitHash("1300")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, hash, should.Equal("0dfc81bbe403cd98f4cd2d58e7817cdc8a881a5f"))
		})

		t.Run("with non-existent position", func(t *ftt.Test) {
			_, err := client.commitHash("0")
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("with invalid returned JSON", func(t *ftt.Test) {
			_, err := client.commitHash("500")
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("with HTTP error", func(t *ftt.Test) {
			client.BaseURL = "invalid-url"
			_, err := client.commitHash("500")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})

	ftt.Run("revisionHandler", t, func(t *ftt.Test) {
		ctx = urlfetch.Set(ctx, http.DefaultTransport)

		client := &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		t.Run("with valid range", func(t *ftt.Test) {
			resp, err := client.Get(
				srv.URL + "/revision_range?start=1300&end=1350&n=1000")
			assert.Loosely(t, err, should.BeNil)
			defer resp.Body.Close()

			assert.Loosely(t, resp.StatusCode, should.Equal(http.StatusMovedPermanently))
			assert.Loosely(t, resp.Header.Get("Location"), should.Equal(
				"https://chromium.googlesource.com/chromium/src/+log/"+
					"0dfc81bbe403cd98f4cd2d58e7817cdc8a881a5f^.."+
					"2a2d3d036c39043ef5f8232493252732a17f7e16?pretty=fuller&n=1000"))
		})

		t.Run("with invalid range", func(t *ftt.Test) {
			resp, err := client.Get(srv.URL + "/revision_range?start=0&end=100")
			assert.Loosely(t, err, should.BeNil)
			defer resp.Body.Close()
			assert.Loosely(t, resp.StatusCode, should.Equal(http.StatusInternalServerError))
		})
	})
}
