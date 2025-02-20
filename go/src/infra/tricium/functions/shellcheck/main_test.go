// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows
// +build !windows

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	findingspb "go.chromium.org/luci/common/proto/findings"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/tricium/functions/shellcheck/runner"
)

const testInputDir = "testdata"

func TestRun(t *testing.T) {
	r := &runner.Runner{
		Path:   findShellCheckBin(),
		Dir:    testInputDir,
		Logger: runnerLogger,
		Enable: "require-variable-braces",
	}
	version, err := r.Version()
	if err != nil {
		t.Skip("no valid shellcheck bin found; skipping test")
	}
	if !strings.HasPrefix(version, "0.8.") {
		t.Skipf("got shellcheck version %q want 0.8.x; skipping test", version)
	}

	outputDir := t.TempDir()

	run(r, testInputDir, outputDir, "*.other,*.sh")

	findingsData, err := os.ReadFile(filepath.Join(outputDir, "findings.out"))
	assert.NoErr(t, err)

	findings := &findingspb.Findings{}
	assert.NoErr(t, proto.Unmarshal(findingsData, findings))

	slices.SortFunc(findings.Findings, func(a, b *findingspb.Finding) int {
		return strings.Compare(a.Message, b.Message)
	})

	assert.That(t, findings, should.Match(&findingspb.Findings{
		Findings: []*findingspb.Finding{
			{
				Category:      "shellcheck",
				Message:       "error code: SC1037\n\nBraces are required for positionals over 9, e.g. ${10}.\n\nhttps://github.com/koalaman/shellcheck/wiki/SC1037",
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
				Location: &findingspb.Location{
					FilePath: "bad.sh",
					Range: &findingspb.Location_Range{
						StartLine:   6,
						EndLine:     6,
						StartColumn: 6,
						EndColumn:   6,
					},
				},
			},
			{
				Category:      "shellcheck",
				Message:       "error code: SC2034\n\nFLAGS_flag appears unused. Verify use (or export if used externally).\n\nhttps://github.com/koalaman/shellcheck/wiki/SC2034",
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "bad.sh",
					Range: &findingspb.Location_Range{
						StartLine:   3,
						EndLine:     3,
						StartColumn: 15,
						EndColumn:   19,
					},
				},
			},
			{
				Category:      "shellcheck",
				Message:       "error code: SC2034\n\nunused appears unused. Verify use (or export if used externally).\n\nhttps://github.com/koalaman/shellcheck/wiki/SC2034",
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
				Location: &findingspb.Location{
					FilePath: "bad.sh",
					Range: &findingspb.Location_Range{
						StartLine:   5,
						EndLine:     5,
						StartColumn: 1,
						EndColumn:   7,
					},
				},
			},
			{
				Category:      "shellcheck",
				Message:       "error code: SC2086\n\nDouble quote to prevent globbing and word splitting.\n\nhttps://github.com/koalaman/shellcheck/wiki/SC2086",
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_INFO,
				Location: &findingspb.Location{
					FilePath: "bad.sh",
					Range: &findingspb.Location_Range{
						StartLine:   8,
						EndLine:     8,
						StartColumn: 5,
						EndColumn:   9,
					},
				},
			},
			{
				Category:      "shellcheck",
				Message:       "error code: SC2250\n\nPrefer putting braces around variable references even when not strictly required.\n\nhttps://github.com/koalaman/shellcheck/wiki/SC2250",
				SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_INFO,
				Location: &findingspb.Location{
					FilePath: "bad.sh",
					Range: &findingspb.Location_Range{
						StartLine:   8,
						EndLine:     8,
						StartColumn: 5,
						EndColumn:   9,
					},
				},
			},
		},
	}))
}
