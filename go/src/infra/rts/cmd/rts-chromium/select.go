// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/maruel/subcommands"
	"golang.org/x/sync/errgroup"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/rts/internal/chromium"
)

func cmdSelect() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `select -checkout <path> -model-dir <path> -out <path>`,
		ShortDesc: "compute the set of test files to skip",
		LongDesc: text.Doc(`
			Compute the set of test files to skip.

			Flags -checkout, -model-dir and -out are required.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &selectRun{}
			r.Flags.StringVar(&r.Checkout, "checkout", "", "Path to a src.git checkout")
			r.Flags.StringVar(&r.ModelDir, "model-dir", "", text.Doc(`
				Path to the directory with the model files.
				Normally it is coming from CIPD package "chromium/rts/model"
				and precomputed by "rts-chromium create-model" command.
			`))
			r.Flags.StringVar(&r.Out, "out", "", text.Doc(`
				Path to a directory where to write test filter files.
				A file per test target is written, e.g. browser_tests.filter.
				The file format is described in https://chromium.googlesource.com/chromium/src/+/HEAD/testing/buildbot/filters/README.md.
				Before writing, all .filter files in the directory are deleted.

				The out directory may be empty. It may happen if the selection strategy
				decides to run all tests, e.g. if //DEPS is changed.
			`))
			r.Flags.Float64Var(&r.TargetChangeRecall, "target-change-recall", 0.99, text.Doc(`
				The target fraction of bad changes to be caught by the selection strategy.
				It must be a value in (0.0, 1.0) range.
			`))
			r.Flags.BoolVar(&r.IgnoreExceptions, "ignore-exceptions", false, "For debugging. Whether we should ignore exceptions.")
			r.Flags.BoolVar(&r.GenerateInverse, "gen-inverse", false, "Generates the inverse filter files.")
			r.Flags.StringVar(&r.ChangeRef, "change-ref", "", text.Doc(`
				Git ref to calculate the changed files (e.g origin/main). By
				default will use the current staged change.
			`))
			return r
		},
	}
}

type selectRun struct {
	chromium.BaseSelectRun
}

func (r *selectRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	if len(args) != 0 {
		return r.Done(errors.New("unexpected positional arguments"))
	}

	if err := r.ValidateFlags(); err != nil {
		return r.Done(err)
	}

	if err := r.loadInput(ctx); err != nil {
		return r.Done(err)
	}

	if err := chromium.PrepareOutDir(r.Out, false, "*.filter"); err != nil {
		return r.Done(errors.Annotate(err, "failed to prepare filter file dir %q", r.Out).Err())
	}

	// Do this check only after existing .filter files are deleted.
	if len(r.ChangedFiles) == 0 {
		logging.Warningf(ctx, "no changed files detected")
		return 0
	}
	r.LogChangedFiles(ctx)

	logging.Infof(ctx, "chosen threshold: %f", r.Strategy.MaxDistance)

	// Select the tests and write .filter files.
	err := r.writeFilterFiles()
	if disableRTS.In(err) {
		logging.Warningf(ctx, "disabling RTS: %s", err)
		err = nil
	}
	return r.Done(err)
}

// writeFilterFiles writes filter files in r.filterFilesDir directory.
func (r *selectRun) writeFilterFiles() error {
	// Maps a test target to the list of tests to skip.
	testsToSkip := map[string][]string{}
	err := r.selectTests(func(testFileToSkip *chromium.TestFile) error {
		for _, target := range testFileToSkip.TestTargets {
			testsToSkip[target] = append(testsToSkip[target], testFileToSkip.TestNames...)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Write the files.
	for target, testNames := range testsToSkip {
		fileName := filepath.Join(r.Out, target+".filter")
		if err := chromium.WriteFilterFile(fileName, testNames); err != nil {
			return errors.Annotate(err, "failed to write %q", fileName).Err()
		}
		fmt.Printf("wrote %s\n", fileName)

		if r.GenerateInverse {
			invertedFileName := filepath.Join(r.Out, target+"_inverted.filter")
			if err := chromium.WriteInvertedFilterFile(invertedFileName, testNames); err != nil {
				return errors.Annotate(err, "failed to write %q", invertedFileName).Err()
			}
			fmt.Printf("wrote %s\n", invertedFileName)
		}
	}
	return nil
}

// loadInput loads all the input of the subcommand.
func (r *selectRun) loadInput(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	defer eg.Wait()

	gitGraphDir := filepath.Join(r.ModelDir, "git-file-graph")
	eg.Go(func() error {
		err := r.LoadGraph(filepath.Join(gitGraphDir, "graph.fg"))
		return errors.WrapIf(err, "failed to load file graph")
	})
	eg.Go(func() error {
		err := r.LoadStrategy(filepath.Join(gitGraphDir, "config.json"))
		return errors.WrapIf(err, "failed to load eval results")
	})

	eg.Go(func() (err error) {
		err = r.LoadTestFileSet(filepath.Join(r.ModelDir, "test-files.jsonl"))
		return errors.WrapIf(err, "failed to load test files set")
	})

	eg.Go(func() (err error) {
		err = r.LoadChangedFiles()
		return errors.WrapIf(err, "failed to load changed files")
	})

	return eg.Wait()
}
