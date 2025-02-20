// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package run holds all of the internal logic for the execution steps of a
// SuiteScheduler run.
package run

import (
	"context"

	cloudPubsub "cloud.google.com/go/pubsub"

	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth/client/authcli"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cros/cmd/kron/buildbucket"
	"go.chromium.org/infra/cros/cmd/kron/builds"
	"go.chromium.org/infra/cros/cmd/kron/common"
	"go.chromium.org/infra/cros/cmd/kron/configparser"
	"go.chromium.org/infra/cros/cmd/kron/totmanager"
)

// CrOSNewBuild3dCommand implements DDDCommand.
type CrOSNewBuild3dCommand struct {
	authOpts *authcli.Flags
	isProd   bool
	isTest   bool
	dryRun   bool

	labConfigs            *configparser.LabConfigs
	suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs
	all3dConfigs          configparser.ConfigList

	projectID string

	// buildPackagesMap stores list of builds per parent release orchestrator
	buildPackagesMap map[int64]*BuildPackage3d
}

// BuildPackage3d represents a struct responsible for managing pubsub messages
// and later building CTP requests for 3D configurations.
type BuildPackage3d struct {
	Branch   suschpb.Branch
	Builds   []*kronpb.Build
	Messages []*cloudPubsub.Message
}

// IsBuildStatusComplete checks if a given buildID is completed. Success or Fail are both completed state.
func isBuildStatusComplete(buildID int64, schedulerClient buildbucket.Scheduler) bool {
	build, err := schedulerClient.GetBuildStatus(buildID)
	if err != nil {
		common.Stderr.Printf("Failed to fetch build status for parent build id :%d. This build will be tried in next run to trigger new build 3d configs. %v", buildID, err)
		return false
	}
	if build == nil || int(build.GetStatus())&int(buildbucketpb.Status_ENDED_MASK) == 0 {
		return false
	}
	return true
}

// InitCrOSNewBuild3dCommand generates and returns a CrOS NEW_BUILD_3D client which
// implements the DDDCommand interface. This client does not handle
// firmware, Android, nor multi-DUT configs.
func InitCrOSNewBuild3dCommand(authOpts *authcli.Flags, isProd, dryRun, isTest bool, labConfigs *configparser.LabConfigs, suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs, projectID string) DDDCommand {
	return &CrOSNewBuild3dCommand{
		authOpts:              authOpts,
		isProd:                isProd,
		dryRun:                dryRun,
		isTest:                isTest,
		labConfigs:            labConfigs,
		suiteSchedulerConfigs: suiteSchedulerConfigs,
		projectID:             projectID,
		buildPackagesMap:      make(map[int64]*BuildPackage3d),
	}
}

// addToBuildPackagesMap appends kron build and pubsub messages to buildPackage3d based on parent build Id
func (c *CrOSNewBuild3dCommand) addToBuildPackagesMap(kronBuild *kronpb.Build, msg *cloudPubsub.Message) {
	parentBuildID := kronBuild.GetReleaseOrchBbid()
	if _, ok := c.buildPackagesMap[parentBuildID]; !ok {
		// Key doesn't exist, create a new BuildPackage3d and add to the lists.
		c.buildPackagesMap[parentBuildID] = &BuildPackage3d{
			Builds:   []*kronpb.Build{},
			Messages: []*cloudPubsub.Message{},
		}
	}
	c.buildPackagesMap[parentBuildID].Builds = append(c.buildPackagesMap[parentBuildID].Builds, kronBuild)
	c.buildPackagesMap[parentBuildID].Messages = append(c.buildPackagesMap[parentBuildID].Messages, msg)
}

