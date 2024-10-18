// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"

	goconfig "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/build/api"
	testapi "go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	buildbucket_pb "go.chromium.org/luci/buildbucket/proto"
	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/common/testing/typed"
	"google.golang.org/protobuf/types/known/timestamppb"

	"infra/libs/skylab/inventory"
	"infra/libs/skylab/request"
)

func TestBuilderID(t *testing.T) {
	ftt.Run("Given request arguments that specify a builder ID", t, func(t *ftt.Test) {
		id := buildbucket_pb.BuilderID{
			Project: "foo-project",
			Bucket:  "foo-bucket",
			Builder: "foo-builder",
		}
		args := request.Args{
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(&id)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have a builder ID.", func(t *ftt.Test) {
				assert.Loosely(t, req.Builder, should.Match(&id))
			})
		})
	})
}

func TestDimensionsBB(t *testing.T) {
	ftt.Run("Given request arguments that specify provisionable and regular dimenisons and inventory labels", t, func(t *ftt.Test) {
		model := "foo-model"
		args := request.Args{
			Dimensions:                       []string{"k1:v1"},
			ProvisionableDimensions:          []string{"provisionable-k2:v2", "provisionable-k3:v3"},
			ProvisionableDimensionExpiration: 30 * time.Second,
			SchedulableLabels:                &inventory.SchedulableLabels{Model: &model},
			SecondaryDevicesLabels: []*inventory.SchedulableLabels{
				{Model: &model},
			},
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have correct dimensions.", func(t *ftt.Test) {
				assert.Loosely(t, req.Dimensions, should.HaveLength(5))

				want := []*buildbucket_pb.RequestedDimension{
					{
						Key:        "provisionable-k2",
						Value:      "v2",
						Expiration: ptypes.DurationProto(30 * time.Second),
					},
					{
						Key:        "provisionable-k3",
						Value:      "v3",
						Expiration: ptypes.DurationProto(30 * time.Second),
					},
					{
						Key:   "k1",
						Value: "v1",
					},
					{
						Key:   "label-model",
						Value: "foo-model",
					},
					{
						Key:   "label-model",
						Value: "foo-model_2",
					},
				}

				assert.Loosely(t, sortBBDimensions(req.Dimensions), should.Match(sortBBDimensions(want)))
			})
		})
	})
}

func TestPropertiesBB(t *testing.T) {
	ftt.Run("Given request arguments that specify a test runner request", t, func(t *ftt.Test) {
		want := skylab_test_runner.Request{
			Prejob: &skylab_test_runner.Request_Prejob{
				SoftwareDependencies: []*test_platform.Request_Params_SoftwareDependency{
					{
						Dep: &test_platform.Request_Params_SoftwareDependency_ChromeosBuild{
							ChromeosBuild: "foo-build",
						},
					},
				},
				ProvisionableLabels: map[string]string{
					"key": "value",
				},
			},
			Test: &skylab_test_runner.Request_Test{
				Harness: &skylab_test_runner.Request_Test_Autotest_{
					Autotest: &skylab_test_runner.Request_Test_Autotest{
						Name:     "foo-test",
						TestArgs: "a1=v1 a2=v2",
						Keyvals: map[string]string{
							"k1": "v1",
							"k2": "v2",
						},
						IsClientTest: true,
						DisplayName:  "fancy-name",
					},
				},
			},
		}
		args := request.Args{
			TestRunnerRequest: &want,
		}
		t.Run("when a BB request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("it should contain the test runner request.", func(t *ftt.Test) {
				assert.Loosely(t, req.Properties, should.NotBeNil)

				reqStruct, ok := req.Properties.Fields["request"]
				assert.Loosely(t, ok, should.BeTrue)

				m := jsonpb.Marshaler{}
				s, err := m.MarshalToString(reqStruct)
				assert.Loosely(t, err, should.BeNil)

				var got skylab_test_runner.Request
				err = jsonpb.UnmarshalString(s, &got)
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, &got, should.Match(&want))
			})
		})
	})
}

