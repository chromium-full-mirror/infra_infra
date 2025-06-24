// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/flag"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/rts/filegraph/git"
	"go.chromium.org/infra/rts/internal/chromium"
	"go.chromium.org/infra/rts/presubmit/eval"
)

func cmdCreateModel(authOpt *auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `create-model -model-dir <path>`,
		ShortDesc: "create a model to be used by select subcommand",
		LongDesc:  "Create a model to be used by select subcommand",
		CommandRun: func() subcommands.CommandRun {
			r := &createModelRun{authOpt: authOpt}
			r.Flags.StringVar(&r.modelDir, "model-dir", "", text.Doc(`
				Path to the directory where to write the model files.
				The directory will be created if it does not exist.
			`))

			r.Flags.StringVar(&r.checkout, "checkout", "", "Path to a src.git checkout")
			r.Flags.IntVar(&r.loadOptions.MaxCommitSize, "fg-max-commit-size", 100, text.Doc(`
				Maximum number of files touched by a commit.
				Commits that exceed this limit are ignored.
				The rationale is that large commits provide a weak signal of file
				relatedness and are expensive to process, O(N^2).
			`))
			r.Flags.StringVar(&r.trainingData, "ml-training-data", "", text.Doc(`
				Path to the csv file with the ml training data.
			`))
			r.Flags.StringVar(&r.builder, "builder", "", text.Doc(`
				The specific builder to collect stability information for.
			`))
			r.Flags.StringVar(&r.testSuite, "test-suite", "", text.Doc(`
				The specific test-suite to collect stability information for.
			`))
			r.Flags.Var(flag.Date(&r.startDate), "from", text.Doc(`
				Fetch results starting on this day. Stability information will
				be gathered based on this day as a UTC date.
				format: yyyy-mm-dd
			`))
			r.Flags.Var(flag.Date(&r.endDate), "to", text.Doc(`
				Fetch results up to this date as a UTC date. By default only the
				start-day will be gathered.
				format: yyyy-mm-dd
			`))

			r.ev.LogProgressInterval = 100
			r.ev.RegisterFlags(&r.Flags)
			return r
		},
	}
}

type createModelRun struct {
	baseCommandRun
	modelDir string

	checkout     string
	loadOptions  git.LoadOptions
	fg           *git.Graph
	builder      string
	testSuite    string
	startDate    time.Time
	endDate      time.Time
	trainingData string

	// Stability for tests on each day of the evaluation
	stabilityMap       map[stabilityMapKey]*mlExample
	missingStabilities map[string]struct{}
	ev                 eval.Eval

	authOpt  *auth.Options
	bqClient *bigquery.Client
}

func (r *createModelRun) validateFlags() error {
	if err := r.ev.ValidateFlags(); err != nil {
		return err
	}
	switch {
	case r.modelDir == "":
		return errors.New("-model-dir is required")
	case r.checkout == "":
		return errors.New("-checkout is required")
	case r.trainingData == "":
		return errors.New("-ml-training-data is required")
	case r.startDate.IsZero():
		return errors.New("-from is required")
	case r.endDate.IsZero():
		return errors.New("-to is required")
	default:
		return nil
	}
}

func (r *createModelRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)
	if len(args) != 0 {
		return r.done(errors.New("unexpected positional arguments"))
	}
	if err := r.validateFlags(); err != nil {
		return r.done(err)
	}

	r.missingStabilities = make(map[string]struct{})

	var err error
	if r.bqClient, err = chromium.NewBQClient(ctx, auth.NewAuthenticator(ctx, auth.InteractiveLogin, *r.authOpt)); err != nil {
		return r.done(errors.Annotate(err, "failed to create BigQuery client").Err())
	}

	return r.done(r.writeModel(ctx, r.modelDir))
}

