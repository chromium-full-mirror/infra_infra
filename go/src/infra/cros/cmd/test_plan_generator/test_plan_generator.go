// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/maruel/subcommands"

	"go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	bbproto "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/generator"
	igerrit "go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/manifestutil"
	"go.chromium.org/infra/cros/internal/shared"
)

const (
	boardPriorityConfigPathDefault    = "board_config/generated/board_priority.binaryproto"
	sourceTreeTestConfigPathDefault   = "board_config/generated/source_tree_test_config.binaryproto"
	targetTestRequirementsPathDefault = "board_config/generated/target_test_requirements.binaryproto"
	sourceGitilesRepoDefault          = "chromeos/config-internal"
	sourceGitilesBranchDefault        = "main"
)

var (
	unmarshaler = jsonpb.Unmarshaler{AllowUnknownFields: true}
)

type getTestPlanRun struct {
	subcommands.CommandRunBase
	authFlags                  authcli.Flags
	inputJSON                  string
	outputJSON                 string
	inputBinaryPb              string
	outputBinaryPb             string
	sourceGitilesRepo          string
	sourceGitilesBranch        string
	boardPriorityConfigPath    string
	sourceTreeTestConfigPath   string
	targetTestRequirementsPath string
	manifestFile               string
	localConfigDir             string
	targetTestReqsRepo         string
	targetTestReqsBranch       string
	stagingUsesProdConfig      bool
}

func cmdGenTestPlan(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "gen-test-plan --input_json=/path/to/input.json --output_json=/path/to/output.json",
		ShortDesc: "Generates a test plan",
		LongDesc:  "Generates a test plan",
		CommandRun: func() subcommands.CommandRun {
			c := &getTestPlanRun{}
			c.authFlags = authcli.Flags{}
			c.authFlags.Register(c.GetFlags(), authOpts)
			c.Flags.StringVar(&c.inputJSON, "input_json", "", "Path to JSON proto representing a GenerateTestPlanRequest")
			c.Flags.StringVar(&c.outputJSON, "output_json", "", "Path to file to write output GenerateTestPlanResponse JSON proto")
			c.Flags.StringVar(&c.inputBinaryPb, "input_binary_pb", "", "Path to binaryproto file representing a GenerateTestPlanRequest")
			c.Flags.StringVar(&c.outputBinaryPb, "output_binary_pb", "", "Path to file to write output GenerateTestPlanResponse binaryproto")
			c.Flags.StringVar(&c.sourceGitilesRepo, "gitiles_repo", sourceGitilesRepoDefault, "The gitiles repo to fetch test requirements from")
			c.Flags.StringVar(&c.sourceGitilesBranch, "gitiles_branch", sourceGitilesBranchDefault, "The gitiles branch to fetch test requirements from")
			c.Flags.StringVar(&c.boardPriorityConfigPath, "board_priority_config", boardPriorityConfigPathDefault, "Path to board priority input proto")
			c.Flags.StringVar(&c.sourceTreeTestConfigPath, "source_tree_test_config", sourceTreeTestConfigPathDefault, "Path to the source tree test config input proto")
			c.Flags.StringVar(&c.targetTestRequirementsPath, "target_test_requirements",
				targetTestRequirementsPathDefault, "Path to the target test requirements input proto")
			c.Flags.StringVar(&c.manifestFile, "manifest_file", "", "Path to local manifest file. If given, will be used instead of default snapshot.xml")
			c.Flags.StringVar(&c.localConfigDir, "local_config_dir", "", "Path to a config checkout, to be used rather than gitiles")
			c.Flags.StringVar(&c.targetTestReqsRepo, "target_test_requirements_repo", "",
				"Path to src/config-internal checkout. If set, target test requirements will be generated from this directory using "+
					"./board_config/generate_test_config instead of fetching an already-generated file from Gitiles. Takes precendent "+
					"over all other flags (e.g. -target_test_requirements).")
			c.Flags.StringVar(&c.targetTestReqsBranch, "target_test_requirements_branch", "",
				"Branch to be passed into generate_test_config, ignored if -target_test_requirements_repo is not set.")
			c.Flags.BoolVar(&c.stagingUsesProdConfig, "use_prod_config", false,
				"Whether staging should use testing config from the corresponding production builder(s) instead of staging config. "+
					"Used for tryjobs.")
			return c
		}}
}