// processBuildPackagesMap takes list of buildReports and forms map with parent build id as key and buildPackage3d
// as value. Identifies branch for each parent build id. Checks if parent buildf is complete. If not removes from map
// and nacks all associated pubsub messages for incomplete parent builds.
func (c *CrOSNewBuild3dCommand) processBuildPackagesMap(buildReports *[]*builds.BuildReportPackage) error {
	for _, buildReport := range *buildReports {
		kronBuild, err := builds.TransformReportToKronBuild(buildReport.Report)
		if err != nil {
			return err
		}
		c.addToBuildPackagesMap(kronBuild, buildReport.Message)
	}

	parentBuildsToRemove := []int64{}
	schedulerClient, err := buildbucket.InitScheduler(context.Background(), c.authOpts, c.isProd, c.dryRun)
	if err != nil {
		return err
	}
	for parentBuildID, buildPackage3d := range c.buildPackagesMap {
		// Identify branch and update build package
		branch, err := totmanager.IdentifyBranch(int(buildPackage3d.Builds[0].GetMilestone()))
		if err != nil {
			return err
		}
		buildPackage3d.Branch = branch

		if !isBuildStatusComplete(parentBuildID, schedulerClient) {
			common.Stdout.Printf("Release Orchestrator R%d-%s go/bbid/%d identified as %s branch is removed because it is still actively running. Has %d completed builds so far\n", buildPackage3d.Builds[0].Milestone, buildPackage3d.Builds[0].Version, parentBuildID, buildPackage3d.Branch, len(buildPackage3d.Builds))
			// Nack build messages for incomplete status parent builds
			for _, msg := range buildPackage3d.Messages {
				msg.Nack()
			}
			parentBuildsToRemove = append(parentBuildsToRemove, parentBuildID)
		} else {
			// Ack build messages for complete status parent builds
			for _, msg := range buildPackage3d.Messages {
				if c.isTest {
					msg.Nack()
				} else {
					msg.Ack()
				}
			}
		}
	}
	// Remove parent BBIDs which are still actively running.
	for _, parentBuildID := range parentBuildsToRemove {
		delete(c.buildPackagesMap, parentBuildID)
	}

	return nil
}

// Name returns the custom name of the command. This will be used in logging.
func (c *CrOSNewBuild3dCommand) Name() string {
	return "CrOSNewBuilds3d"
}

// FetchBuilds retrieves all builds currently sitting in the release team's
// completed build Pub/Sub queue.
func (c *CrOSNewBuild3dCommand) FetchBuilds() error {
	// Fetch BuildReports from the Release Pub/Sub firehose.
	common.Stdout.Println("Fetching builds from Pub/Sub.")

	// If we are in test mode then pull from the testing Pub/Sub subscription
	// where we do not ACK messages.
	subscriptionID := common.BuildsSubscription3d
	if c.isTest {
		subscriptionID = common.BuildsSubscription3dTesting
	}

	// NOTE: We are ignoring the response from this function because finalize function here
	// populates the buildPackagesMap field in CrOSNewBuild3dCommand.
	_, err := builds.IngestBuildsFromPubSub(c.projectID, subscriptionID, c.isProd, c.processBuildPackagesMap)
	if err != nil {
		return err

	}
	for parentBuildID, buildPackage3d := range c.buildPackagesMap {
		common.Stdout.Printf("Release Orchestrator R%d-%s go/bbid/%d identified as %s branch. Has %d completed builds.\n", buildPackage3d.Builds[0].Milestone, buildPackage3d.Builds[0].Version, parentBuildID, buildPackage3d.Branch, len(buildPackage3d.Builds))
	}
	return nil
}

