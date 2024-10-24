// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/buildbucket"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/errors"
	luciproto "go.chromium.org/luci/common/proto"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/logdog/common/types"
	"go.chromium.org/luci/lucictx"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
)

func newAnn(stepNames ...string) *annopb.Step {
	ann := &annopb.Step{
		Substep: make([]*annopb.Step_Substep, len(stepNames)),
	}
	for i, n := range stepNames {
		ann.Substep[i] = &annopb.Step_Substep{
			Substep: &annopb.Step_Substep_Step{
				Step: &annopb.Step{Name: n},
			},
		}
	}
	return ann
}

func newAnnBytes(stepNames ...string) []byte {
	ret, err := proto.Marshal(newAnn(stepNames...))
	if err != nil {
		panic(err)
	}
	return ret
}

func TestBuildUpdater(t *testing.T) {
	t.Parallel()

	ftt.Run(`buildUpdater`, t, func(c *ftt.Test) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := buildbucketpb.NewMockBuildsClient(ctrl)

		// Ensure tests don't hang.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		ctx, clk := testclock.UseTime(ctx, testclock.TestRecentTimeUTC)

		bu := &buildUpdater{
			buildID:    42,
			buildToken: "build token",
			annAddr: &types.StreamAddr{
				Host:    "logdog.example.com",
				Project: "chromium",
				Path:    "prefix/+/annotations",
			},
			client:      client,
			annotations: make(chan []byte),
		}

		c.Run("build token is sent", func(c *ftt.Test) {
			updateBuild := func(ctx context.Context, req *buildbucketpb.UpdateBuildRequest, opts ...grpc.CallOption) (*buildbucketpb.Build, error) {
				md, ok := metadata.FromOutgoingContext(ctx)
				assert.Loosely(c, ok, should.BeTrue)
				assert.Loosely(c, md.Get(buildbucket.BuildTokenHeader), should.Resemble([]string{"build token"}))
				res := &buildbucketpb.Build{}
				return res, nil
			}
			client.EXPECT().
				UpdateBuild(gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(updateBuild)

			err := bu.UpdateBuild(ctx, &buildbucketpb.UpdateBuildRequest{})
			assert.Loosely(c, err, should.BeNil)
		})

		c.Run(`run`, func(c *ftt.Test) {
			update := func(ctx context.Context, annBytes []byte) error {
				return nil
			}

			errC := make(chan error)
			done := make(chan struct{})
			start := func() {
				go func() {
					errC <- bu.run(ctx, done, update)
				}()
			}

			run := func(err1, err2 error) error {
				update = func(ctx context.Context, annBytes []byte) error {
					if string(annBytes) == "1" {
						return err1
					}
					return err2
				}
				start()
				bu.AnnotationUpdated([]byte("1"))
				bu.AnnotationUpdated([]byte("2"))
				cancel()
				return <-errC
			}

			c.Run("two successful requests", func(c *ftt.Test) {
				assert.Loosely(c, run(nil, nil), should.BeNil)
			})

			c.Run("first failed, second succeeded", func(c *ftt.Test) {
				assert.Loosely(c, run(fmt.Errorf("transient"), nil), should.BeNil)
			})

			c.Run("first succeeded, second failed", func(c *ftt.Test) {
				assert.Loosely(c, run(nil, fmt.Errorf("fatal")), should.ErrLike("fatal"))
			})

			c.Run("minDistance", func(c *ftt.Test) {
				var sleepDuration time.Duration
				open := true
				clk.SetTimerCallback(func(d time.Duration, t clock.Timer) {
					if testclock.HasTags(t, "update-build-distance") {
						sleepDuration += d
						clk.Add(d)

						if open {
							close(done)
							open = false
						}

					}
				})

				start()
				bu.AnnotationUpdated([]byte("1"))
				assert.Loosely(c, <-errC, should.BeNil)
				assert.Loosely(c, sleepDuration, should.BeGreaterThanOrEqual(time.Second))
			})

			c.Run("errSleep", func(c *ftt.Test) {
				attempt := 0
				clk.SetTimerCallback(func(d time.Duration, t clock.Timer) {
					switch {
					case testclock.HasTags(t, "update-build-distance"):
						clk.Add(d)
					case testclock.HasTags(t, "update-build-error"):
						clk.Add(d)
						attempt++
						if attempt == 4 {
							bu.AnnotationUpdated([]byte("2"))
						}
					}
				})

				update = func(ctx context.Context, annBytes []byte) error {
					if string(annBytes) == "1" {
						return fmt.Errorf("err")
					}

					close(done)
					return nil
				}

				start()
				bu.AnnotationUpdated([]byte("1"))
				assert.Loosely(c, <-errC, should.BeNil)
			})

			c.Run("first is fatal, second never occurs", func(c *ftt.Test) {
				fatal := status.Error(codes.InvalidArgument, "too large")
				calls := 0
				update = func(ctx context.Context, annBytes []byte) error {
					calls++
					return fatal
				}
				start()
				bu.AnnotationUpdated([]byte("1"))
				cancel()
				assert.Loosely(c, errors.Unwrap(<-errC), should.Equal(fatal))
				assert.Loosely(c, calls, should.Equal(1))
			})

			c.Run("done is closed", func(c *ftt.Test) {
				start()
				bu.AnnotationUpdated([]byte("1"))
				close(done)
				assert.Loosely(c, <-errC, should.BeNil)
			})
		})

		c.Run("ParseAnnotations", func(c *ftt.Test) {
			ann := &annopb.Step{}
			err := luciproto.UnmarshalTextML(`
				substep: <
					step: <
						name: "bot_update"
						status: SUCCESS
						started: < seconds: 1400000000 >
						ended: < seconds: 1400001000 >
						property: <
							name: "$recipe_engine/buildbucket/output_gitiles_commit"
							value: <<END
							{
								"host": "chrome-internal.googlesource.com",
								"project": "chromeos/manifest-internal",
								"ref": "refs/heads/master",
								"id": "91401dc270212d98734ab894bd90609b882aa458",
								"position": 2
							}
END
						>
					>
				>
				substep: <
					step: <
						name: "compile"
						status: RUNNING
						started: < seconds: 1400001000 >
						property: <
							name: "foo"
							value: "\"bar\""
						>
					>
				>
			`, ann)
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, ann.Substep, should.HaveLength(2))

			expected := &buildbucketpb.UpdateBuildRequest{}
			err = jsonpb.UnmarshalString(`{
				"build": {
					"id": 42,
					"steps": [
						{
							"name": "bot_update",
							"status": "SUCCESS",
							"startTime": "2014-05-13T16:53:20.0Z",
							"endTime": "2014-05-13T17:10:00.0Z"

						},
						{
							"name": "compile",
							"status": "STARTED",
							"startTime": "2014-05-13T17:10:00.0Z"
						}
					],
					"output": {
						"properties": {"foo": "bar"},
						"gitilesCommit": {
							"host": "chrome-internal.googlesource.com",
							"project": "chromeos/manifest-internal",
							"ref": "refs/heads/master",
							"id": "91401dc270212d98734ab894bd90609b882aa458",
							"position": 2
						}
					}
				},
				"updateMask": {
					"paths": [
						"build.steps",
						"build.output.properties",
						"build.output.gitiles_commit"
					]
				}
			}`, expected)
			assert.Loosely(c, err, should.BeNil)

			actual, err := bu.ParseAnnotations(ctx, ann)
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, actual, should.Resemble(expected))
		})

		c.Run(`test addtional tags`, func(c *ftt.Test) {
			ann := &annopb.Step{}
			err := luciproto.UnmarshalTextML(`
					substep: <
						step: <
							name: "buildbucket.add_tags_to_current_build"
							status: SUCCESS
							started: < seconds: 1400000000 >
							ended: < seconds: 1400001000 >
							property: <
								name: "$recipe_engine/buildbucket/runtime-tags"
								value:<<END
								{
									"k1": ["v1"],
									"k2": ["v2"]
								}
	END
							>
						>
					>
				`, ann)
			assert.Loosely(c, err, should.BeNil)
			assert.Loosely(c, ann.Substep, should.HaveLength(1))

			expected := &buildbucketpb.UpdateBuildRequest{}
			err = jsonpb.UnmarshalString(`{
					"build": {
						"id": 42,
						"steps": [
							{
								"name": "buildbucket.add_tags_to_current_build",
								"status": "SUCCESS",
								"startTime": "2014-05-13T16:53:20.0Z",
								"endTime": "2014-05-13T17:10:00.0Z"

							}
						],
						"output": {
							"properties": {}
						},
						"tags": [
							{
								"key": "k1",
								"value": "v1"
							},
							{
								"key": "k2",
								"value": "v2"
							}
						]
					},
					"updateMask": {
						"paths": [
							"build.steps",
							"build.output.properties",
							"build.tags"
						]
					}
				}`, expected)
			assert.Loosely(c, err, should.BeNil)

			actual, err := bu.ParseAnnotations(ctx, ann)
			assert.Loosely(c, err, should.BeNil)

			// Because the order is undeterministic when iterating the dict during
			// the process of converting to Tags proto in ParseAnnotations func.
			sortTagsFunc := func(tags []*buildbucketpb.StringPair) {
				sort.Slice(tags, func(i, j int) bool {
					if tags[i].Key != tags[j].Key {
						return tags[i].Key < tags[j].Key
					}
					return tags[i].Value < tags[j].Value
				})
			}
			sortTagsFunc(actual.Build.Tags)
			sortTagsFunc(expected.Build.Tags)

			assert.Loosely(c, actual, should.Resemble(expected))
		})
	})
}