func (c *getTestPlanRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	flag.Parse()

	req, err := c.readInput()
	if err != nil {
		log.Print(err)
		return 1
	}

	bbBuilds, err := readBuildbucketBuilds(req.BuildbucketProtos)
	if err != nil {
		log.Print(err)
		return 3
	}

	var boardPriorityList *testplans.BoardPriorityList
	var sourceTreeConfig *testplans.SourceTreeTestCfg
	var testReqsConfig *testplans.TargetTestRequirementsCfg
	if c.localConfigDir == "" {
		boardPriorityList, sourceTreeConfig, testReqsConfig, err = c.fetchConfigFromGitiles()
	} else {
		boardPriorityList, sourceTreeConfig, testReqsConfig, err = c.readLocalConfigFiles()
	}
	if err != nil {
		log.Print(err)
		return 2
	}

	if c.targetTestReqsRepo != "" {
		builderNames := []string{}
		for _, builder := range bbBuilds {
			builderNames = append(builderNames, builder.Builder.Builder)
		}
		testReqsConfig, err = c.genTargetTestRequirements(builderNames)
	}
	if err != nil {
		log.Print(err)
		return 10
	}

	gerritChanges, err := readGerritChanges(req.GerritChanges)
	if err != nil {
		log.Print(err)
		return 8
	}

	changeRevs, err := c.fetchGerritData(gerritChanges)
	if err != nil {
		log.Print(err)
		return 4
	}

	var repoToSrcRoot *map[string]map[string]string
	// If we have a local manifest file provided, use that. Else get it from Gerrit.
	if c.manifestFile == "" {
		gitilesCommit, err := readGitilesCommit(req.GitilesCommit)
		if err != nil {
			log.Print(err)
			return 5
		}

		repoToSrcRoot, err = c.getRepoToSourceRoot(gitilesCommit)
		if err != nil {
			log.Print(err)
			return 6
		}
	} else if len(gerritChanges) > 0 {
		log.Printf("Reading local manifest from %s", c.manifestFile)
		repoToSrcRootMap, err := manifestutil.GetRepoToRemoteBranchToSourceRootFromFile(c.manifestFile)
		if err != nil {
			log.Print(err)
			return 9
		}
		repoToSrcRoot = &repoToSrcRootMap
	} else {
		// No need to map non-existent Gerrit changes to path.
		repoToSrcRoot = &map[string]map[string]string{}
	}

	testPlan, err := generator.CreateTestPlan(testReqsConfig, sourceTreeConfig, boardPriorityList, bbBuilds, gerritChanges, changeRevs, *repoToSrcRoot)
	if err != nil {
		log.Printf("Error creating test plan:\n%v", err)
		return 7
	}

	if err = c.writeOutput(testPlan); err != nil {
		log.Print(err)
		return 8
	}
	return 0
}
func (c *getTestPlanRun) readInput() (*testplans.GenerateTestPlanRequest, error) {
	// use input_binary_pb if it's specified
	if len(c.inputBinaryPb) > 0 {
		inputPb, err := ioutil.ReadFile(c.inputBinaryPb)
		if err != nil {
			return nil, fmt.Errorf("Failed reason input_binary_pb\n%w", err)
		}
		req := &testplans.GenerateTestPlanRequest{}
		if err := proto.Unmarshal(inputPb, req); err != nil {
			return nil, fmt.Errorf("Failed parsing input_binary_pb as proto\n%w", err)
		}
		return req, nil
		// otherwise use input_json
	}
	inputBytes, err := ioutil.ReadFile(c.inputJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed reading input_json\n%w", err)
	}
	req := &testplans.GenerateTestPlanRequest{}
	if err := unmarshaler.Unmarshal(bytes.NewReader(inputBytes), req); err != nil {
		return nil, fmt.Errorf("Couldn't decode %s as a GenerateTestPlanRequest\n%w", c.inputJSON, err)
	}
	return req, nil
}

