// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	androidlib "go.chromium.org/infra/cros/cmd/common_lib/android_api"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// TestCommand executed the provisioning as a Server
type TestCommand struct {
	invocationID string
	workunitID   string
	testResultID int64
	flagSet      *flag.FlagSet
}

func NewTestCommand() *TestCommand {
	tc := &TestCommand{
		flagSet: flag.NewFlagSet("test", flag.ContinueOnError),
	}

	tc.flagSet.StringVar(&tc.invocationID, "invocation-id", "", "invocation ID to get details for.")
	tc.flagSet.StringVar(&tc.workunitID, "workunit-id", "", "workunit ID to get details for.")
	tc.flagSet.Int64Var(&tc.testResultID, "testresult-id", int64(0), "testresult ID to get details for.")
	return tc
}

func (tc *TestCommand) Init(args []string) error {
	err := tc.flagSet.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

func (tc *TestCommand) Run() error {
	if err := tc.Init(os.Args[2:]); err != nil {
		return err
	}

	log.Printf("running test mode:")
	ctx := context.Background()

	s, err := androidlib.NewAndroidBuildService(ctx, androidlib.Local, common.Prod)
	if err != nil {
		return err
	}

	if tc.invocationID != "" {
		inv, err := s.InvocationService.Get(ctx, tc.invocationID)
		if err != nil {
			fmt.Println("cannot get invocation details due to: ", err)
		}
		fmt.Println("\n ---------- \nInvocation ID details: ")
		fmt.Printf("%#v", inv)

		resp, err := s.WorkUnitService.List(ctx, tc.invocationID, androidlib.AndroidBuildAPIOptions{})
		if err != nil {
			fmt.Println("cannot get work units for this invocation due to: ", err)
		}
		fmt.Println("\n ---------- \nWorkunits in this invocation: ")
		for _, wu := range resp.WorkUnits {
			fmt.Printf("%#v\n", wu)
		}

		trResp, err := s.TestResultService.List(ctx, tc.invocationID, androidlib.AndroidBuildAPIOptions{})
		if err != nil {
			fmt.Println("cannot get test results for this invocation due to: ", err)
		}
		fmt.Println("\n ---------- \nTest results in this invocation: ")
		for _, tr := range trResp.TestResults {
			fmt.Printf("%#v\n", tr)
			fmt.Printf("Test identifier: %+v\n\n", tr.TestIdentifier)
		}
	}

	if tc.workunitID != "" {
		wu, err := s.InvocationService.Get(ctx, tc.invocationID)
		if err != nil {
			fmt.Println("cannot get work unit details due to: ", err)
		}
		fmt.Println("\n ---------- \nWork unit details: ")
		fmt.Printf("%#v", wu)
	}

	if tc.testResultID > 0 {
		tr, err := s.TestResultService.Get(tc.testResultID)
		if err != nil {
			fmt.Println("cannot get test result details due to: ", err)
		}
		fmt.Println("\n ---------- \nTest Result details: ")
		fmt.Printf("%#v\n", tr)
	}

	return nil
}
