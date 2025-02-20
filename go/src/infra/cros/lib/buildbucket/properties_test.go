// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package buildbucket

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	bbpb "go.chromium.org/luci/buildbucket/proto"

	"go.chromium.org/infra/cros/internal/assert"
	"go.chromium.org/infra/cros/internal/cmd"
)

const (
	invalidJSON = "{'is-this-valid-json?': False"
	// experiment_reasons have enums as integers
	validJSON = `{
		"buildbucket": {
			"bbagent_args": {
				"build": {
					"input": {
						"properties": {
							"$chromeos/my_module": {
								"my_prop": 100
							},
							"my_other_prop": 101
						}
					},
					"infra": {
						"buildbucket": {
							"experiment_reasons": {
								"chromeos.cros_artifacts.use_gcloud_storage": 1
							}
						}
					}
				}
			}
		}
	}`
	// experiment_reasons have enums as strings
	unmarshalErrorButInputPropsOK = `{
		"buildbucket": {
			"bbagent_args": {
				"build": {
					"input": {
						"properties": {
							"$chromeos/my_module": {
								"my_prop": 100
							},
							"my_other_prop": 101
						}
					},
					"infra": {
						"buildbucket": {
							"experiment_reasons": {
								"chromeos.cros_artifacts.use_gcloud_storage": "EXPERIMENT_REASON_BUILDER_CONFIG"
							}
						}
					}
				}
			}
		}
	}`
	// unknown enum value
	unknownEnumValue = `{
		"buildbucket": {
			"bbagent_args": {
				"build": {
					"input": {
						"properties": {
							"$chromeos/my_module": {
								"my_prop": 100
							},
							"my_other_prop": 101
						}
					},
					"infra": {
						"buildbucket": {
							"experiment_reasons": {
								"chromeos.cros_artifacts.use_gcloud_storage": "EXPERIMENT_MAKE_BELIEVE"
							}
						}
					}
				}
			}
		}
	}`
	// "input" is misspelled
	unmarshalErrorWithNoInputProperties = `{
		"buildbucket": {
			"bbagent_args": {
				"build": {
					"inputt": {
						"properties": {
							"$chromeos/my_module": {
								"my_prop": 100
							},
							"my_other_prop": 101
						}
					},
					"infra": {
						"buildbucket": {
							"experiment_reasons": {
								"chromeos.cros_artifacts.use_gcloud_storage": "EXPERIMENT_REASON_BUILDER_CONFIG"
							}
						}
					}
				}
			}
		}
	}`
)

// TestGetBuilderInputProps tests GetBuilderInputProps.
// The most interesting logic to test is where it permits certain json.UnmarshalTypeErrors.
func TestGetBuilderInputProps(t *testing.T) {
	t.Parallel()
	okInputProperties, err := structpb.NewStruct(map[string]interface{}{
		"$chromeos/my_module": map[string]interface{}{
			"my_prop": 100,
		},
		"my_other_prop": 101,
	})
	if err != nil {
		t.Fatal("Error constructing okInputProperties:", err)
	}
	for i, tc := range []struct {
		ledGetBuilderStdout string
		expectError         bool
		expectedInputProps  *structpb.Struct // Unchecked if expectError
	}{
		{validJSON, false, okInputProperties},
		{invalidJSON, true, nil},
		{unmarshalErrorButInputPropsOK, false, okInputProperties},
		{unknownEnumValue, false, okInputProperties},
		{unmarshalErrorWithNoInputProperties, true, nil},
	} {
		c := NewClient(cmd.FakeCommandRunner{
			ExpectedCmd: []string{"led", "get-builder", "chromeos/release:release-main-orchestrator"},
			Stdout:      tc.ledGetBuilderStdout,
		}, nil, nil)
		propsStruct, err := c.GetBuilderInputProps(context.Background(), "chromeos/release/release-main-orchestrator")
		if err != nil && !tc.expectError {
			t.Errorf("#%d: Unexpected error running GetBuilderInputProps: %+v", i, err)
		}
		if err == nil && tc.expectError {
			t.Errorf("#%d: Expected error running GetBuilderInputProps; got no error. props: %+v", i, propsStruct)
		}
		if !tc.expectError && propsStruct.String() != tc.expectedInputProps.String() {
			t.Errorf("#%d: Unexpected input props: got %+v; want %+v", i, propsStruct, tc.expectedInputProps)
		}
	}
}

