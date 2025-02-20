// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"encoding/json"
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

	"go.chromium.org/infra/cros/cmd/kron/buildbucket"
	"go.chromium.org/infra/cros/cmd/kron/builds"
	"go.chromium.org/infra/cros/cmd/kron/cloudsql"
	"go.chromium.org/infra/cros/cmd/kron/common"
	"go.chromium.org/infra/cros/cmd/kron/configparser"
	"go.chromium.org/infra/cros/cmd/kron/ctprequest"
	"go.chromium.org/infra/cros/cmd/kron/metrics"
	"go.chromium.org/infra/cros/cmd/kron/pubsub"
	"go.chromium.org/infra/cros/cmd/kron/totmanager"
)

type ctpEvent struct {
	event      *kronpb.Event
	ctpRequest *requestpb.Request
	config     *suschpb.SchedulerConfig
}

type ctpEventBatch struct {
	events    []*kronpb.Event
	bbRequest *buildbucketpb.ScheduleBuildRequest
}

const (
	limitBatches      = true
	doNotLimitBatches = false
)

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

// fetchTriggeredFortnightlyEvents returns all FORTNIGHTLY configs which are triggered at
// the current run's operating time. Logging is also wrapped within this function.
func fetchTriggeredNDayEvents(currTime common.KronTime, ingestedConfigs *configparser.SuiteSchedulerConfigs, configs *configparser.ConfigList) error {
	factors := common.FindFactors(currTime.StartTime.YearDay())

	common.Stdout.Printf("Gathering N_DAY configs triggered at day %d hour %d\n", currTime.FortnightDay, currTime.Hour)
	triggeredNDayConfigs := configparser.ConfigList{}

	for _, interval := range factors {
		triggeredConfigs, err := ingestedConfigs.FetchNDayByDaysHour(interval, currTime.Hour)
		if err != nil {
			return err
		}
		common.Stdout.Printf("The following %d configs are triggered at interval %d hour %d:\n", len(triggeredNDayConfigs), interval, currTime.Hour)
		for _, config := range triggeredNDayConfigs {
			common.Stdout.Printf("\t%s\n", config.Name)
		}

		triggeredNDayConfigs = append(triggeredNDayConfigs, triggeredConfigs...)
	}

	*configs = append(*configs, triggeredNDayConfigs...)
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

	// N_DAYS
	err = fetchTriggeredNDayEvents(currTime, ingestedConfigs, &timedConfigs)
	if err != nil {
		return nil, err
	}

	return timedConfigs, nil
}

