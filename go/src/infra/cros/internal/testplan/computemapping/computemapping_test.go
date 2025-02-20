package computemapping_test

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/plan"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/git"
	"go.chromium.org/infra/cros/internal/testplan/computemapping"
	"go.chromium.org/infra/tools/dirmd"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
	"go.chromium.org/infra/tools/dirmd/proto/chromeos"
)

func TestComputeProjectMappingInfos(t *testing.T) {
	ctx := context.Background()

	tz, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	// Two changes from testprojectA on branch "main", one from testprojectA on
	// branch "otherbranch", one from testprojectB on branch "main".
	changeRevs := []*gerrit.ChangeRev{
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 456,
			},
			Project:       "chromium/testprojectA",
			Branch:        "main",
			Ref:           "refs/changes/45/456/2",
			Files:         []string{"DIR_METADATA"},
			ChangeCreated: timestamppb.New(time.Date(2022, time.December, 20, 9, 42, 30, 0, tz)),
		},
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 789,
			},
			Project:       "chromium/testprojectB",
			Branch:        "main",
			Ref:           "refs/changes/78/789/5",
			Files:         []string{"test.c", "test.h", "a/b/DIR_METADATA"},
			ChangeCreated: timestamppb.New(time.Date(2022, time.December, 15, 12, 23, 15, 0, tz)),
		},
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 123,
			},
			Project:       "chromium/testprojectA",
			Branch:        "main",
			Ref:           "refs/changes/23/123/5",
			Files:         []string{"a/b/test1.txt", "a/b/test2.txt"},
			ChangeCreated: timestamppb.New(time.Date(2022, time.December, 21, 9, 42, 30, 0, tz)),
		},
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 1011,
			},
			Project:       "chromium/testprojectA",
			Branch:        "otherbranch",
			Ref:           "refs/changes/10/1011/1",
			Files:         []string{"branchfile.txt"},
			ChangeCreated: timestamppb.New(time.Date(2022, time.December, 19, 9, 42, 30, 0, tz)),
		},
	}

	// Changes should be merged, sorted by project and branch. Changes on the
	// same branch will be merged in the same order they are passed to
	// computemapping.ProjectInfos.
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
					"https://chromium.googlesource.com/chromium/testprojectA", "refs/changes/45/456/2",
					"--no-tags",
					"--shallow-since", "Dec 10 2022",
				},
			},
			{
				ExpectedCmd: []string{"git", "merge", "FETCH_HEAD"},
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
			{
				ExpectedCmd: []string{
					"git", "clone",
					"https://chromium.googlesource.com/chromium/testprojectA", "good_dirmd",
					"--no-tags", "--branch", "otherbranch", "--depth", "1",
				},
			},
			{
				ExpectedCmd: []string{
					"git", "clone",
					"https://chromium.googlesource.com/chromium/testprojectB", "good_dirmd",
					"--no-tags", "--branch", "main",
					"--shallow-since", "Dec 05 2022",
				},
			},
			{
				ExpectedCmd: []string{
					"git", "fetch",
					"https://chromium.googlesource.com/chromium/testprojectB", "refs/changes/78/789/5",
					"--no-tags",
					"--shallow-since", "Dec 05 2022",
				},
			},
			{
				ExpectedCmd: []string{"git", "merge", "FETCH_HEAD"},
			},
		},
	}

	// Set workdirFn so the CommandRunners can know where commands are run,
	// and the DIR_METADATA in testdata/good_dirmd is read. Don't cleanup the testdata.
	workdirFn := func() (string, func() error, error) {
		cleanup := func() error { return nil }
		return "../testdata/good_dirmd", cleanup, nil
	}

	projectMappingInfos, err := computemapping.ProjectInfos(ctx, changeRevs, workdirFn, time.Hour*24*10)
	if err != nil {
		t.Fatalf("computeProjectMappingInfos(%v) failed: %s", changeRevs, err)
	}

	// Both projects read the same DIR_METADATA, so have the same expected
	// Mapping.
	expectedMapping := &dirmd.Mapping{
		Dirs: map[string]*dirmdpb.Metadata{
			"go/src/infra/cros/internal/testplan/testdata/good_dirmd": {
				Chromeos: &chromeos.ChromeOS{
					Cq: &chromeos.ChromeOS_CQ{
						SourceTestPlans: []*plan.SourceTestPlan{
							{
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
							},
						},
					},
				},
			},
		},
	}
	expectedAffectedFiles := [][]string{
		{"a/b/test1.txt", "a/b/test2.txt", "DIR_METADATA"},
		{"branchfile.txt"},
		{"test.c", "test.h", "a/b/DIR_METADATA"},
	}

	for i, pmi := range projectMappingInfos {
		if diff := cmp.Diff(
			expectedMapping.Dirs, pmi.Mapping.Dirs, protocmp.Transform(),
		); diff != "" {
			t.Errorf(
				"computeProjectMappingInfos returned unexpected diff in mappings at index %d (-want +got):\n%s",
				i, diff,
			)
		}

		sort.Strings(expectedAffectedFiles[i])
		sort.Strings(pmi.AffectedFiles)

		if !reflect.DeepEqual(expectedAffectedFiles[i], pmi.AffectedFiles) {
			t.Errorf(
				"computeProjectMappingInfos returned affectedFiles %v, expected %v",
				pmi.AffectedFiles,
				expectedAffectedFiles[i],
			)
		}
	}
}

