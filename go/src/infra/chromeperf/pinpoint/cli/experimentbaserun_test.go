// Copyright 2020 The Chromium Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCLFlagParsing(t *testing.T) {
	t.Parallel()
	ftt.Run("When provided a valid input", t, func(t *ftt.Test) {
		fs := flag.NewFlagSet("cl-flag-parsing", flag.PanicOnError)
		clFlag := clValue{}
		fs.Var(&clFlag, "cl", "a gerrit CL")
		t.Run("/c/<repo>/+/<CL>", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/d/d/+/1234"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.BeNil)
			assert.Loosely(t, clFlag.clNum, should.Equal(1234))
			assert.Loosely(t, clFlag.patchSet, should.BeZero)
		})
		t.Run("/c/<repo>/+/<CL>/<patch>", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/d/d/+/1234/12"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.BeNil)
			assert.Loosely(t, clFlag.clNum, should.Equal(1234))
			assert.Loosely(t, clFlag.patchSet, should.Equal(12))
		})
		t.Run("/c/<repo>/+/<CL>/<patch> short repo", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/d/+/1234/12"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.BeNil)
			assert.Loosely(t, clFlag.clNum, should.Equal(1234))
			assert.Loosely(t, clFlag.patchSet, should.Equal(12))
		})
		t.Run("/c/<CL>", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/1234"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.BeNil)
			assert.Loosely(t, clFlag.clNum, should.Equal(1234))
			assert.Loosely(t, clFlag.patchSet, should.BeZero)
		})
		t.Run("/c/<CL>/<patch>", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/1234/12"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.BeNil)
			assert.Loosely(t, clFlag.clNum, should.Equal(1234))
			assert.Loosely(t, clFlag.patchSet, should.Equal(12))
		})
		t.Run("crrev.com", func(t *ftt.Test) {
			s := "https://crrev.com/c/1234/12"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.BeNil)
			assert.Loosely(t, clFlag.clNum, should.Equal(1234))
			assert.Loosely(t, clFlag.patchSet, should.Equal(12))
		})
	})
	ftt.Run("When provided some invalid cases", t, func(t *ftt.Test) {
		fs := flag.NewFlagSet("cl-error-flag-parsing", flag.ContinueOnError)
		clFlag := clValue{}
		fs.Var(&clFlag, "cl", "a gerrit CL")
		t.Run("<CL>/<patch> without host", func(t *ftt.Test) {
			s := "1234/12"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.NotBeNil)
		})
		t.Run("/c/<repo>/+/", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/d/d/+"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.NotBeNil)
		})
		t.Run("/c/<repo>/+/0/<patch>", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/d/d/+/0/01"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.NotBeNil)
		})
		t.Run("/c/<repo>/+/<CL>/", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/d/d/+/1234/"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.NotBeNil)
		})
		t.Run("/c/+/<CL>", func(t *ftt.Test) {
			s := "https://chromium-review.googlesource.com/c/+/1234"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.NotBeNil)
		})
		t.Run("<CL>/<patch> wrong host", func(t *ftt.Test) {
			s := "https://crev.com/c/1234/12"
			assert.Loosely(t, fs.Parse([]string{"-cl", s}), should.NotBeNil)
		})
	})
}

func TestBugFlagParsing(t *testing.T) {
	t.Parallel()
	ftt.Run("When provided a valid case", t, func(t *ftt.Test) {
		fs := flag.NewFlagSet("bug-flag-parsing", flag.ContinueOnError)
		bug := &bugValue{}
		fs.Var(bug, "bug", "a Monorail issue in the form <project>:<id>")
		err := fs.Parse([]string{"-bug", "chromium:1234"})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, bug.project, should.Equal("chromium"))
		assert.Loosely(t, bug.issueID, should.Equal(1234))
	})
	ftt.Run("When provided some invalid cases", t, func(t *ftt.Test) {
		fs := flag.NewFlagSet("errors-flag-parsing", flag.ContinueOnError)
		bug := &bugValue{}
		fs.Var(bug, "bug", "a Monorail issue in the form <project>:<id>")
		t.Run(":<id>", func(t *ftt.Test) {
			assert.Loosely(t, fs.Parse([]string{"-bug", ":1"}), should.NotBeNil)
		})
		t.Run("project:0", func(t *ftt.Test) {
			assert.Loosely(t, fs.Parse([]string{"-bug", "project:0"}), should.NotBeNil)
		})
		t.Run("project:01", func(t *ftt.Test) {
			assert.Loosely(t, fs.Parse([]string{"-bug", "project:01"}), should.NotBeNil)
		})
	})
}

func hardcodedCommandOutput(data string) writeGitCLJSON {
	return func(intoFile string) error {
		if err := os.WriteFile(intoFile, []byte(data), 0666); err != nil {
			panic(fmt.Sprintf("unexpected error while writing out fake data to %q: %v", intoFile, err))
		}
		return nil
	}
}

const (
	// The output from running `git cl issue --json $FILE` inside the pinpoint
	// directory as of 2021-03-23.
	infraGitClIssueOutput = `{"gerrit_host": "chromium-review.googlesource.com", "gerrit_project": "infra/infra", "issue_url": null, "issue": null}`
	// Constants to represent the values in infraGitClIssueOutput
	infraGerritHost  = "chromium-review.googlesource.com"
	infraGitilesHost = "chromium.googlesource.com"
	infraRepository  = "infra/infra"

	// The output from running `git cl issue --json $FILE` before
	// https://crrev.com/c/2766153 was applied.
	oldGitClIssueOutput = `{"issue_url": null, "issue": null}`
)

func TestGuessRepositoryDefaults(t *testing.T) {
	t.Parallel()
	ftt.Run("When provided appropriate JSON data", t, func(t *ftt.Test) {
		gitiles, gerrit, repo, err := guessRepositoryDefaults(hardcodedCommandOutput(infraGitClIssueOutput))

		t.Run("should return infra information", func(t *ftt.Test) {
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, gitiles, should.Equal(infraGitilesHost))
			assert.Loosely(t, gerrit, should.Equal(infraGerritHost))
			assert.Loosely(t, repo, should.Equal(infraRepository))
		})
	})

	ftt.Run("When provided outdated JSON data", t, func(t *ftt.Test) {
		gitiles, gerrit, repo, err := guessRepositoryDefaults(hardcodedCommandOutput(oldGitClIssueOutput))

		t.Run("should return default information", func(t *ftt.Test) {
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, gitiles, should.Equal(defaultGitilesHost))
			assert.Loosely(t, gerrit, should.Equal(defaultGerritHost))
			assert.Loosely(t, repo, should.Equal(defaultRepository))
		})
	})

	ftt.Run("When provided invalid JSON data", t, func(t *ftt.Test) {
		gitiles, gerrit, repo, err := guessRepositoryDefaults(hardcodedCommandOutput("invalid json"))

		t.Run("should return default information", func(t *ftt.Test) {
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, gitiles, should.Equal(defaultGitilesHost))
			assert.Loosely(t, gerrit, should.Equal(defaultGerritHost))
			assert.Loosely(t, repo, should.Equal(defaultRepository))
		})
	})
}
