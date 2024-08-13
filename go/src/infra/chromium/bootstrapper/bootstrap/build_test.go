// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"context"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	. "go.chromium.org/luci/common/testing/assertions"

	fakegerrit "infra/chromium/bootstrapper/clients/fakes/gerrit"
	fakegitiles "infra/chromium/bootstrapper/clients/fakes/gitiles"
	"infra/chromium/bootstrapper/clients/gclient"
	"infra/chromium/bootstrapper/clients/gerrit"
	"infra/chromium/bootstrapper/clients/gitiles"
	"infra/chromium/bootstrapper/clients/gob"
	"infra/chromium/util"
)

func TestGetBootstrapConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = gob.UseTestClock(ctx)

	Convey("BuildBootstrapper.GetBootstrapConfig", t, func() {

		build := &buildbucketpb.Build{
			Input: &buildbucketpb.Build_Input{
				Properties: &structpb.Struct{},
			},
		}

		topLevelGitiles := &fakegitiles.Project{
			Refs:      map[string]string{},
			Revisions: map[string]*fakegitiles.Revision{},
		}
		dependencyGitiles := &fakegitiles.Project{
			Refs:      map[string]string{},
			Revisions: map[string]*fakegitiles.Revision{},
		}
		ctx = gitiles.UseGitilesClientFactory(ctx, fakegitiles.Factory(map[string]*fakegitiles.Host{
			"chromium.googlesource.com": {
				Projects: map[string]*fakegitiles.Project{
					"top/level":  topLevelGitiles,
					"dependency": dependencyGitiles,
				},
			},
		}))

		topLevelGerrit := &fakegerrit.Project{
			Changes: map[int64]*fakegerrit.Change{},
		}
		dependencyGerrit := &fakegerrit.Project{
			Changes: map[int64]*fakegerrit.Change{},
		}
		ctx = gerrit.UseGerritClientFactory(ctx, fakegerrit.Factory(map[string]*fakegerrit.Host{
			"chromium-review.googlesource.com": {
				Projects: map[string]*fakegerrit.Project{
					"top/level":  topLevelGerrit,
					"dependency": dependencyGerrit,
				},
			},
		}))

		setBootstrapPropertiesProperties(build, `{
			"top_level_project": {
				"repo": {
					"host": "chromium.googlesource.com",
					"project": "top/level"
				},
				"ref": "refs/heads/top-level"
			},
			"properties_file": "infra/config/fake-bucket/fake-builder/properties.json"
		}`)
		setBootstrapExeProperties(build, `{
			"exe": {
				"cipd_package": "fake-package",
				"cipd_version": "fake-version",
				"cmd": ["fake-exe"]
			}
		}`)

		gclientClient, err := gclient.NewClientForTesting()
		util.PanicOnError(err)

		bootstrapper := NewBuildBootstrapper(gitiles.NewClient(ctx), gerrit.NewClient(ctx), func(ctx context.Context) (*gclient.Client, error) {
			return gclientClient, nil
		})

		Convey("fails", func() {

			Convey("if unable to get revision", func() {
				input := getInput(build)
				topLevelGitiles.Refs["refs/heads/top-level"] = ""

				properties, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldNotBeNil)
				So(properties, ShouldBeNil)
			})

			Convey("if unable to get file", func() {
				input := getInput(build)

				properties, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldNotBeNil)
				So(properties, ShouldBeNil)
			})

			Convey("if unable to get change info", func() {
				build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
					Host:     "chromium-review.googlesource.com",
					Project:  "top/level",
					Change:   2345,
					Patchset: 1,
				})
				topLevelGerrit.Changes[2345] = nil
				input := getInput(build)

				properties, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldErrLike, "failed to get change info for config change")
				So(properties, ShouldBeNil)
			})

			Convey("if the properties file is invalid", func() {
				input := getInput(build)
				topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
				topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(""),
					},
				}

				properties, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldNotBeNil)
				So(properties, ShouldBeNil)
			})

			Convey("if unable to get diff for properties file", func() {
				build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
					Host:     "chromium-review.googlesource.com",
					Project:  "top/level",
					Change:   2345,
					Patchset: 1,
				})
				topLevelGerrit.Changes[2345] = &fakegerrit.Change{
					Ref: "top-level-some-branch-head",
					Patchsets: map[int32]*fakegerrit.Patchset{
						1: {
							Revision: "cl-revision",
						},
					},
				}
				topLevelGitiles.Refs["top-level-some-branch-head"] = "top-level-some-branch-head-revision"
				topLevelGitiles.Revisions["top-level-some-branch-head-revision"] = &fakegitiles.Revision{
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File("{}"),
					},
				}
				topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
					Parent: "non-existent-base",
				}
				topLevelGitiles.Revisions["non-existent-base"] = nil
				input := getInput(build)

				properties, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldErrLike, "failed to get diff")
				So(properties, ShouldBeNil)
			})

			Convey("if patch for properties file does not apply", func() {
				build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
					Host:     "chromium-review.googlesource.com",
					Project:  "top/level",
					Change:   2345,
					Patchset: 1,
				})
				topLevelGerrit.Changes[2345] = &fakegerrit.Change{
					Ref: "top-level-some-branch-head",
					Patchsets: map[int32]*fakegerrit.Patchset{
						1: {
							Revision: "cl-revision",
						},
					},
				}
				topLevelGitiles.Refs["top-level-some-branch-head"] = "top-level-some-branch-head-revision"
				topLevelGitiles.Revisions["top-level-some-branch-head-revision"] = &fakegitiles.Revision{
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
							"test_property": "foo"
						}`),
					},
				}
				topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
					Parent: "cl-base",
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
							"test_property": "bar"
						}`),
					},
				}
				topLevelGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File("{}"),
					},
				}
				input := getInput(build)

				properties, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldNotBeNil)
				So(PatchRejected.In(err), ShouldBeTrue)
				So(properties, ShouldBeNil)
			})

		})

		Convey("returns config", func() {

			Convey("with buildProperties from input", func() {
				topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
				topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{}`),
					},
				}
				setBootstrapPropertiesProperties(build, `{
					"top_level_project": {
						"repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"ref": "refs/heads/top-level"
					},
					"properties_file": "infra/config/fake-bucket/fake-builder/properties.json"
				}`)
				build.Input.Properties.Fields["test_property"] = structpb.NewStringValue("foo")
				build.Infra = &buildbucketpb.BuildInfra{
					Buildbucket: &buildbucketpb.BuildInfra_Buildbucket{
						RequestedProperties: jsonToStruct(`{
							"test_property": "foo"
						}`),
					},
				}
				input := getInput(build)

				config, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldBeNil)
				So(config.buildProperties, ShouldResembleProtoJSON, `{
					"test_property": "foo"
				}`)
				So(config.buildRequestedProperties, ShouldResembleProtoJSON, `{
					"test_property": "foo"
				}`)
				So(config.preferBuildProperties, ShouldBeFalse)
			})

			Convey("for polymorphic bootstrapping", func() {
				topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
				topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
					Files: map[string]*fakegitiles.PathObject{
						"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{}`),
					},
				}
				setBootstrapPropertiesProperties(build, `{
					"top_level_project": {
						"repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"ref": "refs/heads/top-level"
					},
					"properties_file": "infra/config/fake-bucket/fake-builder/properties.json"
				}`)
				inputOpts := InputOptions{Polymorphic: true}
				input, err := inputOpts.NewInput(build)
				util.PanicOnError(err)

				config, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldBeNil)
				So(config.preferBuildProperties, ShouldBeTrue)
			})

			Convey("for properties-optional bootstrapping", func() {
				inputOpts := InputOptions{PropertiesOptional: true}
				delete(build.Input.Properties.Fields, "$bootstrap/properties")
				input, err := inputOpts.NewInput(build)
				util.PanicOnError(err)

				config, err := bootstrapper.GetBootstrapConfig(ctx, input)

				So(err, ShouldBeNil)
				So(config.configCommit, ShouldBeNil)
				So(config.change, ShouldBeNil)
				So(config.builderProperties, ShouldBeNil)
			})

			Convey("for top-level project", func() {

				setBootstrapPropertiesProperties(build, `{
					"top_level_project": {
						"repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"ref": "refs/heads/top-level"
					},
					"properties_file": "infra/config/fake-bucket/fake-builder/properties.json"
				}`)

				Convey("returns config with properties from top level ref when no commit or change for project", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Parent: "config-changed-revision",
					}
					topLevelGitiles.Revisions["config-changed-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "config-changed-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/top-level",
						"id": "top-level-top-level-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "config-changed-value"
					}`)
					So(config.configSource, ShouldResembleProtoJSON, `{
						"last_changed_commit": {
							"host": "chromium.googlesource.com",
							"project": "top/level",
							"ref": "refs/heads/top-level",
							"id": "config-changed-revision"
						},
						"path": "infra/config/fake-bucket/fake-builder/properties.json"
					}`)
				})

				Convey("returns config with properties from commit ref when commit for project without ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "top/level",
						Ref:     "refs/heads/some-branch",
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-head-value"
					}`)
				})

				Convey("returns config with properties from commit revision when commit for project with ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "top/level",
						Ref:     "refs/heads/some-branch",
						Id:      "some-branch-revision",
					}
					topLevelGitiles.Revisions["some-branch-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "some-branch-revision"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-revision-value"
					}`)
				})

				Convey("returns config with properties from target ref and patch applied when change for project", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "top/level",
						Change:   2345,
						Patchset: 1,
					})
					topLevelGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-value5"
							}`),
						},
					}
					topLevelGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-old-value5"
							}`),
						},
					}
					topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-new-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-old-value5"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change.GerritChange, ShouldResembleProtoJSON, `{
						"host": "chromium-review.googlesource.com",
						"project": "top/level",
						"change": 2345,
						"patchset": 1
					}`)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-head-new-value",
						"test_property2": "some-branch-head-value2",
						"test_property3": "some-branch-head-value3",
						"test_property4": "some-branch-head-value4",
						"test_property5": "some-branch-head-value5"
					}`)
					So(config.skipAnalysisReasons, ShouldResemble, []string{
						"properties file infra/config/fake-bucket/fake-builder/properties.json is affected by CL",
					})
				})

			})

			Convey("for dependency project with config repo path", func() {

				setBootstrapPropertiesProperties(build, `{
					"dependency_project": {
						"top_level_repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"top_level_ref": "refs/heads/top-level",
						"config_repo": {
							"host": "chromium.googlesource.com",
							"project": "dependency"
						},
						"config_repo_path": "config/repo/path"
					},
					"properties_file": "infra/config/fake-bucket/fake-builder/properties.json"
				}`)

				Convey("returns config with properties from ref pinned by top level ref when no commit or change for either project", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@refs/heads/dependency',
							}`),
						},
					}
					dependencyGitiles.Refs["refs/heads/dependency"] = "dependency-dependency-head"
					dependencyGitiles.Revisions["dependency-dependency-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-head-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/dependency",
						"id": "dependency-dependency-head"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/top-level",
						"id": "top-level-top-level-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-head-value"
					}`)
				})

				Convey("returns config with properties from revision pinned by top level ref when no commit or change for either project", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@dependency-revision',
							}`),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/top-level",
						"id": "top-level-top-level-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-revision-value"
					}`)
				})

				Convey("returns config with properties from commit ref when commit for dependency project without ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "dependency",
						Ref:     "refs/heads/some-branch",
					}
					dependencyGitiles.Refs["refs/heads/some-branch"] = "dependency-some-branch-head"
					dependencyGitiles.Revisions["dependency-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/some-branch",
						"id": "dependency-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-head-value"
					}`)
				})

				Convey("returns config with properties from commit revision when commit for dependency project with ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "dependency",
						Ref:     "refs/heads/some-branch",
						Id:      "dependency-some-branch-revision",
					}
					dependencyGitiles.Revisions["dependency-some-branch-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/some-branch",
						"id": "dependency-some-branch-revision"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-revision-value"
					}`)
				})

				Convey("returns config with properties from revision pinned by commit ref when commit for top level project without ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "top/level",
						Ref:     "refs/heads/some-branch",
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@dependency-revision',
							}`),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-revision-value"
					}`)
				})

				Convey("returns config with properties from revision pinned by commit revision when commit for top level project with ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "top/level",
						Ref:     "refs/heads/some-branch",
						Id:      "top-level-some-branch-revision",
					}
					topLevelGitiles.Revisions["top-level-some-branch-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@dependency-revision',
							}`),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-revision"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-revision-value"
					}`)
				})

				Convey("returns config with properties from target ref and patch applied when change for dependency project", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "dependency",
						Change:   2345,
						Patchset: 1,
					})
					dependencyGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					dependencyGitiles.Refs["refs/heads/some-branch"] = "dependency-some-branch-head"
					dependencyGitiles.Revisions["dependency-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-value5"
							}`),
						},
					}
					dependencyGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-old-value5"
							}`),
						},
					}
					dependencyGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-new-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-old-value5"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/some-branch",
						"id": "dependency-some-branch-head"
					}`)
					So(config.change.GerritChange, ShouldResembleProtoJSON, `{
						"host": "chromium-review.googlesource.com",
						"project": "dependency",
						"change": 2345,
						"patchset": 1
					}`)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-head-new-value",
						"test_property2": "some-branch-head-value2",
						"test_property3": "some-branch-head-value3",
						"test_property4": "some-branch-head-value4",
						"test_property5": "some-branch-head-value5"
					}`)
					So(config.skipAnalysisReasons, ShouldResemble, []string{
						"properties file infra/config/fake-bucket/fake-builder/properties.json is affected by CL",
					})
				})

				Convey("returns config with properties from patched pinned revision when change for top level project that changes pin", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "top/level",
						Change:   2345,
						Patchset: 1,
					})
					topLevelGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@old-dependency-revision',
								'other/repo/path': 'https://chromium.googlesource.com/other.git@new-other-revision',
							}`),
						},
					}
					topLevelGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@old-dependency-revision',
								'other/repo/path': 'https://chromium.googlesource.com/other.git@old-other-revision',
							}`),
						},
					}
					topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@new-dependency-revision',
								'other/repo/path': 'https://chromium.googlesource.com/other.git@old-other-revision',
							}`),
						},
					}
					dependencyGitiles.Revisions["old-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "old-dependency-revision-value"
							}`),
						},
					}
					dependencyGitiles.Revisions["new-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "new-dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "new-dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "new-dependency-revision-value"
					}`)
					So(config.skipAnalysisReasons, ShouldResemble, []string{
						"properties file infra/config/fake-bucket/fake-builder/properties.json is affected by CL (via DEPS change)",
					})
				})

				Convey("returns config with properties from patched pinned revision when change for top level project that does not change properties file", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "top/level",
						Change:   2345,
						Patchset: 1,
					})
					topLevelGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@old-dependency-revision',
								'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
								'bar': 'https://chromium.googlesource.com/foo.git@bar-revision',
								'baz': 'https://chromium.googlesource.com/foo.git@baz-revision',
								'other/repo/path': 'https://chromium.googlesource.com/other.git@new-other-revision',
							}`),
						},
					}
					topLevelGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@old-dependency-revision',
								'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
								'bar': 'https://chromium.googlesource.com/foo.git@bar-revision',
								'baz': 'https://chromium.googlesource.com/foo.git@baz-revision',
								'other/repo/path': 'https://chromium.googlesource.com/other.git@old-other-revision',
							}`),
						},
					}
					topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@new-dependency-revision',
								'foo': 'https://chromium.googlesource.com/foo.git@foo-revision',
								'bar': 'https://chromium.googlesource.com/foo.git@bar-revision',
								'baz': 'https://chromium.googlesource.com/foo.git@baz-revision',
								'other/repo/path': 'https://chromium.googlesource.com/other.git@old-other-revision',
							}`),
						},
					}
					dependencyGitiles.Revisions["old-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-value"
							}`),
						},
					}
					dependencyGitiles.Revisions["new-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "new-dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-value"
					}`)
					So(config.skipAnalysisReasons, ShouldBeEmpty)
				})

				Convey("fails with a tagged error when the properties file does not exist at pinned revision", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"DEPS": fakegitiles.File(`deps = {
								'config/repo/path': 'https://chromium.googlesource.com/dependency.git@refs/heads/dependency',
							}`),
						},
					}
					dependencyGitiles.Refs["refs/heads/dependency"] = "dependency-dependency-head"
					dependencyGitiles.Revisions["dependency-dependency-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"": fakegitiles.File("fake-root-contents"),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldErrLike, `dependency properties file infra/config/fake-bucket/fake-builder/properties.json does not exist in pinned revision chromium.googlesource.com/dependency/+/dependency-dependency-head
This should resolve once the CL that adds this builder rolls into chromium.googlesource.com/top/level`)
					sleepDuration, errHasSleepTag := SleepBeforeExiting.In(err)
					So(errHasSleepTag, ShouldBeTrue)
					So(sleepDuration, ShouldEqual, 10*time.Minute)
					So(config, ShouldBeNil)
				})

			})

			Convey("for dependency project with config repo submodule path", func() {

				setBootstrapPropertiesProperties(build, `{
					"dependency_project": {
						"top_level_repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"top_level_ref": "refs/heads/top-level",
						"config_repo": {
							"host": "chromium.googlesource.com",
							"project": "dependency"
						},
						"config_repo_submodule_path": "submodule/path"
					},
					"properties_file": "infra/config/fake-bucket/fake-builder/properties.json"
				}`)

				Convey("returns config with properties from revision pinned by top level ref when no commit or change for either project", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("dependency-revision"),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/top-level",
						"id": "top-level-top-level-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-revision-value"
					}`)
				})

				Convey("returns config with properties from commit ref when commit for dependency project without ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "dependency",
						Ref:     "refs/heads/some-branch",
					}
					dependencyGitiles.Refs["refs/heads/some-branch"] = "dependency-some-branch-head"
					dependencyGitiles.Revisions["dependency-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/some-branch",
						"id": "dependency-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-head-value"
					}`)
				})

				Convey("returns config with properties from commit revision when commit for dependency project with ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "dependency",
						Ref:     "refs/heads/some-branch",
						Id:      "dependency-some-branch-revision",
					}
					dependencyGitiles.Revisions["dependency-some-branch-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/some-branch",
						"id": "dependency-some-branch-revision"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-revision-value"
					}`)
				})

				Convey("returns config with properties from revision pinned by commit ref when commit for top level project without ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "top/level",
						Ref:     "refs/heads/some-branch",
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("dependency-revision"),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-revision-value"
					}`)
				})

				Convey("returns config with properties from revision pinned by commit revision when commit for top level project with ID", func() {
					build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
						Host:    "chromium.googlesource.com",
						Project: "top/level",
						Ref:     "refs/heads/some-branch",
						Id:      "top-level-some-branch-revision",
					}
					topLevelGitiles.Revisions["top-level-some-branch-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("dependency-revision"),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-revision"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-revision-value"
					}`)
				})

				Convey("returns config with properties from target ref and patch applied when change for dependency project", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "dependency",
						Change:   2345,
						Patchset: 1,
					})
					dependencyGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					dependencyGitiles.Refs["refs/heads/some-branch"] = "dependency-some-branch-head"
					dependencyGitiles.Revisions["dependency-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-value5"
							}`),
						},
					}
					dependencyGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-old-value5"
							}`),
						},
					}
					dependencyGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "some-branch-head-new-value",
								"test_property2": "some-branch-head-value2",
								"test_property3": "some-branch-head-value3",
								"test_property4": "some-branch-head-value4",
								"test_property5": "some-branch-head-old-value5"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"ref": "refs/heads/some-branch",
						"id": "dependency-some-branch-head"
					}`)
					So(config.change.GerritChange, ShouldResembleProtoJSON, `{
						"host": "chromium-review.googlesource.com",
						"project": "dependency",
						"change": 2345,
						"patchset": 1
					}`)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "some-branch-head-new-value",
						"test_property2": "some-branch-head-value2",
						"test_property3": "some-branch-head-value3",
						"test_property4": "some-branch-head-value4",
						"test_property5": "some-branch-head-value5"
					}`)
					So(config.skipAnalysisReasons, ShouldResemble, []string{
						"properties file infra/config/fake-bucket/fake-builder/properties.json is affected by CL",
					})
				})

				Convey("returns config with properties from patched pinned revision when change for top level project that changes pin", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "top/level",
						Change:   2345,
						Patchset: 1,
					})
					topLevelGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("old-dependency-revision"),
						},
					}
					topLevelGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("old-dependency-revision"),
						},
					}
					topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("new-dependency-revision"),
						},
					}
					dependencyGitiles.Revisions["old-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "old-dependency-revision-value"
							}`),
						},
					}
					dependencyGitiles.Revisions["new-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "new-dependency-revision-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "new-dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "new-dependency-revision-value"
					}`)
					So(config.skipAnalysisReasons, ShouldResemble, []string{
						"properties file infra/config/fake-bucket/fake-builder/properties.json is affected by CL (via DEPS change)",
					})
				})

				Convey("returns config with properties from patched pinned revision when change for top level project that does not change properties file", func() {
					build.Input.GerritChanges = append(build.Input.GerritChanges, &buildbucketpb.GerritChange{
						Host:     "chromium-review.googlesource.com",
						Project:  "top/level",
						Change:   2345,
						Patchset: 1,
					})
					topLevelGerrit.Changes[2345] = &fakegerrit.Change{
						Ref: "refs/heads/some-branch",
						Patchsets: map[int32]*fakegerrit.Patchset{
							1: {
								Revision: "cl-revision",
							},
						},
					}
					topLevelGitiles.Refs["refs/heads/some-branch"] = "top-level-some-branch-head"
					topLevelGitiles.Revisions["top-level-some-branch-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("old-dependency-revision"),
						},
					}
					topLevelGitiles.Revisions["cl-base"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("old-dependency-revision"),
						},
					}
					topLevelGitiles.Revisions["cl-revision"] = &fakegitiles.Revision{
						Parent: "cl-base",
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("new-dependency-revision"),
						},
					}
					dependencyGitiles.Revisions["old-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-value"
							}`),
						},
					}
					dependencyGitiles.Revisions["new-dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "dependency-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.configCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "dependency",
						"id": "new-dependency-revision"
					}`)
					So(config.inputCommit.GitilesCommit, ShouldResembleProtoJSON, `{
						"host": "chromium.googlesource.com",
						"project": "top/level",
						"ref": "refs/heads/some-branch",
						"id": "top-level-some-branch-head"
					}`)
					So(config.change, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "dependency-value"
					}`)
					So(config.skipAnalysisReasons, ShouldBeEmpty)
				})

				Convey("fails with a tagged error when the properties file does not exist at pinned revision", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"submodule/path": fakegitiles.Submodule("dependency-revision"),
						},
					}
					dependencyGitiles.Revisions["dependency-revision"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"": fakegitiles.File("fake-root-contents"),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldErrLike, `dependency properties file infra/config/fake-bucket/fake-builder/properties.json does not exist in pinned revision chromium.googlesource.com/dependency/+/dependency-revision