// onlyStagingRequests scrubs out ctp requests to ensure that only staging
// configs are be sent to CTP-staging.
func onlyStagingRequests(ctpRequests []*ctpEvent) []*ctpEvent {
	common.Stdout.Printf("limiting staging requests to those prefixed with %s", common.StagingConfigsPrefix)
	if len(ctpRequests) == 0 {
		return nil
	}

	var limitedRequests []*ctpEvent
	for _, configWrapper := range ctpRequests {
		// Only add configs with the staging config prefix.
		if common.IsStagingConfig(configWrapper.config) {
			limitedRequests = append(limitedRequests, configWrapper)
		}
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

			// Add the event ID to the list of tags so that it gets propagated to the
			// child test runners.
			ctpRequest = ctprequest.AddTagToRequest("event-id", event.GetEventUuid(), ctpRequest)

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

		// Add the event ID to the list of tags so that it gets propagated to the
		// child test runners.
		ctpRequest = ctprequest.AddTagToRequest("event-id", event.GetEventUuid(), ctpRequest)

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
func generateBuilderTags(configs []*suschpb.SchedulerConfig, requests []*ctpEvent) ([]*buildbucketpb.StringPair, error) {
	tags := []*buildbucketpb.StringPair{
		{
			Key:   "kron-run",
			Value: metrics.GetRunID(),
		},
		{
			Key:   "user_agent",
			Value: "kron",
		},
	}

	for _, config := range configs {
		tags = append(tags, []*buildbucketpb.StringPair{
			{
				Key:   "suite-scheduler-config",
				Value: config.GetName(),
			},
			{
				Key:   "label-suite",
				Value: config.GetSuite(),
			},
			{
				Key:   "suite",
				Value: config.GetSuite(),
			},
		}...)
	}

	if parentTaskID := buildbucket.GetSwarmingParentTaskID(); parentTaskID != common.DefaultString {
		tags = append(tags, &buildbucketpb.StringPair{
			Key:   "parent_task_id",
			Value: parentTaskID,
		})
	}

	if parentBBID := buildbucket.GetParentBBID(); parentBBID != common.DefaultString {
		tags = append(tags, &buildbucketpb.StringPair{
			Key:   "parent_buildbucket_id",
			Value: parentBBID,
		})
	}

	// if we have too many requests then we hit the 256 tag limit quite easily.
	// 80 requests is around when this limit will be hit. Skip the below tags.
	if len(requests) > 80 {
		return tags, nil
	}

	// Add all image, buildUuid, and eventUuid fields per test request.
	for _, request := range requests {
		image := ""
		for _, dep := range request.ctpRequest.GetParams().GetSoftwareDependencies() {
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
		key := fmt.Sprintf("%s.%s.%s", request.ctpRequest.GetParams().GetSoftwareAttributes().GetBuildTarget().GetName(), request.event.ConfigName, request.event.SuiteName)
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

// mergeRequests merge all CTP requests into one CTP recipe input properties object.
func mergeRequests(requests []*ctpEvent, configs []*suschpb.SchedulerConfig, isProd, dryRun bool, builder builderInfo) (*ctpEventBatch, error) {
	properties, err := generateGenericBBProperties(requests)
	if err != nil {
		return nil, err
	}

	// Based on the isProd flag choose the corresponding builder identification.
	builderID := buildbucket.GenerateBuilderID(builder.project, builder.bucket, builder.builder, isProd)

	tags, err := generateBuilderTags(configs, requests)
	if err != nil {
		return nil, err
	}

	// Generate the generic BuildBucket request from the items build above.
	bbRequest := generateBBRequest(dryRun, builderID, properties, tags...)

	batch := &ctpEventBatch{
		events:    []*kronpb.Event{},
		bbRequest: bbRequest,
	}
	for _, request := range requests {
		batch.events = append(batch.events, request.event)
	}

	return batch, nil
}

type builderInfo struct {
	project string
	bucket  string
	builder string
}

// batchCTPRequests groups configs/events into common.MultirequestSize sized
// batches.
func batchCTPRequests(ctpEvents []*ctpEvent, isProd, dryRun, limitBatches bool) ([]*ctpEventBatch, error) {
	eventsByBuilderID := map[builderInfo][]*ctpEvent{}

	// Group the requests by custom builder input. This is going to give us
	// partner support coverage
	for _, event := range ctpEvents {
		builderID := event.config.GetRunOptions().GetBuilderId()
		builder := builderInfo{}
		if builderID != nil {
			builder = builderInfo{
				project: builderID.GetProject(),
				bucket:  builderID.GetBucket(),
				builder: builderID.GetBuilder(),
			}
		}

		if _, ok := eventsByBuilderID[builder]; !ok {
			eventsByBuilderID[builder] = []*ctpEvent{}
		}

		eventsByBuilderID[builder] = append(eventsByBuilderID[builder], event)
	}

	// Create batches of common.MultirequestSize size.
	batches := []*ctpEventBatch{}

	// Shadow lists/maps to aggregate metadata on the current batch.
	currentConfigs := []*suschpb.SchedulerConfig{}
	dupeConfigs := map[string]struct{}{}
	currentBuilderID := builderInfo{}

	currentBatch := []*ctpEvent{}

	// Create request batches that share builder IDs. This means that batches
	// can share requests from multiple configs but only if they share the same
	// builderID.
	for builderID, events := range eventsByBuilderID {
		currentBuilderID = builderID
		for _, event := range events {
			// Merge requests when we hit the max batch limit
			if limitBatches && len(currentBatch) == common.MultirequestSize {
				batch, err := mergeRequests(currentBatch, currentConfigs, isProd, dryRun, currentBuilderID)
				if err != nil {
					return nil, err
				}

				batches = append(batches, batch)

				// Reset the tracking lists/maps so that the next batch starts
				// fresh.
				currentBatch = []*ctpEvent{}
				currentConfigs = []*suschpb.SchedulerConfig{}
				dupeConfigs = map[string]struct{}{}
			}

			// Check to see if the config needs to be added to the config list
			// that will generate tags for the CTP Request.
			if _, ok := dupeConfigs[event.config.Name]; !ok {
				dupeConfigs[event.config.Name] = struct{}{}
				currentConfigs = append(currentConfigs, event.config)
			}

			currentBatch = append(currentBatch, event)
		}

		// If a partial batch was left after iterating through all the events, merge
		// the last requests and add it to the batch list.
		if len(currentBatch) != 0 {
			batch, err := mergeRequests(currentBatch, currentConfigs, isProd, dryRun, currentBuilderID)
			if err != nil {
				return nil, err
			}

			batches = append(batches, batch)

			// Reset the tracking lists/maps so that the next batch starts
			// fresh.
			currentBatch = []*ctpEvent{}
			currentConfigs = []*suschpb.SchedulerConfig{}
			dupeConfigs = map[string]struct{}{}
		}
	}

	// If a partial batch was left after iterating through all the events, merge
	// the last requests and add it to the batch list.
	if len(currentBatch) != 0 {
		batch, err := mergeRequests(currentBatch, currentConfigs, isProd, dryRun, currentBuilderID)
		if err != nil {
			return nil, err
		}

		batches = append(batches, batch)
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

const (
	maxWorkers = 500
)

var (
	workerSem = make(chan struct{}, maxWorkers)
)

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
		// If we are at the maxWorkers limit then wait for an open position in
		// the queue.
		workerSem <- struct{}{}

		wg.Add(1)
		go func(wg *sync.WaitGroup, schedulerClient buildbucket.Scheduler, publishClient pubsub.PublishClient, batch *ctpEventBatch) {
			defer func() {
				wg.Done()

				// Open a position in the worker queue.
				<-workerSem
			}()

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

// fetchTriggeredConfigs takes in a list of kron builds and uses the provided
// search function to return NEW_BUILD/MULTI_DUT configs triggered by them.
func fetchTriggeredConfigs(kronBuilds []*kronpb.Build, fetchConfigsByBuildTarget func(target configparser.BuildTarget) configparser.ConfigList) (map[*kronpb.Build][]*suschpb.SchedulerConfig, error) {
	// Build the list of all configs triggered by the ingested build images.
	common.Stdout.Println("Gathering all configs triggered from retrieved build images.")

	// Group the list of configs by the kron build which triggered them. This
	// will save us time later on recomputing which config needs what builds.
	//
	// NOTE: While the build is unique as it is the map key, the configs may be
	// found in multiple map buckets. This is because each config likely targets
	// multiple build targets.
	buildToConfigsMap := map[*kronpb.Build][]*suschpb.SchedulerConfig{}
	for _, build := range kronBuilds {
		// Gather all configs which are triggered by the current builds
		// buildTarget.
		//
		// NOTE: This cache is formed at the beginning of the run when we ingest
		// the ToT SuiteScheduler configs.
		configs := fetchConfigsByBuildTarget(configparser.BuildTarget(build.BuildTarget))

		// Iterate through the triggered configs and verify that they should be
		// triggered in this run.
		for _, config := range configs {
			// If the build's milestone did not match the config's targeted
			// branches then do not add this config to the build's to run list.
			targeted, _, err := totmanager.IsTargetedBranch(int(build.Milestone), config.Branches)
			if err != nil {
				return nil, err
			}
			if !targeted {
				common.Stdout.Printf("Config %s did not match milestone %d for buildTarget %s on build %s\n", config.Name, build.Milestone, build.BuildTarget, build.BuildUuid)
				continue
			}
			common.Stdout.Printf("Config %s matched with build %s for buildTarget %s and milestone %d", config.Name, build.BuildUuid, build.BuildTarget, build.Milestone)

			// If this is the first entry to the map create a list that we can
			// append to.
			if _, ok := buildToConfigsMap[build]; !ok {
				buildToConfigsMap[build] = []*suschpb.SchedulerConfig{}
			}

			buildToConfigsMap[build] = append(buildToConfigsMap[build], config)
		}
	}

	common.Stdout.Printf("%d builds being sent", len(buildToConfigsMap))
	return buildToConfigsMap, nil
}

// formatAndBatchCTPRequests limits total request count in staging and merges
// all requests into batches.
func formatAndBatchCTPRequests(isProd, dryRun bool, ctpRequests []*ctpEvent) ([]*ctpEventBatch, error) {
	// Only send staging requests to the staging environment.
	if !isProd {
		ctpRequests = onlyStagingRequests(ctpRequests)
	}

	if len(ctpRequests) == 0 {
		common.Stdout.Println("No CTP requests to schedule")
		return nil, nil
	}

	return batchCTPRequests(ctpRequests, isProd, dryRun, limitBatches)
}

// scheduleRequests generates CTP Requests, batches them into BuildBucket
// requests, and Schedules them via the BuildBucket API.
//
// NOTE: This is a generic version of the ScheduleRequests command used by
// NEW_BUILD and TIMED_EVENT command types.
func scheduleRequests(kronBuildMap map[*kronpb.Build][]*suschpb.SchedulerConfig, suiteSchedulerConfigs *configparser.SuiteSchedulerConfigs, authOpts *authcli.Flags, projectID string, isProd, dryRun bool) error {
	// Build CTP Requests for all triggered configs.
	ctpRequests, err := buildCTPRequests(kronBuildMap, suiteSchedulerConfigs)
	if err != nil {
		return err
	}

	ctpRequests, err = removeDuplicateRequests(ctpRequests)
	if err != nil {
		return err
	}

	batches, err := formatAndBatchCTPRequests(isProd, dryRun, ctpRequests)
	if err != nil {
		return err
	}

	return scheduleBatches(batches, isProd, dryRun, projectID, authOpts)
}

// publishBuild uploads each build information proto to our long term storage
// PSQL database and our Pub/Sub metrics pipeline.
//
// NOTE: We will attempt to write the build message to the PSQL DB before we try
// uploading to pubsub. Since the BuildUUID is a hash, we will not be able to
// upload the build twice.
func publishBuild(ctx context.Context, kronBuild *kronpb.Build, psClient pubsub.PublishClient, sqlClient cloudsql.Client) error {
	common.Stdout.Printf("Publishing build %s for build target %s and milestone %d to long term storage", kronBuild.BuildUuid, kronBuild.BuildTarget, kronBuild.Milestone)

	// Convert the build to a PSQL compatible type.
	psqlBuild, err := cloudsql.ConvertBuildToPSQLRow(kronBuild)
	if err != nil {
		return err
	}

	// Insert the row into Cloud SQL PSQL.
	_, err = sqlClient.Exec(ctx, cloudsql.InsertBuildsTemplate, cloudsql.RowToSlice(psqlBuild)...)
	if err != nil {
		return err
	}
	common.Stdout.Printf("Published build %s for build target %s and milestone %d to PSQL", kronBuild.BuildUuid, kronBuild.BuildTarget, kronBuild.Milestone)

	// Publish the build to Pub/Sub.
	data, err := protojson.Marshal(kronBuild)
	if err != nil {
		return err
	}
	err = psClient.PublishMessage(ctx, data)
	if err != nil {
		return err
	}
	common.Stdout.Printf("Published build %s for build target %s and milestone %d to pub sub", kronBuild.BuildUuid, kronBuild.BuildTarget, kronBuild.Milestone)

	return nil
}

// fetchRequiredBuildsFromLTS gathers all requested images from the PostgreSQL
// long term storage.
//
// TODO: return a list of required builds that failed to return entries from
// LTS.
func fetchRequiredBuildsFromLTS(ctx context.Context, requiredBuildsList []*builds.RequiredBuild, isProd bool) ([]*kronpb.Build, error) {
	// Generate a human readable string for logging requiredBuildsList.
	buildsList, err := json.MarshalIndent(requiredBuildsList, "", "  ")
	if err != nil {
		return nil, err
	}

	common.Stdout.Printf("The following %d builds are being requested from long term storage", len(requiredBuildsList))
	common.Stdout.Printf("************************************************")
	common.Stdout.Printf("%s", string(buildsList))
	common.Stdout.Printf("************************************************")

	common.Stdout.Println("Fetching Builds from PSQL long term storage")
	fetchedBuilds, err := builds.FetchBuildsFromPSQL(ctx, requiredBuildsList, isProd)
	if err != nil {
		return nil, err
	}

	// Generate a human readable string for logging fetchedBuilds.
	fetchedBuildsList, err := json.MarshalIndent(fetchedBuilds, "", "  ")
	if err != nil {
		return nil, err
	}

	common.Stdout.Printf("The following %d builds were fetched from long term storage", len(fetchedBuilds))
	common.Stdout.Printf("************************************************")
	common.Stdout.Printf("%s", string(fetchedBuildsList))
	common.Stdout.Printf("************************************************")

	return fetchedBuilds, nil
}

// removeDuplicateRequests reads through all CTP requests and removes duplicate entries.
// Note the eventUUID.
func removeDuplicateRequests(incomingRequests []*ctpEvent) ([]*ctpEvent, error) {
	seenBefore := map[string]string{}
	dupeFreeEvents := []*ctpEvent{}

	for _, event := range incomingRequests {
		data, err := protojson.Marshal(event.ctpRequest)
		if err != nil {
			return nil, err
		}

		// If the proto output has been seen before then that means this is a
		// duplicated task.
		if originalEventUUID, ok := seenBefore[string(data)]; !ok {
			seenBefore[string(data)] = event.event.EventUuid

			dupeFreeEvents = append(dupeFreeEvents, event)
		} else {
			common.Stderr.Printf("Event %s is a duplicate of %s, removing from CTP request list.", event.event.EventUuid, originalEventUUID)
		}
	}
	return dupeFreeEvents, nil
}