// buildCTPRequestsFor3dConfigs creates list of ctp requests for each config per branch.
func buildCTPRequestsFor3dConfigs(buildPackagesMap map[int64]*BuildPackage3d, all3dConfigs configparser.ConfigList, newBuild3dMap map[*suschpb.SchedulerConfig]map[configparser.BuildTarget]bool) (map[suschpb.Branch]map[*suschpb.SchedulerConfig][]*ctpEvent, error) {
	eventsPerBranchPerConfig := map[suschpb.Branch]map[*suschpb.SchedulerConfig][]*ctpEvent{}

	// processing each config
	for _, config := range all3dConfigs {
		buildTargetsMap := newBuild3dMap[config]
		// processing for each release orch. If the milestone for release orch is targeted branch for config, then ctpEvents is created else skipped
		for _, buildPackage3d := range buildPackagesMap {
			// skip if no builds
			if len(buildPackage3d.Builds) == 0 {
				continue
			}
			// if the branch corresponding to this release orchestrator is found in the targeted branch configuration,
			// generate CTP requests for all child builds.
			targeted, _, err := totmanager.IsTargetedBranch(int(buildPackage3d.Builds[0].GetMilestone()), config.Branches)
			if err != nil {
				return nil, err
			}
			if targeted {
				allCtpEvents := []*ctpEvent{}
				// create ctpRequest for each build in release orchestrator
				for _, kronBuild := range buildPackage3d.Builds {
					// // skip if buildTarget is not targeted
					if _, ok := buildTargetsMap[configparser.BuildTarget(kronBuild.BuildTarget)]; !ok {
						common.Stdout.Printf("3dConfig:%s, skipping build target:%s\n", config.Name, kronBuild.BuildTarget)
						continue
					}
					ctpRequests, err := buildPerModelConfigs(nil, config, kronBuild, suschpb.Branch_name[int32(buildPackage3d.Branch)])
					if err != nil {
						return nil, err
					}
					allCtpEvents = append(allCtpEvents, ctpRequests...)
				}

				if _, ok := eventsPerBranchPerConfig[buildPackage3d.Branch]; !ok {
					eventsPerBranchPerConfig[buildPackage3d.Branch] = map[*suschpb.SchedulerConfig][]*ctpEvent{
						config: {},
					}
				}
				eventsPerBranchPerConfig[buildPackage3d.Branch][config] = append(eventsPerBranchPerConfig[buildPackage3d.Branch][config], allCtpEvents...)

			}
		}
	}
	// logging map
	common.Stdout.Printf("Preparing requests for ...\n")
	for branch, configMap := range eventsPerBranchPerConfig {
		for _, events := range configMap {
			for _, event := range events {
				common.Stdout.Printf("Config:%s for branch:%s using build %s\n", event.config.Name, branch.String(), event.event.BuildUuid)
			}
		}
	}

	return eventsPerBranchPerConfig, nil
}

// FetchTriggeredConfigs returns a map where the keys are release orchestrator bbid values
// and the values are lists of completed builds. The function ensures that the map only
// contains entries for release orchestrators that have completed.
func (c *CrOSNewBuild3dCommand) FetchTriggeredConfigs() error {
	c.all3dConfigs = c.suiteSchedulerConfigs.FetchAllNewBuild3dConfigs()
	common.Stdout.Printf("Length of 3d configs fetched : %d\n", len(c.all3dConfigs))
	return nil
}

// batchCTPRequests3d groups all events per config per branch in one batch.
//
// NOTE: Requests in these batches will all share the same SuiteScheduler
// Config.
func batchCTPRequests3d(eventsPerBranchPerConfig map[suschpb.Branch]map[*suschpb.SchedulerConfig][]*ctpEvent, isProd, dryRun bool) ([]*ctpEventBatch, error) {
	eventBatches := []*ctpEventBatch{}

	// Create batches for each config per branch
	for _, configMap := range eventsPerBranchPerConfig {
		for _, events := range configMap {
			branchBatches, err := batchCTPRequests(events, isProd, dryRun, doNotLimitBatches)
			if err != nil {
				return nil, err
			}

			eventBatches = append(eventBatches, branchBatches...)
		}
	}

	return eventBatches, nil
}

// ScheduleRequests generates CTP Requests, batches them into BuildBucket
// requests, and Schedules them via the BuildBucket API.
func (c *CrOSNewBuild3dCommand) ScheduleRequests() error {
	// Build CTP Requests for all 3d configs.
	eventsPerBranchPerConfig, err := buildCTPRequestsFor3dConfigs(c.buildPackagesMap, c.all3dConfigs, c.suiteSchedulerConfigs.FetchNewBuild3dMap())
	if err != nil {
		return err
	}

	// check if map is empty
	if len(eventsPerBranchPerConfig) == 0 {
		common.Stdout.Println("No CTP requests to schedule")
		return nil
	}

	if !c.isProd {
		for branch, configMap := range eventsPerBranchPerConfig {
			for config, events := range configMap {
				eventsPerBranchPerConfig[branch][config] = onlyStagingRequests(events)
			}
		}
	}

	// Create batches request for scheduling. Each batch represents one config per branch
	batches, err := batchCTPRequests3d(eventsPerBranchPerConfig, c.isProd, c.dryRun)
	if err != nil {
		return err
	}

	return scheduleBatches(batches, c.isProd, c.dryRun, c.projectID, c.authOpts)
}