func TestCFTPropertiesBB(t *testing.T) {
	ftt.Run("Given request arguments that specify a cft test runner request", t, func(t *ftt.Test) {
		want := skylab_test_runner.CFTTestRequest{
			Deadline:         &timestamppb.Timestamp{Seconds: timestamppb.Now().Seconds},
			ParentRequestUid: "foo-parentRequestUid",
			ParentBuildId:    12345678,
			PrimaryDut: &skylab_test_runner.CFTTestRequest_Device{
				DutModel: &labapi.DutModel{
					BuildTarget: "foo-buildTarget",
					ModelName:   "foo-modelName",
				},
				ProvisionState: &testapi.ProvisionState{
					SystemImage: &testapi.ProvisionState_SystemImage{
						SystemImagePath: &goconfig.StoragePath{
							HostType: goconfig.StoragePath_GS,
							Path:     "gs://foo-image-bucket/foo-build-subdir",
						},
					},
				},
				ContainerMetadataKey: "foo-buildTarget",
			},
			ContainerMetadata: &api.ContainerMetadata{
				Containers: map[string]*api.ContainerImageMap{
					"foo-buildTarget": {
						Images: map[string]*api.ContainerImageInfo{
							"foo-image": {
								Repository: &api.GcrRepository{
									Hostname: "foo-hostName",
									Project:  "foo-Project",
								},
								Name:   "foo-name",
								Digest: "foo-digest",
								Tags:   []string{"foo-tag1", "foo-tag2"},
							},
						},
					},
				},
			},
			TestSuites: []*testapi.TestSuite{
				{
					Name: "foo-suiteName",
					Spec: &testapi.TestSuite_TestCaseIds{
						TestCaseIds: &testapi.TestCaseIdList{
							TestCaseIds: []*testapi.TestCase_Id{
								{
									Value: "tauto.foo-test",
								},
							},
						},
					},
				},
			},
			DefaultTestExecutionBehavior: test_platform.Request_Params_NON_CRITICAL,
			AutotestKeyvals: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
		}

		args := request.Args{
			CFTIsEnabled:         true,
			CFTTestRunnerRequest: &want,
		}
		t.Run("when a BB request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("it should contain the cft test runner request.", func(t *ftt.Test) {
				assert.Loosely(t, req.Properties, should.NotBeNil)

				cftReqStruct, ok := req.Properties.Fields["cft_test_request"]
				assert.Loosely(t, ok, should.BeTrue)

				m := jsonpb.Marshaler{}
				s, err := m.MarshalToString(cftReqStruct)
				assert.Loosely(t, err, should.BeNil)

				var got skylab_test_runner.CFTTestRequest
				err = jsonpb.UnmarshalString(s, &got)
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, &got, should.Match(&want))
			})
		})
	})
}

func TestExperimentsBB(t *testing.T) {
	ftt.Run("Given request arguments that specify experiments", t, func(t *ftt.Test) {
		args := request.Args{
			Experiments:       []string{"chromeos.a.b", "chromeos.c.d"},
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have correct experiments.", func(t *ftt.Test) {
				assert.Loosely(t, req.Experiments, should.HaveLength(2))
				want := map[string]bool{
					"chromeos.a.b": true,
					"chromeos.c.d": true,
				}
				assert.Loosely(t, req.Experiments, should.Match(want))
			})
		})
	})
}

func TestTagsBB(t *testing.T) {
	ftt.Run("Given request arguments that specify tags", t, func(t *ftt.Test) {
		args := request.Args{
			SwarmingTags:      []string{"k1:v1", "k2:v2"},
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have correct tags.", func(t *ftt.Test) {
				assert.Loosely(t, req.Tags, should.HaveLength(2))

				want := []*buildbucket_pb.StringPair{
					{
						Key:   "k1",
						Value: "v1",
					},
					{
						Key:   "k2",
						Value: "v2",
					},
				}

				assert.Loosely(t, sortBBStringPairs(req.Tags), should.Match(sortBBStringPairs(want)))
			})
		})
	})
}

