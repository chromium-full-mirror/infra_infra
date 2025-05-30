// Copyright 2021 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"io"
	"os"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
)

type goRun struct {
	baseRun

	// CopyTestOutput, if non-nil, specifies where test output
	// is written to in addition to being uploaded to ResultDB.
	CopyTestOutput io.Writer

	// VerboseTestOutput, if true, will emit the full test output akin
	// to go test -v.
	VerboseTestOutput bool

	// DumpJSONFile, if not empty, causes goRun to dump the raw JSON produced
	// by the Go command to the filepath specified.
	DumpJSONFile string
}

func cmdGo() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `go -- go test [TEST_ARG]...`,
		ShortDesc: "Batch upload results of golang test result format to ResultSink",
		LongDesc: text.Doc(`
			Runs the test command and waits for it to finish, then converts the json output
			test results to ResultSink native format and uploads them to ResultDB via ResultSink.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &goRun{
				// Print the test output to stdout so that
				// it's also available as a step log.
				CopyTestOutput:    os.Stdout,
				VerboseTestOutput: true,
			}
			r.Flags.BoolVar(&r.VerboseTestOutput, "v", r.VerboseTestOutput, text.Doc(`
				Flag to emit the full go test -v output to stdout.
				If false, then the output will look more like go test without -v.
			`))
			r.Flags.StringVar(&r.DumpJSONFile, "dump-json", r.DumpJSONFile, text.Doc(`
				Flag to dump raw Go test JSON to a file.
			`))
			// Capture output. All known Go test commands produce test results solely to stdout.
			r.captureOutput = true
			// Ignore global flags, go tests are expected to only produce
			// standard output.
			return r
		},
	}
}

func (r *goRun) Run(a subcommands.Application, args []string, env subcommands.Env) (ret int) {
	args, err := r.ensureArgsValid(args)
	if err != nil {
		return r.done(err)
	}

	ctx := cli.GetContext(a, r, env)
	return r.run(ctx, args, r.generateTestResults)
}
