// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package subcommands includes subcommand logic that will be used for the CLI
// front end.
package subcommands

import (
	"context"
	"fmt"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"

	"infra/cros/cmd/kron/buildbucket"
	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/configparser"
	"infra/cros/cmd/kron/metrics"
	"infra/cros/cmd/kron/pubsub"
	"infra/cros/cmd/kron/run"
	"infra/cros/cmd/kron/totmanager"
)

type runCommand struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	authOpts  *auth.Options

	runID              string
	buildBucketID      string
	configCFGInputPath string
	labCFGInputPath    string
	isProd             bool
	isTest             bool
	dryRun             bool
	newBuilds          bool
	timedEvents        bool
	newBuilds3d        bool
	multiDUT           bool
}

// setFlags adds also CLI flags to the subcommand.
func (c *runCommand) setFlags() {
	// TODO(b/319463660): Allow for execution time to be set for TIMED_EVENTS
	// TODO(TBD): Allow for execution of only specified types of TIMED_EVENTS.
	// E.g. (DAILY | WEEKLY), (DAILY), (FORTNIGHTLY | WEEKLY), etc.

	c.Flags.StringVar(&c.runID, "run-id", common.DefaultString, "Used to manually set the runID. Should only be used by the recipe builder.")
	c.Flags.StringVar(&c.buildBucketID, "bbid", common.DefaultString, "Used to pass in the BBID of the current kron builder running for use as a parentBBID in the build tree. Should only be used by the recipe builder.")
	c.Flags.StringVar(&c.configCFGInputPath, "config-input-path", common.DefaultString, "Provide if a local version of the config .cfg is planned on being used. If omitted, the program will fetch the ToT config .cfg from gerrit.")
	c.Flags.StringVar(&c.labCFGInputPath, "lab-input-path", common.DefaultString, "Provide if a local version of the lab .cfg is planned on being used. If omitted, the program will fetch the ToT lab .cfg from gerrit.")

	c.Flags.BoolVar(&c.isTest, "test", false, "Run the binary as a test. This means that we will pull from the testing Pub/Sub, Nack all Messages, and run in the staging environment.")
	c.Flags.BoolVar(&c.isProd, "prod", false, "Run using prod environments.")
	c.Flags.BoolVar(&c.dryRun, "dry-run", false, "Do not send the builds to BB.")

	// TODO(b/319464677): Implement a backfill run command

	c.Flags.BoolVar(&c.newBuilds, "new-builds", false, "Check for new build images and launch NEW_BUILD type suites.")
	c.Flags.BoolVar(&c.timedEvents, "timed-events", false, "Launch TIMED_EVENT suites which are eligible to be triggered.")
	c.Flags.BoolVar(&c.newBuilds3d, "new-builds-3d", false, "Check for all new builds completion and launch 3d suites")
	c.Flags.BoolVar(&c.multiDUT, "multidut", false, "Launch MULTI_DUT configs.")
}

// validate ensures that the provided flags are being used in an expected
// manner.
func (c *runCommand) validate() error {
	if !c.newBuilds && !c.timedEvents && !c.newBuilds3d && !c.multiDUT {
		return fmt.Errorf("-new-builds, -timed-events, -new-builds-3d, or -multidut must be specified")
	}

	if totmanager.GetTot() == 0 {
		return fmt.Errorf("totmanager was not initialized")
	}

	if c.isTest && c.isProd {
		return fmt.Errorf("-test can only be run in the staging environment")
	}

	if c.buildBucketID != common.DefaultString {
		if buildbucket.GetSwarmingParentTaskID() == common.DefaultString {
			return fmt.Errorf("-bbid can only be used within a BuildBucket builder")
		}
	}

	return nil
}

// endRun ends the timer and publishes the message to pubsub.
func endRun(isProd bool) error {
	err := metrics.SetEndTime()
	if err != nil {
		return err
	}

	run := metrics.GenerateRunMessage()

	data, err := protojson.Marshal(run)
	if err != nil {
		return err
	}

	projectID := common.StagingProjectID
	if isProd {
		projectID = common.ProdProjectID
	}

	client, err := pubsub.InitPublishClient(context.Background(), projectID, common.RunsPubSubTopic)
	if err != nil {
		return err
	}

	err = client.PublishMessage(context.Background(), data)
	if err != nil {
		return err
	}

	return nil
}