// writeModel writes the model files to the directory.
func (r *createModelRun) writeModel(ctx context.Context, dir string) error {
	// Ensure model dir exists.
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Annotate(err, "failed to create model dir at %q", dir).Err()
	}

	logging.Infof(ctx, "Training the ML model...")
	if err := trainMlModel(ctx, r.trainingData, dir); err != nil {
		return errors.Annotate(err, "failed to train the ml model").Err()
	}

	eg, ctx := errgroup.WithContext(ctx)
	defer eg.Wait()

	eg.Go(func() error {
		err := r.writeFileGraphModel(ctx, filepath.Join(dir, "git-file-graph"))
		return errors.WrapIf(err, "failed to write file graph model")
	})

	eg.Go(func() error {
		err := r.writeTestFileSet(ctx, filepath.Join(dir, "test-files.jsonl"))
		return errors.WrapIf(err, "failed to write test file set")
	})

	eg.Go(func() error {
		err := r.writeCurrentStability(ctx, filepath.Join(dir, "test-stability.jsonl"))
		return errors.WrapIf(err, "failed to write test stability set")
	})

	return eg.Wait()
}

// writeFileGraphModel writes the file graph model to the model dir.
func (r *createModelRun) writeFileGraphModel(ctx context.Context, dir string) error {
	var err error
	if r.fg, err = git.Load(ctx, r.checkout, r.loadOptions); err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	if err := r.writeFileGraph(ctx, filepath.Join(dir, "graph.fg")); err != nil {
		return errors.Annotate(err, "failed to write file graph").Err()
	}

	if err := r.writeStrategyConfig(ctx, dir); err != nil {
		return errors.Annotate(err, "failed to write strategy config").Err()
	}

	return nil
}

// writeFileGraph writes the graph file.
func (r *createModelRun) writeFileGraph(ctx context.Context, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	bufW := bufio.NewWriter(f)
	if err := r.fg.Write(bufW); err != nil {
		return err
	}
	return bufW.Flush()
}

// writeStrategyConfig computes and writes the GitBasedStrategyConfig.
func (r *createModelRun) writeStrategyConfig(ctx context.Context, dir string) error {
	logging.Infof(ctx, "Collecting the stability info...")
	testStability, err := getTestIdToStabilityRowMap(ctx, r.bqClient, r.builder, r.testSuite, r.startDate, r.endDate)
	if err != nil {
		return errors.Annotate(err, "failed to collect stability info for the eval period").Err()
	}
	r.stabilityMap = testStability

	// No need to calibrate the edge reader
	logging.Infof(ctx, "Evaluating the strategy...")
	res, err := r.ev.Run(ctx, r.evalStrategy())
	if err != nil {
		return err
	}

	// Print the furthest CLs after processing to keep the polled prints from
	// obscuring the log
	r.ev.LogAndClearFurthest(ctx)

	// Print any test IDs that were missing as info
	for testId := range r.missingStabilities {
		logging.Warningf(ctx, "Stability info not found: %s", testId)
	}

	eval.PrintResults(res, os.Stdout, 0)
	cfgBytes, err := protojson.Marshal(&chromium.GitBasedStrategyConfig{
		ChangeLogDistanceFactor:     1,
		FileStructureDistanceFactor: 1,
		Thresholds:                  res.Thresholds,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), cfgBytes, 0777)
}

// writeTestFileSet writes the test file set in Chromium to the file.
// It skips tests that match neverSkipTestFileRegexp.
//
// The file format is JSON Lines of TestFile protobufs.
func (r *createModelRun) writeTestFileSet(ctx context.Context, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	bufW := bufio.NewWriter(f)

	if err := chromium.WriteTestFiles(ctx, r.bqClient, bufW); err != nil {
		return err
	}

	if err := bufW.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// writeCurrentStability writes the test stability info at the time of model
// creation
//
// The file format is JSON Lines of TestFile protobufs.
func (r *createModelRun) writeCurrentStability(ctx context.Context, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()
	bufW := bufio.NewWriter(f)

	if err := WriteCurrentStability(ctx, r.builder, r.testSuite, r.bqClient, bufW); err != nil {
		return err
	}

	if err := bufW.Flush(); err != nil {
		return err
	}
	return f.Close()
}
