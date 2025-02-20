// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/maruel/subcommands"
	"github.com/pkg/errors"
	"google.golang.org/api/option"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"

	"go.chromium.org/infra/cros/cmd/suite_publisher/internal/bqsuites"
	"go.chromium.org/infra/cros/cmd/suite_publisher/internal/parse"
)

const (
	defaultProject      = "cros-test-analytics"
	defaultDataset      = "testinfo"
	defaultTable        = "centralized_suites"
	defaultClosureTable = "centralized_suite_closures"
)

type suitePublisher struct {
	subcommands.CommandRunBase
	authFlags         authcli.Flags
	suiteProtoPath    string
	suiteSetProtoPath string
	dataset           string
	project           string
	buildTarget       string
	milestone         string
	version           string
}

func cmdSuitePublisher(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "publish [flags]",
		ShortDesc: "Publish suite/suiteset proto files to BigQuery",
		CommandRun: func() subcommands.CommandRun {
			b := &suitePublisher{}
			b.authFlags = authcli.Flags{}
			b.authFlags.Register(b.GetFlags(), authOpts)
			b.Flags.StringVar(&b.suiteProtoPath, "suite-proto", "",
				"Path to Suite proto file to use.")
			b.Flags.StringVar(&b.suiteSetProtoPath, "suiteset-proto", "",
				"Path to SuiteSet proto file to use.")
			b.Flags.StringVar(&b.dataset, "dataset", defaultDataset,
				"Bigquery dataset to use.")
			b.Flags.StringVar(&b.project, "project", defaultProject,
				"GCP project to publish to.")
			b.Flags.StringVar(&b.buildTarget, "build-target", "",
				"ChromeOS build target to label suites with.")
			b.Flags.StringVar(&b.milestone, "milestone", "",
				"ChromeOS milestone to label suites with.")
			b.Flags.StringVar(&b.version, "version", "",
				"ChromeOS version to use label suites with.")
			return b
		}}
}

func (s *suitePublisher) validate() error {
	if len(s.suiteProtoPath) == 0 {
		return fmt.Errorf("must specify --suite-proto")
	}
	if len(s.suiteSetProtoPath) == 0 {
		return fmt.Errorf("must specify --suiteset-proto")
	}
	if len(s.buildTarget) == 0 {
		return fmt.Errorf("must specify --build-target")
	}
	if len(s.milestone) == 0 {
		return fmt.Errorf("must specify --milestone")
	}
	if len(s.version) == 0 {
		return fmt.Errorf("must specify --version")
	}
	return nil
}

func (s *suitePublisher) newAuthenticator(ctx context.Context) (*auth.Authenticator, error) {
	authOpts, err := s.authFlags.Options()
	if err != nil {
		return nil, err
	}
	authenticator := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts)
	return authenticator, nil
}

// Run is the main entry point for the publish command, it will parse the args
// and then do the work to publish the suites to BigQuery.
func (s *suitePublisher) Run(a subcommands.Application, args []string, env subcommands.Env) (ret int) {
	ctx := context.Background()
	authenticator, err := s.newAuthenticator(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to create authenticator")
		LogErr(err.Error())
		return 2
	}
	authToken, err := authenticator.TokenSource()
	if err != nil {
		err = errors.Wrap(err, "failed to create token source")
		LogErr(err.Error())
		return 3
	}
	bqClient, err := bigquery.NewClient(ctx, s.project, option.WithTokenSource(authToken))
	if err != nil {
		err = errors.Wrap(err, "failed to create bigquery client")
		LogErr(err.Error())
		return 4
	}
	ret = 0
	defer func() {
		if err := bqClient.Close(); err != nil {
			err = errors.Wrap(err, "failed to close bigquery client")
			LogErr(err.Error())
			ret = 1
		}
	}()

	if err := s.publishSuites(ctx, bqClient); err != nil {
		err = errors.Wrap(err, "failed to publish suites")
		LogErr(err.Error())
		ret = 1
	}
	return ret
}

// publishSuites is responsible for doing the actual work of reading protobuf files
// and publishing them to BigQuery.
func (s *suitePublisher) publishSuites(ctx context.Context, bqClient *bigquery.Client) error {
	if err := s.validate(); err != nil {
		return err
	}

	suites, err := parse.ReadSuitesAndSuiteSets(s.suiteProtoPath, s.suiteSetProtoPath)
	if err != nil {
		return err
	}

	inserter := bqClient.Dataset(s.dataset).Table(defaultTable).Inserter()
	closureInserter := bqClient.Dataset(s.dataset).Table(defaultClosureTable).Inserter()
	build := bqsuites.BuildInfo{
		BuildTarget:   s.buildTarget,
		CrosMilestone: s.milestone,
		CrosVersion:   s.version,
	}
	for _, s := range suites {
		LogOut("Publishing %s\n", s.ID())
		p := &bqsuites.PublishInfo{
			Suite: s,
			Build: build,
		}
		if err := bqsuites.PublishSuite(ctx, inserter, p); err != nil {
			return err
		}
		c, err := s.Closures(suites)
		if err != nil {
			return err
		}
		var closures []*bqsuites.ClosurePublishInfo
		for _, closure := range c {
			closures = append(closures, &bqsuites.ClosurePublishInfo{
				Closure: closure,
				Build:   build,
			})
		}
		if err := bqsuites.PublishSuiteClosures(ctx, closureInserter, closures); err != nil {
			return err
		}
	}

	return nil
}
