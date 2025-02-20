// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli defines different commands for the test_plan tool.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/common/logging"
	cvpb "go.chromium.org/luci/cv/api/config/v2"

	"go.chromium.org/infra/cros/internal/manifestutil"
	"go.chromium.org/infra/cros/internal/testplan/migrationstatus"
)

func unmarshalTextproto(path string, m proto.Message) error {
	protoBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return prototext.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(protoBytes, m)
}

func CmdMigrationStatus(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "migration-status -crossrcroot ~/chromiumos [-project PROJECT1 -project PROJECT2...]",
		ShortDesc: "summarize the migration status of projects",
		LongDesc: text.Doc(`
		Summarize the migration status of projects in the manifest.

		Reads the default manifest, Buildbucket config, and CV config from
		-crossrcroot, and for each project in the manifest checks if it has a
		matching CrosTestPlanV2Properties.ProjectMigrationConfig in the input
		properties of the CQ orchestrators. Prints a summary of the number of
		projects migrated.

		Projects that are not in the "ToT" ConfigGroup of cvConfig or are
		excluded from the CQ orchestrator by a LocationFilter are skipped.

		Optionally takes multiple -project arguments, and prints whether those
		specific projects are migrated. If one of these projects does not exist
		in the manifest, an error is returned.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &migrationStatusRun{}
			r.addSharedFlags(authOpts)

			r.Flags.StringVar(&r.crosSrcRoot, "crossrcroot", "", text.Doc(`
			Required, path to the root of a ChromeOS checkout. The manifest and
			generated Buildbucket config found in this checkout will be used.
			`))
			r.Flags.Var(flag.StringSlice(&r.projects), "project", text.Doc(`
			Optional, projects to check the specific migration status of. If one
			of these projects does not exist in the manifest, an error is
			returned.
			`))
			r.Flags.StringVar(&r.csvOut, "csvout", "", text.Doc(`
			Optional, a path to output a CSV with the migration statuses for all
			projects.
			`))
			return r
		},
	}
}

type migrationStatusRun struct {
	baseTestPlanRun
	crosSrcRoot string
	projects    []string
	csvOut      string
}

func (r *migrationStatusRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	return errToCode(a, r.run(ctx, args))
}

func (r *migrationStatusRun) validateFlags(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected positional arguments: %q", args)
	}

	if r.crosSrcRoot == "" {
		return fmt.Errorf("-crossrcroot must be set")
	}

	return nil
}

func (r *migrationStatusRun) run(ctx context.Context, args []string) (err error) {
	if err = r.validateFlags(args); err != nil {
		return err
	}

	manifestPath := filepath.Join(r.crosSrcRoot, "manifest-internal", "default.xml")
	logging.Debugf(ctx, "reading manifest from %q", manifestPath)
	manifest, err := manifestutil.LoadManifestFromFileWithIncludes(manifestPath)
	if err != nil {
		return err
	}

	infraCfgPath := filepath.Join(r.crosSrcRoot, "infra", "config", "generated")

	cvCfgPath := filepath.Join(infraCfgPath, "commit-queue.cfg")
	bbCfgPath := filepath.Join(infraCfgPath, "cr-buildbucket.cfg")

	logging.Debugf(ctx, "reading CV config from %q", cvCfgPath)
	cvConfig := &cvpb.Config{}
	if err := unmarshalTextproto(cvCfgPath, cvConfig); err != nil {
		return err
	}

	logging.Debugf(ctx, "reading Buildbucket config from %q", bbCfgPath)
	bbCfg := &bbpb.BuildbucketCfg{}
	if err := unmarshalTextproto(bbCfgPath, bbCfg); err != nil {
		return err
	}

	statuses, err := migrationstatus.Compute(ctx, manifest, bbCfg, cvConfig)
	if err != nil {
		return err
	}

	textSummary, err := migrationstatus.TextSummary(ctx, statuses, r.projects)
	if err != nil {
		return err
	}

	fmt.Print(textSummary)

	if r.csvOut != "" {
		logging.Debugf(ctx, "writing CSV to %q", r.csvOut)

		f, err := os.Create(r.csvOut)
		defer func() {
			err = f.Close()
		}()
		if err != nil {
			return err
		}

		if err := migrationstatus.CSV(ctx, statuses, f); err != nil {
			return err
		}
	}

	return nil
}