func TestComputeProjectMappingInfosBadDirmd(t *testing.T) {
	ctx := context.Background()

	// One change from testprojectA.
	changeRevs := []*gerrit.ChangeRev{
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 123,
			},
			Project: "chromium/testprojectA",
			Branch:  "main",
			Ref:     "refs/changes/23/123/5",
			Files:   []string{"a/b/test1.txt", "a/b/test2.txt"},
		},
	}

	// The change for testprojectA should be merged.
	git.CommandRunnerImpl = &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{
					"git", "clone",
					"https://chromium.googlesource.com/chromium/testprojectA", "bad_dirmd",
					"--no-tags", "--branch", "main", "--depth", "1",
				},
			},
		},
	}

	// Set workdirFn so the CommandRunners can know where commands are run,
	// and the DIR_METADATA in testdata/bad_dirmd is read. Don't cleanup the testdata.
	workdirFn := func() (string, func() error, error) {
		cleanup := func() error { return nil }
		return "../testdata/bad_dirmd", cleanup, nil
	}

	_, err := computemapping.ProjectInfos(ctx, changeRevs, workdirFn, time.Hour*24*10)
	if err == nil {
		t.Fatalf("expected error from computeProjectMappingInfos(%v)", changeRevs)
	}

	assert.ErrorContains(t, err, "failed to read DIR_METADATA")
}

func TestComputeMergeFailsCherryPickSucceeds(t *testing.T) {
	ctx := context.Background()

	tz, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	// One change from testprojectA.
	changeRevs := []*gerrit.ChangeRev{
		{
			ChangeRevKey: gerrit.ChangeRevKey{
				Host:      "chromium-review.googlesource.com",
				ChangeNum: 123,
			},
			Project:       "chromium/testprojectA",
			Branch:        "main",
			Ref:           "refs/changes/23/123/5",
			Files:         []string{"a/b/test1.txt", "a/b/test2.txt", "DIR_METADATA"},
			ChangeCreated: timestamppb.New(time.Date(2022, time.December, 20, 9, 42, 30, 0, tz)),
		},
	}

	// There merge of the change fails, then the cherry-pick succeeds.
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
					"--shallow-since", "Dec 10 2022"},
			},
			{
				ExpectedCmd: []string{"git", "merge", "FETCH_HEAD"},
				FailCommand: true,
				FailError:   errors.New("conflict on file test.txt"),
			},
			{
				ExpectedCmd: []string{"git", "merge", "--abort"},
			},
			{
				ExpectedCmd: []string{"git", "cherry-pick", "FETCH_HEAD"},
			},
		},
	}

	// Set workdirFn so the CommandRunners can know where commands are run,
	// and the DIR_METADATA in testdata/good_dirmd is read. Don't cleanup the testdata.
	workdirFn := func() (string, func() error, error) {
		cleanup := func() error { return nil }
		return "../testdata/good_dirmd", cleanup, nil
	}

	projectMappingInfos, err := computemapping.ProjectInfos(ctx, changeRevs, workdirFn, time.Hour*24*10)
	if err != nil {
		t.Fatalf("computeProjectMappingInfos(%v) failed: %s", changeRevs, err)
	}

	expectedMapping := &dirmd.Mapping{
		Dirs: map[string]*dirmdpb.Metadata{
			"go/src/infra/cros/internal/testplan/testdata/good_dirmd": {
				Chromeos: &chromeos.ChromeOS{
					Cq: &chromeos.ChromeOS_CQ{
						SourceTestPlans: []*plan.SourceTestPlan{
							{
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
							},
						},
					},
				},
			},
		},
	}
	expectedAffectedFiles := [][]string{
		{"a/b/test1.txt", "a/b/test2.txt", "DIR_METADATA"},
	}

	for i, pmi := range projectMappingInfos {
		if diff := cmp.Diff(
			expectedMapping.Dirs, pmi.Mapping.Dirs, protocmp.Transform(),
		); diff != "" {
			t.Errorf(
				"computeProjectMappingInfos returned unexpected diff in mappings at index %d (-want +got):\n%s",
				i, diff,
			)
		}

		sort.Strings(expectedAffectedFiles[i])
		sort.Strings(pmi.AffectedFiles)

		if !reflect.DeepEqual(expectedAffectedFiles[i], pmi.AffectedFiles) {
			t.Errorf(
				"computeProjectMappingInfos returned affectedFiles %v, expected %v",
				pmi.AffectedFiles,
				expectedAffectedFiles[i],
			)
		}
	}
}

