// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	requestpb "go.chromium.org/chromiumos/infra/proto/go/test_platform"
	ctppb "go.chromium.org/chromiumos/infra/proto/go/test_platform/cros_test_platform"
	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth/client/authcli"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"infra/cros/cmd/kron/buildbucket"
	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/configparser"
	"infra/cros/cmd/kron/ctprequest"
	"infra/cros/cmd/kron/metrics"
	"infra/cros/cmd/kron/pubsub"
	"infra/cros/cmd/kron/totmanager"
)

type ctpEvent struct {
	event      *kronpb.Event
	ctpRequest *requestpb.Request
	config     *suschpb.SchedulerConfig
}

type ctpEventsPerBranch struct {
	events []*ctpEvent
	branch suschpb.Branch
}

type ctpEventBatch struct {
	events    []*kronpb.Event
	bbRequest *buildbucketpb.ScheduleBuildRequest
}

// fetchTriggeredDailyEvents returns all DAILY configs which are triggered at
// the current run's operating time. Logging is also wrapped within this function.
func fetchTriggeredDailyEvents(currTime common.KronTime, ingestedConfigs *configparser.SuiteSchedulerConfigs, configs *configparser.ConfigList) error {
	common.Stdout.Printf("Gathering DAILY configs triggered at hour %d\n", currTime.Hour)
	triggeredConfigs, err := ingestedConfigs.FetchDailyByHour(currTime.Hour)
	if err != nil {
		return err
	}

	common.Stdout.Printf("The following %d configs are triggered at hour %d:\n", len(triggeredConfigs), currTime.Hour)
	for _, config := range triggeredConfigs {
		common.Stdout.Printf("\t%s\n", config.Name)
	}

	*configs = append(*configs, triggeredConfigs...)
	return nil
}

// fetchTriggeredWeeklyEvents returns all WEEKLY configs which are triggered at
// the current run's operating time. Logging is also wrapped within this function.
func fetchTriggeredWeeklyEvents(currTime common.KronTime, ingestedConfigs *configparser.SuiteSchedulerConfigs, configs *configparser.ConfigList) error {
	common.Stdout.Printf("Gathering WEEKLY configs triggered at day %d hour %d\n", currTime.WeeklyDay, currTime.Hour)
	triggeredConfigs, err := ingestedConfigs.FetchWeeklyByDayHour(currTime.WeeklyDay, currTime.Hour)
	if err != nil {
		return err
	}

	common.Stdout.Printf("The following %d configs are triggered at day %d hour %d:\n", len(triggeredConfigs), currTime.WeeklyDay, currTime.Hour)
	for _, config := range triggeredConfigs {
		common.Stdout.Printf("\t%s\n", config.Name)
	}

	*configs = append(*configs, triggeredConfigs...)

	return nil
}

// fetchTriggeredFortnightlyEvents returns all FORTNIGHTLY configs which are triggered at
// the current run's operating time. Logging is also wrapped within this function.
func fetchTriggeredFortnightlyEvents(currTime common.KronTime, ingestedConfigs *configparser.SuiteSchedulerConfigs, configs *configparser.ConfigList) error {
	common.Stdout.Printf("Gathering FORTNIGHTLY configs triggered at day %d hour %d\n", currTime.FortnightDay, currTime.Hour)
	triggeredConfigs, err := ingestedConfigs.FetchFortnightlyByDayHour(currTime.FortnightDay, currTime.Hour)
	if err != nil {
		return err
	}

	common.Stdout.Printf("The following %d configs are triggered at day %d hour %d:\n", len(triggeredConfigs), currTime.FortnightDay, currTime.Hour)
	for _, config := range triggeredConfigs {
		common.Stdout.Printf("\t%s\n", config.Name)
	}

	*configs = append(*configs, triggeredConfigs...)
	return nil
}

