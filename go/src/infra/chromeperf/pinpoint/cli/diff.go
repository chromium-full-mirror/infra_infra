package cli

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/maruel/subcommands"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/protobuf/testing/protocmp"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/grpc/prpc"

	"go.chromium.org/infra/chromeperf/pinpoint"
	pppb "go.chromium.org/infra/chromeperf/pinpoint/proto"
)

const (
	TASK    = "task"
	BOT     = "bot"
	BUILD   = "build"
	ISOLATE = "isolate"
)

// customDiffReporter is a simple custom reporter that only records differences
// detected during comparison.
type customDiffReporter struct {
	path  cmp.Path
	diffs []customDiff
}

type customDiff struct {
	name       string
	aVal, bVal reflect.Value
}

func (r *customDiffReporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func valStr(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	return fmt.Sprintf("%+v", v)
}

func (r *customDiffReporter) Report(rs cmp.Result) {
	if !rs.Equal() {
		vx, vy := r.path.Last().Values()
		r.diffs = append(r.diffs, customDiff{r.path.GoString(), vx, vy})
	}
}

func (r *customDiffReporter) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

func (r *customDiffReporter) String() string {
	lines := []string{}
	for _, d := range r.diffs {
		lines = append(lines, fmt.Sprintf("%s\n-:%v\n+:%v\n", d.name, valStr(d.aVal), valStr(d.bVal)))
	}
	return strings.Join(lines, "\n")
}

type diffReportLine struct {
	label, message string
}

func (dc *diffCmd) newBuildsClient(ctx context.Context) (bbpb.BuildsClient, error) {
	httpClient, err := dc.baseCommandRun.httpClient(ctx)
	if err != nil {
		return nil, err
	}
	rpcOpts := prpc.DefaultOptions()
	buildsClient := bbpb.NewBuildsPRPCClient(&prpc.Client{
		C:       httpClient,
		Host:    "cr-buildbucket.appspot.com:443",
		Options: rpcOpts,
	})

	return buildsClient, nil
}

type diffCmd struct {
	baseCommandRun
	params Param
	name   string
}

func (dc *diffCmd) getJob(ctx context.Context, name string) (*pppb.Job, error) {
	c, err := dc.pinpointClient(ctx)
	if err != nil {
		return nil, err
	}

	req := &pppb.GetJobRequest{Name: pinpoint.LegacyJobName(name)}
	j, err := c.GetJob(ctx, req)
	if err != nil {
		return nil, errors.Annotate(err, "failed during GetJob").Err()
	}
	return j, nil
}

func diffJob(job *pppb.Job) (map[string]customDiffReporter, error) {
	e := job.GetJobSpec().GetExperiment()
	if e == nil {
		return nil, errors.Reason("unsupported job kind: %+v", job).Err()
	}
	comps := []struct {
		label       string
		left, right any
	}{
		{"Commit", e.GetBaseCommit(), e.GetExperimentCommit()},
		{"Patch", e.GetBasePatch(), e.GetExperimentPatch()},
	}
	ret := map[string]customDiffReporter{}
	for _, c := range comps {
		r := customDiffReporter{}
		cmp.Equal(c.left, c.right, protocmp.Transform(), protocmp.IgnoreEmptyMessages(), cmp.Reporter(&r))
		ret[c.label] = r
	}
	return ret, nil
}

type executionSummary struct {
	task, bot, build string
}

func botsAndBuildsForAttempts(attempts []*pppb.Attempt) ([]string, []string) {
	// TODO: Expand the diffs to include data from execution *tasks* as well.
	ret := []executionSummary{}
	bots := map[string]any{}
	builds := map[string]any{}

	for _, a := range attempts {
		for _, e := range a.GetExecutions() {
			ex := executionSummary{}
			for _, d := range e.GetDetails() {
				if d.Key == TASK {
					ex.task = d.GetValue()
				} else if d.Key == BOT {
					ex.bot = d.GetValue()
					bots[ex.bot] = nil
				} else if d.Key == BUILD {
					ex.build = d.GetValue()
					builds[ex.build] = nil
				}
			}
			ret = append(ret, ex)
		}
	}
	botNames := []string{}
	for k := range bots {
		botNames = append(botNames, k)
	}
	buildIDs := []string{}
	for k := range builds {
		buildIDs = append(buildIDs, k)
	}

	sort.Strings(botNames)
	sort.Strings(buildIDs)
	return botNames, buildIDs
}

func getBuildIDs(job *pppb.Job) (string, string, error) {
	results := job.GetAbExperimentResults()
	aResult := results.GetAChangeResult()
	bResult := results.GetBChangeResult()

	aAttempts := aResult.GetAttempts()
	bAttempts := bResult.GetAttempts()

	aBots, aBuilds := botsAndBuildsForAttempts(aAttempts)
	bBots, bBuilds := botsAndBuildsForAttempts(bAttempts)

	r := &customDiffReporter{}
	cmp.Equal(aBots, bBots, cmp.Reporter(r))
	if r.String() != "" {
		fmt.Printf("Bot id differences:\n%s\n", r.String())
	}

	// Now go get the build protos so we can compare more detailed info about inputs and outputs of the builds.
	if len(aBuilds) != 1 || len(bBuilds) != 1 {
		return "", "", errors.New("unexepceted number of builds for a/b arm.")
	}
	return aBuilds[0], bBuilds[0], nil
}

func diffBuilds(aBuild, bBuild *bbpb.Build) map[string]customDiffReporter {
	ret := map[string]customDiffReporter{}

	r := customDiffReporter{}
	cmp.Equal(aBuild.GetTags(), bBuild.GetTags(), protocmp.Transform(), protocmp.IgnoreEmptyMessages(), cmp.Reporter(&r))
	if len(r.diffs) > 0 {
		ret["Build.Tags"] = r
	}

	r = customDiffReporter{}
	cmp.Equal(aBuild.GetInput(), bBuild.GetInput(), protocmp.Transform(), protocmp.IgnoreEmptyMessages(), cmp.Reporter(&r))
	if len(r.diffs) > 0 {
		ret["Build.Input"] = r
	}

	// Clear the log file links, since those differences are just noise.
	if aBuild.GetOutput() != nil {
		aBuild.GetOutput().Logs = nil
	}
	if bBuild.GetOutput() != nil {
		bBuild.GetOutput().Logs = nil
	}

	r = customDiffReporter{}
	cmp.Equal(aBuild.GetOutput(), bBuild.GetOutput(), protocmp.Transform(), protocmp.IgnoreEmptyMessages(), cmp.Reporter(&r))
	if len(r.diffs) > 0 {
		ret["Build.Output"] = r
	}

	return ret
}

func (dc *diffCmd) getBuild(ctx context.Context, id string) (*bbpb.Build, error) {
	ID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	bb, err := dc.newBuildsClient(ctx)
	if err != nil {
		return nil, err
	}
	req := &bbpb.GetBuildRequest{
		Id: ID,
		Mask: &bbpb.BuildMask{
			Fields: &field_mask.FieldMask{
				Paths: []string{"tags", "input", "output"},
			},
		},
	}

	aBuild, err := bb.GetBuild(ctx, req)
	if err != nil {
		return nil, err
	}
	return aBuild, nil
}

func (dc *diffCmd) Run(ctx context.Context, a subcommands.Application, args []string) error {
	job, err := dc.getJob(ctx, dc.name)
	if err != nil {
		return err
	}
	e := job.GetJobSpec().GetExperiment()
	if e == nil {
		return errors.Reason("unsupported job kind: %+v", job).Err()
	}
	diffs, err := diffJob(job)
	if err != nil {
		return err
	}
	for label, d := range diffs {
		fmt.Printf("%s diff results (-base, +experiment):\n%s\n", label, d.String())
	}

	aID, bID, err := getBuildIDs(job)
	if err != nil {
		return err
	}

	aBuild, err := dc.getBuild(ctx, aID)
	if err != nil {
		return err
	}

	bBuild, err := dc.getBuild(ctx, bID)
	if err != nil {
		return err
	}

	diffs = diffBuilds(aBuild, bBuild)
	for label, d := range diffs {
		fmt.Printf("%s diff results (-base, +experiment):\n%s\n", label, d.String())
	}

	return nil
}

func (dc *diffCmd) RegisterFlags(p Param) {
	dc.baseCommandRun.RegisterFlags(p)
	dc.Flags.StringVar(&dc.name, "name", "", text.Doc(`
		Required; the name of the job to diff.
		Example: "-name=XXXXXXXXXXXXXX"
	`))
}

func cmdDiff(p Param) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "diff",
		ShortDesc: "Show differences between job specifications for each arm of an experiment.",
		CommandRun: wrapCommand(p, func() pinpointCommand {
			return &diffCmd{params: p}
		}),
	}
}