func TestComputeProjectMappingInfosShallowCloneFails(t *testing.T) {
	{
		ctx := context.Background()

		tz, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			t.Fatal(err)
		}

		changeRevs := []*gerrit.ChangeRev{
			{
				ChangeRevKey: gerrit.ChangeRevKey{
					Host:      "chromium-review.googlesource.com",
					ChangeNum: 456,
				},
				Project:       "chromium/testprojectA",
				Branch:        "main",
				Ref:           "refs/changes/45/456/2",
				Files:         []string{"DIR_METADATA"},
				ChangeCreated: timestamppb.New(time.Date(2022, time.December, 20, 9, 42, 30, 0, tz)),
			},
		}

		git.CommandRunnerImpl = &cmd.FakeCommandRunnerMulti{
			CommandRunners: []cmd.FakeCommandRunner{
				// The first clone with --shallow-since fails. The function
				// should fall back to doing a full clone, and also do a full
				// fetch on the ref.
				{
					ExpectedCmd: []string{
						"git", "clone",
						"https://chromium.googlesource.com/chromium/testprojectA", "good_dirmd",
						"--no-tags", "--branch", "main",
						"--shallow-since", "Dec 10 2022",
					},
					FailCommand: true,
					FailError:   errors.New("fatal: expected 'packfile'"),
				},
				{
					ExpectedCmd: []string{
						"git", "clone",
						"https://chromium.googlesource.com/chromium/testprojectA", "good_dirmd",
						"--no-tags", "--branch", "main",
					},
				},
				{
					ExpectedCmd: []string{
						"git", "fetch",
						"https://chromium.googlesource.com/chromium/testprojectA", "refs/changes/45/456/2",
						"--no-tags",
					},
				},
				{
					ExpectedCmd: []string{"git", "merge", "FETCH_HEAD"},
				},
			},
		}

		// Set workdirFn so the CommandRunners can know where commands are run,
		// and the DIR_METADATA in testdata/good_dirmd is read. Don't cleanup the testdata.
		workdirFn := func() (string, func() error, error) {
			cleanup := func() error { return nil }
			return "../testdata/good_dirmd", cleanup, nil
		}

		projectMappingInfos, err := computemapping.ProjectInfos(ctx, changeRevs, workdirFn, time.Hour*24*10)
		if err != nil {
			t.Fatalf("computeProjectMappingInfos(%v) failed: %s", changeRevs, err)
		}

		expectedMapping := &dirmd.Mapping{
			Dirs: map[string]*dirmdpb.Metadata{
				"go/src/infra/cros/internal/testplan/testdata/good_dirmd": {
					Chromeos: &chromeos.ChromeOS{
						Cq: &chromeos.ChromeOS_CQ{
							SourceTestPlans: []*plan.SourceTestPlan{
								{
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
								},
							},
						},
					},
				},
			},
		}
		expectedAffectedFiles := [][]string{
			{"DIR_METADATA"},
		}

		for i, pmi := range projectMappingInfos {
			if diff := cmp.Diff(
				expectedMapping.Dirs, pmi.Mapping.Dirs, protocmp.Transform(),
			); diff != "" {
				t.Errorf(
					"computeProjectMappingInfos returned unexpected diff in mappings at index %d (-want +got):\n%s",
					i, diff,
				)
			}

			sort.Strings(expectedAffectedFiles[i])
			sort.Strings(pmi.AffectedFiles)

			if !reflect.DeepEqual(expectedAffectedFiles[i], pmi.AffectedFiles) {
				t.Errorf(
					"computeProjectMappingInfos returned affectedFiles %v, expected %v",
					pmi.AffectedFiles,
					expectedAffectedFiles[i],
				)
			}
		}
	}
}