// fetchTimedEvents gathers all timed event config which will are triggered at
// the provided time.
//
// NOTE: This function in conjunction with the KronTime struct handles
// fortnightly/weekly differences natively.
func fetchTimedEvents(currTime common.KronTime, ingestedConfigs *configparser.SuiteSchedulerConfigs) (configparser.ConfigList, error) {
	timedConfigs := configparser.ConfigList{}

	// Daily
	err := fetchTriggeredDailyEvents(currTime, ingestedConfigs, &timedConfigs)
	if err != nil {
		return nil, err
	}

	// Weekly
	err = fetchTriggeredWeeklyEvents(currTime, ingestedConfigs, &timedConfigs)
	if err != nil {
		return nil, err
	}

	// Fortnightly
	err = fetchTriggeredFortnightlyEvents(currTime, ingestedConfigs, &timedConfigs)
	if err != nil {
		return nil, err
	}

	return timedConfigs, nil
}

// limitStagingRequests scrubs outs ctp requests to ensure that only
// common.StagingMaxRequests maximum requests can be sent to CTP-staging. This
// limits the pressure that kron places on our staging pools while allowing us a
// functional staging environment.
func limitStagingRequests(ctpRequests []*ctpEvent) []*ctpEvent {
	common.Stdout.Printf("limiting staging requests to %d max, starting with %d", common.StagingMaxRequests, len(ctpRequests))
	if len(ctpRequests) == 0 {
		return nil
	}

	limitedRequests := []*ctpEvent{}
	for _, configWrapper := range ctpRequests {
		if len(limitedRequests) == common.StagingMaxRequests {
			break
		}

		limitedRequests = append(limitedRequests, configWrapper)
	}

	return limitedRequests
}

// buildPerModelConfigs builds a CTP request per model (if it exists) for the
// given config.
func buildPerModelConfigs(models []string, config *suschpb.SchedulerConfig, build *kronpb.Build, branch string) ([]*ctpEvent, error) {
	ctpRequests := []*ctpEvent{}
	// If provided, build a CTP request per model, otherwise leave the model
	// field absent.
	if len(models) > 0 {
		// Generate a CTP Request for each model.
		for _, model := range models {
			ctpRequest := ctprequest.BuildCTPRequest(config, build.GetBoard(), model, build.GetBuildTarget(), strconv.FormatInt(build.GetMilestone(), 10), build.GetVersion(), branch)

			event, err := metrics.GenerateEventMessage(config, nil, 0, build.GetBuildUuid(), build.GetBoard(), model, build.GetBuildTarget())
			if err != nil {
				return nil, err
			}
			request := &ctpEvent{
				event:      event,
				ctpRequest: ctpRequest,
				config:     config,
			}

			ctpRequests = append(ctpRequests, request)
		}
	} else {
		ctpRequest := ctprequest.BuildCTPRequest(config, build.GetBoard(), "", build.GetBuildTarget(), strconv.FormatInt(build.GetMilestone(), 10), build.GetVersion(), branch)

		event, err := metrics.GenerateEventMessage(config, nil, 0, build.GetBuildUuid(), build.GetBoard(), "", build.GetBuildTarget())
		if err != nil {
			return nil, err
		}

		request := &ctpEvent{
			event:      event,
			ctpRequest: ctpRequest,
			config:     config,
		}
		ctpRequests = append(ctpRequests, request)
	}

	return ctpRequests, nil
}

// buildCTPRequests iterates through all the provided triggered configs and
// generates BuildBucket CTP requests for all triggered configs.
func buildCTPRequests(buildToConfigsMap map[*kronpb.Build][]*suschpb.SchedulerConfig, suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs) ([]*ctpEvent, error) {
	requests := []*ctpEvent{}

	// Iterate through the wrapped builds and insert CTP request and their
	// associated metrics events into the package.
	for kronBuild, configs := range buildToConfigsMap {
		// Iterate through all
		for _, triggeredConfig := range configs {
			// Fetch the target options requested for the current board on the
			// current configuration.
			boardTargetOption, err := suiteSchedulerConfigs.FetchConfigTargetOptionsForBoard(triggeredConfig.Name, configparser.Board(kronBuild.Board))
			if err != nil {
				return nil, err
			}

			// Get get the branch target which this build matched with.
			_, branch, err := totmanager.IsTargetedBranch(int(kronBuild.Milestone), triggeredConfig.Branches)
			if err != nil {
				return nil, err
			}

			ctpRequests, err := buildPerModelConfigs(boardTargetOption.Models, triggeredConfig, kronBuild, suschpb.Branch_name[int32(branch)])
			if err != nil {
				return nil, err
			}

			requests = append(requests, ctpRequests...)
		}
	}
	return requests, nil
}

