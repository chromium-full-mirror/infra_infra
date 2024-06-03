// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package builds fetches and handles the build image information from the
// release team.
package builds

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	cloudPubsub "cloud.google.com/go/pubsub"
	"golang.org/x/sync/errgroup"

	buildpb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"

	"infra/cros/cmd/kron/cloudsql"
	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/metrics"
	"infra/cros/cmd/kron/pubsub"
)

// extractMilestoneAndVersion returns the milestone and platform version from
// the build report's versions lists.
func extractMilestoneAndVersion(versions []*buildpb.BuildReport_BuildConfig_Version) (int64, string, error) {
	var err error
	milestone := int64(0)
	version := ""

	// Extract milestone and platform version from the versions list.
	for _, versionInfo := range versions {
		switch versionInfo.Kind {
		case buildpb.BuildReport_BuildConfig_VERSION_KIND_MILESTONE:
			milestone, err = strconv.ParseInt(versionInfo.Value, 10, 64)
			if err != nil {
				return 0, "", err
			}
		case buildpb.BuildReport_BuildConfig_VERSION_KIND_PLATFORM:
			version = versionInfo.Value

		}
	}

	return milestone, version, nil
}

// extractImagePath returns the GCS path for the image.zip from the report's
// artifact list.
func extractImagePath(artifacts []*buildpb.BuildReport_BuildArtifact) (string, error) {
	// Fetch the GCS path to the created image.
	for _, artifact := range artifacts {
		switch artifact.Type.String() {
		case "IMAGE_ZIP":
			return artifact.Uri.GetGcs(), nil
		}
	}

	return "", fmt.Errorf("no imagePath found in artifacts")
}

// extractBoardAndVariant will extract the board and potential variant from the
// build target.
func extractBoardAndVariant(buildTarget string) (string, string, error) {
	board := ""
	variant := ""
	// amd64-generic is a unique board which has a hyphen in its board name. If
	// more boards begin to follow this pattern we may want to turn this into a
	// tracking list.
	if buildTarget == "amd64-generic" || buildTarget == "fizz-labstation" {
		board = buildTarget
	} else if !strings.Contains(buildTarget, "-") && strings.HasSuffix(buildTarget, "64") {
		board = buildTarget[:len(buildTarget)-2]
		variant = "64"
	} else {
		before, after, didCut := strings.Cut(buildTarget, "-")
		board = before
		if didCut {
			variant = after
		}
	}

	if board == "" {
		return "", "", fmt.Errorf("no board provided in build target")
	}

	return board, variant, nil
}

// generateBuildUUIDHash returns a hash of the build dimensions for use in the
// uuid field.
func generateBuildUUIDHash(buildTarget, board, version string, milestone int) (string, error) {
	uniqueTarget := fmt.Sprintf("%s,%s,%d,%s", buildTarget, board, milestone, version)

	hasher := fnv.New64a()

	_, err := hasher.Write([]byte(uniqueTarget))
	if err != nil {
		return "", err
	}

	return strconv.FormatUint(hasher.Sum64(), 10), nil
}

// TransformReportToKronBuild takes a build report and returns all relevant
// builds in a Kron parsable form.
func TransformReportToKronBuild(report *buildpb.BuildReport) (*kronpb.Build, error) {
	milestone, version, err := extractMilestoneAndVersion(report.Config.Versions)
	if err != nil {
		return nil, fmt.Errorf("%d: %w", report.GetBuildbucketId(), err)
	}

	imagePath, err := extractImagePath(report.Artifacts)
	if err != nil {
		return nil, fmt.Errorf("%d: %w", report.GetBuildbucketId(), err)
	}

	board, variant, err := extractBoardAndVariant(report.Config.Target.Name)
	if err != nil {
		return nil, fmt.Errorf("%d: %w", report.GetBuildbucketId(), err)
	}

	buildHash, err := generateBuildUUIDHash(report.Config.Target.Name, board, version, int(milestone))
	if err != nil {
		return nil, err
	}
	return &kronpb.Build{
		BuildUuid:       buildHash,
		RunUuid:         metrics.GetRunID(),
		CreateTime:      common.TimestamppbNowWithoutNanos(),
		Bbid:            report.GetBuildbucketId(),
		BuildTarget:     report.Config.Target.Name,
		Milestone:       milestone,
		Version:         version,
		ImagePath:       imagePath,
		Board:           board,
		Variant:         variant,
		ReleaseOrchBbid: report.GetParent().GetBuildbucketId()}, nil
}

