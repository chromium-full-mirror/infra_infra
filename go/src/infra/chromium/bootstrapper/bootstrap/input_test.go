// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bootstrap

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestInput(t *testing.T) {
	t.Parallel()

	ftt.Run("NewInput", t, func(t *ftt.Test) {
		build := &buildbucketpb.Build{
			Input: &buildbucketpb.Build_Input{
				Properties: &structpb.Struct{},
			},
		}

		opts := InputOptions{}

		t.Run("fails if no required bootstrap properties are not set", func(t *ftt.Test) {
			input, err := opts.NewInput(build)

			assert.Loosely(t, err, should.ErrLike("the following required properties are not set: $bootstrap/exe, $bootstrap/properties"))
			assert.Loosely(t, input, should.BeNil)
		})

		t.Run("fails validating $bootstrap/properties", func(t *ftt.Test) {
			setBootstrapExeProperties(build, `{
				"exe": {
					"cipd_package": "fake-package",
					"cipd_version": "fake-version",
					"cmd": ["fake-exe"]
				}
			}`)

			t.Run("for incorrectly typed $bootstrap/properties", func(t *ftt.Test) {
				setBootstrapPropertiesProperties(build, `{"foo": "bar"}`)

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.ErrLike(`unknown field "foo"`))
				assert.Loosely(t, input, should.BeNil)
			})

			t.Run("for invalid $bootstrap/properties", func(t *ftt.Test) {
				setBootstrapPropertiesProperties(build, "{}")
				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.ErrLike("none of the config_project fields in $bootstrap/properties is set"))
				assert.Loosely(t, input, should.BeNil)
			})

		})

		t.Run("fails validating $bootstrap/exe", func(t *ftt.Test) {
			setBootstrapPropertiesProperties(build, `{
				"top_level_project": {
					"repo": {
						"host": "chromium.googlesource.com",
						"project": "top/level"
					},
					"ref": "refs/heads/top-level"
				},
				"properties_file": "go.chromium.org/infra/config/fake-bucket/fake-builder/properties.json"
			}`)

			t.Run("for incorrectly typed $bootstrap/exe", func(t *ftt.Test) {
				setBootstrapExeProperties(build, `{"foo": "bar"}`)

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.ErrLike(`unknown field "foo"`))
				assert.Loosely(t, input, should.BeNil)
			})

			t.Run("for invalid $bootstrap/exe", func(t *ftt.Test) {
				setBootstrapExeProperties(build, "{}")

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.ErrLike("$bootstrap/exe.exe is not set"))
				assert.Loosely(t, input, should.BeNil)
			})

		})

		t.Run("succeeds", func(t *ftt.Test) {
			setBootstrapPropertiesProperties(build, `{
				"top_level_project": {
					"repo": {
						"host": "chromium.googlesource.com",
						"project": "top/level"
					},
					"ref": "refs/heads/top-level"
				},
				"properties_file": "go.chromium.org/infra/config/fake-bucket/fake-builder/properties.json"
			}`)
			setBootstrapExeProperties(build, `{
				"exe": {
					"cipd_package": "fake-package",
					"cipd_version": "fake-version",
					"cmd": ["fake-exe"]
				}
			}`)
			build.Input.Properties.Fields["foo"] = structpb.NewStringValue("bar")

			t.Run("for well-formed properties", func(t *ftt.Test) {
				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.commits, should.BeEmpty)
				assert.Loosely(t, input.changes, should.BeEmpty)
				assert.That(t, input.buildProperties, should.Match(&structpb.Struct{
					Fields: map[string]*structpb.Value{
						"foo": structpb.NewStringValue("bar"),
					},
				}))
				assert.Loosely(t, input.buildRequestedProperties, should.BeNil)
				assert.That(t, input.propsProperties, should.Match(mustParseBootstrapPropertiesProperties(`{
					"top_level_project": {
						"repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"ref": "refs/heads/top-level"
					},
					"properties_file": "go.chromium.org/infra/config/fake-bucket/fake-builder/properties.json"
				}`)))
				assert.That(t, input.exeProperties, should.Match(mustParseBootstrapExeProperties(`{
					"exe": {
						"cipd_package": "fake-package",
						"cipd_version": "fake-version",
						"cmd": ["fake-exe"]
					}
				}`)))
				assert.Loosely(t, input.casRecipeBundle, should.BeNil)
				assert.Loosely(t, input.ledEditedProperties, should.BeNil)
				assert.Loosely(t, input.ledRemovedProperties, should.BeNil)
				// Make sure the build wasn't modified
				assert.Loosely(t, build.Input.Properties.Fields, should.ContainKey("$bootstrap/properties"))
				assert.Loosely(t, build.Input.Properties.Fields, should.ContainKey("$bootstrap/exe"))
			})

			t.Run("with requested properties", func(t *ftt.Test) {
				build.Infra = &buildbucketpb.BuildInfra{
					Buildbucket: &buildbucketpb.BuildInfra_Buildbucket{
						RequestedProperties: jsonToStruct(`{
							"foo": "bar"
						}`),
					},
				}

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.buildRequestedProperties, should.Match(&structpb.Struct{
					Fields: map[string]*structpb.Value{
						"foo": structpb.NewStringValue("bar"),
					},
				}))
			})

			t.Run("for shadow build", func(t *ftt.Test) {
				build.Input.Properties.Fields["$recipe_engine/led"] = structpb.NewStructValue(jsonToStruct(`{
					"shadowed_bucket": "fake-bucket"
				}`))

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.shadowBuild, should.BeTrue)
			})

			t.Run("for polymorphic option", func(t *ftt.Test) {
				opts := InputOptions{Polymorphic: true}

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.polymorphic, should.BeTrue)
			})

			t.Run("without $bootstrap/properties if PropertiesOptional is set", func(t *ftt.Test) {
				opts := InputOptions{PropertiesOptional: true}
				delete(build.Input.Properties.Fields, "$bootstrap/properties")

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.propertiesOptional, should.BeTrue)
				assert.Loosely(t, input.propsProperties, should.BeNil)
			})

			t.Run("with $bootstrap/properties if PropertiesOptional is set", func(t *ftt.Test) {
				opts := InputOptions{PropertiesOptional: true}

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.propertiesOptional, should.BeTrue)
				assert.Loosely(t, input.propsProperties, should.Match(mustParseBootstrapPropertiesProperties(`{
					"top_level_project": {
						"repo": {
							"host": "chromium.googlesource.com",
							"project": "top/level"
						},
						"ref": "refs/heads/top-level"
					},
					"properties_file": "go.chromium.org/infra/config/fake-bucket/fake-builder/properties.json"
				}`)))
			})

			t.Run("with commits set if build has commit", func(t *ftt.Test) {
				build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
					Host:    "fake-host",
					Project: "fake-project",
					Ref:     "fake-ref",
					Id:      "fake-revision",
				}

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.commits, should.Resemble([]*buildbucketpb.GitilesCommit{build.Input.GitilesCommit}))
				// Make sure we can't modify the build through aliased protos
				assert.Loosely(t, input.commits[0], should.NotEqual(build.Input.GitilesCommit))
			})

			t.Run("with commits set if $bootstrap/trigger has commits", func(t *ftt.Test) {
				setBootstrapTriggerProperties(build, `{
					"commits": [
						{
							"host": "fake-host1",
							"project": "fake-project1",
							"ref": "fake-ref1",
							"id": "fake-revision1"
						},
						{
							"host": "fake-host2",
							"project": "fake-project2",
							"ref": "fake-ref2",
							"id": "fake-revision2"
						}
					]
				}`)

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.commits, should.Resemble([]*buildbucketpb.GitilesCommit{
					{
						Host:    "fake-host1",
						Project: "fake-project1",
						Ref:     "fake-ref1",
						Id:      "fake-revision1",
					},
					{
						Host:    "fake-host2",
						Project: "fake-project2",
						Ref:     "fake-ref2",
						Id:      "fake-revision2",
					},
				}))
			})

			t.Run("with commits set if build has commit and $bootstrap/trigger has commits", func(t *ftt.Test) {
				build.Input.GitilesCommit = &buildbucketpb.GitilesCommit{
					Host:    "fake-host1",
					Project: "fake-project1",
					Ref:     "fake-ref1a",
				}
				setBootstrapTriggerProperties(build, `{
					"commits": [
						{
							"host": "fake-host1",
							"project": "fake-project1",
							"ref": "fake-ref1b",
							"id": "fake-revision1"
						},
						{
							"host": "fake-host2",
							"project": "fake-project2",
							"ref": "fake-ref2",
							"id": "fake-revision2"
						}
					]
				}`)

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.commits, should.Resemble([]*buildbucketpb.GitilesCommit{
					{
						Host:    "fake-host1",
						Project: "fake-project1",
						Ref:     "fake-ref1a",
					},
					{
						Host:    "fake-host1",
						Project: "fake-project1",
						Ref:     "fake-ref1b",
						Id:      "fake-revision1",
					},
					{
						Host:    "fake-host2",
						Project: "fake-project2",
						Ref:     "fake-ref2",
						Id:      "fake-revision2",
					},
				}))
			})

			t.Run("with changes set if build has changes", func(t *ftt.Test) {
				build.Input.GerritChanges = []*buildbucketpb.GerritChange{
					{
						Host:     "fake-host",
						Project:  "fake-project",
						Change:   1234,
						Patchset: 5,
					},
					{
						Host:     "fake-host2",
						Project:  "fake-project2",
						Change:   6789,
						Patchset: 10,
					},
				}

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.changes, should.HaveLength(2))
				assert.Loosely(t, input.changes, should.Resemble(build.Input.GerritChanges))
				// Make sure we can't modify the build through aliased protos
				assert.Loosely(t, input.changes[0], should.NotEqual(build.Input.GerritChanges[0]))
				assert.Loosely(t, input.changes[1], should.NotEqual(build.Input.GerritChanges[1]))
			})

			t.Run("with casRecipeBundle set if build has led_cas_recipe_bundle property", func(t *ftt.Test) {
				build.Input.Properties.Fields["led_cas_recipe_bundle"] = structpb.NewStructValue(jsonToStruct(`{
					"cas_instance": "fake-instance",
					"digest": {
						"hash": "fake-hash",
						"size_bytes": 42
					}
				}`))

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.casRecipeBundle, should.Match(&apipb.CASReference{
					CasInstance: "fake-instance",
					Digest: &apipb.Digest{
						Hash:      "fake-hash",
						SizeBytes: 42,
					},
				}))
				assert.Loosely(t, input.buildProperties.Fields, should.NotContainKey("led_cas_recipe_bundle"))
				// Make sure the build wasn't modified
				assert.Loosely(t, build.Input.Properties.Fields, should.ContainKey("led_cas_recipe_bundle"))
			})

			t.Run("with ledEditedProperties set if build has led_edited_properties property", func(t *ftt.Test) {
				build.Input.Properties.Fields["led_edited_properties"] = structpb.NewStructValue(jsonToStruct(`{
					"foo": "led-foo-value",
					"bar": "led-bar-value"
				}`))

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.That(t, input.ledEditedProperties, should.Match(&structpb.Struct{
					Fields: map[string]*structpb.Value{
						"foo": structpb.NewStringValue("led-foo-value"),
						"bar": structpb.NewStringValue("led-bar-value"),
					},
				}))
				assert.Loosely(t, input.buildProperties.Fields, should.NotContainKey("led_edited_properties"))
				// Make sure the build wasn't modified
				assert.Loosely(t, build.Input.Properties.Fields, should.ContainKey("led_edited_properties"))
			})

			t.Run("with ledRemovedProperties set if build has led_removed_properties property", func(t *ftt.Test) {
				build.Input.Properties.Fields["led_removed_properties"] = structpb.NewListValue(&structpb.ListValue{
					Values: []*structpb.Value{
						structpb.NewStringValue("foo"),
						structpb.NewStringValue("bar"),
					},
				})

				input, err := opts.NewInput(build)

				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, input.ledRemovedProperties, should.Resemble([]string{"foo", "bar"}))
				assert.Loosely(t, input.buildProperties.Fields, should.NotContainKey("led_removed_properties"))
				// Make sure the build wasn't modified
				assert.Loosely(t, build.Input.Properties.Fields, should.ContainKey("led_removed_properties"))
			})
		})

	})
}

func mustParseBootstrapPropertiesProperties(msg string) *BootstrapPropertiesProperties {
	var data BootstrapPropertiesProperties
	if err := protojson.Unmarshal([]byte(msg), &data); err != nil {
		panic(err)
	}
	return &data
}

func mustParseBootstrapExeProperties(msg string) *BootstrapExeProperties {
	var data BootstrapExeProperties
	if err := protojson.Unmarshal([]byte(msg), &data); err != nil {
		panic(err)
	}
	return &data
}
