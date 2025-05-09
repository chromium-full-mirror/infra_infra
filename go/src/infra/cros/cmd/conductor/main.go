// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/maruel/subcommands"

	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/try/try"
	"go.chromium.org/infra/cros/internal/cmd"
	bb "go.chromium.org/infra/cros/lib/buildbucket"
)

var (
	unmarshaler = jsonpb.Unmarshaler{AllowUnknownFields: true}
)

type collectRun struct {
	subcommands.CommandRunBase
	stdoutLog *log.Logger
	stderrLog *log.Logger
	cmdRunner cmd.CommandRunner
	bbClient  *bb.Client
	tryClient try.RetryClient

	inputJSON              string
	outputJSON             string
	pollingIntervalSeconds int
	bbids                  list
	dryrun                 bool
	initialRetry           bool
}

type list []string

func (l *list) Set(value string) error {
	*l = strings.Split(strings.TrimSpace(value), ",")
	return nil
}

func (l *list) String() string {
	return strings.Join(*l, ",")
}

func cmdCollect() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "collect",
		ShortDesc: "Collect on the specified builds, retrying as configured.",
		CommandRun: func() subcommands.CommandRun {
			c := &collectRun{}
			c.cmdRunner = cmd.RealCommandRunner{}
			c.tryClient = &try.Client{}
			c.Flags.StringVar(&c.inputJSON, "input_json", "", "Path to JSON proto representing a CollectConfig")
			c.Flags.StringVar(&c.outputJSON, "output_json", "", "Path to write final set of BBIDs to.")
			c.Flags.IntVar(&c.pollingIntervalSeconds, "polling_interval", 60, "Seconds to wait between polling builders")
			c.Flags.Var(&c.bbids, "bbids", "(comma-separated) initial set of BBIDs to watch.")
			c.Flags.BoolVar(&c.dryrun, "dryrun", false, "Dry run (i.e. don't actually retry builds).")
			c.Flags.BoolVar(&c.initialRetry, "initial_retry", false, "If set, will retry any already failed builds at the beginning of execution.")
			return c
		}}
}

// validate validates release-specific args for the command.
func (c *collectRun) validate() error {
	if c.inputJSON == "" {
		return fmt.Errorf("--input_json is required")
	}

	if c.outputJSON == "" {
		return fmt.Errorf("--output_json is required")
	}

	if len(c.bbids) == 0 {
		return fmt.Errorf("Must specify at least one BBID.")
	}

	return nil
}

func (c *collectRun) readInput() (*pb.CollectConfig, error) {
	inputBytes, err := ioutil.ReadFile(c.inputJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed reading input_json\n%w", err)
	}
	req := &pb.CollectConfig{}
	if err := unmarshaler.Unmarshal(bytes.NewReader(inputBytes), req); err != nil {
		return nil, fmt.Errorf("Couldn't decode %s as a CollectConfig\n%w", c.inputJSON, err)
	}
	return req, nil
}

type CollectOutput struct {
	// Final set of BBIDs (only includes the most recent retry for each build).
	BBIDs  []string       `json:"bbids"`
	Report *CollectReport `json:"report"`
}

// writeOutput writes the given BBIDs to the path provided by --output_json.
func (c *collectRun) writeOutput(output *CollectOutput) error {
	sort.Strings(output.BBIDs)
	data, err := json.MarshalIndent(*output, "", " ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(c.outputJSON, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	f.Sync()
	return nil
}

func (c *collectRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	c.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	c.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	c.bbClient = bb.NewClient(c.cmdRunner, c.stdoutLog, c.stderrLog)

	ctx := context.Background()
	if err := c.bbClient.EnsureLUCIToolsAuthed(ctx, "bb", "led"); err != nil {
		c.LogErr(err.Error())
		// TODO(b/264680777): Factor return_codes.go out of try and use those.
		return 1
	}

	if err := c.validate(); err != nil {
		c.LogErr(err.Error())
		return 2
	}

	collectConfig, err := c.readInput()
	if err != nil {
		c.LogErr(err.Error())
		return 3
	}

	output, err := c.Collect(ctx, collectConfig, c.initialRetry)
	if err != nil {
		c.LogErr(err.Error())
		c.LogOut("Working set of builds: %s", strings.Join(output.BBIDs, ","))
		if err := c.writeOutput(output); err != nil {
			c.LogErr(errors.Annotate(err, "error writing BBIDs to output").Err().Error())
		}
		return 4
	}

	// TODO: move BBIDs under a field in the output, add e2e test.
	c.LogOut("Final set of builds (including only most recent retries): %s", strings.Join(output.BBIDs, ","))
	if err := c.writeOutput(output); err != nil {
		c.LogErr(errors.Annotate(err, "error writing BBIDs to output").Err().Error())
		return 5
	}

	return 0
}

// LogOut logs to stdout.
func (t *collectRun) LogOut(format string, a ...any) {
	if t.stdoutLog != nil {
		t.stdoutLog.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func (t *collectRun) LogErr(format string, a ...any) {
	if t.stderrLog != nil {
		t.stderrLog.Printf(format, a...)
	}
}

// GetApplication returns an instance of the application.
func GetApplication() *cli.Application {
	return &cli.Application{
		Name: "conductor",

		Context: func(ctx context.Context) context.Context {
			return ctx
		},

		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			cmdCollect(),
		},
	}
}

func main() {
	app := GetApplication()
	os.Exit(subcommands.Run(app, nil))
}
