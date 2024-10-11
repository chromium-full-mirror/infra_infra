// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
)

// These tests read from files on the filesystem, so modifying the tests may
// require modifying the example test files.
const (
	baseDir          = "test"
	gLinks           = "src/g_links.md"
	goLinks          = "src/go_links.md"
	httpsURLs        = "src/https_urls.md"
	httpURL          = "src/http_single_url.md"
	multipleHTTPURLs = "src/http_multiple_urls.md"
)

func TestHTTPSChecker(t *testing.T) {

	ftt.Run("Produces no comment for g/ link", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		checkHTTPS(baseDir, gLinks, results)
		assert.Loosely(t, results.Comments, should.BeNil)
	})

	ftt.Run("Produces no comment for file with go/ link", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		checkHTTPS(baseDir, goLinks, results)
		assert.Loosely(t, results.Comments, should.BeNil)
	})

	ftt.Run("Produces no comment for file with httpsURLs", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		checkHTTPS(baseDir, httpsURLs, results)
		assert.Loosely(t, results.Comments, should.BeNil)
	})

	ftt.Run("Flags a single http URL", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		checkHTTPS(baseDir, httpURL, results)
		assert.Loosely(t, results.Comments, should.NotBeNil)
		assert.Loosely(t, results.Comments[0], should.Resemble(&tricium.Data_Comment{
			Category:  "HttpsCheck/Warning",
			Message:   ("Nit: Replace http:// URLs with https://"),
			Path:      httpURL,
			StartLine: 5,
			EndLine:   5,
			StartChar: 7,
			EndChar:   24,
		}))
	})

	ftt.Run("Flags multiple http URLs", t, func(t *ftt.Test) {
		results := &tricium.Data_Results{}
		checkHTTPS(baseDir, multipleHTTPURLs, results)
		assert.Loosely(t, len(results.Comments), should.Equal(2))
		assert.Loosely(t, results.Comments[1], should.Resemble(&tricium.Data_Comment{
			Category:  "HttpsCheck/Warning",
			Message:   ("Nit: Replace http:// URLs with https://"),
			Path:      multipleHTTPURLs,
			StartLine: 9,
			EndLine:   9,
			StartChar: 7,
			EndChar:   26,
		}))
	})
}
