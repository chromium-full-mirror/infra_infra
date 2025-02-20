// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Some expected error strings are filesystem-specific, skip on windows.
//go:build !windows

package testplan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	bapi "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/plan"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/proto/structmask"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/gs"
	"go.chromium.org/infra/tools/dirmd"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
	"go.chromium.org/infra/tools/dirmd/proto/chromeos"
)

func newStructOrFatal(t *testing.T, v map[string]interface{}) *structpb.Struct {
	s, err := structpb.NewStruct(v)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestValidateMapping(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testStarlarkContent := "testcontent"
	templatedStarlarkContent := `testplan.get_suite_name()`

	gerritClient := &gerrit.MockClient{
		T: t,
		ExpectedDownloads: map[gerrit.ExpectedPathParams]*string{
			{
				Host:    "chromium.googlesource.com",
				Project: "test/repo",
				Ref:     "HEAD",
				Path:    "a/b/c/test.star",
			}: &testStarlarkContent,
			{
				Host:    "chromium.googlesource.com",
				Project: "test/repo1",
				Ref:     "HEAD",
				Path:    "a/b/c/test.star",
			}: &testStarlarkContent,
			{
				Host:    "chromium.googlesource.com",
				Project: "test/repo2",
				Ref:     "HEAD",
				Path:    "test2.star",
			}: &testStarlarkContent,
			{
				Host:    "chromium.googlesource.com",
				Project: "test/repo",
				Ref:     "HEAD",
				Path:    "templated.star",
			}: &templatedStarlarkContent,
		},
	}

	bbClient := bbpb.NewMockBuildsClient(ctrl)

	foundBuild := &bbpb.Build{
		Id: 123,
		Input: &bbpb.Build_Input{
			Properties: newStructOrFatal(
				t, map[string]interface{}{
					"$chromeos/build_menu": map[string]interface{}{
						"build_target": map[string]interface{}{
							"name": "targetA",
						},
					},
				},
			),
		},
		Output: &bbpb.Build_Output{
			Properties: newStructOrFatal(
				t, map[string]interface{}{
					"artifact_link": "gs://testbucket/artifacts",
				},
			),
		},
	}

	// Not every test case will call SearchBuilds, because it is only called
	// when there are TemplateParameters to check, but expect at least one call.
	bbClient.EXPECT().
		SearchBuilds(gomock.AssignableToTypeOf(ctx), &bbpb.SearchBuildsRequest{
			Predicate: &bbpb.BuildPredicate{
				Builder: &bbpb.BuilderID{
					Project: "chromeos",
					Bucket:  "postsubmit",
					Builder: "dedede-snapshot",
				},
				Status: bbpb.Status_SUCCESS,
				Tags:   []*bbpb.StringPair{{Key: "relevance", Value: "relevant"}},
			},
			Mask: &bbpb.BuildMask{
				InputProperties: []*structmask.StructMask{
					{
						Path: []string{"$chromeos/build_menu", "build_target", "name"},
					},
				},
				OutputProperties: []*structmask.StructMask{
					{
						Path: []string{"artifact_link"},
					},
				},
			},
			PageSize: 1,
		}).
		Return(&bbpb.SearchBuildsResponse{
			Builds: []*bbpb.Build{foundBuild},
		}, nil).
		MinTimes(1)

	// If the validator is calling cros-test-finder, it uses a temp dir on the
	// host to write the request and read the response, and binds this temp dir
	// to a location on the container. Note that the temp dir created here is
	// in the expected command, and is also used in the tmpdirFn set on the
	// validator.
	tmpDir := t.TempDir()

	cmdRunner := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{"gcloud", "auth", "configure-docker", "us-docker.pkg.dev", "--quiet"},
			},
			{
				ExpectedCmd: []string{
					"docker", "run",
					fmt.Sprintf("--mount=source=%s,target=/tmp/test/cros-test-finder,type=bind", tmpDir),
					"us-docker.pkg.dev/cros-registry/test-services/cros-test-finder@sha256:abc",
					"cros-test-finder",
				},
			},
		},
	}

	containerMetadata := bapi.ContainerMetadata{
		Containers: map[string]*bapi.ContainerImageMap{
			"targetA": {
				Images: map[string]*bapi.ContainerImageInfo{
					"cros-test-finder": {
						Repository: &bapi.GcrRepository{
							Hostname: "us-docker.pkg.dev",
							Project:  "cros-registry/test-services",
						},
						Name:   "cros-test-finder",
						Digest: "sha256:abc",
					},
				},
			},
		},
	}
	containerMetadataJSON, err := protojson.Marshal(&containerMetadata)
	if err != nil {
		t.Fatal(err)
	}

	gsClient := &gs.FakeClient{
		T: t,
		ExpectedReads: map[string][]byte{
			"gs://testbucket/artifacts/metadata/containers.jsonpb": containerMetadataJSON,
		},
	}

	validator := NewValidator(gerritClient, bbClient, gsClient, cmdRunner).SetCheckTagCriteriaNonEmptyEnabled(true)

	// Override the validator's default tmpdir fn, to return a dir with a
	// request already written in it; this simulates what cros-test-finder would
	// do if it actually ran.
	validator.WithTmpdirFn(func(_dir, _pattern string) (string, error) {
		ctfResp := &api.CrosTestFinderResponse{
			TestSuites: []*api.TestSuite{
				{
					Name: "suiteA",
					Spec: &api.TestSuite_TestCases{
						TestCases: &api.TestCaseList{
							TestCases: []*api.TestCase{
								{
									Name: "test1",
								},
							},
						},
					},
				},
			},
		}
		ctfRespJson, err := protojson.Marshal(ctfResp)
		if err != nil {
			return "", err
		}

		if err := os.WriteFile(filepath.Join(tmpDir, "result.json"), ctfRespJson, os.ModePerm); err != nil {
			return "", err
		}

		return tmpDir, nil
	})

	tests := []struct {
		name     string
		mapping  *dirmd.Mapping
		repoRoot string
	}{
		{
			name: "no ChromeOS metadata",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "single starlark file",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "a/b/c/test.star",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "multiple starlark files",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo1",
												Path:    "a/b/c/test.star",
											},
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo2",
												Path:    "test2.star",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "valid regexps",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "a/b/c/test.star",
											},
										},
										PathRegexps:        []string{"a/b/c/d/.*"},
										PathRegexpExcludes: []string{`a/b/c/.*\.md`},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "root directory",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "a/b/c/test.star",
											},
										},
										PathRegexps:        []string{"a/b/c/d/.*"},
										PathRegexpExcludes: []string{`a/b/c/.*\.md`},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "regexp doesn't match file",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "a/b/c/test.star",
											},
										},
										// This doesn't match anything under
										// ./testdata/good_dirmd. This isn't an
										// error, but a warning is logged.
										PathRegexps: []string{"a/b/c/d/nomatch.*"},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "TagCriteria template parameters",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													SuiteName: "mysuiteA",
													TagCriteria: &api.TestSuite_TestCaseTagCriteria{
														Tags:        []string{"group:mygroupA"},
														TagExcludes: []string{"informational"},
													},
												},
											},
										},
									},
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													SuiteName: "tast_gce_suite",
													TagCriteria: &api.TestSuite_TestCaseTagCriteria{
														Tags:        []string{"group:mygroupA"},
														TagExcludes: []string{"informational"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "program TemplateParameters",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"overlay-boardA": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													Program: "boardA",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/good_dirmd",
		},
		{
			name: "program TemplateParameters in private overlay",
			mapping: &dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "test/repo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													Program: "boardA",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			repoRoot: "./testdata/private-overlay-boardA",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NilError(t, validator.ValidateMapping(ctx, test.mapping, test.repoRoot))
		})
	}
}

