// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/ctp/builder"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cmd/crosfleet/internal/buildbucket"
	"go.chromium.org/infra/cmd/crosfleet/internal/common"
	"go.chromium.org/infra/cmd/crosfleet/internal/site"
	models "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsapi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

var testValidateArgsData = []struct {
	testCommonFlags
	args                    []string
	wantValidationErrString string
	mainArgType             string
}{
	{ // All errors raised
		testCommonFlags{
			board:    "",
			models:   []string{"model1", "model2"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
			cft:      true,
		},
		[]string{},
		`missing board flag
missing pool flag
missing harness flag
cannot specify both image and release branch
priority flag should be in [50, 255]
total number of CTP runs launched (# models specified * repeats) cannot exceed 12
missing test arg`,
		"test",
	},
	{ // One error raised
		testCommonFlags{
			board:    "sample-board",
			models:   []string{},
			repeats:  13,
			pool:     "sample-pool",
			image:    "sample-image",
			priority: 255,
		},
		[]string{"sample-suite-name"},
		"total number of CTP runs launched (# models specified * repeats) cannot exceed 12",
		"suite",
	},
	{ // One error raised
		testCommonFlags{
			board:           "sample-board",
			models:          []string{},
			repeats:         1,
			pool:            "sample-pool",
			image:           "sample-image",
			secondaryBoards: []string{"coral", "eve"},
			secondaryImages: []string{"sample-image"},
			priority:        255,
		},
		[]string{"sample-suite-name"},
		"number of requested secondary-boards: 2 does not match with number of requested secondary-images: 1",
		"suite",
	},
	{ // One error raised
		testCommonFlags{
			board:           "sample-board",
			models:          []string{},
			repeats:         1,
			pool:            "sample-pool",
			image:           "sample-image",
			secondaryModels: []string{"babytiger", "eve"},
			priority:        255,
		},
		[]string{"sample-suite-name"},
		"number of requested secondary-boards: 0 does not match with number of requested secondary-models: 2",
		"suite",
	},
	{ // One error raised
		testCommonFlags{
			board:                "sample-board",
			models:               []string{},
			repeats:              1,
			pool:                 "sample-pool",
			image:                "sample-image",
			secondaryLacrosPaths: []string{"foo", "bar"},
			priority:             255,
		},
		[]string{"sample-suite-name"},
		"number of requested secondary-boards: 0 does not match with number of requested secondary-lacros-paths: 2",
		"suite",
	},
	{ // No errors raised
		testCommonFlags{
			board:    "sample-board",
			models:   []string{"model1", "model2"},
			repeats:  6,
			pool:     "sample-pool",
			release:  "sample-release",
			priority: 255,
		},
		[]string{"sample-suite-name"},
		"",
		"suite",
	},
	{ // No errors raised
		testCommonFlags{
			board:    "sample-board",
			models:   []string{"model1", "model2"},
			repeats:  6,
			pool:     "sample-pool",
			release:  "sample-release",
			priority: 255,
		},
		[]string{"sample-suite-name"},
		"",
		"test",
	},
	{ // No errors raised
		testCommonFlags{
			board:       "sample-board",
			models:      []string{"model1", "model2"},
			repeats:     6,
			pool:        "sample-pool",
			release:     "sample-release",
			priority:    255,
			cft:         true,
			testHarness: "sample-harness",
		},
		[]string{"sample-suite-name"},
		"",
		"test",
	},
	{ // One error raised
		testCommonFlags{
			board:       "sample-board",
			models:      []string{"model1", "model2"},
			repeats:     6,
			pool:        "sample-pool",
			release:     "sample-release",
			priority:    255,
			luciProject: "testProject",
		},
		[]string{"sample-suite-name"},
		"if luciProject is specified, PublicBuilderBucket and PublicBuilder should be specified",
		"suite",
	},
	{ // No errors raised
		testCommonFlags{
			board:         "sample-board",
			models:        []string{"model1", "model2"},
			repeats:       6,
			pool:          "sample-pool",
			release:       "sample-release",
			priority:      255,
			publicBuilder: "testBuilder",
		},
		[]string{"sample-suite-name"},
		"both PublicBuilderBucket and PublicBuilder should be specified",
		"suite",
	},
	{ // One errors raised
		testCommonFlags{
			board:       "sample-board",
			models:      []string{"model1", "model2"},
			repeats:     6,
			pool:        "sample-pool",
			release:     "sample-release",
			priority:    255,
			tagIncludes: []string{"tag1"},
		},
		[]string{"sample-suite-name"},
		"cannot run non-cft with tag_criteria args",
		"test",
	},
	{ // One errors raised
		testCommonFlags{
			board:                  "sample-board",
			models:                 []string{"model1", "model2"},
			repeats:                6,
			pool:                   "sample-pool",
			release:                "sample-release",
			priority:               255,
			enableAutotestSharding: true,
		},
		[]string{"sample-suite-name"},
		"cannot run non-cft with autotest sharding",
		"test",
	},
}

func TestValidateArgs(t *testing.T) {
	t.Parallel()
	for _, tt := range testValidateArgsData {
		t.Run(fmt.Sprintf("(%s)", tt.wantValidationErrString), func(t *testing.T) {
			t.Parallel()
			var flagSet flag.FlagSet
			if err := flagSet.Parse(tt.args); err != nil {
				t.Fatalf("unexpected error parsing command line args %v for test: %v", tt.args, err)
			}
			gotValidationErr := tt.testCommonFlags.validateArgs(&flagSet, flagSet.Args(), tt.mainArgType)
			gotValidationErrString := common.ErrToString(gotValidationErr)
			if tt.wantValidationErrString != gotValidationErrString {
				t.Errorf("unexpected error: wanted '%s', got '%s'", tt.wantValidationErrString, gotValidationErrString)
			}
		})
	}
}

var testBuildTagsForCTPData = []struct {
	testCommonFlags
	wantTags map[string]string
}{
	{ // Missing all values
		testCommonFlags{
			addedTags: nil,
		},
		map[string]string{
			"crosfleet-tool": "suite",
			"label-suite":    "sample-suite",
			"user_agent":     "crosfleet",
		},
	},
	{ // Includes all values
		testCommonFlags{
			addedTags: map[string]string{
				"foo": "bar",
			},
		},
		map[string]string{
			"foo":            "bar",
			"crosfleet-tool": "suite",
			"label-suite":    "sample-suite",
			"user_agent":     "crosfleet",
		},
	},
}

func TestBuildTagsForCTPBuilds(t *testing.T) {
	t.Parallel()
	for _, tt := range testBuildTagsForCTPData {
		t.Run(fmt.Sprintf("(%s)", tt.wantTags), func(t *testing.T) {
			t.Parallel()
			gotTags := tt.testCommonFlags.buildTagsForCTPBuilds("suite", "sample-suite")
			if diff := cmp.Diff(tt.wantTags, gotTags); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}

var testCommonTagsData = []struct {
	testCommonFlags
	wantTags map[string]string
}{
	{ // Missing all values
		testCommonFlags{
			addedTags: nil,
		},
		map[string]string{
			"crosfleet-tool": "suite",
			"label-suite":    "sample-suite",
		},
	},
	{ // Includes all values
		testCommonFlags{
			addedTags: map[string]string{
				"foo": "bar",
			},
		},
		map[string]string{
			"foo":            "bar",
			"crosfleet-tool": "suite",
			"label-suite":    "sample-suite",
		},
	},
}

func TestCommonTagsForAllBuilds(t *testing.T) {
	t.Parallel()
	for _, tt := range testCommonTagsData {
		t.Run(fmt.Sprintf("(%s)", tt.wantTags), func(t *testing.T) {
			t.Parallel()
			gotTags := tt.testCommonFlags.commonTagsForAllBuilds("suite", "sample-suite")
			if diff := cmp.Diff(tt.wantTags, gotTags); diff != "" {
				t.Errorf("unexpected diff (%s)", diff)
			}
		})
	}
}

var testSoftwareDependenciesData = []struct {
	testCommonFlags
	wantDeps      []*test_platform.Request_Params_SoftwareDependency
	wantErrString string
}{
	{ // Invalid label
		testCommonFlags{
			bucket:          "",
			image:           "",
			provisionLabels: map[string]string{"foo-invalid": "bar"},
		},
		nil,
		"invalid provisionable label foo-invalid",
	},
	{ // No labels, bucket, or image
		testCommonFlags{
			bucket:          "",
			image:           "",
			provisionLabels: nil,
		},
		nil,
		"",
	},
	{ // Bucket, image, Lacros path, and one label
		testCommonFlags{
			bucket:     "sample-bucket",
			image:      "sample-image",
			lacrosPath: "sample-lacros-path",
			provisionLabels: map[string]string{
				"fwrw-version": "foo-rw",
			},
		},
		[]*test_platform.Request_Params_SoftwareDependency{
			{Dep: &test_platform.Request_Params_SoftwareDependency_RwFirmwareBuild{RwFirmwareBuild: "foo-rw"}},
			{Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuildGcsBucket{ChromeosBuildGcsBucket: "sample-bucket"}},
			{Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuild{ChromeosBuild: "sample-image"}},
			{Dep: &test_platform.Request_Params_SoftwareDependency_LacrosGcsPath{LacrosGcsPath: "sample-lacros-path"}},
		},
		"",
	},
}

func stringOfLength(length int) string {
	letters := make([]rune, length)
	for i := range length {
		letters[i] = 'a'
	}
	return string(letters)
}

var testTestOrSuiteNamesLabelData = []struct {
	names     []string
	wantLabel string
}{
	{
		[]string{"foo", "bar"},
		"[foo bar]",
	},
	{
		[]string{"foo"},
		"foo",
	},
	{
		[]string{stringOfLength(301)},
		stringOfLength(256),
	},
}

func TestTestOrSuiteNamesLabel(t *testing.T) {
	t.Parallel()
	for _, tt := range testTestOrSuiteNamesLabelData {
		t.Run(fmt.Sprintf("(%s)", tt.wantLabel), func(t *testing.T) {
			t.Parallel()
			gotLabel := testOrSuiteNamesTag(tt.names)
			if tt.wantLabel != gotLabel {
				t.Errorf("unexpected error: wanted '%s', got '%s'", tt.wantLabel, gotLabel)
			}
		})
	}
}

const (
	INVALID_BOARD      = "Invalid Board"
	INVALID_MODEL      = "Invalid Model"
	INVALID_IMAGE      = "Invalid Image"
	INVALID_TEST       = "Invalid Test"
	INVALID_QS_ACCOUNT = "Invalid QSAccount"
)

var testValidatePublicChromiumOnChromeOsData = []struct {
	testCommonFlags
	wantValidationErrString string
	testNames               []string
	validTests              []string
	validModels             []string
	testCmdName             string
	status                  bool
	ufsError                string
	allowPublicUserAcct     bool
}{
	{ // Invalid Board
		testCommonFlags{
			board:    "eve",
			models:   []string{"eve", "kevin"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		INVALID_BOARD,
		[]string{"tast.lacros"},
		nil,
		nil,
		"",
		false,
		INVALID_BOARD,
		true,
	},
	{ // Invalid Model
		testCommonFlags{
			board:    "eve",
			models:   []string{"eve"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		INVALID_MODEL,
		[]string{"tast.lacros"},
		nil,
		nil,
		"",
		false,
		INVALID_MODEL,
		true,
	},
	{ // Invalid Image
		testCommonFlags{
			board:    "eve",
			models:   []string{"eve", "kevin"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		INVALID_IMAGE,
		[]string{"tast.lacros"},
		nil,
		nil,
		"",
		false,
		INVALID_IMAGE,
		true,
	},
	{ // Invalid Test
		testCommonFlags{
			board:    "eve",
			models:   []string{"eve"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		INVALID_TEST,
		[]string{"tast.lacros"},
		nil,
		nil,
		"",
		false,
		INVALID_TEST,
		true,
	},
	{ // One valid Test
		testCommonFlags{
			board:    "eve",
			models:   []string{"eve"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		INVALID_TEST,
		[]string{"tast.lacros", "tast.lacros2"},
		[]string{"tast.lacros2"},
		[]string{"eve"},
		"",
		true,
		INVALID_TEST,
		true,
	},
	{ // One valid Model
		testCommonFlags{
			board:    "eve",
			models:   []string{"eve", "kevin"},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		INVALID_MODEL,
		[]string{"tast.lacros"},
		[]string{"tast.lacros"},
		[]string{"kevin"},
		"",
		true,
		INVALID_MODEL,
		true,
	},
	{ // No Models Specified
		testCommonFlags{
			board:    "eve",
			models:   []string{},
			repeats:  7,
			pool:     "",
			image:    "sample-image",
			release:  "sample-release",
			priority: 256,
		},
		"",
		[]string{"tast.lacros"},
		[]string{"tast.lacros"},
		[]string{""},
		"",
		true,
		"",
		true,
	},
	{ // Invalid QsAccount
		testCommonFlags{
			board:     "eve",
			models:    []string{"eve", "kevin"},
			repeats:   7,
			pool:      "",
			image:     "sample-image",
			release:   "sample-release",
			priority:  256,
			qsAccount: "sample-acct",
		},
		INVALID_QS_ACCOUNT,
		[]string{"tast.lacros"},
		nil,
		nil,
		"",
		false,
		INVALID_QS_ACCOUNT,
		true,
	},
}

func TestValidatePublicChromiumTest(t *testing.T) {
	t.Parallel()
	for _, tt := range testValidatePublicChromiumOnChromeOsData {
		t.Run(fmt.Sprintf("(%s)", tt.wantValidationErrString), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tt.ufsError != "" {
				ctx = context.WithValue(ctx, "status", tt.ufsError)
			}

			results, err := tt.testCommonFlags.verifyFleetTestsPolicy(ctx, &fakeUfsClient{}, tt.testCmdName, tt.testNames, true)

			if err != nil {
				if !strings.Contains(common.ErrToString(err), tt.wantValidationErrString) {
					t.Errorf("unexpected error: wanted '%s', got '%s'", tt.wantValidationErrString, common.ErrToString(err))
				}
				return
			}
			gotValidationErrString := ""
			if results.testValidationErrors != nil && len(results.testValidationErrors) != 0 {
				gotValidationErrString = results.testValidationErrors[0]
			}
			if !strings.Contains(gotValidationErrString, tt.wantValidationErrString) {
				t.Errorf("unexpected error: wanted '%s', got '%s'", tt.wantValidationErrString, gotValidationErrString)
			}
			if results.anyValidTests != tt.status {
				t.Errorf("unexpected error: wanted valid tests : %v", tt.status)
			}
			if diff := cmp.Diff(results.validTests, tt.validTests, common.CmpOpts); diff != "" {
				t.Errorf("unexpected tests (%s)", diff)
			}
			if diff := cmp.Diff(results.validModels, tt.validModels, common.CmpOpts); diff != "" {
				t.Errorf("unexpected models (%s)", diff)
			}
		})
	}
}

// FakeGetPoolsClient mimics a UFS client and records what it was asked to look up.
type fakeUfsClient struct {
	requestCount int
}

func (f *fakeUfsClient) GetMachineLSE(ctx context.Context, req *ufsapi.GetMachineLSERequest, opts ...grpc.CallOption) (*models.MachineLSE, error) {
	return nil, nil
}

func (f *fakeUfsClient) GetMachine(ctx context.Context, req *ufsapi.GetMachineRequest, opts ...grpc.CallOption) (*models.Machine, error) {
	return nil, nil
}

// CheckFleetTestsPolicy returns a dummy response.
func (f *fakeUfsClient) CheckFleetTestsPolicy(ctx context.Context, in *ufsapi.CheckFleetTestsPolicyRequest, opts ...grpc.CallOption) (*ufsapi.CheckFleetTestsPolicyResponse, error) {
	var status ufsapi.TestStatus_Code
	response := &ufsapi.CheckFleetTestsPolicyResponse{}
	f.requestCount++
	if f.requestCount == 1 {
		response.TestStatus = &ufsapi.TestStatus{
			Code: status,
		}
		return response, nil
	}
	msg := fmt.Sprint(ctx.Value("status"))
	if ctx.Value("status") != nil && f.requestCount == 2 {
		if msg == INVALID_BOARD {
			status = ufsapi.TestStatus_NOT_A_PUBLIC_BOARD
		} else if msg == INVALID_MODEL {
			status = ufsapi.TestStatus_NOT_A_PUBLIC_MODEL
		} else if msg == INVALID_IMAGE {
			status = ufsapi.TestStatus_NOT_A_PUBLIC_IMAGE
		} else if msg == INVALID_TEST {
			status = ufsapi.TestStatus_NOT_A_PUBLIC_TEST
		} else if msg == INVALID_QS_ACCOUNT {
			status = ufsapi.TestStatus_INVALID_QS_ACCOUNT
		}
		response.TestStatus = &ufsapi.TestStatus{
			Code:    status,
			Message: msg,
		}
		return response, nil
	}
	response.TestStatus = &ufsapi.TestStatus{
		Code: ufsapi.TestStatus_OK,
	}
	return response, nil
}

var testGetCustomCTPBuilderData = []struct {
	site.Environment
	testCommonFlags
	wantCtpBuilder *buildbucketpb.BuilderID
}{
	{ // With custom bucket and builder
		(&common.EnvFlags{}).Env(),
		testCommonFlags{
			luciProject:         "testProject",
			publicBuilderBucket: "testBucket",
			publicBuilder:       "testBuilder",
		},
		&buildbucketpb.BuilderID{
			Project: "testProject",
			Bucket:  "testBucket",
			Builder: "testBuilder",
		},
	},
	{ // No custom bucket or builder
		(&common.EnvFlags{}).Env(),
		testCommonFlags{},
		&buildbucketpb.BuilderID{
			Project: "chromeos",
			Bucket:  "testplatform",
			Builder: "cros_test_platform",
		},
	},
}

// Tests the functionality for getting a custom CTPBuilder through env and flags
func TestGetCTPBuilder(t *testing.T) {
	t.Parallel()
	for _, tt := range testGetCustomCTPBuilderData {
		t.Run(fmt.Sprintf("(%s)", tt.wantCtpBuilder), func(t *testing.T) {
			t.Parallel()
			gotCtpBuilder := tt.testCommonFlags.getCTPBuilder(tt.Environment)
			if diff := cmp.Diff(tt.wantCtpBuilder, gotCtpBuilder, common.CmpOpts); diff != "" {
				t.Errorf("unexpected diff (%s); got : %s; want : %s", diff, gotCtpBuilder, tt.wantCtpBuilder)
			}
		})
	}
}

func Test_ctpRunLauncher_ctpBuilder(t *testing.T) {
	tests := []struct {
		name     string
		launcher ctpRunLauncher
		model    string
		want     *builder.CTPBuilder
	}{
		{
			"minimal",
			ctpRunLauncher{
				mainArgsTag: "sample-test",
				printer:     common.CLIPrinter{},
				cmdName:     "test",
				bbClient:    buildbucket.NewClientForTesting(&buildbucketpb.BuilderID{Project: "test"}),
				testPlan:    &test_platform.Request_TestPlan{},
				cliFlags:    &testCommonFlags{},
			},
			"model",
			&builder.CTPBuilder{
				CTPBuildTags: map[string]string{
					"crosfleet-tool": "test",
					"label-test":     "sample-test",
					"user_agent":     "crosfleet",
				},
				BuilderID: &buildbucketpb.BuilderID{Project: "test"},
				Model:     "model",
				Properties: map[string]interface{}{
					"$chromeos/service_version": map[string]interface{}{
						// Convert to protoreflect.ProtoMessage for easier type comparison.
						"version": (&test_platform.ServiceVersion{
							CrosfleetTool: 4,
						}).ProtoReflect().Interface(),
					},
				},
				TestPlan: &test_platform.Request_TestPlan{},
				TestRunnerBuildTags: map[string]string{
					"crosfleet-tool": "test",
					"label-test":     "sample-test",
				},
			},
		},
		{
			"minimalAL",
			ctpRunLauncher{
				mainArgsTag: "sample-test",
				printer:     common.CLIPrinter{},
				cmdName:     "test",
				bbClient:    buildbucket.NewClientForTesting(&buildbucketpb.BuilderID{Project: "test"}),
				testPlan: &test_platform.Request_TestPlan{
					Suite: []*test_platform.Request_Suite{{Name: "AL.test"}},
				},
				cliFlags: &testCommonFlags{},
			},
			"model",
			&builder.CTPBuilder{
				CTPBuildTags: map[string]string{
					"crosfleet-tool": "test",
					"label-test":     "sample-test",
					"user_agent":     "crosfleet",
				},
				BuilderID: &buildbucketpb.BuilderID{Project: "test"},
				Model:     "model",
				Properties: map[string]interface{}{
					"$chromeos/service_version": map[string]interface{}{
						// Convert to protoreflect.ProtoMessage for easier type comparison.
						"version": (&test_platform.ServiceVersion{
							CrosfleetTool: 4,
						}).ProtoReflect().Interface(),
					},
				},
				TestPlan: &test_platform.Request_TestPlan{
					Suite: []*test_platform.Request_Suite{{Name: "AL.test"}},
				},
				TestRunnerBuildTags: map[string]string{
					"crosfleet-tool": "test",
					"label-test":     "sample-test",
				},
				UserDefinedFilters: []*api.CTPFilter{
					{ContainerInfo: &api.ContainerInfo{Container: &buildapi.ContainerImageInfo{Name: "al-provision-filter"}}},
					{ContainerInfo: &api.ContainerInfo{Container: &buildapi.ContainerImageInfo{Name: "foil-filter"}}},
					{ContainerInfo: &api.ContainerInfo{Container: &buildapi.ContainerImageInfo{Name: "test-finder"}}},
					{ContainerInfo: &api.ContainerInfo{Container: &buildapi.ContainerImageInfo{Name: "ants-publish-filter"}}},
				},
			},
		},
		{
			"maximal",
			ctpRunLauncher{
				mainArgsTag: "sample-test",
				printer:     common.CLIPrinter{},
				cmdName:     "test",
				bbClient:    buildbucket.NewClientForTesting(&buildbucketpb.BuilderID{Project: "test"}),
				testPlan:    &test_platform.Request_TestPlan{},
				cliFlags: &testCommonFlags{
					board:                "test-board",
					secondaryBoards:      []string{"secondary-board"},
					models:               []string{"model"},
					secondaryModels:      []string{"secondary-model"},
					pool:                 "pool",
					bucket:               "image-bucket",
					image:                "image",
					secondaryImages:      []string{"secondary-image"},
					release:              "release",
					qsAccount:            "qs-account",
					maxRetries:           5,
					repeats:              4,
					priority:             100,
					timeoutMins:          600,
					addedDims:            map[string]string{"dim": "yes"},
					provisionLabels:      map[string]string{"provision": "this"},
					addedTags:            map[string]string{"foo": "bar"},
					keyvals:              map[string]string{"key": "bar"},
					exitEarly:            true,
					lacrosPath:           "lacros",
					secondaryLacrosPaths: []string{"secondary-lacros"},
					cft:                  true,
					testHarness:          "tauto",
					publicBuilderBucket:  "bucket",
					publicBuilder:        "builder",
					luciProject:          "project",
				},
			},
			"model",
			&builder.CTPBuilder{
				BBClient:  nil,
				Board:     "test-board",
				BuilderID: &buildbucketpb.BuilderID{Project: "test"},
				CFT:       true,
				CTPBuildTags: map[string]string{
					"crosfleet-tool": "test",
					"label-test":     "sample-test",
					"user_agent":     "crosfleet",
					"foo":            "bar",
				},
				Dimensions:  map[string]string{"dim": "yes"},
				Image:       "image",
				ImageBucket: "image-bucket",
				Keyvals:     map[string]string{"key": "bar"},
				MaxRetries:  5,
				Model:       "model",
				Pool:        "pool",
				Priority:    100,
				Properties: map[string]interface{}{
					"$chromeos/service_version": map[string]interface{}{
						// Convert to protoreflect.ProtoMessage for easier type comparison.
						"version": (&test_platform.ServiceVersion{
							CrosfleetTool: 4,
						}).ProtoReflect().Interface(),
					},
				},
				ProvisionLabels: map[string]string{"provision": "this"},
				QSAccount:       "qs-account",
				SecondaryBoards: []string{"secondary-board"},
				SecondaryImages: []string{"secondary-image"},
				SecondaryModels: []string{"secondary-model"},
				TestPlan:        &test_platform.Request_TestPlan{},
				TestRunnerBuildTags: map[string]string{
					"crosfleet-tool": "test",
					"label-test":     "sample-test",
					"foo":            "bar",
				},
				TimeoutMins: 600,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.launcher.ctpBuilder(tt.model)
			if diff := cmp.Diff(got, tt.want, common.CmpOpts); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
