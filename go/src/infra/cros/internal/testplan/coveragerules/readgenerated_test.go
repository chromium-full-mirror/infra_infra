// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package coveragerules_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	testpb "go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/git"
	"go.chromium.org/infra/cros/internal/testplan/coveragerules"
)

func TestReadGenerated(t *testing.T) {
	ctx := context.Background()
	git.CommandRunnerImpl = &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{"git", "ls-remote", "--get-url"},
				Stdout:      "https://chromium.googlesource.com/chromiumos/project1\n",
				ExpectedDir: "../testdata/generated_rules",
			},
			{
				ExpectedCmd: []string{"git", "ls-files", "--full-name", "--error-unmatch", filepath.FromSlash("../testdata/generated_rules/test.coveragerules.jsonproto")},
				Stdout:      "generated_rules/test.coveragerules.jsonproto\n",
				ExpectedDir: "../testdata/generated_rules",
			},
		},
	}

	rows, err := coveragerules.ReadGenerated(ctx, "../testdata/generated_rules")
	if err != nil {
		t.Fatal(err)
	}

	expectedRows := []*testpb.CoverageRuleBqRow{
		{
			Host:    "chromium.googlesource.com",
			Project: "chromiumos/project1",
			Path:    "generated_rules/test.coveragerules.jsonproto",
			CoverageRule: &testpb.CoverageRule{
				TestSuites: []*testpb.TestSuite{
					{
						Name: "testsuite1",
						Spec: &testpb.TestSuite_TestCaseTagCriteria_{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags:        []string{"group:mainline", "group:testgroupA"},
								TagExcludes: []string{"informational"},
							},
						},
					},
				},
			},
		},
		{
			Host:    "chromium.googlesource.com",
			Project: "chromiumos/project1",
			Path:    "generated_rules/test.coveragerules.jsonproto",
			CoverageRule: &testpb.CoverageRule{
				TestSuites: []*testpb.TestSuite{
					{
						Name: "testsuite2",
						Spec: &testpb.TestSuite_TestCaseTagCriteria_{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags:        []string{"group:mainline", "group:testgroupB"},
								TagExcludes: []string{"informational"},
							},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(expectedRows, rows, protocmp.Transform()); diff != "" {
		t.Errorf("ReadGenerated diff (-want +got):\n%s", diff)
	}
}

func TestReadGeneratedErrors(t *testing.T) {
	ctx := context.Background()
	git.CommandRunnerImpl = &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{"git", "ls-remote", "--get-url"},
				Stdout:      "https://chromium.googlesource.com/chromiumos/project1\n",
			},
			{
				ExpectedCmd: []string{"git", "ls-files", "--full-name", "--error-unmatch", filepath.FromSlash("../testdata/bad_generated_rules/test.txt")},
				Stdout:      "generated_rules/test.coveragerules.jsonproto\n",
			},
		},
	}

	_, err := coveragerules.ReadGenerated(ctx, "../testdata/bad_generated_rules")

	assert.ErrorContains(t, err,
		fmt.Sprintf("failed to unmarshal JSON in %s", filepath.FromSlash("../testdata/bad_generated_rules/test.txt")))
}

func TestGenerateCoverageRuleBqRowSchema(t *testing.T) {
	_, err := coveragerules.GenerateCoverageRuleBqRowSchema()
	// Just assert error is nil, it is difficult to validate the entire schema.
	assert.NilError(t, err)
}
