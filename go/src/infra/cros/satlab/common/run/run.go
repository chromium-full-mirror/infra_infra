// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/golang/protobuf/jsonpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
	moblabpb "google.golang.org/genproto/googleapis/chromeos/moblab/v1beta1"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/ctp/builder"
	"go.chromium.org/chromiumos/infra/proto/go/satlabrpcserver"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"

	"infra/cros/satlab/common/google.golang.org/google/chromeos/moblab"
	"infra/cros/satlab/common/satlabcommands"
	"infra/cros/satlab/common/site"
	"infra/cros/satlab/common/utils/executor"
	"infra/cros/satlab/common/utils/misc"
)

const (
	hostname                = "us-docker.pkg.dev"
	testServicesRegistry    = "cros-registry/test-services"
	incrementalRunBinary    = "pvs_incremental_run_filter"
	incrementalRunContainer = "pvs-incremental-run-filter"
	incrementalRunDigest    = "sha256:815065a0f464c3f64d6dee321ef9abff9530fc8ca5d8808fc225ccca8ff37ecb"
	prodTag                 = "prod"
	desktopPrefix           = "AL."
	dummySuiteName          = "TestSuite"
)

// Run holds the arguments that are needed for the run command.
type Run struct {
	Image         string
	Model         string
	Board         string
	Milestone     string
	Build         string
	Pool          string
	Suite         string
	Tests         []string
	Testplan      string
	TestplanLocal string
	Harness       string
	TestArgs      string
	SatlabId      string
	Desktop       bool
	CFT           bool
	// TRV2 determines whether we will use Test Runner V2
	TRV2             bool
	DynamicTRV2      bool
	Local            bool
	TimeoutMins      int
	TagIncludes      []string
	TagExcludes      []string
	TestNameIncludes []string
	TestNameExcludes []string
	// Runs with Ctpv2 and Quota Scheduler if true and CFT is true
	RunCtpv2WithQs bool
	// If true, only runs the tests that have not passed
	IsIncrementalRun bool
	// Any configs related to results upload for this test run.
	AddedDims map[string]string
	Tags      map[string]string

	UploadToCpcon bool
	MaxInShard    int64
}

// TriggerRun triggers the Run with the given information
// (it could be either single test or a suite or a test_plan in the GCS bucket or test_plan saved locally)
func (c *Run) TriggerRun(ctx context.Context) (string, error) {
	bbClients, err := c.createCTPBuilders(ctx)
	if err != nil {
		return "", err
	}
	var links []string
	for _, bbClient := range bbClients {
		// Create default client
		err = bbClient.AddDefaultBBClient(ctx)
		if err != nil {
			return "", err
		}

		moblabClient, err := moblab.NewBuildClient(ctx, option.WithCredentialsFile(site.GetServiceAccountPath()))
		if err != nil {
			return "", errors.Annotate(err, "satlab new moblab api build client").Err()
		}

		link, err := c.triggerRunWithClients(ctx, moblabClient, bbClient, site.GetGCSImageBucket())
		if err != nil {
			return "", errors.Annotate(err, "triggerRunWithClients").Err()
		}
		links = append(links, link)
	}

	if len(links) > 0 {
		return strings.Join(links, "\n"), nil
	}
	return "", nil
}