func TestSetProperty(t *testing.T) {
	t.Parallel()
	s, err := structpb.NewStruct(map[string]interface{}{
		"$chromeos/my_module": map[string]interface{}{
			"my_prop": 100,
		},
		"my_other_prop": "101",
	})
	assert.NilError(t, err)

	SetProperty(s, "$chromeos/my_module.my_prop", 200)
	SetProperty(s, "$chromeos/my_module.new_prop.foo", []string{"a", "b", "c"})
	SetProperty(s, "my_other_prop", "201")

	fields := s.GetFields()
	assert.IntsEqual(t, int(fields["$chromeos/my_module"].GetStructValue().GetFields()["my_prop"].GetNumberValue()), 200)

	myProp := fields["$chromeos/my_module"].GetStructValue().GetFields()["new_prop"].GetStructValue().GetFields()["foo"].GetListValue().AsSlice()
	myPropStr := make([]string, len(myProp))
	for i := range myProp {
		myPropStr[i] = myProp[i].(string)
	}
	assert.StringArrsEqual(t, myPropStr, []string{"a", "b", "c"})

	assert.StringsEqual(t, fields["my_other_prop"].GetStringValue(), "201")
}

func TestSetProperty_error(t *testing.T) {
	t.Parallel()
	s, err := structpb.NewStruct(map[string]interface{}{
		"$chromeos/my_module": map[string]interface{}{
			"my_prop": 100,
		},
		"my_other_prop": "101",
	})
	assert.NilError(t, err)

	invalidValue := struct {
		String string
		Number int
	}{
		"foo",
		123,
	}
	err = SetProperty(s, "$chromeos/my_module.my_prop", invalidValue)
	assert.ErrorContains(t, err, "invalid type")

	err = SetProperty(s, "totally_new_prop", invalidValue)
	assert.ErrorContains(t, err, "invalid type")

	err = SetProperty(s, "my_other_prop.invalid_nest", 123)
	assert.ErrorContains(t, err, "not a struct")
}

const (
	buildUnmarshalErrorButInputPropsOK = `{
		"id": "12345",
		"createTime": "2023-04-10T04:00:03.884668293Z",
		"builder": {
			"project": "chromeos",
			"bucket": "staging",
			"builder": "staging-release-main-orchestrator"
		},
		"status": "SUCCESS",
		"output": {
			"properties": {
				"$chromeos/my_module": {
					"my_prop": 100
				},
				"my_other_prop": 101
			}
		}
	}`
	buildUnmarshalNoError = `{
		"id": "12346",
		"createTime": "2023-04-10T04:00:03.884668293Z",
		"builder": {
			"project": "chromeos",
			"bucket": "staging",
			"builder": "staging-release-main-orchestrator"
		},
		"status": "FAILURE"
	}`
	// "outputt" is misspelled.
	buildUnmarshalErrorWithNoInputProperties = `{
		"id": "8794230068334833057",
		"createTime": "2023-04-10T04:00:03.884668293Z",
		"builder": {
			"project": "chromeos",
			"bucket": "staging",
			"builder": "staging-release-main-orchestrator"
		},
		"outputt": {
			"properties": {
				"$chromeos/my_module": {
					"my_prop": 100
				},
				"my_other_prop": 101
			}
		}
	}`
)

// TestGetBuild tests GetBuild.
func TestGetBuild(t *testing.T) {
	t.Parallel()
	bbid := "12345"
	var okBuild bbpb.Build

	outputProps, err := structpb.NewStruct(map[string]interface{}{
		"$chromeos/my_module": map[string]interface{}{
			"my_prop": 100,
		},
		"my_other_prop": 101,
	})
	if err != nil {
		t.Fatal("Error constructing outputProps:", err)
	}
	okBuild.Id = 12345
	okBuild.Status = bbpb.Status_SUCCESS
	okBuild.Builder = &bbpb.BuilderID{
		Project: "chromeos",
		Bucket:  "staging",
		Builder: "staging-release-main-orchestrator",
	}
	okBuild.Output = &bbpb.Build_Output{
		Properties: outputProps,
	}
	okBuild.CreateTime = &timestamppb.Timestamp{
		Seconds: 1681099203,
		Nanos:   884668293,
	}

	for i, tc := range []struct {
		bbGetStdout   string
		expectError   bool
		expectedBuild *bbpb.Build // Unchecked if expectError
	}{
		{stripNewlines(invalidJSON), true, nil},
		{stripNewlines(buildUnmarshalErrorButInputPropsOK), false, &okBuild},
		{stripNewlines(buildUnmarshalErrorWithNoInputProperties), true, nil},
	} {
		c := NewClient(cmd.FakeCommandRunner{
			ExpectedCmd: []string{"bb", "get", bbid, "-p", "-json"},
			Stdout:      tc.bbGetStdout,
		}, nil, nil)
		build, err := c.GetBuild(context.Background(), bbid)
		if err != nil && !tc.expectError {
			t.Errorf("#%d: Unexpected error running GetBuild: %+v", i, err)
		}
		if err == nil && tc.expectError {
			t.Errorf("#%d: Expected error running GetBuild; got no error. build: %+v", i, build)
		}
		if !tc.expectError && build.String() != tc.expectedBuild.String() {
			t.Errorf("#%d: Unexpected build:\ngot\n%+v\n\nwant\n%+v", i, build, tc.expectedBuild)
		}
	}
}

func stripNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", "")
}

// TestGetBuilds tests GetBuilds.
func TestGetBuilds(t *testing.T) {
	t.Parallel()

	outputProps, err := structpb.NewStruct(map[string]interface{}{
		"$chromeos/my_module": map[string]interface{}{
			"my_prop": 100,
		},
		"my_other_prop": 101,
	})
	if err != nil {
		t.Fatal("Error constructing outputProps:", err)
	}

	var expectedBuild bbpb.Build
	expectedBuild.Status = bbpb.Status_SUCCESS
	expectedBuild.Builder = &bbpb.BuilderID{
		Project: "chromeos",
		Bucket:  "staging",
		Builder: "staging-release-main-orchestrator",
	}
	expectedBuilds := []bbpb.Build{expectedBuild, expectedBuild}
	expectedBuilds[0].Id = 12345
	expectedBuilds[0].Output = &bbpb.Build_Output{
		Properties: outputProps,
	}
	expectedBuilds[0].CreateTime = &timestamppb.Timestamp{
		Seconds: 1681099203,
		Nanos:   884668293,
	}
	expectedBuilds[1].Id = 12346
	expectedBuilds[1].Status = bbpb.Status_FAILURE
	expectedBuilds[1].CreateTime = &timestamppb.Timestamp{
		Seconds: 1681099203,
		Nanos:   884668293,
	}

	stdout := (stripNewlines(buildUnmarshalErrorButInputPropsOK) + "\n" +
		stripNewlines(buildUnmarshalNoError))
	c := NewClient(cmd.FakeCommandRunner{
		ExpectedCmd: []string{"bb", "get", "12345", "12346", "-p", "-json"},
		Stdout:      stdout,
	}, nil, nil)
	builds, err := c.GetBuilds(context.Background(), []string{"12345", "12346"})
	if err != nil {
		t.Errorf("Unexpected error running GetBuild: %+v", err)
	}
	for i := range expectedBuilds {
		if builds[i].String() != expectedBuilds[i].String() {
			t.Errorf("Unexpected build #%d:\ngot\n%+v\n\nwant\n%+v\n", i, builds[i].String(), expectedBuilds[i].String())
		}
	}
}

// TestListBuilds tests ListBuilds.
func TestListBuilds(t *testing.T) {
	t.Parallel()

	outputProps, err := structpb.NewStruct(map[string]interface{}{
		"$chromeos/my_module": map[string]interface{}{
			"my_prop": 100,
		},
		"my_other_prop": 101,
	})
	if err != nil {
		t.Fatal("Error constructing outputProps:", err)
	}

	var expectedBuild bbpb.Build
	expectedBuild.Status = bbpb.Status_SUCCESS
	expectedBuild.Builder = &bbpb.BuilderID{
		Project: "chromeos",
		Bucket:  "staging",
		Builder: "staging-release-main-orchestrator",
	}
	expectedBuilds := []bbpb.Build{expectedBuild, expectedBuild}
	expectedBuilds[0].Id = 12345
	expectedBuilds[0].Output = &bbpb.Build_Output{
		Properties: outputProps,
	}
	expectedBuilds[0].CreateTime = &timestamppb.Timestamp{
		Seconds: 1681099203,
		Nanos:   884668293,
	}
	expectedBuilds[1].Id = 12346
	expectedBuilds[1].Status = bbpb.Status_FAILURE
	expectedBuilds[1].CreateTime = &timestamppb.Timestamp{
		Seconds: 1681099203,
		Nanos:   884668293,
	}

	stdout := (stripNewlines(buildUnmarshalErrorButInputPropsOK) + "\n" +
		stripNewlines(buildUnmarshalNoError))
	predicate := `{"foo": "bar"}`
	c := NewClient(cmd.FakeCommandRunner{
		ExpectedCmd: []string{"bb", "ls", "-predicate", predicate, "-p", "-json"},
		Stdout:      stdout,
	}, nil, nil)
	builds, err := c.ListBuildsWithPredicate(context.Background(), predicate)
	if err != nil {
		t.Errorf("Unexpected error running GetBuild: %+v", err)
	}
	for i := range expectedBuilds {
		if builds[i].String() != expectedBuilds[i].String() {
			t.Errorf("Unexpected build #%d:\ngot\n%+v\n\nwant\n%+v\n", i, builds[i].String(), expectedBuilds[i].String())
		}
	}
}
