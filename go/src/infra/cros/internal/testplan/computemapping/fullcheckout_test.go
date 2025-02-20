// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package computemapping_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/plan"
	"go.chromium.org/luci/common/clock/testclock"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/git"
	"go.chromium.org/infra/cros/internal/repo"
	"go.chromium.org/infra/cros/internal/testplan/computemapping"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
	"go.chromium.org/infra/tools/dirmd/proto/chromeos"
)

func TestToDirBQRows(t *testing.T) {
	ctx := context.Background()
	ctx, _ = testclock.UseTime(ctx, testclock.TestRecentTimeUTC)
	manifest := &repo.Manifest{
		Projects: []repo.Project{
			{
				Path:       "good_dirmd",
				Name:       "testproj",
				Revision:   "refs/heads/main",
				RemoteName: "cros",
			},
		},
		Remotes: []repo.Remote{
			{
				Name:  "cros",
				Fetch: "https://chromium.googlesource.com",
			},
		},
	}

	git.CommandRunnerImpl = &cmd.FakeCommandRunner{
		ExpectedCmd: []string{"git", "rev-parse", "HEAD"},
		Stdout:      "123",
	}

	rows, err := computemapping.ToDirBQRows(ctx, "../testdata", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	expectedRows := []*dirmdpb.DirBQRow{
		{
			PartitionTime: timestamppb.New(testclock.TestRecentTimeUTC),
			Source: &dirmdpb.Source{
				GitHost:  "https://chromium.googlesource.com",
				RootRepo: "testproj",
				Ref:      "refs/heads/main",
				Revision: "123",
			},
			Dir: "go/src/infra/cros/internal/testplan/testdata/good_dirmd",
			TeamSpecificMetadata: &dirmdpb.TeamSpecific{
				Chromeos: &chromeos.ChromeOS{
					Cq: &chromeos.ChromeOS_CQ{
						SourceTestPlans: []*plan.SourceTestPlan{
							{
								TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
									{
										Host: "chromium.googlesource.com", Project: "repo1", Path: "test1.star",
									},
									{
										Host: "chromium.googlesource.com", Project: "repo2", Path: "test2.star",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(
		expectedRows, rows, protocmp.Transform(),
	); diff != "" {
		t.Errorf("unexpected diff in mapping (-want +got):\n%s", diff)
	}
}

func TestToDirBQRowsBadDirmd(t *testing.T) {
	ctx := context.Background()
	manifest := &repo.Manifest{
		Projects: []repo.Project{
			{
				Path:       "bad_dirmd",
				Name:       "testproj",
				Revision:   "refs/heads/main",
				RemoteName: "cros",
			},
		},
	}

	_, err := computemapping.ToDirBQRows(ctx, "../testdata", manifest)
	if err == nil {
		t.Error("expected error from ToDirBQRows")
	}
}

func TestToDirBQRowsSkipsNotDefault(t *testing.T) {
	ctx := context.Background()
	manifest := &repo.Manifest{
		Projects: []repo.Project{
			{
				Name:   "notdefaultproj",
				Groups: "notdefault,othergroup",
			},
		},
	}

	rows, err := computemapping.ToDirBQRows(ctx, "../testdata", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 0 {
		t.Fatalf("expected 0 CommitAndMappings, got %d", len(rows))
	}
}

func TestToDirBQRowsSkipsChromium(t *testing.T) {
	ctx := context.Background()
	manifest := &repo.Manifest{
		Projects: []repo.Project{
			{
				Name: "chromiumproj",
				Path: "src/chromium/a/b",
			},
		},
	}

	rows, err := computemapping.ToDirBQRows(ctx, "../testdata", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 0 {
		t.Fatalf("expected 0 CommitAndMappings, got %d", len(rows))
	}
}