func (c *Run) createCTPBuilders(ctx context.Context) ([]*builder.CTPBuilder, error) {
	// Create TestPlan for suite or test
	tp, err := c.createTestPlan()
	if err != nil {
		return nil, err
	}
	var res []*builder.CTPBuilder
	// Set tags to pass to ctp and test runner builds
	tags := c.setTags(ctx)

	dims := c.AddedDims
	// Will be nil if not provided by user.
	if dims == nil {
		dims = make(map[string]string)
	}

	// Get drone target based on user input, defaulting to the current box.
	droneDim, err := c.getDroneTarget(ctx)
	if err != nil {
		return nil, err
	}
	if droneDim != "" {
		dims["drone"] = droneDim
	}

	builderId := &buildbucketpb.BuilderID{
		Project: site.GetLUCIProject(),
		Bucket:  site.GetCTPBucket(),
		Builder: site.GetCTPBuilder(),
	}

	if c.Image == "" {
		if misc.IsCustomBuild(c.Build) {
			c.Image = fmt.Sprintf("%s-local/R%s-%s", c.Board, c.Milestone, c.Build)
		} else {
			c.Image = fmt.Sprintf("%s-release/R%s-%s", c.Board, c.Milestone, c.Build)
		}
	}
	opt := site.GetAuthOption(ctx)

	if tp.Cft != nil {
		singleTestPlans, err := splitTestPlan(tp.Cft)
		if err != nil {
			return nil, err
		}
		for _, stp := range singleTestPlans {
			// append the args to the first suite if a suite exists
			if len(stp.Suite) > 0 && c.TestArgs != "" {
				stp.Suite[0].TestArgs = c.TestArgs
			}
			if c.MaxInShard > 0 {
				stp.MaxInShard = c.MaxInShard
			}
			res = append(res, &builder.CTPBuilder{
				Image:               c.Image,
				Board:               c.Board,
				Model:               c.Model,
				Pool:                c.Pool,
				CFT:                 true,
				RunCtpv2WithQs:      c.RunCtpv2WithQs,
				TestPlan:            stp,
				BuilderID:           builderId,
				Dimensions:          dims,
				ImageBucket:         site.GetGCSImageBucket(),
				AuthOptions:         &opt,
				TestRunnerBuildTags: tags,
				TimeoutMins:         c.setTimeout(),
				CTPBuildTags:        tags,
				TRV2:                c.TRV2 || c.DynamicTRV2,
				DynamicTRV2:         c.DynamicTRV2,
				CpconPublish:        c.UploadToCpcon,
				UserDefinedFilters:  c.userDefinedFilters(),
			})
		}
	}

	if tp.NonCft != nil {
		res = append(res, &builder.CTPBuilder{
			Image:               c.Image,
			Board:               c.Board,
			Model:               c.Model,
			Pool:                c.Pool,
			CFT:                 false,
			TestPlan:            tp.NonCft,
			BuilderID:           builderId,
			Dimensions:          dims,
			ImageBucket:         site.GetGCSImageBucket(),
			AuthOptions:         &opt,
			TestRunnerBuildTags: tags,
			TimeoutMins:         c.setTimeout(),
			CTPBuildTags:        tags,
		})
	}
	return res, nil
}

// splitTestPlans splits the testplan with suites into those with single suite.
func splitTestPlan(tp *test_platform.Request_TestPlan) ([]*test_platform.Request_TestPlan, error) {
	suites := tp.GetSuite()
	tests := tp.GetTest()
	res := []*test_platform.Request_TestPlan{}
	for _, suite := range suites {
		singleTestPlan := &test_platform.Request_TestPlan{
			Suite:                  []*test_platform.Request_Suite{suite},
			Enumeration:            tp.GetEnumeration(),
			TagCriteria:            tp.GetTagCriteria(),
			Seed:                   tp.GetSeed(),
			TestArgs:               tp.GetTestArgs(),
			TotalShards:            tp.GetTotalShards(),
			MaxInShard:             tp.GetMaxInShard(),
			EnableAutotestSharding: tp.GetEnableAutotestSharding(),
		}
		res = append(res, singleTestPlan)
	}
	if len(tests) > 0 {
		if err := adaptTestsToCftFormat(tests); err != nil {
			return nil, err
		}
		res = append(res, &test_platform.Request_TestPlan{
			Test:                   tests,
			Enumeration:            tp.GetEnumeration(),
			TagCriteria:            tp.GetTagCriteria(),
			Seed:                   tp.GetSeed(),
			TestArgs:               tp.GetTestArgs(),
			TotalShards:            tp.GetTotalShards(),
			MaxInShard:             tp.GetMaxInShard(),
			EnableAutotestSharding: tp.GetEnableAutotestSharding(),
		})
	}
	return res, nil
}