// validateReport checks that all necessary fields are not nil.
func validateReport(report *buildpb.BuildReport) error {
	reportBBID := report.GetBuildbucketId()
	if report.Config == nil {
		return fmt.Errorf("report for go/bbid/%d contains a nil config", reportBBID)
	}

	if report.Config.Target == nil {
		return fmt.Errorf("report for go/bbid/%d contains a nil build target", reportBBID)
	}

	if report.Status == nil {
		return fmt.Errorf("report for go/bbid/%d contains a nil status field", reportBBID)
	}
	if report.GetParent().GetBuildbucketId() == 0 {
		return fmt.Errorf("report for go/bbid/%d does not contains parent bbid", reportBBID)
	}
	return nil
}

// BuildReportPackage contains the unmarshalled report and the Pub/Sub message
// in which it came from.
type BuildReportPackage struct {
	Report  *buildpb.BuildReport
	Message *cloudPubsub.Message
}

// handler is the implements the client to receive and process pub/aub messages.
type handler struct {
	buildsChan chan *BuildReportPackage
}

// processPSMessage is called within the Pubsub receive callback to process the
// contents of the message.
func (h *handler) processPSMessage(msg *cloudPubsub.Message) error {
	// Unmarshall the raw data into the BuildReport format.
	buildReport := buildpb.BuildReport{}
	// google.golang.org/protobuf/proto specifically needs to be used here to
	// handle the format of proto serialization being done from the recipes
	// builder.
	err := common.ProtoUnmarshaller.Unmarshal(msg.Data, &buildReport)
	if err != nil {
		return err
	}
	if err := validateReport(&buildReport); err != nil {
		// Ack the invalid report because it will just sit in the queue otherwise.
		msg.Ack()
		common.Stderr.Println(err)
		return nil
	}

	// Check for a successful release build. Ignore all types of reports.
	if !(buildReport.Type == buildpb.BuildReport_BUILD_TYPE_RELEASE && buildReport.Status.Value.String() == "SUCCESS") {
		msg.Ack()
		return nil
	}

	common.Stdout.Printf("Processing build report for go/bbid/%d\n", buildReport.GetBuildbucketId())
	report := &BuildReportPackage{
		Report:  &buildReport,
		Message: msg,
	}

	h.buildsChan <- report
	return nil
}

func (h *handler) closeChan() {
	close(h.buildsChan)
}

// handleFetchedBuilds will aggregate all messages into a list for later
// processing. When the builds channel is closed then the finalize call will be
// used.
//
// NOTE: finalize() will need to Nack/AcK all messages otherwise the
// subscription client from Pub/Sub will hang.
func (h *handler) handleFetchedBuilds(builds *[]*BuildReportPackage, buildsChan chan *BuildReportPackage, finalize func(*[]*BuildReportPackage) error) error {
	for build := range buildsChan {
		*builds = append(*builds, build)
	}

	common.Stdout.Println("Initializing finalize before ending Pub/Sub Subscription")
	err := finalize(builds)
	if err != nil {
		return err
	}
	common.Stdout.Println("Ending finalize before ending Pub/Sub Subscription")
	return nil
}

