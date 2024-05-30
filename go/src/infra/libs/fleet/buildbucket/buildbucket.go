// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package buildbucket contains all the necessary code to schedule a CTP
// build for running a test using buildbucket APIs.
package buildbucket

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/ctp/builder"
	luciauth "go.chromium.org/luci/auth"
	bb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/server/auth"
)

const (
	// DefaultSwarmingPriority is the default priority for a Swarming task.
	DefaultSwarmingPriority = int64(140)
	// MinSwarmingPriority is the lowest-allowed priority for a Swarming task.
	MinSwarmingPriority = int64(50)
	// MaxSwarmingPriority is the highest-allowed priority for a Swarming task.
	MaxSwarmingPriority = int64(255)
	// buildBucketHost is the URL host for the BuildBucket API.
	buildBucketHost = "cr-buildbucket.appspot.com"
	// defaultImageBucket is the default bucket for getting image archives
	defaultImageBucket = "chromeos-image-archive"
	// defaultTestTypeTag is the default type of build bucket test
	defaultTestTypeTag = "test"
	// defaultCTPTimeout is the default timeout for CTP builder build.
	defaultCTPTimeout = 1200
)

var (
	ctpBuilderIDProd = bb.BuilderID{
		Project: "chromeos",
		Bucket:  "testplatform",
		Builder: "cros_test_platform",
	}
	ctpBuilderIDStaging = bb.BuilderID{
		Project: "chromeos",
		Bucket:  "testplatform",
		Builder: "cros_test_platform-dev",
	}
)

// BuildbucketClient interface provides subset of Buildbucket methods relevant to Fleet use cases
type BuildbucketClient interface {
	ScheduleCTPBuild(ctx context.Context) (*bb.Build, error)
}

// client wraps the buildbucket client.
type client struct {
	buildBucketClient BuildsClient
}

// BuildsClient is a subset of buildbucketpb.BuildsClient providing a smaller surface area for unit tests
type BuildsClient interface {
	SearchBuilds(ctx context.Context, in *bb.SearchBuildsRequest, opts ...grpc.CallOption) (*bb.SearchBuildsResponse, error)
	GetBuild(context.Context, *bb.GetBuildRequest, ...grpc.CallOption) (*bb.Build, error)
	ScheduleBuild(context.Context, *bb.ScheduleBuildRequest, ...grpc.CallOption) (*bb.Build, error)
}

// NewBuildBucketClient creates a client to communicate with Buildbucket.
func NewBuildBucketClient(ctx context.Context) (*client, error) {
	bbClient, err := newBuildsClient(ctx, buildBucketHost)
	if err != nil {
		return nil, err
	}

	return &client{
		buildBucketClient: bbClient,
	}, nil
}

func newBuildsClient(ctx context.Context, host string) (bb.BuildsClient, error) {
	t, err := auth.GetRPCTransport(ctx, auth.AsSelf)
	if err != nil {
		return nil, err
	}
	return bb.NewBuildsPRPCClient(
		&prpc.Client{
			C:       &http.Client{Transport: t},
			Host:    host,
			Options: prpc.DefaultOptions(),
		}), nil
}

// GetLatestGreenBuild gets the latest green build for the given builder.
func (c *client) GetLatestGreenBuild(ctx context.Context) (*bb.Build, error) {
	searchBuildsRequest := &bb.SearchBuildsRequest{
		Predicate: &bb.BuildPredicate{
			Builder: &ctpBuilderIDProd,
			Status:  bb.Status_SUCCESS,
		},
		Fields: &field_mask.FieldMask{Paths: []string{
			"builds.*.id",
			"builds.*.output.properties",
		}},
	}
	// Avoid the getAllBuilds function since it scrolls through all pages of
	// the search result, and we only want the most recent build.
	response, err := c.buildBucketClient.SearchBuilds(ctx, searchBuildsRequest)
	if err != nil {
		return nil, err
	}
	if len(response.Builds) == 0 {
		return nil, fmt.Errorf("no green builds found for builder %s", &ctpBuilderIDProd)
	}
	return response.Builds[0], nil
}

// Run holds the arguments that are needed for the run command.
type Run struct {
	Image     string
	Model     string
	Board     string
	Milestone string
	Build     string
	Pool      string
	Suite     string
	Tests     []string
	Testplan  string
	Harness   string
	TestArgs  string
	CFT       bool
	// TRV2 determines whether we will use Test Runner V2
	TRV2        bool
	TimeoutMins int
	// Any configs related to results upload for this test run.
	AddedDims map[string]string
	Tags      map[string]string
	IsProd    bool
	BBClient  BuildsClient
	Priority  int64

	UploadToCpcon bool
}