// adaptTestsToCftFormat adjusts the old tests format to new CFT version.
func adaptTestsToCftFormat(tests []*test_platform.Request_Test) error {
	tastTautoNames := []string{"tast.generic-servo", "tast.generic"}
	for _, test := range tests {
		testName := test.GetAutotest().GetName()
		testArgs := misc.StrTestArgsToMap(test.GetAutotest().GetTestArgs())

		// Tast test run via test_that.
		if slices.Contains(tastTautoNames, testName) {
			if len(testArgs) == 0 || testArgs["tast_expr"] == "" {
				return fmt.Errorf("empty testArgs field or no 'tast_expr' for %s test! You need to specify at least 'tast_expr'", testName)
			}
			test.GetAutotest().Name = "tast." + testArgs["tast_expr"]
			delete(testArgs, "tast_expr")
			tastTestArgs := misc.RemovePrefixFromTestArgs(testArgs, "tast.")
			test.GetAutotest().TestArgs = misc.MapTestArgsToStr(tastTestArgs)
			continue
		}
		// Already compatible tests.
		if strings.HasPrefix(testName, "tast.") || strings.HasPrefix(testName, "tauto.") {
			continue
		}
		// The rest tests need to be autotest.
		test.GetAutotest().Name = "tauto." + testName
	}
	return nil
}

// userDefinedFilters configures the ctpv2 filters based on the parameters set
// for this Run.
func (c *Run) userDefinedFilters() []*api.CTPFilter {
	var userDefinedFilters []*api.CTPFilter
	if c.IsIncrementalRun {
		userDefinedFilters = append(userDefinedFilters, &api.CTPFilter{
			ContainerInfo: &api.ContainerInfo{
				BinaryName: incrementalRunBinary,
				Container: &buildapi.ContainerImageInfo{
					Repository: &buildapi.GcrRepository{
						Hostname: hostname,
						Project:  testServicesRegistry,
					},
					Name:   incrementalRunContainer,
					Digest: incrementalRunDigest,
					Tags:   []string{prodTag},
				},
			},
		})
	}
	if c.Desktop {
		userDefinedFilters = append(userDefinedFilters, &api.CTPFilter{
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{
					Name: "al-provision-filter",
				},
			},
		}, &api.CTPFilter{
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{
					Name: "foil-filter",
				},
			},
		}, &api.CTPFilter{
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{
					Name: "test-finder",
				},
			},
		}, &api.CTPFilter{
			ContainerInfo: &api.ContainerInfo{
				Container: &buildapi.ContainerImageInfo{
					Name: "ants-publish-filter",
				},
			},
		})
	}
	return userDefinedFilters
}

func (c *Run) triggerRunWithClients(ctx context.Context, moblabClient MoblabClient, bbClient BuildbucketClient, gcsBucket string) (string, error) {

	// There is no explicit check on whether staging of the image is successful or not
	// There are 2 reasons for this:
	// 1. "Custom chromeOS builds" are expected to be already in the partner bucket. There is no
	// check on whether that already exists in the bucket. (In an ideal world, there would be
	// one, but right now there is none. This is much harder because there is no list of
	// compulsory artifacts that should exist in the folder)
	// 2. Latency: Waiting for the copying to take place is not a good user experience and
	// is not necessary anyway in this case. Although copying is fairly quick, it is left to
	// be handled by server in the background
	_ = StageImageToBucket(ctx, moblabClient, c.Board, c.Model, c.Build)

	link, err := ScheduleBuild(ctx, bbClient)
	if err != nil {
		return "", errors.Annotate(err, "satlab schedule build").Err()
	}
	return link, nil
}

// SetTags set tags for associated tests, testplab and suites
func (c *Run) setTags(ctx context.Context) map[string]string {
	tags := map[string]string{}
	// Add user-added tags.
	for key, val := range c.Tags {
		tags[key] = val
	}

	// Get satlab ID set by user, defaulting to the current box id.
	satlabID, err := c.getDroneTarget(ctx)
	if err == nil && satlabID != "" {
		tags["satlab-id"] = satlabID
	}

	switch {
	case c.Testplan != "":
		// TODO(prasadv): Move this value to proto enum
		tags["test-type"] = "testplan"
		tags["test-plan-id"] = strings.TrimSuffix(c.Testplan, ".json")
	case c.TestplanLocal != "":
		tags["test-type"] = "testplan"
		tags["test-plan-id"] = strings.TrimSuffix(c.TestplanLocal, ".json")
	case c.Suite != "":
		// TODO(prasadv): Move this value to proto enum
		tags["test-type"] = "suite"
		tags["label-suite"] = c.Suite
	case c.Tests != nil:
		// TODO(prasadv): Move this value to proto enum
		tags["test-type"] = "test"
	}

	return tags
}