// generateBuilderTags generates a list of BuildBucket String pairs which will
// be used for a builders tags. These tags contain metadata about the CTP
// request which can be used in PLX analysis later on.
func generateBuilderTags(suiteName, configName string, requests []*ctpEvent, skipTag bool) ([]*buildbucketpb.StringPair, error) {
	tags := []*buildbucketpb.StringPair{
		{
			Key:   "kron-run",
			Value: metrics.GetRunID(),
		},
		{
			Key:   "suite",
			Value: suiteName,
		},
		{
			Key:   "label-suite",
			Value: suiteName,
		},
		{
			Key:   "user_agent",
			Value: "kron",
		},
		{
			Key:   "suite-scheduler-config",
			Value: configName,
		},
	}

	// Add all image, buildUuid, and eventUuid fields per test request.
	for _, request := range requests {
		image := ""
		for _, dep := range request.ctpRequest.Params.SoftwareDependencies {
			// The SoftwareDependencies proto type includes many types of deps,
			// so search for one which can provide the image value.
			if dep.GetChromeosBuild() != "" {
				image = dep.GetChromeosBuild()
				break
			}
		}

		// A CTP request cannot function with a nil image value so throw an
		// error here.
		if image == "" {
			return nil, fmt.Errorf("no ChromeOS build found")
		}
		if !skipTag {
			tags = append(tags,
				&buildbucketpb.StringPair{
					Key:   "build-id",
					Value: request.event.BuildUuid,
				})
			tags = append(tags,
				&buildbucketpb.StringPair{
					Key:   "event-id",
					Value: request.event.EventUuid,
				})
		}
		tags = append(tags,
			&buildbucketpb.StringPair{
				Key:   "label-image",
				Value: image,
			})
	}

	return tags, nil
}

// generateGenericBBProperties takes in the CTPEvents and generates a generic
// structpb type for the buildBucketProperties.
func generateGenericBBProperties(requests []*ctpEvent) (*structpb.Struct, error) {
	ctpRequestInputProps := &ctppb.CrosTestPlatformProperties{
		Requests: map[string]*requestpb.Request{},
	}

	// Add all CTP Test Requests to the input properties struct mapped by their
	// unique request metadata.
	for _, request := range requests {
		key := fmt.Sprintf("%s.%s.%s", request.ctpRequest.Params.SoftwareAttributes.BuildTarget.Name, request.event.ConfigName, request.event.SuiteName)
		if _, ok := ctpRequestInputProps.Requests[key]; ok {
			// If the key is duplicated for some reason then add the eventUuid
			// to differentiate.
			key = fmt.Sprintf("%s.%s", key, request.event.EventUuid)

		}
		ctpRequestInputProps.Requests[key] = request.ctpRequest
	}
	// Transform the properties proto into a json string.
	msgJSON, err := protojson.Marshal(ctpRequestInputProps)
	if err != nil {
		return nil, err
	}

	// Now that we have the raw json unmarshal, transform the text into the
	// "generic" proto struct. This "generic" proto struct type is required by
	// the BuildBucket API.
	//
	// NOTE: The default unmarshall-er from the protojson package does not throw
	// errors on unknown JSON fields. This is required because the generic
	// struct and CTP struct do not share any field names. If a different
	// unmarshall-er is chosen down the line, ensure that this functionality is
	// maintained.
	properties := &structpb.Struct{}
	err = protojson.Unmarshal(msgJSON, properties)
	if err != nil {
		return nil, err
	}

	return properties, nil
}

