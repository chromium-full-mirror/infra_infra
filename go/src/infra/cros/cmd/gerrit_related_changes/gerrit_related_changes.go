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

	"github.com/golang/protobuf/jsonpb"
	"github.com/maruel/subcommands"

	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	"go.chromium.org/luci/auth"
	lucigerrit "go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
)

var (
	unmarshaler = jsonpb.Unmarshaler{AllowUnknownFields: true}
)

type relatedRun struct {
	subcommands.CommandRunBase
	stdoutLog    *log.Logger
	stderrLog    *log.Logger
	cmdRunner    cmd.CommandRunner
	gerritClient gerrit.Client

	inputJSON  string
	outputJSON string
}

func cmdRelated() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "gerrit_related_changes",
		ShortDesc: "Fetch info from Gerrit REST API about related changes.",
		CommandRun: func() subcommands.CommandRun {
			r := &relatedRun{}
			r.cmdRunner = cmd.RealCommandRunner{}
			r.Flags.StringVar(&r.inputJSON, "input_json", "", "Path to JSON input.")
			r.Flags.StringVar(&r.outputJSON, "output_json", "", "Path to write out final output.")
			return r
		}}
}

// validate ensures args for this command meet requirements.
func (r *relatedRun) validate() error {
	if r.inputJSON == "" {
		return fmt.Errorf("--input_json is required")
	}

	if r.outputJSON == "" {
		return fmt.Errorf("--output_json is required")
	}

	return nil
}

// CreateOAuthGerritClient returns a Gerrit client with OAuth scope.
func (r *relatedRun) CreateOAuthGerritClient(ctx context.Context) (gerrit.Client, error) {
	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.Scopes = append(authOpts.Scopes, lucigerrit.OAuthScope)

	authedClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return nil, err
	}

	gerritClient, err := gerrit.NewClient(authedClient)
	if err != nil {
		r.LogErr("Failed to create gerrit client")
		r.LogErr(fmt.Sprintf("Error: %s", err))
		return nil, err
	}

	return gerritClient, nil
}

type RelatedOutput struct {
	// Related Gerrit changes.
	Related      []gerrit.Change `json:"related"`
	RelatedCount int             `json:"relatedCount"`
	HasRelated   bool            `json:"hasRelated"`
}

// writeOutput writes the related changes to the path provided by --output_json.
func (r *relatedRun) writeOutput(output *RelatedOutput) error {
	data, err := json.MarshalIndent(output, "", " ")
	if err != nil {
		return err
	}
	// Overwrite if output file already exists.
	f, err := os.OpenFile(r.outputJSON, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	f.Sync()
	return nil
}

func (r *relatedRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	// Set up logging.
	r.stdoutLog = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	r.stderrLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

	// Validate command args.
	if err := r.validate(); err != nil {
		r.LogErr(err.Error())
		return 1
	}

	// Get authed gerrit client with OAuth scope.
	ctx := context.Background()

	// Do not create a gerritClient for test structs with a mockClient.
	if r.gerritClient == nil {
		gc, err := r.CreateOAuthGerritClient(context.Background())
		if err != nil {
			r.LogErr("Error: %s. Please run `cros try auth-login` from a ChromiumOS checkout and sign in with your @google.com account.", err)
			return 2
		}
		r.gerritClient = gc
	}

	// Get request info from input JSON.
	inputBytes, err := ioutil.ReadFile(r.inputJSON)
	if err != nil {
		r.LogErr(fmt.Sprintf("Failed reading %s. Error: %s", r.inputJSON, err))
		return 3
	}
	req := &pb.GerritChange{}
	if err := unmarshaler.Unmarshal(bytes.NewReader(inputBytes), req); err != nil {
		r.LogErr(fmt.Sprintf("Failed unmarshalling %s. Error: %s", r.inputJSON, err))
		return 4
	}
	host := req.Host
	changeNumber := int(req.Change)

	// Get the list of relatedChanges for a Gerrit change. If there are related
	// changes, they include changeNumber itself.
	relatedChanges, err := r.gerritClient.GetRelatedChanges(ctx, host, changeNumber)

	if err != nil {
		r.LogErr(fmt.Sprintf("Failed to check related changes for GetRelatedChanges(ctx, %s, %d): %s", host, changeNumber, err))
		return 5
	}

	// Report whether we're part of a relation chain or not.
	hasRelatedGerritChanges := len(relatedChanges) > 0

	if hasRelatedGerritChanges {
		r.LogOut("Found change: %d (host: %s) is part of a relation change.", changeNumber, host)
	} else {
		r.LogOut("Found change: %d (host: %s) is NOT part of a relation change.", changeNumber, host)
	}

	output := &RelatedOutput{
		Related:      relatedChanges,
		RelatedCount: len(relatedChanges), // Includes self if nonzero.
		HasRelated:   hasRelatedGerritChanges,
	}

	// Write output
	err = r.writeOutput(output)
	if err != nil {
		r.LogErr(fmt.Sprintf("Failed write output. Error: %s", err))
		return 6
	}

	return 0
}

// LogOut logs to stdout.
func (r *relatedRun) LogOut(format string, a ...interface{}) {
	if r.stdoutLog != nil {
		r.stdoutLog.Printf(format, a...)
	}
}

// LogErr logs to stderr.
func (r *relatedRun) LogErr(format string, a ...interface{}) {
	if r.stderrLog != nil {
		r.stderrLog.Printf(format, a...)
	}
}

// GetApplication returns an instance of the application.
func GetApplication() *cli.Application {
	return &cli.Application{
		Name:  "gerrit_related_changes",
		Title: `Gerrit CL related changes tool`,
		Context: func(ctx context.Context) context.Context {
			return ctx
		},

		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			cmdRelated(),
		},
	}
}

func main() {
	app := GetApplication()
	os.Exit(subcommands.Run(app, nil))
}