// Determine CTP timeout based on user input
func (c *Run) setTimeout() int {
	if c.TimeoutMins != 0 {
		return c.TimeoutMins
	}
	return site.DefaultCTPTimeoutMins
}

func (c *Run) createTestPlan() (*satlabrpcserver.CftMixTestplan, error) {
	var tp *test_platform.Request_TestPlan
	if c.Desktop {
		c.adaptSuiteName()
	}
	if c.Suite != "" {
		tp = builder.TestPlanForSuites([]string{c.Suite})
		if len(c.TagIncludes) > 0 || len(c.TagExcludes) > 0 || len(c.TestNameIncludes) > 0 || len(c.TestNameExcludes) > 0 {
			tp.TagCriteria = &api.TestSuite_TestCaseTagCriteria{
				Tags:             c.TagIncludes,
				TagExcludes:      c.TagExcludes,
				TestNames:        c.TestNameIncludes,
				TestNameExcludes: c.TestNameExcludes,
			}
		}
		if c.CFT {
			return &satlabrpcserver.CftMixTestplan{Cft: tp}, nil
		} else {
			return &satlabrpcserver.CftMixTestplan{NonCft: tp}, nil
		}
	} else if c.Tests != nil {
		tp = builder.TestPlanForTests(c.TestArgs, c.Harness, c.Tests)
		if c.CFT {
			return &satlabrpcserver.CftMixTestplan{Cft: tp}, nil
		} else {
			return &satlabrpcserver.CftMixTestplan{NonCft: tp}, nil
		}
	} else if c.Testplan != "" {
		fmt.Printf("Fetching testplan...\n")
		var w bytes.Buffer
		path, err := downloadTestPlan(&w, site.GetGCSPartnerBucket(), c.Testplan)
		if err != nil {
			return nil, err
		}
		return c.readTestPlan(path)
	} else if c.TestplanLocal != "" {
		return c.readTestPlan(c.TestplanLocal)
	}
	return nil, fmt.Errorf("createTestPlan: must provide a suite/test/testplan")
}

// StageImageToBucket stages the specified Chrome OS image to the user GCS bucket
func StageImageToBucket(ctx context.Context, moblabClient MoblabClient, board string, model string, buildVersion string) error {
	bucket := site.GetGCSImageBucket()
	if bucket == "" {
		return errors.New("GCS_BUCKET not found")
	}

	buildTarget := fmt.Sprintf("buildTargets/%s/models/%s", board, model)
	artifactName := fmt.Sprintf("%s/builds/%s/artifacts/%s", buildTarget, buildVersion, bucket)
	stageReq := &moblabpb.StageBuildRequest{
		Name: artifactName,
	}

	_, err := moblabClient.StageBuild(ctx, stageReq)
	if err != nil {
		return err
	}
	var stageStatus *moblabpb.CheckBuildStageStatusResponse
	count := 10
	for {
		count--
		req := &moblabpb.CheckBuildStageStatusRequest{
			Name: artifactName,
		}
		stageStatus, err = moblabClient.CheckBuildStageStatus(ctx, req)
		if err != nil {
			return err
		}
		if stageStatus.IsBuildStaged {
			break
		}
		if count == 0 {
			return fmt.Errorf("stage not completed within 10 retries")
		}
	}
	destPath := stageStatus.StagedBuildArtifact.Path

	fmt.Printf("Artifacts staged to %s\n", path.Join(bucket, destPath))
	return nil
}

// ScheduleBuild register a build. If it successes, it returns a link of build. Otherwise,
// return an error.
func ScheduleBuild(ctx context.Context, bbClient BuildbucketClient) (string, error) {
	ctpBuild, err := bbClient.ScheduleCTPBuild(ctx)
	if err != nil {
		return "", err
	}
	link := fmt.Sprintf("https://ci.chromium.org/ui/b/%s", strconv.Itoa(int(ctpBuild.Id)))
	return link, nil
}