func (c *getTestPlanRun) fetchConfigFromGitiles() (*testplans.BoardPriorityList, *testplans.SourceTreeTestCfg, *testplans.TargetTestRequirementsCfg, error) {
	// Create an authenticated client for Gerrit RPCs, then fetch all required CL data from Gerrit.
	ctx := context.Background()
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return nil, nil, nil, err
	}
	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return nil, nil, nil, err
	}

	gerritClient, err := igerrit.NewClient(authedClient)
	if err != nil {
		return nil, nil, nil, err
	}
	m, err := gerritClient.FetchFilesFromGitiles(ctx,
		"chrome-internal.googlesource.com",
		c.sourceGitilesRepo,
		c.sourceGitilesBranch,
		[]string{c.boardPriorityConfigPath, c.sourceTreeTestConfigPath, c.targetTestRequirementsPath},
		shared.LongerOpts,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	boardPriorityList := &testplans.BoardPriorityList{}
	if err := proto.Unmarshal([]byte((*m)[c.boardPriorityConfigPath]), boardPriorityList); err != nil {
		return nil, nil, nil, fmt.Errorf("Couldn't decode %s as a BoardPriorityList\n%w", (*m)[c.boardPriorityConfigPath], err)
	}
	sourceTreeConfig := &testplans.SourceTreeTestCfg{}
	if err := proto.Unmarshal([]byte((*m)[c.sourceTreeTestConfigPath]), sourceTreeConfig); err != nil {
		return nil, nil, nil, fmt.Errorf("Couldn't decode %s as a SourceTreeTestCfg\n%w", (*m)[c.sourceTreeTestConfigPath], err)
	}
	testReqsConfig := &testplans.TargetTestRequirementsCfg{}
	if err := proto.Unmarshal([]byte((*m)[c.targetTestRequirementsPath]), testReqsConfig); err != nil {
		return nil, nil, nil, fmt.Errorf("Couldn't decode %s as a TargetTestRequirementsCfg\n%w", (*m)[c.targetTestRequirementsPath], err)
	}
	log.Printf("Fetched config from Gitiles:\n%s\n\n%s\n\n%s", proto.MarshalTextString(boardPriorityList),
		proto.MarshalTextString(sourceTreeConfig), proto.MarshalTextString(testReqsConfig))
	return boardPriorityList, sourceTreeConfig, testReqsConfig, nil
}

func (c *getTestPlanRun) readLocalConfigFiles() (*testplans.BoardPriorityList, *testplans.SourceTreeTestCfg, *testplans.TargetTestRequirementsCfg, error) {
	log.Print("--------------------------------------------")
	log.Print("WARNING: Reading config from local dir.")
	log.Print("Be sure that you've run `./regenerate_configs.sh -b` first to generate binaryproto files")
	log.Print("--------------------------------------------")

	bplBytes, err := ioutil.ReadFile(path.Join(c.localConfigDir, c.boardPriorityConfigPath))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't read BoardPriorityList file: %w", err)
	}
	boardPriorityList := &testplans.BoardPriorityList{}
	if err := proto.Unmarshal(bplBytes, boardPriorityList); err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't decode file as BoardPriorityList: %w", err)
	}

	stcBytes, err := ioutil.ReadFile(path.Join(c.localConfigDir, c.sourceTreeTestConfigPath))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't read SourceTreeTestCfg file: %w", err)
	}
	sourceTreeConfig := &testplans.SourceTreeTestCfg{}
	if err := proto.Unmarshal(stcBytes, sourceTreeConfig); err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't decode file as SourceTreeTestCfg: %w", err)
	}

	ttrBytes, err := ioutil.ReadFile(path.Join(c.localConfigDir, c.targetTestRequirementsPath))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't read TargetTestRequirementsCfg file: %w", err)
	}
	testReqsConfig := &testplans.TargetTestRequirementsCfg{}
	if err := proto.Unmarshal(ttrBytes, testReqsConfig); err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't decode file as TargetTestRequirementsCfg: %w", err)
	}
	log.Printf("Read local config:\n%s\n\n%s\n\n%s", proto.MarshalTextString(boardPriorityList),
		proto.MarshalTextString(sourceTreeConfig), proto.MarshalTextString(testReqsConfig))
	return boardPriorityList, sourceTreeConfig, testReqsConfig, nil
}

