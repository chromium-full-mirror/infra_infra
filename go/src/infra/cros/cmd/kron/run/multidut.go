// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"fmt"
	"strconv"

	"go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	requestpb "go.chromium.org/chromiumos/infra/proto/go/test_platform"
	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth/client/authcli"

	"infra/cros/cmd/kron/builds"
	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/configparser"
	"infra/cros/cmd/kron/ctprequest"
	"infra/cros/cmd/kron/metrics"
	"infra/cros/cmd/kron/pubsub"
	"infra/cros/cmd/kron/totmanager"
)

// CrOSMultiDUTCommand implements NewBuildCommand.
type CrOSMultiDUTCommand struct {
	authOpts *authcli.Flags
	isProd   bool
	isTest   bool
	dryRun   bool

	labConfigs            *configparser.LabConfigs
	suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs

	projectID string
}

// InitCrOSMultiDUTCommand generates and returns a CrOS MULTI_DUT client which
// implements the NewBuildCommand interface. This client will only handle
// MULTI_DUT configs.
func InitCrOSMultiDUTCommand(authOpts *authcli.Flags, isProd, dryRun, isTest bool, labConfigs *configparser.LabConfigs, suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs, projectID string) NewBuildCommand {
	return &CrOSMultiDUTCommand{
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
func (c *CrOSMultiDUTCommand) Name() string {
	return "CrOSMultiDUT"
}

func (c *CrOSMultiDUTCommand) finalizeIngest(buildReports *[]*builds.BuildReportPackage) error {
	// Exit early if no buildReports were received from the Pub/Sub queue. This
	// is not an error, it just means that all builds have completed for the day
	// or are in flight.
	if len(*buildReports) == 0 {
		return nil
	}

	for _, report := range *buildReports {
		// If we are testing then do not Ack the message as we want to store
		// messages in this queue for rapid testing.
		if c.isTest {
			report.Message.Nack()
		} else {
			report.Message.Ack()
		}
	}

	return nil
}

// FetchBuilds gathers builds out of the Release Pub/Sub stream and converts
// them to kronpb.Builds.
func (c *CrOSMultiDUTCommand) FetchBuilds() ([]*kronpb.Build, error) {
	// Fetch BuildReports from the Release Pub/Sub firehose.
	common.Stdout.Println("Fetching builds from Pub/Sub.")

	// If we are in test mode then pull from the testing Pub/Sub subscription
	// where we do not ACK messages.
	subscriptionID := common.BuildsSubscriptionMultiDUT
	if c.isTest {
		subscriptionID = common.BuildsSubscriptionTesting
	}

	// NOTE: We are ignoring the response from this function because our helper
	// function gives us the list of Kron builds as a struct field.
	buildReports, err := builds.IngestBuildsFromPubSub(c.projectID, subscriptionID, c.isProd, c.finalizeIngest)
	if err != nil {
		return nil, err
	}

	// Transform the release BuildReports into Kron usable kronpb.Build.
	kronBuilds := []*kronpb.Build{}
	for _, report := range buildReports {
		kronBuild, err := builds.TransformReportToKronBuild(report.Report)
		if err != nil {
			return nil, err
		}

		kronBuilds = append(kronBuilds, kronBuild)
	}

	return kronBuilds, nil
}

// FetchTriggeredConfigs takes in the list of received builds and gathers all
// configs which track those buildTargets.
func (c *CrOSMultiDUTCommand) FetchTriggeredConfigs(kronBuilds []*kronpb.Build) (map[*kronpb.Build][]*suschpb.SchedulerConfig, error) {
	return fetchTriggeredConfigs(kronBuilds, c.suiteSchedulerConfigs.FetchMultiDUTConfigsByBuildTarget)
}

// multiDUTTargetPair wraps the config, primary and secondary device build
// information.
type multiDUTTargetPair struct {
	config        *suschpb.SchedulerConfig
	primaryBuild  *kronpb.Build
	primaryTarget *configparser.MultiDUTTarget
	secondaries   []*multiDUTSecondary
}

// multiDUTSecondary contains all the information needed by the CTP request to
// describe the secondary target.
type multiDUTSecondary struct {
	BuildTarget string
	Board       string
	Model       string

	// NOTE: Only used for CrOS images. In the form of:
	// <board>((?-)variant)-release/R<milestone>-<version>.
	CrOSBuild string

	// NOTE: These are only used if the secondary is an Android device.
	isAndroid           bool
	gmsCorePackage      string
	AndroidImageVersion string
}

// gatherSecondaryTargets gathers a list of all secondary device targets as
// described in the configs passed in. A list of builds to fetch from LTS is
// provided.
func gatherSecondaryTargets(kronBuildMap map[*kronpb.Build][]*suschpb.SchedulerConfig, suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs) ([]*builds.RequiredBuild, map[builds.RequiredBuild][]*multiDUTSecondary, map[multiDUTSecondary][]*multiDUTTargetPair, []*multiDUTSecondary, error) {
	requiredSecondaryBuilds := []*builds.RequiredBuild{}
	secondaryDevices := []*multiDUTSecondary{}

	// NOTE: These are used to construct a request map after we have fetched
	// the build image information from long term storage.
	requiredBuildsToSecondaryTargets := map[builds.RequiredBuild][]*multiDUTSecondary{}
	secondariesToPrimary := map[multiDUTSecondary][]*multiDUTTargetPair{}

	// Iterate through the triggered configs and generate a list of secondary
	// targets to fetch from LTS.
	for build, configs := range kronBuildMap {
		for _, config := range configs {
			// Gather the MULTI_DUT target options for the current
			// config/primary build.
			targets, err := suiteSchedulerConfigs.FetchMultiDUTConfigTargetOptionsForBoard(config.Name, build.BuildTarget)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			// For each of the specified target options generate requests for
			// secondary builds.
			for _, target := range targets {
				// Create a wrapper for the target information needed to
				// generate a CTP request.
				pairedDeviceSpec := &multiDUTTargetPair{
					config:        config,
					primaryBuild:  build,
					primaryTarget: target.Primary,
					secondaries:   []*multiDUTSecondary{},
				}

				for _, secondaryDevice := range target.Secondaries {
					// If the secondary device is an Android target then do not
					// generate a LTS request and directly add it to the target
					// wrapper object.
					if secondaryDevice.IsAndroid {
						androidDevice := &multiDUTSecondary{
							BuildTarget:         secondaryDevice.BuildTarget,
							Board:               secondaryDevice.Board,
							Model:               secondaryDevice.Model,
							isAndroid:           true,
							gmsCorePackage:      config.GetGmsCorePackage(),
							AndroidImageVersion: config.GetAndroidImageVersion(),
						}

						pairedDeviceSpec.secondaries = append(pairedDeviceSpec.secondaries, androidDevice)
						secondariesToPrimary[*androidDevice] = append(secondariesToPrimary[*androidDevice], pairedDeviceSpec)
						continue
					}

					// if not, then fetch the device from LTS and add into the
					// list
					requiredBuild := builds.RequiredBuild{
						BuildTarget: secondaryDevice.BuildTarget,
						Board:       secondaryDevice.Board,
						Milestone:   int(build.GetMilestone()),
					}
					secondaryTarget := &multiDUTSecondary{
						Board:       secondaryDevice.Board,
						Model:       secondaryDevice.Model,
						BuildTarget: secondaryDevice.BuildTarget,
					}

					// Track for later
					if _, ok := requiredBuildsToSecondaryTargets[requiredBuild]; !ok {
						requiredBuildsToSecondaryTargets[requiredBuild] = []*multiDUTSecondary{}
						requiredSecondaryBuilds = append(requiredSecondaryBuilds, &requiredBuild)
					}
					requiredBuildsToSecondaryTargets[requiredBuild] = append(requiredBuildsToSecondaryTargets[requiredBuild], secondaryTarget)
					secondaryDevices = append(secondaryDevices, secondaryTarget)

					// Add the secondary device to the pair list.
					pairedDeviceSpec.secondaries = append(pairedDeviceSpec.secondaries, secondaryTarget)

					// Map the secondary to all primaries which include it.
					if _, ok := secondariesToPrimary[*secondaryTarget]; !ok {
						secondariesToPrimary[*secondaryTarget] = []*multiDUTTargetPair{}
					}
					secondariesToPrimary[*secondaryTarget] = append(secondariesToPrimary[*secondaryTarget], pairedDeviceSpec)
				}
			}
		}
	}

	return requiredSecondaryBuilds, requiredBuildsToSecondaryTargets, secondariesToPrimary, secondaryDevices, nil
}

// handleSecondaryTargetInformation takes in the received secondary device
// information and populate the image location in the device object.
func handleSecondaryTargetInformation(fetchedKronBuilds []*kronpb.Build, requiredBuildsToSecondaryTargets map[builds.RequiredBuild][]*multiDUTSecondary, secondaryDevices []*multiDUTSecondary, secondariesToPrimary map[multiDUTSecondary][]*multiDUTTargetPair, projectID string) (map[multiDUTSecondary][]*multiDUTTargetPair, error) {
	// Fill in the image information for all secondaries.
	for _, kronBuild := range fetchedKronBuilds {
		requiredBuild := builds.RequiredBuild{
			BuildTarget: kronBuild.GetBuildTarget(),
			Board:       kronBuild.GetBoard(),
			Milestone:   int(kronBuild.GetMilestone()),
		}

		secondaries, ok := requiredBuildsToSecondaryTargets[requiredBuild]
		if !ok {
			return nil, fmt.Errorf("buildTarget %s for board %s on milestone %d was fetched from long term storage when not requested", requiredBuild.BuildTarget, requiredBuild.Board, requiredBuild.Milestone)
		}

		for _, secondaryDevice := range secondaries {
			secondaryDevice.CrOSBuild = fmt.Sprintf("%s-release/R%d-%s", kronBuild.GetBuildTarget(), kronBuild.GetMilestone(), kronBuild.GetVersion())
		}
	}

	rejectSecondaries := []*multiDUTSecondary{}
	for _, secondary := range secondaryDevices {
		if !secondary.isAndroid && secondary.CrOSBuild == "" {
			rejectSecondaries = append(rejectSecondaries, secondary)
		}
	}

	common.Stdout.Printf("Initializing client for pub sub topic %s", common.EventsPubSubTopic)
	eventPublishClient, err := pubsub.InitPublishClient(context.Background(), projectID, common.EventsPubSubTopic)
	if err != nil {
		return nil, err
	}

	// Remove all rejectedSecondaries
	for _, reject := range rejectSecondaries {
		key := multiDUTSecondary{
			Board:       reject.Board,
			Model:       reject.Model,
			BuildTarget: reject.BuildTarget,
		}

		for _, requestPair := range secondariesToPrimary[key] {
			decision := &kronpb.SchedulingDecision{
				Type:         kronpb.DecisionType_BUILD_NOT_FOUND,
				Scheduled:    false,
				FailedReason: fmt.Sprintf("secondary buildTarget %s for board %s on milestone %d could not be fetched from LTS for config %s and primary target %s", reject.BuildTarget, reject.Board, requestPair.primaryBuild.GetMilestone(), requestPair.config.GetName(), requestPair.primaryBuild.GetBuildTarget()),
			}

			event, err := metrics.GenerateEventMessage(requestPair.config, decision, 0, "", requestPair.primaryBuild.GetBoard(), requestPair.primaryTarget.Model, requestPair.primaryBuild.GetBuildTarget())
			if err != nil {
				return nil, err
			}

			err = publishEvent(eventPublishClient, event)
			if err != nil {
				return nil, err
			}
		}

		// Remove all entries from the map which could not be scheduled.
		delete(secondariesToPrimary, key)
	}

	return secondariesToPrimary, nil
}

// transformToRequestMap takes in the secondaries map and returns a config index
// map of target pairs.
func transformToRequestMap(secondariesToPrimary map[multiDUTSecondary][]*multiDUTTargetPair) map[*suschpb.SchedulerConfig][]*multiDUTTargetPair {
	dedupeMap := map[*suschpb.SchedulerConfig]map[*multiDUTTargetPair]bool{}
	requestMap := map[*suschpb.SchedulerConfig][]*multiDUTTargetPair{}

	for _, targetPairs := range secondariesToPrimary {
		for _, targetPair := range targetPairs {
			// Add the config to the deduping map.
			if _, ok := dedupeMap[targetPair.config]; !ok {
				dedupeMap[targetPair.config] = map[*multiDUTTargetPair]bool{}
			}

			// Create a new list for tracking.
			if _, ok := requestMap[targetPair.config]; !ok {
				requestMap[targetPair.config] = []*multiDUTTargetPair{}
			}

			// After checking for duplicates, add the target pair to the request
			// map.
			if _, ok := dedupeMap[targetPair.config][targetPair]; !ok {
				dedupeMap[targetPair.config][targetPair] = true
				requestMap[targetPair.config] = append(requestMap[targetPair.config], targetPair)
			}
		}
	}

	return requestMap
}

// generateRequestMap gathers and creates a list of secondary targets needed
// by the triggered configs, attempts to fetch the target information
// from the PostgreSQL long term storage, and generates a config based request
// map so that we can generate CTP requests afterwards.
func (c *CrOSMultiDUTCommand) generateRequestMap(kronBuildMap map[*kronpb.Build][]*suschpb.SchedulerConfig) (map[*suschpb.SchedulerConfig][]*multiDUTTargetPair, error) {
	// NOTE: These are used to construct a request map after we have fetched
	// the build image information from long term storage.
	requiredSecondaryBuilds, requiredBuildsToSecondaryTargets, secondariesToPrimary, secondaryDevices, err := gatherSecondaryTargets(kronBuildMap, c.suiteSchedulerConfigs)
	if err != nil {
		return nil, err
	}

	// In the case where the only triggered configs have no CrOS secondaries,
	// skip fetching from LTS and merging the requests immediately.
	if len(requiredSecondaryBuilds) == 0 && len(secondariesToPrimary) > 0 {
		// Merge into RequestsMap
		return transformToRequestMap(secondariesToPrimary), nil
	}

	// Fetch secondary device information from from LTS.
	fetchedKronBuilds, err := fetchRequiredBuildsFromLTS(context.Background(), requiredSecondaryBuilds, c.isProd)
	if err != nil {
		return nil, err
	}

	secondariesToPrimary, err = handleSecondaryTargetInformation(fetchedKronBuilds, requiredBuildsToSecondaryTargets, secondaryDevices, secondariesToPrimary, c.projectID)
	if err != nil {
		return nil, err
	}

	// Merge into RequestsMap
	return transformToRequestMap(secondariesToPrimary), nil
}

// BuildMultiDUTCTPRequest generates a single CTP request for a MULTI_DUT
// config.
func BuildMultiDUTCTPRequest(config *suschpb.SchedulerConfig, branchTrigger string, targetPair *multiDUTTargetPair) (*ctpEvent, error) {
	// Generate the base CTP request without any secondary device information.
	request := ctprequest.BuildCTPRequest(config, targetPair.primaryBuild.GetBoard(), targetPair.primaryTarget.Model, targetPair.primaryBuild.GetBuildTarget(), strconv.FormatInt(targetPair.primaryBuild.GetMilestone(), 10), targetPair.primaryBuild.GetVersion(), branchTrigger)

	// Add information to the CTP request for each secondary device.
	for _, secondaryDevice := range targetPair.secondaries {
		// Add Software Attributes
		// NOTE: BuildTarget is used in both Android and CrOS builds.
		ctpDevice := &requestpb.Request_Params_SecondaryDevice{
			SoftwareAttributes: &requestpb.Request_Params_SoftwareAttributes{
				BuildTarget: &chromiumos.BuildTarget{
					Name: secondaryDevice.BuildTarget,
				},
			},
		}

		// Add Hardware Attributes...Only add the field if the model name is provided. Otherwise we risk
		// sending junk fields to CTP which could produce unexpected results.
		if secondaryDevice.Model != "" {
			ctpDevice.HardwareAttributes = &requestpb.Request_Params_HardwareAttributes{
				Model: secondaryDevice.Model,
			}
		}

		// Add Software Dependencies...If the secondary target is a CrOS build attach the image identity.
		// Otherwise add relevant Android config defined dependencies.
		if !secondaryDevice.isAndroid {
			if secondaryDevice.CrOSBuild == "" {
				return nil, fmt.Errorf("secondary CrOS build for config %s primary %s has an empty image specification", config.GetName(), targetPair.primaryBuild.GetBuildTarget())
			}

			ctpDevice.SoftwareDependencies = []*requestpb.Request_Params_SoftwareDependency{
				{
					Dep: &requestpb.Request_Params_SoftwareDependency_ChromeosBuild{
						ChromeosBuild: secondaryDevice.CrOSBuild,
					},
				},
			}
		} else {
			// NOTE: Android devices do not need software dependencies, we will
			// only add them if they are explicitly provided in the config.
			deps := []*requestpb.Request_Params_SoftwareDependency{}

			if secondaryDevice.AndroidImageVersion != "" {
				dep := &requestpb.Request_Params_SoftwareDependency{
					Dep: &requestpb.Request_Params_SoftwareDependency_AndroidImageVersion{
						AndroidImageVersion: secondaryDevice.AndroidImageVersion,
					},
				}

				deps = append(ctpDevice.SoftwareDependencies, dep)
			}

			if secondaryDevice.gmsCorePackage != "" {
				dep := &requestpb.Request_Params_SoftwareDependency{
					Dep: &requestpb.Request_Params_SoftwareDependency_GmsCorePackage{
						GmsCorePackage: secondaryDevice.gmsCorePackage,
					},
				}

				deps = append(ctpDevice.SoftwareDependencies, dep)
			}

			// NOTE: Only add dependencies if not empty so that we do not get
			// any unexpected behaviors in the CTP builder if empty values are
			// present.
			if len(deps) >= 0 {
				ctpDevice.SoftwareDependencies = deps
			}
		}

		request.Params.SecondaryDevices = append(request.Params.SecondaryDevices, ctpDevice)
	}

	// Generate a metric event sans BuildBucket information for the above CTP
	// request.
	metricEvent, err := metrics.GenerateEventMessage(config, nil, 0, targetPair.primaryBuild.GetBuildUuid(), targetPair.primaryBuild.GetBoard(), targetPair.primaryTarget.Model, targetPair.primaryBuild.GetBuildTarget())
	if err != nil {
		return nil, err
	}

	return &ctpEvent{
		event:      metricEvent,
		ctpRequest: request,
		config:     config,
	}, nil
}

// buildMultiDUTCTPRequests returns a list of MULTI_DUT CTP requests for all
// all configs and target pairs.
func buildMultiDUTCTPRequests(requestMap map[*suschpb.SchedulerConfig][]*multiDUTTargetPair) ([]*ctpEvent, error) {
	ctpRequests := []*ctpEvent{}
	for config, targets := range requestMap {
		for _, target := range targets {
			// Get get the branch target which this build matched with.
			_, branch, err := totmanager.IsTargetedBranch(int(target.primaryBuild.GetMilestone()), config.GetBranches())
			if err != nil {
				return nil, err
			}

			event, err := BuildMultiDUTCTPRequest(config, branch.String(), target)
			if err != nil {
				return nil, err
			}

			ctpRequests = append(ctpRequests, event)
		}
	}

	return ctpRequests, nil
}

// ScheduleRequests locates secondary device information, generates CTP
// Requests, batches them into BuildBucket requests, and Schedules them via the
// BuildBucket API.
func (c *CrOSMultiDUTCommand) ScheduleRequests(kronBuildMap map[*kronpb.Build][]*suschpb.SchedulerConfig) error {
	// Gather requestMap for all secondary targets.
	requestMap, err := c.generateRequestMap(kronBuildMap)
	if err != nil {
		return err
	}

	ctpRequests, err := buildMultiDUTCTPRequests(requestMap)
	if err != nil {
		return err
	}

	ctpRequests, err = removeDuplicateRequests(ctpRequests)
	if err != nil {
		return err
	}

	// Pre-batch the requests according to the max batch size.
	batches, err := formatAndBatchCTPRequests(c.isProd, c.dryRun, ctpRequests)
	if err != nil {
		return err
	}

	return scheduleBatches(batches, c.isProd, c.dryRun, c.projectID, c.authOpts)
}