// Run is the "main" function of the subcommand.
func (c *runCommand) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	// Initialize the ToT Manager at the start of the run. If this isn't
	// initialized then no builds will be targeted as ToT will be set to 0 by
	// default.
	common.Stdout.Printf("Initializing ToTManager")
	err := totmanager.InitTotManager()
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}

	// Ingest lab configs into memory.
	common.Stdout.Println("Fetch lab configs")
	labConfigs, err := configparser.FetchLabConfigs(c.labCFGInputPath, c.authOpts)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}

	// Ingest SuiteScheduler configs into memory.
	common.Stdout.Println("Fetch SuSch configs")
	suiteSchedulerConfigs, err := configparser.FetchSchedulerConfigs(c.configCFGInputPath, labConfigs, c.authOpts)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}

	// Determine which GCP Project should be used for the run.
	projectID := common.StagingProjectID
	if c.isProd {
		projectID = common.ProdProjectID
	}

	common.Stdout.Println("Validating flags")
	err = c.validate()
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}

	if c.buildBucketID != common.DefaultString {
		common.Stdout.Printf("Setting the parentBBID value as %s", c.buildBucketID)
		err := buildbucket.SetParentBuildBucketID(c.buildBucketID)
		if err != nil {
			common.Stderr.Println(err)
			return 1
		}
	}

	common.Stdout.Printf("Running Kron... ")

	// Set the RunID for the entire execution run.
	err = metrics.SetSuiteSchedulerRunID(c.runID)
	if err != nil {
		common.Stderr.Println(err)
		return 1
	}
	common.Stdout.Printf("runID: %s\n", metrics.GetRunID())

	// Start the clock for the run metrics
	err = metrics.SetStartTime()
	if err != nil {
		// Stop run timer and publish the message to pubsub
		endRunErr := endRun(c.isProd)
		if endRunErr != nil {
			common.Stderr.Println(err)
		}

		common.Stderr.Println(err)
		return 1
	}

	// nbCommands stores the NEW_BUILD-esque commands chosen to run according to
	// the passed in CLI flags.
	nbCommands := []run.NewBuildCommand{}

	// timedEventCommands stores the TIMED_EVENT-esque commands chosen to run
	// according to the passed in CLI flags.
	//
	// TODO(b/319463660): Launch these commands like the NEW_BUILD commands once
	// we have implemented the final parts of the timed events pipeline.
	timedEventCommands := []run.TimedEventCommand{}

	// dddCommands stores the NEW_BUILD-3D-esque commands chosen to run according to
	// the passed in CLI flags.
	dddCommands := []run.DDDCommand{}

	// Launch execution path for NEW_BUILD type configs
	if c.newBuilds {
		nbCommands = append(nbCommands, run.InitCrOSNewBuildCommand(&c.authFlags, c.isProd, c.dryRun, c.isTest, labConfigs, suiteSchedulerConfigs, projectID))
	}

	if c.multiDUT {
		nbCommands = append(nbCommands, run.InitCrOSMultiDUTCommand(&c.authFlags, c.isProd, c.dryRun, c.isTest, labConfigs, suiteSchedulerConfigs, projectID))
	}

	// Launch execution path for all TIMED_EVENT configs
	if c.timedEvents {
		timedEventCommands = append(timedEventCommands, run.InitCrOSTimedEventCommand(&c.authFlags, c.isProd, c.dryRun, c.isTest, labConfigs, suiteSchedulerConfigs, projectID))
	}

	if c.newBuilds3d {
		dddCommands = append(dddCommands, run.InitCrOSNewBuild3dCommand(&c.authFlags, c.isProd, c.dryRun, c.isTest, labConfigs, suiteSchedulerConfigs, projectID))
	}

	// Run all NEW_BUILD type command types requested. Each run is hermetic and
	// will not affect the runs of the other commands
	for _, nbCommand := range nbCommands {
		common.Stdout.Printf("**********************************************")
		common.Stdout.Printf("Launching %s", nbCommand.Name())
		common.Stdout.Printf("**********************************************")
		err = run.RunNewBuildCommand(nbCommand)
		if err != nil {
			common.Stderr.Printf("%s terminated with error: %s", nbCommand.Name(), err)
			continue
		}
		common.Stdout.Printf("**********************************************")
		common.Stdout.Printf("Done launching %s", nbCommand.Name())
		common.Stdout.Printf("**********************************************")
	}

	// Run all NEW_BUILD type command types requested. Each run is hermetic and
	// will not affect the runs of the other commands
	timeEventTriggerTime := common.TimeToKronTime(time.Now())
	for _, teCommand := range timedEventCommands {
		common.Stdout.Printf("**********************************************")
		common.Stdout.Printf("Launching %s", teCommand.Name())
		common.Stdout.Printf("**********************************************")
		err = run.RunTimedEventsCommand(teCommand, timeEventTriggerTime)
		if err != nil {
			common.Stderr.Printf("%s terminated with error: %s", teCommand.Name(), err)
			continue
		}
		common.Stdout.Printf("**********************************************")
		common.Stdout.Printf("Done launching %s", teCommand.Name())
		common.Stdout.Printf("**********************************************")
	}

	// Run all 3D type command types requested. Each run is hermetic and
	// will not affect the runs of the other commands
	for _, dddCommand := range dddCommands {
		common.Stdout.Printf("**********************************************")
		common.Stdout.Printf("Launching %s", dddCommand.Name())
		common.Stdout.Printf("**********************************************")
		err = run.RunDDDCommand(dddCommand)
		if err != nil {
			common.Stderr.Printf("%s terminated with error: %s", dddCommand.Name(), err)
			continue
		}
		common.Stdout.Printf("**********************************************")
		common.Stdout.Printf("Done launching %s", dddCommand.Name())
		common.Stdout.Printf("**********************************************")
	}

	// Stop run timer and publish the message to pubsub
	endRunErr := endRun(c.isProd)
	if endRunErr != nil {
		common.Stderr.Println(err)
		return 1
	}

	return 0

}

// GetRunCommand forms and returns the encapsulated Run subcommand for CLI use.
func GetRunCommand(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "run",
		LongDesc:  "The run command is used to launch full SuiteScheduler executions.",
		ShortDesc: "The run command is used to launch full SuiteScheduler executions.",
		CommandRun: func() subcommands.CommandRun {
			cmd := &runCommand{}
			cmd.authFlags = authcli.Flags{}
			cmd.authFlags.Register(cmd.GetFlags(), authOpts)
			cmd.setFlags()
			cmd.authOpts = &authOpts
			return cmd
		},
	}
}