// buildCTPRequestsFor3dConfigs creates list of ctp requests for each config per branch.
func buildCTPRequestsFor3dConfigs(buildPackagesMap map[int64]*BuildPackage3d, all3dConfigs configparser.ConfigList) (map[*suschpb.SchedulerConfig][]ctpEventsPerBranch, error) {
	ctpMapByConfig := make(map[*suschpb.SchedulerConfig][]ctpEventsPerBranch)

	// processing each config
	for _, config := range all3dConfigs {
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
					ctpRequests, err := buildPerModelConfigs(nil, config, kronBuild, suschpb.Branch_name[int32(buildPackage3d.Branch)])
					if err != nil {
						return nil, err
					}
					allCtpEvents = append(allCtpEvents, ctpRequests...)
				}
				ctpEventsPerBranch := []ctpEventsPerBranch{
					{
						branch: buildPackage3d.Branch,
						events: allCtpEvents,
					},
				}
				ctpMapByConfig[config] = append(ctpMapByConfig[config], ctpEventsPerBranch...)

			}
		}
	}
	// logging map
	common.Stdout.Printf("Preparing requests for ...\n")
	for config, ctpEventsPerBranch := range ctpMapByConfig {
		for _, ctpEvents := range ctpEventsPerBranch {
			common.Stdout.Printf("Config:%s for branch:%s for %d builds\n", config.Name, ctpEvents.branch, len(ctpEvents.events))
		}
	}

	return ctpMapByConfig, nil
}

// limitStagingRequests3d limits the number of CTP requests to 5 per branch for a given config.
func limitStagingRequests3d(ctpMapByConfig map[*suschpb.SchedulerConfig][]ctpEventsPerBranch) map[*suschpb.SchedulerConfig][]ctpEventsPerBranch {
	// Create a new map to hold the result
	resultCtpMapByConfig := make(map[*suschpb.SchedulerConfig][]ctpEventsPerBranch)

	// Iterate over the input map
	for config, ctpEventsPerBranchList := range ctpMapByConfig {
		// Create a new list to hold the limited events per branch
		newCtpEventsPerBranchList := make([]ctpEventsPerBranch, len(ctpEventsPerBranchList))
		for i, ctpEvents := range ctpEventsPerBranchList {
			if len(ctpEvents.events) > 5 {
				newCtpEventsPerBranchList[i] = ctpEventsPerBranch{
					events: ctpEvents.events[:5],
					branch: ctpEvents.branch,
				}
			} else {
				newCtpEventsPerBranchList[i] = ctpEvents
			}
		}
		resultCtpMapByConfig[config] = newCtpEventsPerBranchList
	}

	return resultCtpMapByConfig
}

// mergeRequests merge all CTP requests into one CTP recipe input properties object.
func mergeRequests(requests []*ctpEvent, suiteName, configName string, isProd, dryRun bool, skipTag bool) (*ctpEventBatch, error) {
	properties, err := generateGenericBBProperties(requests)
	if err != nil {
		return nil, err
	}

	// Based on the isProd flag choose the corresponding builder identification.
	builder := &buildbucket.CtpBuilderIDStaging
	if isProd {
		builder = &buildbucket.CtpBuilderIDProd
	}

	tags, err := generateBuilderTags(suiteName, configName, requests, skipTag)
	if err != nil {
		return nil, err
	}

	// Generate the generic BuildBucket request from the items build above.
	bbRequest := generateBBRequest(dryRun, builder, properties, tags...)

	batch := &ctpEventBatch{
		events:    []*kronpb.Event{},
		bbRequest: bbRequest,
	}
	for _, request := range requests {
		batch.events = append(batch.events, request.event)
	}

	return batch, nil
}