func TestValidateMappingNoCheckTagCriteriaNonEmpty(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	templatedStarlarkContent := `testplan.get_suite_name()`

	gerritClient := &gerrit.MockClient{
		T: t,
		ExpectedDownloads: map[gerrit.ExpectedPathParams]*string{
			{
				Host:    "chromium.googlesource.com",
				Project: "test/repo",
				Ref:     "HEAD",
				Path:    "templated.star",
			}: &templatedStarlarkContent,
		},
	}

	gsClient := &gs.FakeClient{
		T: t,
	}

	mapping := &dirmd.Mapping{
		Dirs: map[string]*dirmdpb.Metadata{
			"a/b": {
				Chromeos: &chromeos.ChromeOS{
					Cq: &chromeos.ChromeOS_CQ{
						SourceTestPlans: []*plan.SourceTestPlan{
							{
								TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
									{
										Host:    "chromium.googlesource.com",
										Project: "test/repo",
										Path:    "templated.star",
										TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
											SuiteName: "mysuiteA",
											TagCriteria: &api.TestSuite_TestCaseTagCriteria{
												Tags:        []string{"group:mygroupA"},
												TagExcludes: []string{"informational"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	validator := NewValidator(gerritClient, bbpb.NewMockBuildsClient(ctrl), gsClient, cmd.FakeCommandRunner{})

	assert.NilError(t, validator.ValidateMapping(ctx, mapping, "./testdata/good_dirmd"))
}

func TestValidateMappingErrors(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	textfileContents := "testtext"
	testfileContents := "print('hello')"
	templatedStarlarkContent := `testplan.get_suite_name()`

	gerritClient := &gerrit.MockClient{
		T: t,
		ExpectedDownloads: map[gerrit.ExpectedPathParams]*string{
			{
				Host:    "chromium.googlesource.com",
				Project: "testrepo",
				Ref:     "HEAD",
				Path:    "missingtestfile.star",
			}: nil,
			{
				Host:    "chromium.googlesource.com",
				Project: "testrepo",
				Ref:     "HEAD",
				Path:    "testfile.txt",
			}: &textfileContents,
			{
				Host:    "chromium.googlesource.com",
				Project: "testrepo",
				Ref:     "HEAD",
				Path:    "testfile.star",
			}: &testfileContents,
			{
				Host:    "chromium.googlesource.com",
				Project: "testrepo",
				Ref:     "HEAD",
				Path:    "templated.star",
			}: &templatedStarlarkContent,
		},
	}

	bbClient := bbpb.NewMockBuildsClient(ctrl)

	foundBuild := &bbpb.Build{
		Id: 123,
		Input: &bbpb.Build_Input{
			Properties: newStructOrFatal(
				t, map[string]interface{}{
					"$chromeos/build_menu": map[string]interface{}{
						"build_target": map[string]interface{}{
							"name": "targetA",
						},
					},
				},
			),
		},
		Output: &bbpb.Build_Output{
			Properties: newStructOrFatal(
				t, map[string]interface{}{
					"artifact_link": "gs://testbucket/artifacts",
				},
			),
		},
	}

	bbClient.EXPECT().
		SearchBuilds(gomock.AssignableToTypeOf(ctx), &bbpb.SearchBuildsRequest{
			Predicate: &bbpb.BuildPredicate{
				Builder: &bbpb.BuilderID{
					Project: "chromeos",
					Bucket:  "postsubmit",
					Builder: "dedede-snapshot",
				},
				Status: bbpb.Status_SUCCESS,
				Tags:   []*bbpb.StringPair{{Key: "relevance", Value: "relevant"}},
			},
			Mask: &bbpb.BuildMask{
				InputProperties: []*structmask.StructMask{
					{
						Path: []string{"$chromeos/build_menu", "build_target", "name"},
					},
				},
				OutputProperties: []*structmask.StructMask{
					{
						Path: []string{"artifact_link"},
					},
				},
			},
			PageSize: 1,
		}).
		Return(&bbpb.SearchBuildsResponse{
			Builds: []*bbpb.Build{foundBuild},
		}, nil).
		MinTimes(1)

	// If the validator is calling cros-test-finder, it uses a temp dir on the
	// host to write the request and read the response, and binds this temp dir
	// to a location on the container. Note that the temp dir created here is
	// in the expected command, and is also used in the tmpdirFn set on the
	// validator.
	tmpDir := t.TempDir()

	cmdRunner := &cmd.FakeCommandRunnerMulti{
		CommandRunners: []cmd.FakeCommandRunner{
			{
				ExpectedCmd: []string{"gcloud", "auth", "configure-docker", "us-docker.pkg.dev", "--quiet"},
			},
			{
				ExpectedCmd: []string{
					"docker", "run",
					fmt.Sprintf("--mount=source=%s,target=/tmp/test/cros-test-finder,type=bind", tmpDir),
					"us-docker.pkg.dev/cros-registry/test-services/cros-test-finder@sha256:abc",
					"cros-test-finder",
				},
			},
		},
	}

	containerMetadata := bapi.ContainerMetadata{
		Containers: map[string]*bapi.ContainerImageMap{
			"targetA": {
				Images: map[string]*bapi.ContainerImageInfo{
					"cros-test-finder": {
						Repository: &bapi.GcrRepository{
							Hostname: "us-docker.pkg.dev",
							Project:  "cros-registry/test-services",
						},
						Name:   "cros-test-finder",
						Digest: "sha256:abc",
					},
				},
			},
		},
	}
	containerMetadataJSON, err := protojson.Marshal(&containerMetadata)
	if err != nil {
		t.Fatal(err)
	}

	gsClient := &gs.FakeClient{
		T: t,
		ExpectedReads: map[string][]byte{
			"gs://testbucket/artifacts/metadata/containers.jsonpb": containerMetadataJSON,
		},
	}

	validator := NewValidator(gerritClient, bbClient, gsClient, cmdRunner).SetCheckTagCriteriaNonEmptyEnabled(true)

	// Override the validator's default tmpdir fn, to return a dir with a
	// request with no found test cases already written in it; this simulates
	// what cros-test-finder would do if it actually ran and didn't find test
	// cases
	validator.WithTmpdirFn(func(_dir, _pattern string) (string, error) {
		ctfResp := &api.CrosTestFinderResponse{
			TestSuites: []*api.TestSuite{
				{
					Name: "suiteA",
					Spec: &api.TestSuite_TestCases{
						TestCases: &api.TestCaseList{
							TestCases: []*api.TestCase{},
						},
					},
				},
			},
		}
		ctfRespJson, err := protojson.Marshal(ctfResp)
		if err != nil {
			return "", err
		}

		if err := os.WriteFile(filepath.Join(tmpDir, "result.json"), ctfRespJson, os.ModePerm); err != nil {
			return "", err
		}

		return tmpDir, nil
	})

	tests := []struct {
		name           string
		mapping        *dirmd.Mapping
		repoRoot       string
		errorSubstring string
	}{
		{
			"starlark files empty",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										PathRegexps: []string{"a/b/c/.*"},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"at least one TestPlanStarlarkFile must be specified",
		},
		{
			"invalid regexp",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "testfile.star",
											},
										},
										PathRegexps: []string{"a/b/c/d/["},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"failed to compile path regexp",
		},
		{
			"invalid regexp prefix",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b/c": {
						TeamEmail: "exampleteam@google.com",
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "testfile.star",
											},
										},
										PathRegexps: []string{`a/b/e/.*\.txt`},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"path_regexp(_exclude)s defined in a directory that is not the root of the repo must have the sub-directory as a prefix",
		},
		{
			"invalid file type",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {Chromeos: &chromeos.ChromeOS{
						Cq: &chromeos.ChromeOS_CQ{
							SourceTestPlans: []*plan.SourceTestPlan{
								{
									TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
										{
											Host:    "chromium.googlesource.com",
											Project: "testrepo",
											Path:    "testfile.txt",
										},
									},
								},
							},
						},
					},
					},
				},
			},
			"./testdata/good_dirmd",
			"all TestPlanStarlarkFile must specify \".star\" files, got \"testfile.txt\"",
		},
		{
			"starlark file missing",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "missingtestfile.star",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"failed downloading file",
		},
		{
			"non-existant repo root",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "testfile.star",
											},
										},
										PathRegexps: []string{`a/b/e/.*\.txt`},
									},
								},
							},
						},
					},
				},
			},
			"badreporoot",
			"lstat badreporoot: no such file or directory",
		},
		{
			"TemplateParameters empty",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:               "chromium.googlesource.com",
												Project:            "testrepo",
												Path:               "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"either tag_criteria or program must be set on TemplateParameters",
		},
		{
			"suite name missing",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													TagCriteria: &api.TestSuite_TestCaseTagCriteria{
														Tags:        []string{"group:mygroupA"},
														TagExcludes: []string{"informational"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"suite_name must not be empty",
		},
		{
			"informational not excluded",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													SuiteName: "mysuiteA",
													TagCriteria: &api.TestSuite_TestCaseTagCriteria{
														Tags: []string{"group:mygroupA"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			`tag_excludes must exclude "informational"`,
		},
		{
			"not templated file",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "testfile.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													SuiteName: "mysuiteA",
													TagCriteria: &api.TestSuite_TestCaseTagCriteria{
														Tags:        []string{"group:mygroupA"},
														TagExcludes: []string{"informational"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"setting TemplateParameters has no effect",
		},
		{
			"test criteria empty",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													SuiteName: "mysuiteA",
													TagCriteria: &api.TestSuite_TestCaseTagCriteria{
														Tags:        []string{"group:mygroupA"},
														TagExcludes: []string{"informational"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			"no test cases found for test suite",
		},
		{
			"program TemplateParameter outside of overlay",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a/b": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													Program: "boardA",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			`program TemplateParameter is only allowed in overlay directories. Got: "testdata/good_dirmd/a/b"`,
		},
		{
			"program TemplateParameter wrong overlay",
			&dirmd.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"overlay-boardA": {
						Chromeos: &chromeos.ChromeOS{
							Cq: &chromeos.ChromeOS_CQ{
								SourceTestPlans: []*plan.SourceTestPlan{
									{
										TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
											{
												Host:    "chromium.googlesource.com",
												Project: "testrepo",
												Path:    "templated.star",
												TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
													Program: "boardB",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"./testdata/good_dirmd",
			`program TemplateParameter must match the overlay it is in. Got parameter "boardB", expected "boardA"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validator.ValidateMapping(ctx, test.mapping, test.repoRoot)
			assert.ErrorContains(t, err, test.errorSubstring)
		})
	}
}

func TestValidateMappingCTFErrors(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	templatedStarlarkContent := `testplan.get_suite_name()`

	gerritClient := &gerrit.MockClient{
		T: t,
		ExpectedDownloads: map[gerrit.ExpectedPathParams]*string{
			{
				Host:    "chromium.googlesource.com",
				Project: "test/repo",
				Ref:     "HEAD",
				Path:    "templated.star",
			}: &templatedStarlarkContent,
		},
	}

	mapping := &dirmd.Mapping{
		Dirs: map[string]*dirmdpb.Metadata{
			"a/b": {
				Chromeos: &chromeos.ChromeOS{
					Cq: &chromeos.ChromeOS_CQ{
						SourceTestPlans: []*plan.SourceTestPlan{
							{
								TestPlanStarlarkFiles: []*plan.SourceTestPlan_TestPlanStarlarkFile{
									{
										Host:    "chromium.googlesource.com",
										Project: "test/repo",
										Path:    "templated.star",
										TemplateParameters: &plan.SourceTestPlan_TestPlanStarlarkFile_TemplateParameters{
											SuiteName: "mysuiteA",
											TagCriteria: &api.TestSuite_TestCaseTagCriteria{
												Tags:        []string{"group:mygroupA"},
												TagExcludes: []string{"informational"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name              string
		inputProps        *structpb.Struct
		outputProps       *structpb.Struct
		containerMetadata *bapi.ContainerMetadata
		errorSubstring    string
	}{
		{
			name: "artifact link missing",
			inputProps: newStructOrFatal(
				t, map[string]interface{}{
					"$chromeos/build_menu": map[string]interface{}{
						"build_target": map[string]interface{}{
							"name": "targetA",
						},
					},
				},
			),
			outputProps: newStructOrFatal(
				t, map[string]interface{}{"otheroutputprop": 1},
			),
			errorSubstring: "artifact_link output property not found on build 123",
		},
		{
			name: "build target missing",
			inputProps: newStructOrFatal(
				t, map[string]interface{}{
					"$chromeos/build_menu": map[string]interface{}{
						"otherprop": 1,
					},
				},
			),
			outputProps: newStructOrFatal(
				t, map[string]interface{}{"artifact_link": "gs://bucket/artifact"},
			),
			errorSubstring: "$chromeos/build_menu.build_target.name input property not found on build 123",
		},
		{
			name: "cros test finder missing from metadata",
			inputProps: newStructOrFatal(
				t, map[string]interface{}{
					"$chromeos/build_menu": map[string]interface{}{
						"build_target": map[string]interface{}{
							"name": "targetA",
						},
					},
				},
			),
			outputProps: newStructOrFatal(
				t, map[string]interface{}{"artifact_link": "gs://testbucket/artifacts"},
			),
			containerMetadata: &bapi.ContainerMetadata{
				Containers: map[string]*bapi.ContainerImageMap{
					"targetA": {
						Images: map[string]*bapi.ContainerImageInfo{
							"other-container": {
								Repository: &bapi.GcrRepository{
									Hostname: "us-docker.pkg.dev",
									Project:  "cros-registry/test-services",
								},
								Name:   "other-container",
								Digest: "sha256:abc",
							},
						},
					},
				},
			},
			errorSubstring: "cros-test-finder container not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bbClient := bbpb.NewMockBuildsClient(ctrl)
			bbClient.EXPECT().
				SearchBuilds(gomock.AssignableToTypeOf(ctx), &bbpb.SearchBuildsRequest{
					Predicate: &bbpb.BuildPredicate{
						Builder: &bbpb.BuilderID{
							Project: "chromeos",
							Bucket:  "postsubmit",
							Builder: "dedede-snapshot",
						},
						Status: bbpb.Status_SUCCESS,
						Tags:   []*bbpb.StringPair{{Key: "relevance", Value: "relevant"}},
					},
					Mask: &bbpb.BuildMask{
						InputProperties: []*structmask.StructMask{
							{
								Path: []string{"$chromeos/build_menu", "build_target", "name"},
							},
						},
						OutputProperties: []*structmask.StructMask{
							{
								Path: []string{"artifact_link"},
							},
						},
					},
					PageSize: 1,
				}).Return(&bbpb.SearchBuildsResponse{
				Builds: []*bbpb.Build{{
					Id: 123,
					Input: &bbpb.Build_Input{
						Properties: test.inputProps,
					},
					Output: &bbpb.Build_Output{
						Properties: test.outputProps,
					},
				},
				}}, nil).MinTimes(1)

			containerMetadataJSON, err := protojson.Marshal(test.containerMetadata)
			if err != nil {
				t.Fatal(err)
			}

			gsClient := &gs.FakeClient{
				T: t,
				ExpectedReads: map[string][]byte{
					"gs://testbucket/artifacts/metadata/containers.jsonpb": containerMetadataJSON,
				},
			}

			validator := NewValidator(
				gerritClient,
				bbClient,
				gsClient,
				cmd.FakeCommandRunner{},
			).SetCheckTagCriteriaNonEmptyEnabled(true)
			err = validator.ValidateMapping(ctx, mapping, "./testdata/good_dirmd")
			assert.ErrorContains(t, err, test.errorSubstring)
		})
	}
}