func TestGerritChangesBB(t *testing.T) {
	ftt.Run("Given request arguments that specify Gerrit Changes", t, func(t *ftt.Test) {
		gc := &buildbucket_pb.GerritChange{
			Host:     "a",
			Project:  "b",
			Change:   123,
			Patchset: 1,
		}
		args := request.Args{
			SwarmingTags:      []string{"k1:v1", "k2:v2"},
			TestRunnerRequest: &skylab_test_runner.Request{},
			GerritChanges:     []*buildbucket_pb.GerritChange{gc},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have the correct Gerrit Changes", func(t *ftt.Test) {
				assert.Loosely(t, req.GerritChanges, should.HaveLength(1))
				assert.Loosely(t, req.GerritChanges, should.Match([]*buildbucket_pb.GerritChange{gc}))
			})
			t.Run("and the hide-in-gerrit tag", func(t *ftt.Test) {
				assert.Loosely(t, req.Tags, should.HaveLength(3))

				want := []*buildbucket_pb.StringPair{
					{
						Key:   "k1",
						Value: "v1",
					},
					{
						Key:   "k2",
						Value: "v2",
					},
					{
						Key:   "hide-in-gerrit",
						Value: "test_runner",
					},
				}

				assert.Loosely(t, sortBBStringPairs(req.Tags), should.Match(sortBBStringPairs(want)))
			})
		})
	})
}

func TestPriorityBB(t *testing.T) {
	ftt.Run("Given request arguments that specify tags", t, func(t *ftt.Test) {
		args := request.Args{
			Priority:          42,
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have correct priority.", func(t *ftt.Test) {
				assert.Loosely(t, req.Priority, should.Equal(42))
			})
		})
	})
}

func TestStatusTopicBB(t *testing.T) {
	ftt.Run("Given request arguments that specify a Pubsub topic for status updates", t, func(t *ftt.Test) {
		args := request.Args{
			StatusTopic:       "a topic name",
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have the Pubsub topic assigned.", func(t *ftt.Test) {
				assert.Loosely(t, req.Notify, should.NotBeNil)
				assert.Loosely(t, req.Notify.PubsubTopic, should.Equal("a topic name"))
			})
		})
	})
}

func TestNoStatusTopicBB(t *testing.T) {
	ftt.Run("Given request arguments that specify a Pubsub topic for status updates", t, func(t *ftt.Test) {
		args := request.Args{
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have no notify field.", func(t *ftt.Test) {
				assert.Loosely(t, req.Notify, should.BeNil)
			})
		})
	})
}

func TestResultsConfigBB(t *testing.T) {
	ftt.Run("Given request arguments that specify ResultsConfig", t, func(t *ftt.Test) {
		want := test_platform.Request_Params_ResultsUploadConfig{
			Mode: test_platform.Request_Params_ResultsUploadConfig_TEST_RESULTS_VISIBILITY_CUSTOM_REALM,
		}
		args := request.Args{
			ResultsConfig:     &want,
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have correct results_upload_config.", func(t *ftt.Test) {
				assert.Loosely(t, req.Properties, should.NotBeNil)

				resultsCfg, ok := req.Properties.Fields["results_upload_config"]
				assert.Loosely(t, ok, should.BeTrue)

				m := jsonpb.Marshaler{}
				s, err := m.MarshalToString(resultsCfg)
				assert.Loosely(t, err, should.BeNil)

				var got test_platform.Request_Params_ResultsUploadConfig
				err = jsonpb.UnmarshalString(s, &got)
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, &got, should.Match(&want))
			})
		})
	})

	ftt.Run("Given request arguments that do NOT specify ResultsConfig", t, func(t *ftt.Test) {
		args := request.Args{
			TestRunnerRequest: &skylab_test_runner.Request{},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.NewBBRequest(nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should not have a results_upload_config", func(t *ftt.Test) {
				assert.Loosely(t, req.Properties, should.NotBeNil)

				_, ok := req.Properties.Fields["results_upload_config"]
				assert.Loosely(t, ok, should.BeFalse)
			})
		})
	})
}