// batchCTPRequests groups configs/events into common.MultirequestSize sized
// batches.
//
// NOTE: Requests in these batches will all share the same SuiteScheduler
// Config.
func batchCTPRequests(ctpRequests map[*suschpb.SchedulerConfig][]*ctpEvent, isProd, dryRun bool) ([]*ctpEventBatch, error) {
	batches := []*ctpEventBatch{}

	// Create batches of common.MultirequestSize size.
	for config, configWrappers := range ctpRequests {
		currentBatch := []*ctpEvent{}

		for _, request := range configWrappers {
			// If we have reached the max length, merge the current batch list
			// into a batch event and start a new batch.
			if len(currentBatch) == common.MultirequestSize {
				batch, err := mergeRequests(currentBatch, config.Suite, config.Name, isProd, dryRun, false)
				if err != nil {
					return nil, err
				}

				batches = append(batches, batch)

				currentBatch = []*ctpEvent{}
			}

			currentBatch = append(currentBatch, request)
		}

		if len(currentBatch) != 0 {
			batch, err := mergeRequests(currentBatch, config.Suite, config.Name, isProd, dryRun, false)
			if err != nil {
				return nil, err
			}

			batches = append(batches, batch)
		}
	}
	return batches, nil
}

// batchCTPRequests3d groups all events per config per branch in one batch.
//
// NOTE: Requests in these batches will all share the same SuiteScheduler
// Config.
func batchCTPRequests3d(ctpMapByConfig map[*suschpb.SchedulerConfig][]ctpEventsPerBranch, isProd, dryRun bool) ([]*ctpEventBatch, error) {
	batches := []*ctpEventBatch{}

	// Create batches for each config per branch
	for config, ctpReqsByBranch := range ctpMapByConfig {
		for _, ctpEvents := range ctpReqsByBranch {
			batch, err := mergeRequests(ctpEvents.events, config.Suite, config.Name, isProd, dryRun, true)
			if err != nil {
				return nil, err
			}
			batches = append(batches, batch)
		}
	}
	return batches, nil
}

// generateBBRequest creates a BuildBucket Request proto with proper metadata in
// the tags.
func generateBBRequest(dryRun bool, builder *buildbucketpb.BuilderID, properties *structpb.Struct, tags ...*buildbucketpb.StringPair) *buildbucketpb.ScheduleBuildRequest {
	return &buildbucketpb.ScheduleBuildRequest{
		Builder:    builder,
		Properties: properties,
		DryRun:     dryRun,
		// These tags will appear on the Milo UI and will help us search for
		// builds in plx.
		Tags: tags,
	}
}

// mapEventsByConfig iterates though the list of individual CTP events and
// groups them by the SuiteScheduler which it was generated from.
func mapEventsByConfig(ctpRequests []*ctpEvent) map[*suschpb.SchedulerConfig][]*ctpEvent {
	configToEventsMap := map[*suschpb.SchedulerConfig][]*ctpEvent{}
	for _, ctpRequest := range ctpRequests {
		// If the config key hasn't been added to the event yet then instantiate
		// it's key.
		if _, ok := configToEventsMap[ctpRequest.config]; !ok {
			configToEventsMap[ctpRequest.config] = []*ctpEvent{}
		}

		configToEventsMap[ctpRequest.config] = append(configToEventsMap[ctpRequest.config], ctpRequest)
	}
	return configToEventsMap
}

// initPubSubAndSchedulerClients builds clients for later use.
func initPubSubAndSchedulerClients(isProd, dryRun bool, projectID string, authOpts *authcli.Flags) (buildbucket.Scheduler, pubsub.PublishClient, error) {
	// Initialize an authenticated BuildBucket client for scheduling.
	common.Stdout.Printf("Initializing BuildBucket scheduling client prod: %t dryRun: %t", isProd, dryRun)
	schedulerClient, err := buildbucket.InitScheduler(context.Background(), authOpts, isProd, dryRun)
	if err != nil {
		return nil, nil, err
	}

	// Initialize the Pub/Sub client for event message publishing.
	common.Stdout.Printf("Initializing client for pub sub topic %s", common.EventsPubSubTopic)
	publishClient, err := pubsub.InitPublishClient(context.Background(), projectID, common.EventsPubSubTopic)
	if err != nil {
		return nil, nil, err
	}

	return schedulerClient, publishClient, nil
}

