// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cli defines different commands for the test_plan tool.
package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/chromiumos/config/go/test/plan"
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/shared"
	"go.chromium.org/infra/cros/internal/testplan"
)

// getChangeRevs parses each of rawCLURLs and returns a ChangeRev.
func getChangeRevs(ctx context.Context, authedClient *http.Client, rawCLURLs []string) ([]*gerrit.ChangeRev, error) {
	changeRevs := make([]*gerrit.ChangeRev, len(rawCLURLs))

	for i, cl := range rawCLURLs {
		changeRevKey, err := gerrit.ParseCLURL(cl)
		if err != nil {
			return nil, err
		}

		changeRev, err := gerrit.GetChangeRev(
			ctx, authedClient, changeRevKey.ChangeNum, changeRevKey.Revision, changeRevKey.Host, shared.DefaultOpts,
		)
		if err != nil {
			return nil, err
		}

		changeRevs[i] = changeRev
	}

	return changeRevs, nil
}

// writePlans writes each of plans to a textproto file. The first plan is in a
// file named "relevant_plan_1.textpb", the second is in
// "relevant_plan_2.textpb", etc.
//
// TODO(b/182898188): Consider making a message to hold multiple SourceTestPlans
// instead of writing multiple files.
func writePlans(ctx context.Context, plans []*plan.SourceTestPlan, outPath string) (err error) {
	logging.Infof(ctx, "writing output to %s", outPath)

	err = os.MkdirAll(outPath, os.ModePerm)
	if err != nil {
		return err
	}

	for i, plan := range plans {
		outFile, err := os.Create(path.Join(outPath, fmt.Sprintf("relevant_plan_%d.textpb", i)))
		if err != nil {
			return err
		}
		defer func() {
			err = outFile.Close()
		}()

		b, err := prototext.Marshal(plan)
		if err != nil {
			return err
		}

		if _, err = outFile.Write(b); err != nil {
			return err
		}

	}

	return nil
}

func CmdRelevantPlans(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "relevant-plans -cl CL1 [-cl CL2] -out OUTPUT",
		ShortDesc: "Find SourceTestPlans relevant to a set of CLs",
		LongDesc: text.Doc(`
		Find SourceTestPlans relevant to a set of CLs.

		Computes SourceTestPlans from "DIR_METADATA" files and returns plans
		relevant to the files changed by a CL.
	`),
		CommandRun: func() subcommands.CommandRun {
			r := &relevantPlansRun{}
			r.addSharedFlags(authOpts)

			r.Flags.Var(flag.StringSlice(&r.cls), "cl", text.Doc(`
			CL URL for the patchsets being tested. Must be specified at least once.
			Changes will be merged in the order they are passed on the command line.

			Example: https://chromium-review.googlesource.com/c/chromiumos/platform2/+/123456
		`))
			r.Flags.StringVar(&r.out, "out", "", "Path to the output test plan")
			r.Flags.DurationVar(&r.cloneDepth, "clonedepth", time.Hour*24*180, text.Doc(`
			When this command clones and fetches CLs to compute DIR_METADATA
			files, it will compute the earliest creation time of a group of CLs in
			a repo, and then clone up to a depth of this earliest creation time
			+ this duration.

			For example, if the earliest CL in a repo was created on Dec 20 and
			this argument is 10 days, the clones and fetches will be up to Dec 10.

			A larger duration means there is less chance of a false merge conflict,
			but the clone and fetch times will be longer. Defaults to 180d.
			`))

			return r
		},
	}
}

type relevantPlansRun struct {
	baseTestPlanRun
	cls        []string
	out        string
	cloneDepth time.Duration
}

func (r *relevantPlansRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	return errToCode(a, r.run(ctx))
}

func (r *relevantPlansRun) validateFlags() error {
	if len(r.cls) == 0 {
		return errors.New("-cl must be specified at least once")
	}

	if r.out == "" {
		return errors.New("-out is required")
	}

	return nil
}

func (r *relevantPlansRun) run(ctx context.Context) error {
	if err := r.validateFlags(); err != nil {
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

	var changeRevs []*gerrit.ChangeRev

	logging.Infof(ctx, "fetching metadata for CLs")

	changeRevs, err = getChangeRevs(ctx, authedClient, r.cls)
	if err != nil {
		return err
	}

	for i, changeRev := range changeRevs {
		logging.Debugf(ctx, "change rev %d: %q", i, changeRev)
	}

	// Use a workdir creation function that returns a tempdir, and removes the
	// entire tempdir on cleanup.
	workdirFn := func() (string, func() error, error) {
		workdir, err := os.MkdirTemp("", "")
		if err != nil {
			return "", nil, err
		}

		return workdir, func() error { return os.RemoveAll((workdir)) }, nil
	}

	plans, err := testplan.FindRelevantPlans(ctx, changeRevs, workdirFn, r.cloneDepth)
	if err != nil {
		return err
	}

	return writePlans(ctx, plans, r.out)
}