// TriggerRun triggers the Run with the given information
// (it could be either single test or a suite or a test_plan in the GCS bucket or test_plan saved locally)
func (c *Run) TriggerRun(ctx context.Context) (string, error) {
	err := c.validateDimensions(ctx)
	if err != nil {
		return "", err
	}
	bbClient, err := c.createCTPBuilder(ctx)
	if err != nil {
		return "", err
	}

	bbClient.BBClient = c.BBClient
	link, err := ScheduleBuild(ctx, bbClient)
	if err != nil {
		return "", err
	}
	return link, nil
}

// ScheduleBuild register a build. If it successes, it returns a link of build. Otherwise,
// return an error.
func ScheduleBuild(ctx context.Context, bbClient BuildbucketClient) (string, error) {
	ctpBuild, err := bbClient.ScheduleCTPBuild(ctx)
	if err != nil {
		return "", err
	}
	link := fmt.Sprintf("https://ci.chromium.org/ui/b/%s", strconv.Itoa(int(ctpBuild.Id)))
	logging.Infof(ctx, "Scheduled build at %s", link)
	return link, nil
}

func (c *Run) createCTPBuilder(ctx context.Context) (*builder.CTPBuilder, error) {
	// Create TestPlan for suite or test
	tp := builder.TestPlanForTests("", c.Harness, c.Tests)
	if tp == nil {
		return nil, fmt.Errorf("failed to build test plan for tests")
	}
	var res *builder.CTPBuilder
	// Set tags to pass to ctp and test runner builds
	tags := map[string]string{}
	tags["test-type"] = defaultTestTypeTag

	dims := c.AddedDims
	// Will be nil if not provided by user.
	if dims == nil {
		dims = make(map[string]string)
	}

	builderID := &ctpBuilderIDStaging
	if c.IsProd {
		builderID = &ctpBuilderIDProd
	}

	if c.Image == "" {
		c.Image = fmt.Sprintf("%s-release/R%s-%s", c.Board, c.Milestone, c.Build)
	}
	if c.TimeoutMins == 0 {
		c.TimeoutMins = defaultCTPTimeout
	}
	if c.Priority == 0 {
		c.Priority = DefaultSwarmingPriority
	}
	res = &builder.CTPBuilder{
		Image:               c.Image,
		Board:               c.Board,
		Model:               c.Model,
		Pool:                c.Pool,
		CFT:                 true,
		TestPlan:            tp,
		BuilderID:           builderID,
		Dimensions:          dims,
		ImageBucket:         defaultImageBucket,
		AuthOptions:         &luciauth.Options{},
		TestRunnerBuildTags: tags,
		TimeoutMins:         c.TimeoutMins,
		CTPBuildTags:        tags,
		TRV2:                c.TRV2,
		CpconPublish:        c.UploadToCpcon,
		Priority:            c.Priority,
	}
	return res, nil
}

func (c *Run) validateDimensions(ctx context.Context) error {
	errs := make(errors.MultiError, 0)
	if c.Board == "" {
		errs = append(errs, fmt.Errorf("missing board field"))
	}
	if c.Pool == "" {
		errs = append(errs, fmt.Errorf("missing pool field"))
	}

	// If running an individual test via CTP, we require the test harness to be
	// specified.
	if c.CFT && c.Harness == "" {
		errs = append(errs, fmt.Errorf("missing harness flag"))
	}
	// harness should not be provided for non-cft.
	if !c.CFT && c.Harness != "" {
		errs = append(errs, fmt.Errorf("harness should only be provided for single cft test case"))
	}
	// trv2 should be false for non-cft.
	if !c.CFT && c.TRV2 {
		errs = append(errs, fmt.Errorf("cannot run non-cft test case via trv2"))
	}
	if c.Image != "" && c.Milestone != "" {
		errs = append(errs, fmt.Errorf("cannot specify both image and release branch"))
	}
	if c.Priority != 0 && (c.Priority < MinSwarmingPriority || c.Priority > MaxSwarmingPriority) {
		errs = append(errs, fmt.Errorf("priority flag should be in [%d, %d]", MinSwarmingPriority, MaxSwarmingPriority))
	}

	if errs.First() != nil {
		return errs.AsError()
	}
	return nil
}