func TestReadBuildSecrets(t *testing.T) {
	t.Parallel()

	ftt.Run("readBuildSecrets", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx = lucictx.SetSwarming(ctx, nil)

		t.Run("empty", func(t *ftt.Test) {
			secrets, err := readBuildSecrets(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, secrets, should.Resemble(&buildbucketpb.BuildSecrets{}))
		})

		t.Run("build token", func(t *ftt.Test) {
			secretBytes, err := proto.Marshal(&buildbucketpb.BuildSecrets{
				BuildToken: "build token",
			})
			assert.Loosely(t, err, should.BeNil)

			ctx = lucictx.SetSwarming(ctx, &lucictx.Swarming{SecretBytes: secretBytes})

			secrets, err := readBuildSecrets(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, string(secrets.BuildToken), should.Equal("build token"))
		})
	})
}

func TestOutputCommitFromLegacyProperties(t *testing.T) {
	t.Parallel()

	parse := func(propJSON string) (*buildbucketpb.GitilesCommit, error) {
		propStruct := &structpb.Struct{}
		err := jsonpb.UnmarshalString(propJSON, propStruct)
		assert.Loosely(t, err, should.BeNil)

		return outputCommitFromLegacyProperties(propStruct)
	}

	ftt.Run("TestOutputCommitFromLegacyProperties", t, func(t *ftt.Test) {
		t.Run("no properties", func(t *ftt.Test) {
			actual, err := parse(`{}`)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.BeNil)
		})

		t.Run("got_revision id", func(t *ftt.Test) {
			actual, err := parse(`{
				"$recipe_engine/buildbucket": {
					"build": {
						"input": {
							"gitilesCommit": {
								"host": "chromium.googlesource.com",
								"project": "chromium/src",
								"id": "deadbeef"
							}
						}
					}
				},
				"got_revision": "e57f4e87022d765b45e741e478a8351d9789bc37"
			}`)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.Resemble(&buildbucketpb.GitilesCommit{
				Host:    "chromium.googlesource.com",
				Project: "chromium/src",
				Ref:     "refs/heads/master",
				Id:      "e57f4e87022d765b45e741e478a8351d9789bc37",
			}))
		})

		t.Run("got_revision non-default ref", func(t *ftt.Test) {
			actual, err := parse(`{
				"$recipe_engine/buildbucket": {
					"build": {
						"input": {
							"gitilesCommit": {
								"host": "chromium.googlesource.com",
								"project": "chromium/src",
								"ref": "refs/heads/x"
							}
						}
					}
				},
				"got_revision": "e57f4e87022d765b45e741e478a8351d9789bc37"
			}`)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.Resemble(&buildbucketpb.GitilesCommit{
				Host:    "chromium.googlesource.com",
				Project: "chromium/src",
				Ref:     "refs/heads/x",
				Id:      "e57f4e87022d765b45e741e478a8351d9789bc37",
			}))
		})

		t.Run("got_revision id tryjob", func(t *ftt.Test) {
			actual, err := parse(`{
				"$recipe_engine/buildbucket": {
					"build": {
						"input": {
							"gerritChanges": [{
								"host": "chromium.googlesource.com",
								"project": "chromium/src",
								"change": 1,
								"patchset": 2
							}]
						}
					}
				},
				"got_revision": "e57f4e87022d765b45e741e478a8351d9789bc37"
			}`)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.Resemble(&buildbucketpb.GitilesCommit{
				Host:    "chromium.googlesource.com",
				Project: "chromium/src",
				Ref:     "refs/heads/master",
				Id:      "e57f4e87022d765b45e741e478a8351d9789bc37",
			}))
		})

		t.Run("got_revision ref", func(t *ftt.Test) {
			actual, err := parse(`{
				"$recipe_engine/buildbucket": {
					"build": {
						"input": {
							"gitilesCommit": {
								"host": "chromium.googlesource.com",
								"project": "chromium/src"
							}
						}
					}
				},
				"got_revision": "refs/heads/master"
			}`)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.Resemble(&buildbucketpb.GitilesCommit{
				Host:    "chromium.googlesource.com",
				Project: "chromium/src",
				Ref:     "refs/heads/master",
			}))
		})

		t.Run("got_revision_cp", func(t *ftt.Test) {
			actual, err := parse(`{
				"$recipe_engine/buildbucket": {
					"build": {
						"input": {
							"gitilesCommit": {
								"host": "chromium.googlesource.com",
								"project": "chromium/src"
							}
						}
					}
				},
				"got_revision": "e57f4e87022d765b45e741e478a8351d9789bc37",
				"got_revision_cp": "refs/heads/master@{#673406}"
			}`)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, actual, should.Resemble(&buildbucketpb.GitilesCommit{
				Host:     "chromium.googlesource.com",
				Project:  "chromium/src",
				Ref:      "refs/heads/master",
				Id:       "e57f4e87022d765b45e741e478a8351d9789bc37",
				Position: 673406,
			}))
		})
	})
}
