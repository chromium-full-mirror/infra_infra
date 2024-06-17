// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli defines different commands for the test_plan tool.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/grpc/prpc"

	"infra/cros/internal/cmd"
	"infra/cros/internal/gerrit"
	"infra/cros/internal/gs"
	"infra/cros/internal/testplan"
	"infra/cros/lib/buildbucket"
	"infra/tools/dirmd"
	dirmdpb "infra/tools/dirmd/proto"
)

// findRepoRoot finds the absolute path to the root of the repo dir is in.
func findRepoRoot(ctx context.Context, dir string) (string, error) {
	stdout, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}

	repoRoot := string(bytes.TrimSpace(stdout))
	return repoRoot, nil
}

func CmdValidate(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "validate DIR",
		ShortDesc: "validate metadata files",
		LongDesc: text.Doc(`
		Validate metadata files.

		Validation logic on "DIR_METADATA" files specific to ChromeOS test planning.

		The positional argument should be a path to a directory to compute and validate
		metadata for. All sub-directories will also be validated.

		The subcommand returns a non-zero exit code if any of the files is invalid.
	`),
		CommandRun: func() subcommands.CommandRun {
			r := &validateRun{}
			r.addSharedFlags(authOpts)

			r.Flags.BoolVar(&r.checkTagCriteriaNonEmpty, "checktagcriteria", false, text.Doc(`
			If enabled, use the cros-test-finder Docker container to check that
			the TestCaseTagCriteria in TemplateParameters match at least one test.
			`))

			return r
		},
	}
}

type validateRun struct {
	baseTestPlanRun
	checkTagCriteriaNonEmpty bool
}

func (r *validateRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	return errToCode(a, r.run(a, args, env))
}

func (r *validateRun) validateFlagsAndGetDir(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("exactly one directory must be specified as a positional argument")
	}

	return args[0], nil
}

func (r *validateRun) run(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, r, env)

	dir, err := r.validateFlagsAndGetDir(args)
	if err != nil {
		return err
	}

	authOpts, err := r.authFlags.Options()
	if err != nil {
		return err
	}

	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return err
	}

	gerritClient, err := gerrit.NewClient(authedClient)
	if err != nil {
		return err
	}

	bbClient := bbpb.NewBuildsPRPCClient(&prpc.Client{
		C:       authedClient,
		Host:    "cr-buildbucket.appspot.com",
		Options: buildbucket.DefaultPRPCOpts(),
	})

	gsClient, err := gs.NewProdClient(ctx, authedClient)
	if err != nil {
		return err
	}

	mapping, err := dirmd.ReadMapping(ctx, dirmdpb.MappingForm_ORIGINAL, true, dir)
	if err != nil {
		return err
	}

	repoRoot, err := findRepoRoot(ctx, dir)
	if err != nil {
		return err
	}

	validator := testplan.NewValidator(
		gerritClient, bbClient, gsClient, cmd.RealCommandRunner{},
	).SetCheckTagCriteriaNonEmptyEnabled(r.checkTagCriteriaNonEmpty)
	return validator.ValidateMapping(ctx, mapping, repoRoot)
}