func (c *getTestPlanRun) genTargetTestRequirements(builderNames []string) (*testplans.TargetTestRequirementsCfg, error) {
	cmdRunner := cmd.RealCommandRunner{}
	testReqsConfig := &testplans.TargetTestRequirementsCfg{}

	// If no build results are passed to the test_plan_generator, we have nothing to test.
	// This is handled later on for other target test req sources. For this source (direct
	// generation), we just return empty target test reqes -- the result is the same (no
	// tests).
	if len(builderNames) == 0 {
		log.Printf("--target_test_requirements_repo was passed but there are no builds attached " +
			"to this run, returning empty target test reqs config")
		return testReqsConfig, nil
	}

	ctx := context.Background()
	if c.stagingUsesProdConfig {
		for i, builderName := range builderNames {
			builderNames[i] = strings.TrimPrefix(builderName, "staging-")
		}
	}
	cmd := []string{strings.Join(builderNames, ",")}

	if c.targetTestReqsBranch != "" {
		// If --branch is supported on this branch, use it.
		// TODO(b/262910381): Maybe in many years we can do this
		// unconditionally.
		var helpBuf bytes.Buffer
		err := cmdRunner.RunCommand(ctx, &helpBuf, nil, c.targetTestReqsRepo, "./board_config/generate_test_config", "-h")
		if err == nil && strings.Contains(helpBuf.String(), "--branch") {
			cmd = append(cmd, "--branch", c.targetTestReqsBranch)
		}
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	if err := cmdRunner.RunCommand(ctx, &stdoutBuf, &stderrBuf, c.targetTestReqsRepo, "./board_config/generate_test_config", cmd...); err != nil {
		return nil, fmt.Errorf("error running ./board_config/generate_test_config: %s", stderrBuf.String())
	}
	if err := unmarshaler.Unmarshal(bytes.NewReader(stdoutBuf.Bytes()), testReqsConfig); err != nil {
		return nil, fmt.Errorf("couldn't decode file as TargetTestRequirementsCfg: %w", err)
	}
	log.Printf("Overriding target test config with directly generated config:\n%s", proto.MarshalTextString(testReqsConfig))
	return testReqsConfig, nil
}

func readBuildbucketBuilds(bbBuildsBytes []*testplans.ProtoBytes) ([]*bbproto.Build, error) {
	bbBuilds := make([]*bbproto.Build, 0)
	for _, bbBuildBytes := range bbBuildsBytes {
		bbBuild := &bbproto.Build{}
		if err := proto.Unmarshal(bbBuildBytes.SerializedProto, bbBuild); err != nil {
			return bbBuilds, fmt.Errorf("Couldn't decode %s as a Buildbucket Build\n%w", bbBuildBytes.String(), err)
		}
		bbBuilds = append(bbBuilds, bbBuild)
	}
	if len(bbBuilds) > 0 {
		log.Printf("Sample buildbucket proto:\n%s", proto.MarshalTextString(bbBuilds[0]))
	}
	return bbBuilds, nil
}

func readGerritChanges(changesBytes []*testplans.ProtoBytes) ([]*bbproto.GerritChange, error) {
	changes := make([]*bbproto.GerritChange, 0)
	for _, changeBytes := range changesBytes {
		change := &bbproto.GerritChange{}
		if err := proto.Unmarshal(changeBytes.SerializedProto, change); err != nil {
			return changes, fmt.Errorf("Couldn't decode %s as a GerritChange\n%w", changeBytes.String(), err)
		}
		changes = append(changes, change)
	}
	if len(changes) > 0 {
		log.Printf("Sample GerritChange proto:\n%s", proto.MarshalTextString(changes[0]))
	}
	return changes, nil
}

func (c *getTestPlanRun) fetchGerritData(changes []*bbproto.GerritChange) (*igerrit.ChangeRevData, error) {
	// Create an authenticated client for Gerrit RPCs, then fetch all required CL data from Gerrit.
	ctx := context.Background()
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return nil, err
	}
	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return nil, err
	}
	changeIds := make([]igerrit.ChangeRevKey, 0)
	for _, c := range changes {
		changeIds = append(changeIds, igerrit.ChangeRevKey{Host: c.Host, ChangeNum: c.Change, Revision: int32(c.Patchset)})
	}
	chRevData, err := igerrit.GetChangeRevData(ctx, authedClient, changeIds)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch CL data from Gerrit. "+
			"Note that a NotFound error may indicate authorization issues.\n%v", err)
	}
	return chRevData, nil
}

