// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gerrit

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tricium/appengine/common/track"
)

func TestCLAndPatchSetNumberFunctions(t *testing.T) {
	ftt.Run("Extracts patch set number from valid ref", t, func(t *ftt.Test) {
		assert.Loosely(t, PatchSetNumber("refs/changes/43/6543/7"), should.Equal("7"))
		assert.Loosely(t, PatchSetNumber("refs/changes/45/12345/12"), should.Equal("12"))
	})

	ftt.Run("Extracts CL number from valid ref", t, func(t *ftt.Test) {
		assert.Loosely(t, CLNumber("refs/changes/43/6543/7"), should.Equal("6543"))
		assert.Loosely(t, CLNumber("refs/changes/45/12345/12"), should.Equal("12345"))
	})

	ftt.Run("Both numbers can be extracted in one invocation", t, func(t *ftt.Test) {
		cl, patch := ExtractCLPatchSetNumbers("refs/changes/45/12345/12")
		assert.Loosely(t, cl, should.Equal("12345"))
		assert.Loosely(t, patch, should.Equal("12"))
	})

	ftt.Run("When input format is invalid, empty strings are returned", t, func(t *ftt.Test) {
		cl, patch := ExtractCLPatchSetNumbers("refs/changes/1111/1")
		assert.Loosely(t, cl, should.BeEmpty)
		assert.Loosely(t, patch, should.BeEmpty)
	})

	ftt.Run("Given invalid input, returns empty string", t, func(t *ftt.Test) {
		assert.Loosely(t, PatchSetNumber("foorefs/changes/45/12345/7bar"), should.BeEmpty)
		assert.Loosely(t, PatchSetNumber("refs/changes/45/12345"), should.BeEmpty)
		assert.Loosely(t, PatchSetNumber("refs/changes/45/12345/"), should.BeEmpty)
		assert.Loosely(t, PatchSetNumber(""), should.BeEmpty)
	})

	ftt.Run("Given invalid input, returns empty string", t, func(t *ftt.Test) {
		assert.Loosely(t, CLNumber("foorefs/changes/45/12345/7bar"), should.BeEmpty)
		assert.Loosely(t, CLNumber("refs/changes/45/12345"), should.BeEmpty)
		assert.Loosely(t, CLNumber("refs/changes/45/12345/"), should.BeEmpty)
		assert.Loosely(t, CLNumber(""), should.BeEmpty)
	})
}

func TestComposeCreateURL(t *testing.T) {
	ftt.Run("createURL with well-formed Gerrit change ref", t, func(t *ftt.Test) {
		assert.Loosely(t,
			CreateURL("https://chromium-review.googlesource.com", "refs/changes/10/12310/3"),
			should.Equal("https://chromium-review.googlesource.com/c/12310/3"))
	})

	ftt.Run("with badly-formed ref returns empty string", t, func(t *ftt.Test) {
		assert.Loosely(t, CreateURL("foo.com", "xxrefs/changes/10/12310/3xx"), should.BeEmpty)
		assert.Loosely(t, CreateURL("foo.com", "refs/changes/123/4"), should.BeEmpty)
	})
}

func TestIsGerritProjectRequest(t *testing.T) {
	ftt.Run("Test Gerrit project request", t, func(t *ftt.Test) {
		assert.Loosely(t, IsGerritProjectRequest(&track.AnalyzeRequest{
			ID:            123,
			Project:       "my-luci-config-project-id",
			GitURL:        "http://my-gerrit.com/my-project",
			GitRef:        "refs/changes/97/597/2",
			GerritHost:    "http://my-gerrit-review.com/my-project",
			GerritProject: "my-project",
			GerritChange:  "my-project~master~I8473b95934b5732ac55d26311a706c9c2bde9940",
		}), should.BeTrue)
	})

	ftt.Run("Test non Gerrit project request", t, func(t *ftt.Test) {
		assert.Loosely(t, IsGerritProjectRequest(&track.AnalyzeRequest{
			ID:      123,
			Project: "my-luci-config-project-id",
			GitURL:  "http://my-gerrit.com/my-project",
			GitRef:  "refs/changes/97/597/2",
		}), should.BeFalse)
	})
}
