// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"errors"
	"math"
	"os"
	"strings"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/rts"
	"go.chromium.org/infra/rts/presubmit/eval"
)

type analyzeCommandRun struct {
	subcommands.CommandRunBase
	authOpt       *auth.Options
	ev            eval.Eval
	builder       string
	testSuite     string
	testSuiteFile string
	testIdFile    string
}

func cmdAnalyze(authOpt *auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `analyze -rejections <path> -durations <path> -builder <name> -testSuite <name> [-testSuiteFile <path>] [-testIdFile <path>]`,
		ShortDesc: "Prints the expected recall and savings with the provided test id file/test suite/builder combination removed",
		LongDesc:  "Prints the expected recall and savings with the provided test id file/test suite/builder combination removed",
		CommandRun: func() subcommands.CommandRun {
			r := &analyzeCommandRun{authOpt: authOpt}
			r.Flags.StringVar(&r.builder, "builder", "", "Builder running the testSuite to exclude from tests")
			r.Flags.StringVar(&r.testSuite, "testSuite", "", "Test suite of the builder to exclude from tests")
			r.Flags.StringVar(&r.testSuiteFile, "testSuiteFile", "", "A file containing builder and suites separated by a colon : that should also be excluded")
			r.Flags.StringVar(&r.testIdFile, "testIdFile", "", "Test id file to exclude from tests")
			r.ev.LogProgressInterval = 1000
			r.ev.RegisterFlags(&r.Flags)
			return r
		},
	}
}

func (r *analyzeCommandRun) validateFlags() error {
	if err := r.ev.ValidateFlags(); err != nil {
		return err
	}
	switch {
	case r.builder == "":
		return errors.New("-builder is required")
	case r.testSuite == "":
		return errors.New("-testSuite is required")
	default:
		return nil
	}
}

func (r *analyzeCommandRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, r, env)

	if err := r.validateFlags(); err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	// Previously removed builder/suites
	removedBuilderSuites, err := loadTestSuiteFile(r.testSuiteFile)
	if err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	// The builder/suite under analysis
	removedBuilderSuite := r.builder + ":" + r.testSuite

	// Individual tests to remove from the current builder/suite instead of
	// the whole suite
	testIds, err := loadTestIds(r.testIdFile)
	if err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	res, err := r.ev.Run(ctx, func(ctx context.Context, in eval.Input, out *eval.Output) error {
		for i, tv := range in.TestVariants {
			variantBuilderSuite := getBuilderSuiteString(tv.Variant)

			if (r.testSuiteFile != "" && removedBuilderSuites[variantBuilderSuite]) ||
				(removedBuilderSuite == variantBuilderSuite && (r.testIdFile == "" || testIds[tv.Id])) {
				out.TestVariantAffectedness[i] = rts.Affectedness{Distance: math.Inf(1)}
			} else {
				out.TestVariantAffectedness[i] = rts.Affectedness{Distance: 0}
			}
		}

		return nil
	})

	if err != nil {
		logging.Infof(ctx, err.Error())
		return 1
	}

	// We don't care about the 100% recall and 0% savings threshold
	if len(res.Thresholds) > 1 {
		res.Thresholds = res.Thresholds[:1]
	}

	r.ev.LogAndClearFurthest(ctx)
	eval.PrintSpecificResults(res, os.Stdout, 0.0, false, false)
	return 0
}

func loadTestIds(fileName string) (map[string]bool, error) {
	if fileName == "" {
		return nil, nil
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, errors.New("failed to load test id file.")
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	testIds := map[string]bool{}

	// Read through test Id until an EOF is encountered.
	for sc.Scan() {
		testIds[sc.Text()] = true
	}

	return testIds, nil
}

func loadTestSuiteFile(fileName string) (map[string]bool, error) {
	if fileName == "" {
		return nil, nil
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, errors.New("failed to load builder test suite file.")
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	builderTestSuites := map[string]bool{}

	// Read through test Id until an EOF is encountered.
	for sc.Scan() {
		builderTestSuites[sc.Text()] = true
	}

	return builderTestSuites, nil
}

func getBuilderSuiteString(list []string) string {
	builder := ""
	testSuite := ""
	for _, b := range list {
		if strings.HasPrefix(b, "builder:") {
			builder = b[len("builder:"):]
		}
		if strings.HasPrefix(b, "test_suite:") {
			testSuite = b[len("test_suite:"):]
		}
	}
	if builder == "" || testSuite == "" {
		return ""
	}
	return builder + ":" + testSuite
}
