package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/types/known/structpb"

	bbpb "go.chromium.org/luci/buildbucket/proto"

	pppb "go.chromium.org/infra/chromeperf/pinpoint/proto"
)

func TestDiffJob(t *testing.T) {
	tests := []struct {
		job     *pppb.Job
		want    map[string]customDiffReporter
		wantErr bool
	}{
		{&pppb.Job{}, nil, true},
		{&pppb.Job{
			JobSpec: &pppb.JobSpec{
				JobKind: &pppb.JobSpec_Bisection{},
			},
		}, nil, true},
		{&pppb.Job{
			JobSpec: &pppb.JobSpec{
				JobKind: &pppb.JobSpec_Experiment{
					Experiment: &pppb.Experiment{},
				},
			},
		}, map[string]customDiffReporter{
			"Commit": {},
			"Patch":  {},
		}, false},
		{&pppb.Job{
			JobSpec: &pppb.JobSpec{
				JobKind: &pppb.JobSpec_Experiment{
					Experiment: &pppb.Experiment{
						BaseCommit: &pppb.GitilesCommit{
							Project: "project",
						},
						ExperimentCommit: &pppb.GitilesCommit{
							Project: "project",
						},
						BasePatch: &pppb.GerritChange{
							Project: "project",
						},
						ExperimentPatch: &pppb.GerritChange{
							Project:  "project",
							Change:   123,
							Patchset: 4,
						},
					},
				},
			},
		}, map[string]customDiffReporter{
			"Commit": {},
			"Patch":  {},
		}, false},
	}
	for i, test := range tests {
		got, err := diffJob(test.job)
		if test.wantErr && err == nil {
			t.Errorf("%d: unexpected nil error", i)
		}
		if !test.wantErr && err != nil {
			t.Errorf("%d: unexpected error: %#v", i, err)
		}
		if diff := cmp.Diff(got, test.want, cmp.AllowUnexported(customDiffReporter{}), cmp.Comparer(func(a, b customDiffReporter) bool { return true })); diff != "" {
			t.Errorf("%d: got %#v, want %#v", i, got, test.want)
		}
	}
}

func newStruct(in map[string]any) *structpb.Struct {
	ret, err := structpb.NewStruct(in)
	if err != nil {
		panic(err)
	}
	return ret
}

func TestDiffBuilds(t *testing.T) {
	tests := []struct {
		a, b *bbpb.Build
		want map[string]customDiffReporter
	}{
		{&bbpb.Build{}, &bbpb.Build{}, map[string]customDiffReporter{}},
		{
			&bbpb.Build{
				Tags: []*bbpb.StringPair{
					{Key: "a", Value: "b"},
				},
			},
			&bbpb.Build{},
			map[string]customDiffReporter{
				"Build.Tags": {},
			},
		},
		{
			&bbpb.Build{
				Input: &bbpb.Build_Input{},
			},
			&bbpb.Build{
				Input: &bbpb.Build_Input{
					GerritChanges: []*bbpb.GerritChange{
						{
							Host:     "chromium-review.googlesource.com",
							Project:  "chromium/src",
							Change:   3261007,
							Patchset: 3,
						},
					},
				},
			},
			map[string]customDiffReporter{
				"Build.Input": {},
			},
		},
		{
			&bbpb.Build{
				Output: &bbpb.Build_Output{},
			},
			&bbpb.Build{
				Output: &bbpb.Build_Output{
					Properties: newStruct(map[string]any{
						"swarm_hashes_refs/heads/main(at){#938071}_with_patch": "0c8b5db3bf591801b94f62aed9ea5f2e1e24b77b1533752fd360e9220fd8931d/487",
					}),
				},
			},
			map[string]customDiffReporter{
				"Build.Output": {},
			},
		},
	}
	for i, test := range tests {
		got := diffBuilds(test.a, test.b)
		if diff := cmp.Diff(got, test.want, cmp.AllowUnexported(customDiffReporter{}), cmp.Comparer(func(a, b customDiffReporter) bool { return true })); diff != "" {
			t.Errorf("%d: got %#v, want %#v\ndiff: %s\n", i, got, test.want, diff)
		}
	}
}