// fillEventResponse interprets the response from the BuildBucket schedule
// action and fills each affected event with is results.
func fillEventResponse(events []*kronpb.Event, bbResponse *buildbucketpb.Build) {
	for _, event := range events {
		if bbResponse.GetStatus() == buildbucketpb.Status_SCHEDULED {
			event.Decision = &kronpb.SchedulingDecision{
				Type:      kronpb.DecisionType_SCHEDULED,
				Scheduled: true,
			}

			event.Bbid = bbResponse.GetId()

			common.Stdout.Printf("Event %s for config %s scheduled at http://go/bbid/%d using buildId %s", event.GetEventUuid(), event.GetConfigName(), bbResponse.GetId(), event.GetBuildUuid())
		} else {
			event.Decision = &kronpb.SchedulingDecision{
				Type:         kronpb.DecisionType_UNKNOWN,
				Scheduled:    false,
				FailedReason: buildbucketpb.Status_name[int32(bbResponse.GetStatus().Number())],
			}

			common.Stdout.Printf("Event %s failed to schedule for unknown reason", event.GetEventUuid())
		}
	}
}

// publishEvents sends all of the event message to Pub/Sub. A flag is provided
// to skip publishing errors if desired.
func publishEvents(client pubsub.PublishClient, events []*kronpb.Event, allowPublishErrors bool) error {
	for _, event := range events {
		data, err := protojson.Marshal(event)
		if err != nil {
			return err
		}

		err = client.PublishMessage(context.Background(), data)
		if err != nil {
			if allowPublishErrors {
				return err
			} else {
				common.Stderr.Println(err)
			}
		}
	}

	return nil
}

// handleBatch schedules ands publishes results for each of the pre-batched CTP
// requests.
func handleBatch(schedulerClient buildbucket.Scheduler, publishClient pubsub.PublishClient, batch *ctpEventBatch, fillEventResponse func([]*kronpb.Event, *buildbucketpb.Build), publishEvent bool) error {
	bbResponse, err := schedulerClient.Schedule(batch.bbRequest)
	if err != nil {
		for _, event := range batch.events {
			event.Decision = &kronpb.SchedulingDecision{
				Type:         kronpb.DecisionType_UNKNOWN,
				Scheduled:    false,
				FailedReason: err.Error(),
			}
			common.Stderr.Printf("Event %s failed to schedule: %s", event.EventUuid, err)
		}
	} else {
		// Populate scheduling status field.
		fillEventResponse(batch.events, bbResponse)

	}

	// Only publish events if explicitly commanded to.
	if !publishEvent {
		return nil
	}

	// Publish the events that just got sent.
	err = publishEvents(publishClient, batch.events, disallowPublishErrors)
	if err != nil {
		return err
	}
	return nil
}

// scheduleBatches takes in a list of CTPEvent batches and schedules them in
// series to BuildBucket.
func scheduleBatches(batches []*ctpEventBatch, isProd, dryRun bool, projectID string, authOpts *authcli.Flags) error {
	// Initialize an authenticated BuildBucket client for scheduling.
	schedulerClient, publishClient, err := initPubSubAndSchedulerClients(isProd, dryRun, projectID, authOpts)
	if err != nil {
		return err
	}

	common.Stdout.Printf("Scheduling %d batches to BB", len(batches))
	var wg sync.WaitGroup
	for _, batch := range batches {
		wg.Add(1)
		go func(wg *sync.WaitGroup, schedulerClient buildbucket.Scheduler, publishClient pubsub.PublishClient, batch *ctpEventBatch) {
			defer wg.Done()

			err := handleBatch(schedulerClient, publishClient, batch, fillEventResponse, publishEventsToPubSub)
			if err != nil {
				common.Stderr.Println(err)
			}

		}(&wg, schedulerClient, publishClient, batch)

	}

	common.Stdout.Println("Waiting for batches to finish scheduling...")
	wg.Wait()

	return nil
}