// Set drone target to user-provided satlab or local satlab if one isn't provided
func (c *Run) getDroneTarget(ctx context.Context) (string, error) {
	var satlabTarget string
	if c.SatlabId != "" {
		satlabTarget = fmt.Sprintf(c.SatlabId)
	} else if c.Local { // get id of local satlab if one is not provided
		localSatlab, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, &executor.ExecCommander{})
		if err != nil {
			return "", errors.Annotate(err, "satlab get docker host box identifier").Err()
		}
		satlabTarget = fmt.Sprintf("satlab-%s", localSatlab)
	}
	return satlabTarget, nil
}

// BuildbucketClient interface provides subset of Buildbucket methods relevant to Satlab CLI
type BuildbucketClient interface {
	ScheduleCTPBuild(ctx context.Context) (*buildbucketpb.Build, error)
}

// MoblabClient interface provides subset of Moblab API methods relevant to Satlab CLI
type MoblabClient interface {
	StageBuild(ctx context.Context, req *moblabpb.StageBuildRequest, opts ...gax.CallOption) (*moblab.StageBuildOperation, error)
	CheckBuildStageStatus(ctx context.Context, req *moblabpb.CheckBuildStageStatusRequest, opts ...gax.CallOption) (*moblabpb.CheckBuildStageStatusResponse, error)
}

// Downloads specified testplan from bucket to remote access container
func downloadTestPlan(w io.Writer, bucket, testplan string) (string, error) {
	object := "testplans/" + testplan
	destFileName := "/config/" + testplan
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(site.GetServiceAccountPath()))
	if err != nil {
		return "", fmt.Errorf("storage.NewClient: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("%q Error: %w", object, err)
	}
	defer rc.Close()

	err = os.MkdirAll(filepath.Dir(destFileName), 0777)
	if err != nil {
		return "", fmt.Errorf("os.MkdirAll: %w", err)
	}

	f, err := os.Create(destFileName)
	if err != nil {
		return "", fmt.Errorf("os.Create: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return "", fmt.Errorf("io.Copy: %w", err)
	}

	fmt.Fprintf(w, "Blob %v downloaded to local file %v\n", object, destFileName)

	return destFileName, nil
}

// JSONPBUnmarshaler unmarshals JSON into proto messages.
var JSONPBUnmarshaler = jsonpb.Unmarshaler{AllowUnknownFields: true}

func (c *Run) readTestPlan(path string) (*satlabrpcserver.CftMixTestplan, error) {
	tp, err := c.readMixedTestPlan(path)

	if err != nil {
		fmt.Printf("Cannot parse the test plan with mixed format: %v\n", err)
		return c.readSingleTestplan(path)
	}
	return tp, nil
}

func (c *Run) readSingleTestplan(path string) (*satlabrpcserver.CftMixTestplan, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error reading test plan: %w", err)
	}
	defer file.Close()

	testPlan := &test_platform.Request_TestPlan{}
	if err := JSONPBUnmarshaler.Unmarshal(file, testPlan); err != nil {
		return nil, fmt.Errorf("error reading test plan: %w", err)
	}
	if c.CFT {
		return &satlabrpcserver.CftMixTestplan{Cft: testPlan}, nil
	}
	return &satlabrpcserver.CftMixTestplan{NonCft: testPlan}, nil
}

func (c *Run) readMixedTestPlan(path string) (*satlabrpcserver.CftMixTestplan, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error reading test plan: %w", err)
	}
	defer file.Close()
	mixedTestPlan := &satlabrpcserver.CftMixTestplan{}
	if err := JSONPBUnmarshaler.Unmarshal(file, mixedTestPlan); err != nil {
		return nil, fmt.Errorf("error reading test plan: %w", err)
	}
	if mixedTestPlan.Cft == nil && mixedTestPlan.NonCft == nil {
		return nil, fmt.Errorf("readMixedTestPlan: %s is not a mixed testplan", path)
	}
	return mixedTestPlan, nil
}

// adaptSuiteName ensures that suite has correct name for desktop test.
func (c *Run) adaptSuiteName() {
	if c.Suite == "" {
		c.Suite = desktopPrefix + dummySuiteName
	} else if !strings.HasPrefix(c.Suite, desktopPrefix) {
		c.Suite = desktopPrefix + c.Suite
	}
}
