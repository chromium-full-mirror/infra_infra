package testplan_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/plan"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/git"
	"go.chromium.org/infra/cros/internal/testplan"
)

func TestFindRelevantPlans(t *testing.T) {
	ctx := context.Background()

	tz, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	changeRevs := []*gerrit.ChangeRev{
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 123,
			},
			Project:       "chromium/testprojectA",
			Branch:        "main",
			Ref:           "refs/changes/23/123/5",
			Files:         []string{"go/src/infra/cros/internal/testplan/testdata/good_dirmd/DIR_METADATA"},
			ChangeCreated: timestamppb.New(time.Date(2022, time.December, 20, 9, 42, 30, 0, tz)),
		},
	}

	// The change for testprojectA should be cherry-picked.
	git.CommandRunnerImpl = &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{
					"git", "clone",
					"https://chromium.googlesource.com/chromium/testprojectA", "good_dirmd",
					"--no-tags", "--branch", "main",
					"--shallow-since", "Dec 10 2022",
				},
			},
			{
				ExpectedCmd: []string{
					"git", "fetch",
					"https://chromium.googlesource.com/chromium/testprojectA", "refs/changes/23/123/5",
					"--no-tags",
					"--shallow-since", "Dec 10 2022",
				},
			},
			{
				ExpectedCmd: []string{"git", "merge", "FETCH_HEAD"},
			},
		},
	}

	// Set workdirFn so the CommandRunners can know where commands are run,
	// and the DIR_METADATA in testdata is read. Don't cleanup the testdata.
	workdirFn := func() (string, func() error, error) {
		cleanup := func() error { return nil }
		return "./testdata/good_dirmd", cleanup, nil
	}

	relevantPlans, err := testplan.FindRelevantPlans(
		ctx, changeRevs, workdirFn, time.Hour*24*10,
	)
	if err != nil {
		t.Fatalf("testplan.FindRelevantPlans(%q) failed: %s", changeRevs, err)
	}

	expectedPlan := &plan.SourceTestPlan{
		TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
			{
				Host:    "chromium.googlesource.com",
				Project: "repo1",
				Path:    "test1.star",
			},
			{
				Host:    "chromium.googlesource.com",
				Project: "repo2",
				Path:    "test2.star",
			},
		},
	}

	if len(relevantPlans) != 1 {
		t.Fatalf("testplan.FindRelevantPlans(%q) returned %d plans, expected 1", changeRevs, len(relevantPlans))
	}

	if diff := cmp.Diff(expectedPlan, relevantPlans[0], protocmp.Transform()); diff != "" {
		t.Errorf(
			"testplan.FindRelevantPlans(%q) returned unexpected diff on plan (-want +got):\n%s",
			changeRevs, diff,
		)
	}
}