func readGitilesCommit(gitilesBytes *testplans.ProtoBytes) (*bbproto.GitilesCommit, error) {
	gc := &bbproto.GitilesCommit{}
	if err := proto.Unmarshal(gitilesBytes.SerializedProto, gc); err != nil {
		return nil, fmt.Errorf("Couldn't decode %s as a GitilesCommit\n%w", gitilesBytes.String(), err)
	}
	log.Printf("Got GitilesCommit proto:\n%s", proto.MarshalTextString(gc))
	return gc, nil
}

func (c *getTestPlanRun) getRepoToSourceRoot(gc *bbproto.GitilesCommit) (*map[string]map[string]string, error) {
	ctx := context.Background()
	authOpts, err := c.authFlags.Options()
	if err != nil {
		return nil, err
	}
	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return nil, err
	}
	gerritClient, err := igerrit.NewClient(authedClient)
	if err != nil {
		return nil, err
	}
	if gc.Id == "" {
		log.Print("No manifest commit provided. Using 'snapshot' instead.")
		gc.Id = "snapshot"
	}
	repoToRemoteBranchToSrcRoot, err := manifestutil.GetRepoToRemoteBranchToSourceRootFromGitiles(ctx, gerritClient, gc)
	if err != nil {
		return nil, fmt.Errorf("Error with repo tool call\n%w", err)
	}
	return &repoToRemoteBranchToSrcRoot, nil
}
func (c *getTestPlanRun) writeOutput(tp *testplans.GenerateTestPlanResponse) error {
	if len(c.outputJSON) > 0 {
		marshal := &jsonpb.Marshaler{EmitDefaults: true, Indent: "  "}
		jsonOutput, err := marshal.MarshalToString(tp)
		if err != nil {
			return fmt.Errorf("Failed to marshal JSON %v\n%w", tp, err)
		}
		if err = ioutil.WriteFile(c.outputJSON, []byte(jsonOutput), 0644); err != nil {
			return fmt.Errorf("Failed to write output JSON!\n%w", err)
		}
		log.Printf("Wrote output JSON to %s", c.outputJSON)
	}

	if len(c.outputBinaryPb) > 0 {
		binaryOutput, err := proto.Marshal(tp)
		if err != nil {
			return fmt.Errorf("Failed to marshal binaryproto %v\n%w", tp, err)
		}
		if err = ioutil.WriteFile(c.outputBinaryPb, binaryOutput, 0644); err != nil {
			return fmt.Errorf("Failed to write output binary proto!\n%w", err)
		}
		log.Printf("Wrote output binary proto to %s", c.outputBinaryPb)
	}

	return nil
}

// GetApplication returns an instance of the test_planner application.
func GetApplication(authOpts auth.Options) *cli.Application {
	return &cli.Application{
		Name: "test_planner",

		Context: func(ctx context.Context) context.Context {
			return ctx
		},

		Commands: []*subcommands.Command{
			authcli.SubcommandInfo(authOpts, "auth-info", false),
			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),
			cmdGenTestPlan(authOpts),
		},
	}
}

func main() {
	opts := chromeinfra.DefaultAuthOptions()
	opts.Scopes = []string{gerrit.OAuthScope, auth.OAuthScopeEmail}
	app := GetApplication(opts)
	os.Exit(subcommands.Run(app, nil))
}
