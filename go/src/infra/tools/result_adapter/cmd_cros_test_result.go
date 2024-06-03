// Copyright 2022 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"context"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"
)

func cmdCrosTestResult() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `cros-test-result [flags] TEST_CMD [TEST_ARG]...`,
		ShortDesc: "Batch upload ChromeOS test results to ResultSink",
		LongDesc: text.Doc(`
			Runs the test command and waits for it to finish, then converts the
			ChromeOS test results to ResultSink native format and uploads them
			to ResultDB via ResultSink.
			A JSON line file is expected for -result-file.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &crosTestResultRun{}
			r.baseRun.RegisterGlobalFlags()
			r.registerCrosTestResultFlags()
			return r
		},
	}
}

type crosTestResultRun struct {
	baseRun

	// Flags.
	testhausBaseURL string
}

// registerCrosTestResultFlags registers the CrosTestResult specific flags.
func (r *crosTestResultRun) registerCrosTestResultFlags() {
	r.Flags.StringVar(&r.testhausBaseURL, "testhaus-base-url", "", text.Doc(`
				Base URL to the logs of this invocation in Testhaus.  This will be used to emit a per-test link artifact
				that is a deep link to the logs of each specific test in Testhaus.
			`))
}

func (r *crosTestResultRun) validate() (err error) {
	return r.baseRun.validate()
}

func (r *crosTestResultRun) Run(a subcommands.Application, args []string, env subcommands.Env) (ret int) {
	if err := r.validate(); err != nil {
		return r.done(err)
	}

	ctx := cli.GetContext(a, r, env)
	return r.run(ctx, args, r.generateTestResults)
}

// generateTestResults converts test results from results file to sinkpb.TestResult.
func (r *crosTestResultRun) generateTestResults(ctx context.Context, _ []byte) ([]*sinkpb.TestResult, error) {
	f, err := os.Open(r.resultFile)
	if err != nil {
		return nil, errors.Annotate(err, "failed to open cros_test_result file").Err()
	}
	defer f.Close()

	// Convert the results to ResultSink native format.
	crosTestResultFormat := &CrosTestResult{testhausBaseURL: r.testhausBaseURL}
	if err = crosTestResultFormat.ConvertFromJSON(f); err != nil {
		return nil, errors.Annotate(err, "failed to recognize as cros_test_result result").Err()
	}
	trs, err := crosTestResultFormat.ToProtos(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "failed to convert cros_test_result result to ResultSink result").Err()
	}
	return trs, nil
}