func TestProvisionableDimensions(t *testing.T) {
	ftt.Run("Given request arguments that specify provisionable and regular dimenisons and inventory labels", t, func(t *ftt.Test) {
		model := "foo-model"
		args := request.Args{
			Dimensions:              []string{"k1:v1"},
			ProvisionableDimensions: []string{"k2:v2", "k3:v3"},
			SchedulableLabels:       &inventory.SchedulableLabels{Model: &model},
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.SwarmingNewTaskRequest()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have correct slice structure.", func(t *ftt.Test) {
				assert.Loosely(t, req.TaskSlices, should.HaveLength(2))

				// First slice requires all dimensions.
				// Second slice (fallback) requires only non-provisionable dimensions.
				s0 := req.TaskSlices[0]
				s1 := req.TaskSlices[1]
				assert.Loosely(t, s0.Properties.Dimensions, should.HaveLength(6))
				assert.Loosely(t, s1.Properties.Dimensions, should.HaveLength(4))

				s1Expect := toStringPairs([]string{
					"pool:ChromeOSSkylab",
					"dut_state:ready",
					fmt.Sprintf("label-model:%s", model),
					"k1:v1",
				})
				diff := typed.Got(sortDimensions(s1.Properties.Dimensions)).Want(sortDimensions(s1Expect)).Diff()
				assert.Loosely(t, diff, should.BeEmpty)

				s0Expect := append(s1Expect, toStringPairs([]string{"k2:v2", "k3:v3"})...)
				diff = typed.Got(sortDimensions(s0.Properties.Dimensions)).Want(sortDimensions(s0Expect)).Diff()
				assert.Loosely(t, diff, should.BeEmpty)

				// First slice command doesn't include provisioning.
				// Second slice (fallback) does.
				s0FlatCmd := strings.Join(s0.Properties.Command, " ")
				s1FlatCmd := strings.Join(s1.Properties.Command, " ")
				provString := "-provision-labels k2:v2,k3:v3"
				assert.Loosely(t, s0FlatCmd, should.NotContainSubstring(provString))
				assert.Loosely(t, s1FlatCmd, should.ContainSubstring(provString))
			})
		})
	})
}

func TestStatusTopicSwarming(t *testing.T) {
	ftt.Run("Given request arguments that specify a Pubsub topic for status updates", t, func(t *ftt.Test) {
		args := request.Args{
			StatusTopic: "a topic name",
		}
		t.Run("when a request is formed", func(t *ftt.Test) {
			req, err := args.SwarmingNewTaskRequest()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			t.Run("then request should have the Pubsub topic assigned.", func(t *ftt.Test) {
				assert.Loosely(t, req.PubsubTopic, should.Equal("a topic name"))
			})
		})
	})
}

func TestSliceExpiration(t *testing.T) {
	timeout := 11 * time.Minute
	ftt.Run("Given a request arguments with no provisionable dimensions", t, func(t *ftt.Test) {
		args := request.Args{
			Timeout: timeout,
		}
		req, err := args.SwarmingNewTaskRequest()
		assert.Loosely(t, req, should.NotBeNil)
		assert.Loosely(t, err, should.BeNil)
		t.Run("request should have a single slice with provided timeout.", func(t *ftt.Test) {
			assert.Loosely(t, req.TaskSlices, should.HaveLength(1))
			assert.Loosely(t, req.TaskSlices[0].ExpirationSecs, should.Equal(60*11))
		})
	})
	ftt.Run("Given a request arguments with provisionable dimensions", t, func(t *ftt.Test) {
		args := request.Args{
			Timeout:                 timeout,
			ProvisionableDimensions: []string{"k1:v1"},
		}
		req, err := args.SwarmingNewTaskRequest()
		assert.Loosely(t, req, should.NotBeNil)
		assert.Loosely(t, err, should.BeNil)
		t.Run("request should have 2 slices, with provided timeout on only the second.", func(t *ftt.Test) {
			assert.Loosely(t, req.TaskSlices, should.HaveLength(2))
			assert.Loosely(t, req.TaskSlices[0].ExpirationSecs, should.BeLessThan(60*5))
			assert.Loosely(t, req.TaskSlices[1].ExpirationSecs, should.Equal(60*11))
		})
	})
}