This should resolve once the CL that adds this builder rolls into chromium.googlesource.com/top/level`)
					sleepDuration, errHasSleepTag := SleepBeforeExiting.In(err)
					So(errHasSleepTag, ShouldBeTrue)
					So(sleepDuration, ShouldEqual, 10*time.Minute)
					So(config, ShouldBeNil)
				})

			})

			Convey("for builder with shadow props file", func() {
				setBootstrapPropertiesProperties(build, `{
					"top_level_project": {
						"repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"ref": "refs/heads/top-level"
					},
					"properties_file": "infra/config/fake-bucket/fake-builder/properties.json",
					"shadow_properties_file": "infra/config/fake-bucket/fake-builder/shadow-properties.json"
				}`)

				Convey("returns properties without shadow properties if not a shadow build", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "non-shadow-value"
							}`),
							"infra/config/fake-bucket/fake-builder/shadow-properties.json": fakegitiles.File(`{
								"test_property": "shadow-value"
							}`),
						},
					}
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "non-shadow-value"
					}`)
				})

				Convey("returns properties with shadow properties if a shadow build", func() {
					topLevelGitiles.Refs["refs/heads/top-level"] = "top-level-top-level-head"
					topLevelGitiles.Revisions["top-level-top-level-head"] = &fakegitiles.Revision{
						Files: map[string]*fakegitiles.PathObject{
							"infra/config/fake-bucket/fake-builder/properties.json": fakegitiles.File(`{
								"test_property": "non-shadow-value"
							}`),
							"infra/config/fake-bucket/fake-builder/shadow-properties.json": fakegitiles.File(`{
								"test_property": "shadow-value"
							}`),
						},
					}
					setPropertiesFromJson(build, map[string]string{
						"$recipe_engine/led": `{
							"shadowed_bucket": "shadow-fake-bucket"
						}`,
					})
					input := getInput(build)

					config, err := bootstrapper.GetBootstrapConfig(ctx, input)

					So(err, ShouldBeNil)
					So(config.builderProperties, ShouldResembleProtoJSON, `{
						"test_property": "shadow-value"
					}`)
				})

			})

		})

	})
}

func TestUpdateBuild(t *testing.T) {
	t.Parallel()

	Convey("BootstrapConfig.UpdateBuild", t, func() {

		Convey("updates build with gitiles commit, builder properties, $build/chromium_bootstrap module properties and build properties", func() {
			config := &BootstrapConfig{
				inputCommit: &gitilesCommit{&buildbucketpb.GitilesCommit{
					Host:    "fake-host",
					Project: "fake-project",
					Ref:     "fake-ref",
					Id:      "fake-revision",
				}},
				configCommit: &gitilesCommit{&buildbucketpb.GitilesCommit{
					Host:    "fake-host2",
					Project: "fake-project2",
					Ref:     "fake-ref2",
					Id:      "fake-revision2",
				}},
				buildProperties: jsonToStruct(`{
					"foo": "build-requested-foo-value",
					"bar": "build-bar-value",
					"baz": "build-baz-value"
				}`),
				buildRequestedProperties: jsonToStruct(`{
					"foo": "build-requested-foo-value"
				}`),
				builderProperties: jsonToStruct(`{
					"foo": "builder-foo-value",
					"bar": "builder-bar-value",
					"shaz": "builder-shaz-value"
				}`),
				configSource: &ConfigSource{
					LastChangedCommit: &buildbucketpb.GitilesCommit{
						Host:    "fake-host2",
						Project: "fake-project2",
						Ref:     "fake-ref2",
						Id:      "fake-config-revision",
					},
					Path: "path/to/properties/file",
				},
				skipAnalysisReasons: []string{
					"skip-analysis-reason1",
					"skip-analysis-reason2",
				},
			}
			exe := &BootstrappedExe{
				Source: &BootstrappedExe_Cipd{
					Cipd: &Cipd{
						Server:           "fake-cipd-server",
						Package:          "fake-cipd-package",
						RequestedVersion: "fake-cipd-ref",
						ActualVersion:    "fake-cipd-instance-id",
					},
				},
				Cmd: []string{"fake-exe"},
			}
			build := &buildbucketpb.Build{
				Input: &buildbucketpb.Build_Input{
					GitilesCommit: &buildbucketpb.GitilesCommit{
						Host:    "fake-host",
						Project: "fake-project",
						Ref:     "fake-ref",
					},
				},
			}

			Convey("preferring builder properties by default", func() {
				err := config.UpdateBuild(build, exe)

				So(err, ShouldBeNil)
				So(build, ShouldResembleProtoJSON, `{
					"input": {
						"gitiles_commit": {
							"host": "fake-host",
							"project": "fake-project",
							"ref": "fake-ref",
							"id": "fake-revision"
						},
						"properties": {
							"$build/chromium_bootstrap": {
								"commits": [
									{
										"host": "fake-host",
										"project": "fake-project",
										"ref": "fake-ref",
										"id": "fake-revision"
									},
									{
										"host": "fake-host2",
										"project": "fake-project2",
										"ref": "fake-ref2",
										"id": "fake-revision2"
									}
								],
								"exe": {
									"cipd": {
										"server": "fake-cipd-server",
										"package": "fake-cipd-package",
										"requested_version": "fake-cipd-ref",
										"actual_version": "fake-cipd-instance-id"
									},
									"cmd": ["fake-exe"]
								},
								"config_source": {
									"last_changed_commit": {
										"host": "fake-host2",
										"project": "fake-project2",
										"ref": "fake-ref2",
										"id": "fake-config-revision"
									},
									"path": "path/to/properties/file"
								},
								"skip_analysis_reasons": [
									"skip-analysis-reason1",
									"skip-analysis-reason2"
								]
							},
							"foo": "build-requested-foo-value",
							"bar": "builder-bar-value",
							"baz": "build-baz-value",
							"shaz": "builder-shaz-value"
						}
					}
				}`)
			})

			Convey("when preferring build properties", func() {
				config.preferBuildProperties = true

				err := config.UpdateBuild(build, exe)

				So(err, ShouldBeNil)
				So(build, ShouldResembleProtoJSON, `{
					"input": {
						"gitiles_commit": {
							"host": "fake-host",
							"project": "fake-project",
							"ref": "fake-ref",
							"id": "fake-revision"
						},
						"properties": {
							"$build/chromium_bootstrap": {
								"commits": [
									{
										"host": "fake-host",
										"project": "fake-project",
										"ref": "fake-ref",
										"id": "fake-revision"
									},
									{
										"host": "fake-host2",
										"project": "fake-project2",
										"ref": "fake-ref2",
										"id": "fake-revision2"
									}
								],
								"exe": {
									"cipd": {
										"server": "fake-cipd-server",
										"package": "fake-cipd-package",
										"requested_version": "fake-cipd-ref",
										"actual_version": "fake-cipd-instance-id"
									},
									"cmd": ["fake-exe"]
								},
								"config_source": {
									"last_changed_commit": {
										"host": "fake-host2",
										"project": "fake-project2",
										"ref": "fake-ref2",
										"id": "fake-config-revision"
									},
									"path": "path/to/properties/file"
								},
								"skip_analysis_reasons": [
									"skip-analysis-reason1",
									"skip-analysis-reason2"
								]
							},
							"foo": "build-requested-foo-value",
							"bar": "build-bar-value",
							"baz": "build-baz-value",
							"shaz": "builder-shaz-value"
						}
					}
				}`)
			})

		})

		Convey("updates build with $build/chromium_bootstrap module properties and build properties for properties optional bootstrapping", func() {
			config := &BootstrapConfig{
				buildProperties: jsonToStruct(`{
					"foo": "build-foo-value",
					"bar": "build-bar-value"
				}`),
			}
			exe := &BootstrappedExe{
				Source: &BootstrappedExe_Cipd{
					Cipd: &Cipd{
						Server:           "fake-cipd-server",
						Package:          "fake-cipd-package",
						RequestedVersion: "fake-cipd-ref",
						ActualVersion:    "fake-cipd-instance-id",
					},
				},
				Cmd: []string{"fake-exe"},
			}
			build := &buildbucketpb.Build{
				Input: &buildbucketpb.Build_Input{},
			}

			err := config.UpdateBuild(build, exe)

			So(err, ShouldBeNil)
			So(build, ShouldResembleProtoJSON, `{
				"input": {
					"properties": {
						"$build/chromium_bootstrap": {
							"exe": {
								"cipd": {
									"server": "fake-cipd-server",
									"package": "fake-cipd-package",
									"requested_version": "fake-cipd-ref",
									"actual_version": "fake-cipd-instance-id"
								},
								"cmd": ["fake-exe"]
							}
						},
						"foo": "build-foo-value",
						"bar": "build-bar-value"
					}
				}
			}`)
		})

		Convey("does not update gitiles commit for different repo", func() {
			config := &BootstrapConfig{
				configCommit: &gitilesCommit{&buildbucketpb.GitilesCommit{
					Host:    "fake-host",
					Project: "fake-project",
					Ref:     "fake-ref",
					Id:      "fake-revision",
				}},
				buildProperties: jsonToStruct("{}"),
			}
			exe := &BootstrappedExe{
				Source: &BootstrappedExe_Cipd{
					Cipd: &Cipd{
						Server:           "fake-cipd-server",
						Package:          "fake-cipd-package",
						RequestedVersion: "fake-cipd-ref",
						ActualVersion:    "fake-cipd-instance-id",
					},
				},
				Cmd: []string{"fake-exe"},
			}
			build := &buildbucketpb.Build{
				Input: &buildbucketpb.Build_Input{
					GitilesCommit: &buildbucketpb.GitilesCommit{
						Host:    "fake-host",
						Project: "fake-other-project",
						Ref:     "fake-ref",
					},
				},
			}

			err := config.UpdateBuild(build, exe)

			So(err, ShouldBeNil)
			So(build.Input.GitilesCommit, ShouldResembleProtoJSON, `{
				"host": "fake-host",
				"project": "fake-other-project",
				"ref": "fake-ref"
			}`)

		})

		Convey("applies led property edits", func() {
			config := &BootstrapConfig{
				buildProperties: jsonToStruct(`{
					"foo": "build-requested-foo-value",
					"bar": "build-bar-value",
					"baz": "build-baz-value"
				}`),
				builderProperties: jsonToStruct(`{
					"foo": "builder-foo-value"
				}`),
				buildRequestedProperties: jsonToStruct(`{
					"foo": "build-requested-foo-value"
				}`),
				ledEditedProperties: jsonToStruct(`{
					"foo": "led-foo-value",
					"bar": "led-bar-value",
					"baz": "led-baz-value"
				}`),
			}
			exe := &BootstrappedExe{
				Source: &BootstrappedExe_Cipd{
					Cipd: &Cipd{
						Server:           "fake-cipd-server",
						Package:          "fake-cipd-package",
						RequestedVersion: "fake-cipd-ref",
						ActualVersion:    "fake-cipd-instance-id",
					},
				},
				Cmd: []string{"fake-exe"},
			}
			build := &buildbucketpb.Build{
				Input: &buildbucketpb.Build_Input{},
			}

			err := config.UpdateBuild(build, exe)

			So(err, ShouldBeNil)
			So(build.Input.Properties.Fields["foo"], ShouldResembleProtoJSON, `"led-foo-value"`)
			So(build.Input.Properties.Fields["bar"], ShouldResembleProtoJSON, `"led-bar-value"`)
			So(build.Input.Properties.Fields["baz"], ShouldResembleProtoJSON, `"led-baz-value"`)
		})

		Convey("removes properties removed by led", func() {
			config := &BootstrapConfig{
				buildProperties: jsonToStruct(`{
					"foo": "build-requested-foo-value",
					"bar": "build-bar-value",
					"baz": "build-baz-value"
				}`),
				builderProperties: jsonToStruct(`{
					"foo": "builder-foo-value"
				}`),
				buildRequestedProperties: jsonToStruct(`{
					"foo": "build-requested-foo-value"
				}`),
				ledRemovedProperties: []string{"foo", "bar", "baz"},
			}
			exe := &BootstrappedExe{
				Source: &BootstrappedExe_Cipd{
					Cipd: &Cipd{
						Server:           "fake-cipd-server",
						Package:          "fake-cipd-package",
						RequestedVersion: "fake-cipd-ref",
						ActualVersion:    "fake-cipd-instance-id",
					},
				},
				Cmd: []string{"fake-exe"},
			}
			build := &buildbucketpb.Build{
				Input: &buildbucketpb.Build_Input{},
			}

			err := config.UpdateBuild(build, exe)

			So(err, ShouldBeNil)
			So(build.Input.Properties.Fields, ShouldNotContainKey, "foo")
			So(build.Input.Properties.Fields, ShouldNotContainKey, "bar")
			So(build.Input.Properties.Fields, ShouldNotContainKey, "baz")
		})

	})

}