// IngestBuildsFromPubSub connects to pubsub ingests all new build information
// from the releases Pub/Sub stream. Once read, all builds will be written into
// long term storage.
func IngestBuildsFromPubSub(projectID, subscriptionName string, isProd bool, finalize func(*[]*BuildReportPackage) error) ([]*BuildReportPackage, error) {
	ctx := context.Background()

	psHandler := handler{
		buildsChan: make(chan *BuildReportPackage),
	}

	builds := []*BuildReportPackage{}

	// Spin up a goroutine to handle the incoming messages to the channel
	// buffer.
	//
	// NOTE: non-buffered channels in GO require that a ready consumer is
	// receiving before any messages can be passed in. If this is launched after
	// we begin sending messages into the channel the application will hang in a
	// deadlock.
	eg := new(errgroup.Group)
	eg.Go(func() error {
		return psHandler.handleFetchedBuilds(&builds, psHandler.buildsChan, finalize)
	})

	// Initialize the custom pubsub receiver. This custom handler implements a
	// timeout feature which will stop the pubsub Receive() call once no more
	// messages are incoming.
	common.Stdout.Println("Initializing Pub/Sub Receive Client")
	receiveClient, err := pubsub.InitReceiveClientWithTimer(ctx, projectID, subscriptionName, psHandler.closeChan, psHandler.processPSMessage)
	if err != nil {
		return nil, err
	}

	// NOTE: This is a blocking receive call. This will end when the child
	// context in the ReceiveClient expires due to no messages incoming.
	common.Stdout.Println("Pulling messages from Pub/Sub Queue")
	err = receiveClient.PullMessages()
	if err != nil {
		return nil, err
	}

	// Wait for the buffer receive function to end and return if an error has
	// occurred.
	if err = eg.Wait(); err != nil {
		return nil, err
	}

	common.Stdout.Printf("Returning %d Builds from Pub/Sub feed\n", len(builds))

	return builds, nil
}

// RequiredBuild encapsulates the information needed to request a build from
// PSQL for TimedEvents configs.
type RequiredBuild struct {
	BuildTarget string
	Board       string
	Milestone   int
}

// formatQuery inserts filters to select only the required builds specified.
//
// NOTE: We insert %s for the first token because we do not have the database
// name known to us here. This will be inserted by the cloudsql client.
func formatQuery(requiredBuilds []*RequiredBuild) (string, error) {
	if len(requiredBuilds) == 0 {
		return "", fmt.Errorf("no required builds to add to the SQL query")
	}

	// These WHERE clause filters will allow us to only fetch the
	// buildTargets:milestones we need.
	whereClauseItems := fmt.Sprintf(cloudsql.SelectWhereClauseItem, requiredBuilds[0].BuildTarget, requiredBuilds[0].Milestone)

	for _, requiredBuild := range requiredBuilds[1:] {
		item := fmt.Sprintf(cloudsql.SelectWhereClauseItem, requiredBuild.BuildTarget, requiredBuild.Milestone)

		whereClauseItems = strings.Join([]string{whereClauseItems, item}, " OR ")
	}

	// NOTE: Place a '%s' operator back into the string so that the sql client
	// can insert the table name it retrieved from the SecretManager.
	return fmt.Sprintf(cloudsql.SelectBuildsTemplate, "%s", whereClauseItems), nil
}

// IngestBuildsFromPSQL fetches the requested builds from long term PSQL
// storage.
func IngestBuildsFromPSQL(ctx context.Context, requiredBuilds []*RequiredBuild, isProd bool) ([]*kronpb.Build, error) {
	client, err := cloudsql.InitBuildsClient(ctx, isProd, false)
	if err != nil {
		return nil, err
	}

	// Generate the select query using the template.
	query, err := formatQuery(requiredBuilds)
	if err != nil {
		return nil, err
	}

	// Fetch the builds from the LTS PSQL database.
	builds, err := client.Read(ctx, query, cloudsql.ScanBuildRows)
	if err != nil {
		return nil, err
	}

	// Assert that value of the returned type is []*BuildPackage. To make the
	// client generic we have a return of any which means that we must confirm
	// and apply type to this new variable.
	//
	// NOTE: Generics have long been considered an anti-pattern to GO but
	// recently support for them was added at the stdlib level.
	// https://go.dev/ref/spec#Type_assertions for more info.
	buildPackages, ok := builds.([]*kronpb.Build)
	if !ok {
		return nil, fmt.Errorf("returned type from sql read is not of type []*kronpb.Build")
	}

	return buildPackages, nil
}