func TestStaticDimensions(t *testing.T) {
	cases := []struct {
		Tag  string
		Args *request.Args
		Want []*swarming.SwarmingRpcsStringPair
	}{
		{
			Tag:  "empty args",
			Args: &request.Args{},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab"}),
		},
		{
			Tag: "one schedulable label",
			Args: &request.Args{
				SchedulableLabels: &inventory.SchedulableLabels{
					Model: stringPtr("some_model"),
				},
			},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab", "label-model:some_model"}),
		},
		{
			Tag: "one dimension",
			Args: &request.Args{
				Dimensions: []string{"some:value"},
			},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab", "some:value"}),
		},
		{
			Tag: "one provisionable dimension",
			Args: &request.Args{
				ProvisionableDimensions: []string{"cros-version:value"},
			},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab"}),
		},
		{
			Tag: "one of each",
			Args: &request.Args{
				SchedulableLabels: &inventory.SchedulableLabels{
					Model: stringPtr("some_model"),
				},
				Dimensions:              []string{"some:value"},
				ProvisionableDimensions: []string{"cros-version:value"},
			},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab", "label-model:some_model", "some:value"}),
		},
		{
			Tag: "Multi-DUTs without duplicate board/model",
			Args: &request.Args{
				SchedulableLabels: &inventory.SchedulableLabels{
					Board: stringPtr("coral"),
					Model: stringPtr("babytiger"),
				},
				Dimensions:              []string{"some:value"},
				ProvisionableDimensions: []string{"cros-version:value", "cros-version:value2"},
				SecondaryDevicesLabels: []*inventory.SchedulableLabels{
					{
						Board: stringPtr("eve"),
						Model: stringPtr("eve"),
					},
				},
			},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab", "label-board:coral", "label-model:babytiger", "some:value", "label-board:eve", "label-model:eve"}),
		},
		{
			Tag: "Multi-DUTs with duplicate board/model",
			Args: &request.Args{
				SchedulableLabels: &inventory.SchedulableLabels{
					Board: stringPtr("coral"),
					Model: stringPtr("babytiger"),
				},
				Dimensions:              []string{"some:value"},
				ProvisionableDimensions: []string{"cros-version:value", "cros-version:value2"},
				SecondaryDevicesLabels: []*inventory.SchedulableLabels{
					{
						Board: stringPtr("coral"),
						Model: stringPtr("babytiger"),
					},
				},
			},
			Want: toStringPairs([]string{"pool:ChromeOSSkylab", "label-board:coral", "label-model:babytiger", "some:value", "label-board:coral_2", "label-model:babytiger_2"}),
		},
		{
			Tag: "explicitly specified pool in args",
			Args: &request.Args{
				SwarmingPool: "OtherPool",
			},
			Want: toStringPairs([]string{"pool:OtherPool"}),
		},
	}

	for _, c := range cases {
		t.Run(c.Tag, func(t *testing.T) {
			got, err := c.Args.StaticDimensions()
			if err != nil {
				t.Fatalf("error in StaticDimensions(): %s", err)
			}
			want := sortDimensions(c.Want)
			got = sortDimensions(got)
			if diff := typed.Got(got).Want(want).Diff(); diff != "" {
				t.Errorf("Incorrect static dimensions, -want +got: %s", diff)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func toStringPairs(ss []string) []*swarming.SwarmingRpcsStringPair {
	ret := make([]*swarming.SwarmingRpcsStringPair, len(ss))
	for i, s := range ss {
		p := strings.Split(s, ":")
		if len(p) != 2 {
			panic(fmt.Sprintf("Invalid dimension %s", s))
		}
		ret[i] = &swarming.SwarmingRpcsStringPair{
			Key:   p[0],
			Value: p[1],
		}
	}
	return ret
}

func sortBBStringPairs(dims []*buildbucket_pb.StringPair) []*buildbucket_pb.StringPair {
	sort.SliceStable(dims, func(i, j int) bool {
		return dims[i].Key < dims[j].Key || (dims[i].Key == dims[j].Key && dims[i].Value < dims[j].Value)
	})
	return dims
}

func sortBBDimensions(dims []*buildbucket_pb.RequestedDimension) []*buildbucket_pb.RequestedDimension {
	sort.SliceStable(dims, func(i, j int) bool {
		return dims[i].Key < dims[j].Key || (dims[i].Key == dims[j].Key && dims[i].Value < dims[j].Value)
	})
	return dims
}

func sortDimensions(dims []*swarming.SwarmingRpcsStringPair) []*swarming.SwarmingRpcsStringPair {
	sort.SliceStable(dims, func(i, j int) bool {
		return dims[i].Key < dims[j].Key || (dims[i].Key == dims[j].Key && dims[i].Value < dims[j].Value)
	})
	return dims
}
