// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package run holds all of the internal logic for the execution steps of a
// SuiteScheduler run.
package run

import (
	"context"
	"fmt"

	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth/client/authcli"

	"infra/cros/cmd/kron/builds"
	"infra/cros/cmd/kron/cloudsql"
	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/configparser"
	"infra/cros/cmd/kron/pubsub"
)

const (
	disallowPublishErrors = false
	publishEventsToPubSub = true
	isWriter              = true
)

// CrOSNewBuildCommand implements NewBuildCommand.
type CrOSNewBuildCommand struct {
	authOpts *authcli.Flags
	isProd   bool
	isTest   bool
	dryRun   bool

	labConfigs            *configparser.LabConfigs
	suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs

	projectID string

	// Used to store builds made in the finalize step to reduce duplicate
	// generation.
	kronBuilds []*kronpb.Build
}

// InitCrOSNewBuildCommand generates and returns a CrOS NEW_BUILD client which
// implements the NewBuildCommand interface. This client does not handle
// firmware, Android, nor multi-DUT configs.
func InitCrOSNewBuildCommand(authOpts *authcli.Flags, isProd, dryRun, isTest bool, labConfigs *configparser.LabConfigs, suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs, projectID string) NewBuildCommand {
	return &CrOSNewBuildCommand{
		authOpts:              authOpts,
		isProd:                isProd,
		dryRun:                dryRun,
		isTest:                isTest,
		labConfigs:            labConfigs,
		suiteSchedulerConfigs: suiteSchedulerConfigs,
		projectID:             projectID,
	}
}

// Name returns the custom name of the command. This will be used in logging.
func (c *CrOSNewBuildCommand) Name() string {
	return "CrOSNewBuilds"
}

// publishBuildReports generates and publishes a Kron Build message to Pub/Sub
// for each of the successful release builds ingested.
//
// NOTE: To minimize function loss on flaky network issues, each publish action
// is hermetic and will not cause program halt on publishing errors.
//
// NOTE: This function is being handed to the pub/sub ingestion logic as the
// finalize() command.
func (c *CrOSNewBuildCommand) publishBuildReports(buildReports *[]*builds.BuildReportPackage) error {
	ctx := context.Background()
	// Exit early if no buildReports were received from the Pub/Sub queue. This
	// is not an error, it just means that all builds have completed for the day
	// or are in flight.
	if len(*buildReports) == 0 {
		return nil
	}

	common.Stdout.Printf("Initializing client for pub sub topic %s on project %s", common.BuildsPubSubTopic, c.projectID)
	psClient, err := pubsub.InitPublishClient(ctx, c.projectID, common.BuildsPubSubTopic)
	if err != nil {
		return err
	}

	// Initialize PSQL client for long term storage insertion.
	sqlClient, err := cloudsql.InitBuildsClient(ctx, c.isProd, isWriter)
	if err != nil {
		return err
	}

	// Transform build reports to Kron Builds and publish to Pub/Sub.
	publishedReports := []*builds.BuildReportPackage{}
	for _, report := range *buildReports {
		kronBuild, err := builds.TransformReportToKronBuild(report.Report)
		if err != nil {
			return err
		}

		// If we are running in test mode then do not publish build
		// information to the metrics pipeline and Nack all received messages.
		if c.isTest {
			report.Message.Nack()
			// Add to the list of Kron builds to be used in this Kron run.
			c.kronBuilds = append(c.kronBuilds, kronBuild)

			// publishedReports will be used to replace the passed in buildReports
			// at the end. This is because this functions is supposed to change the
			// values of the slice in-place rather than via return.
			publishedReports = append(publishedReports, report)

			// Continue so that we do not proceed to the publishing and acking
			// step.
			continue
		}

		// Publish the kron build to the metrics pipeline and LTS PSQL storage.
		if err = publishBuild(ctx, kronBuild, psClient, sqlClient); err != nil {
			common.Stderr.Println(err)
			// If we failed to republish the message then we should nack the
			// build to be ingested again on the next Kron invocation.
			report.Message.Nack()
			continue
		}

		// The build was successfully retrieved by the metrics pipeline. Ack the
		// message to remove it from the Pub/Sub queue.
		report.Message.Ack()

		// Add to the list of Kron builds to be used in this Kron run.
		c.kronBuilds = append(c.kronBuilds, kronBuild)

		// publishedReports will be used to replace the passed in buildReports
		// at the end. This is because this functions is supposed to change the
		// values of the slice in-place rather than via return.
		publishedReports = append(publishedReports, report)
	}

	// If no kronBuilds were made then that means all publish attempts failed.
	// we will want to halt the run as a deep issue is going on.
	//
	// NOTE: We check to make sure that the provided buildReports value is non
	// nil at the start of the function. If that check is removed then this will
	// cause failures during the quieter periods of the day.
	if len(c.kronBuilds) == 0 {
		return fmt.Errorf("all builds failed to publish")
	}

	// Set the slice to the newly published slice.
	//
	// NOTE: the reason that we are not returning this values is because we are
	// modifying the slice in-place as this is being handled in a goroutine.
	*buildReports = publishedReports
	return nil
}

// FetchBuilds retrieves all builds currently sitting in the release team's
// completed build Pub/Sub queue. We then convert each valid report to a kron
// build for later use. Publishing to the metrics pipeline is performed here as
// well.
func (c *CrOSNewBuildCommand) FetchBuilds() ([]*kronpb.Build, error) {
	// Fetch BuildReports from the Release Pub/Sub firehose.
	common.Stdout.Println("Fetching builds from Pub/Sub.")

	// If we are in test mode then pull from the testing Pub/Sub subscription
	// where we do not ACK messages.
	subscriptionID := common.BuildsSubscription
	if c.isTest {
		subscriptionID = common.BuildsSubscriptionTesting
	}

	// NOTE: We are ignoring the response from this function because our helper
	// function gives us the list of Kron builds as a struct field.
	_, err := builds.IngestBuildsFromPubSub(c.projectID, subscriptionID, c.isProd, c.publishBuildReports)
	if err != nil {
		return nil, err

	}

	return c.kronBuilds, nil
}

// FetchTriggeredConfigs takes in a list of kron builds and finds which
// SuiteScheduler Configs they trigger. This is then organized into a map to be
// used by the next stage in the pipeline.
func (c *CrOSNewBuildCommand) FetchTriggeredConfigs(kronBuilds []*kronpb.Build) (map[*kronpb.Build][]*suschpb.SchedulerConfig, error) {
	return fetchTriggeredConfigs(kronBuilds, c.suiteSchedulerConfigs.FetchNewBuildConfigsByBuildTarget)
}

// ScheduleRequests generates CTP Requests, batches them into BuildBucket
// requests, and Schedules them via the BuildBucket API.
func (c *CrOSNewBuildCommand) ScheduleRequests(kronBuildMap map[*kronpb.Build][]*suschpb.SchedulerConfig) error {
	return scheduleRequests(kronBuildMap, c.suiteSchedulerConfigs, c.authOpts, c.projectID, c.isProd, c.dryRun)
}
