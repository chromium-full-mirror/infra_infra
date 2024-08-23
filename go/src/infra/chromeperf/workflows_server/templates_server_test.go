// Copyright 2020 The Chromium Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	configProto "go.chromium.org/luci/common/proto/config"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/impl/memory"

	"infra/chromeperf/workflows"
)

const bufSize = 1024 * 1024

const configSingleTemplate = `
		templates: {
			name: "chromium-telemetry-bisect-v1"
			display_name: "Chromium Telemetry benchmark bisection (v1)"
			description: "Bisects a Telemetry benchmark+metric+story for culprits between two CLs"
			inputs: [
				{
					name: "benchmark"
					kind: TYPE_STRING
					cardinality: CARDINALITY_REQUIRED
					options: {
						name: "regex_validator"
						value {
							[type.googleapis.com/google.protobuf.Value] {
								string_value: "^[a-zA-Z0-9\\._\\-]+$"
							}
						}
					}
				},
				{
					name: "configuration"
					kind: TYPE_STRING
					cardinality: CARDINALITY_REQUIRED
				},
				{
					name: "story_tags"
					kind: TYPE_STRING
					cardinality: CARDINALITY_OPTIONAL
				},
				{
					name: "metric"
					kind: TYPE_STRING
					cardinality: CARDINALITY_REQUIRED
					number: 4
				},
				{
					name: "commit_range"
					kind: TYPE_MESSAGE
					cardinality: CARDINALITY_REQUIRED
					type_url: "api.pinpoint.cr.dev/GitilesCommitRange"
				}
			]
			task_options: {
				fields: [
					{
						key: "benchmark"
						value {
							string_value: "{benchmark}"
						}
					},
					{
						key: "configuration"
						value {
							string_value: "{configuration}"
						}
					},
					{
						key: "start_git_hash"
						value {
							string_value: "{commit_range.start_git_hash}"
						}
					},
					{
						key: "end_git_hash"
						value {
							string_value: "{commit_range.end_git_hash}"
						}
					}
				]
			}
			graph_creation_module: "chromeperf.pinpoint.bisector"
			cria_readers: [
				'project-pinpoint-api-users'
			]
		}`

const configMultipleTemplates = configSingleTemplate + `
		templates: {
			name: "chromium-telemetry-ab-v1"
			display_name: "Chromium Telemetry benchmark A/B test (v1)"
			description: "Tests a Telemetry benchmark+metric+story between a reference (A) and an experiment (B)"
			inputs: [
				{
					name: "benchmark"
					kind: TYPE_STRING
					cardinality: CARDINALITY_REQUIRED
					options: {
						name: "regex_validator"
						value {
							[type.googleapis.com/google.protobuf.Value] {
								string_value: "^[a-zA-Z0-9\\._\\-]+$"
							}
						}
					}
				},
				{
					name: "configuration"
					kind: TYPE_STRING
					cardinality: CARDINALITY_REQUIRED
				},
				{
					name: "story_tags"
					kind: TYPE_STRING
					cardinality: CARDINALITY_OPTIONAL
				},
				{
					name: "metric"
					kind: TYPE_STRING
					cardinality: CARDINALITY_REQUIRED
					number: 4
				},
				{
					name: "experiment"
					kind: TYPE_MESSAGE
					cardinality: CARDINALITY_REQUIRED
					type_url: "api.pinpoint.cr.dev/infra.chromeperf.pinpoint.Experiment"
				}
			]
			task_options: {
				fields: [
					{
						key: "benchmark"
						value {
							string_value: "{benchmark}"
						}
					},
					{
						key: "configuration"
						value {
							string_value: "{configuration}"
						}
					},
					{
						key: "base_git_hash"
						value {
							string_value: "{experiment.base_commit}"
						}
					},
					{
						key: "base_patch"
						value {
							string_value: "{experiment.base_patch}"
						}
					},
					{
						key: "exp_git_hash"
						value {
							string_value: "{experiment.experiment_commit}"
						}
					},
					{
						key: "exp_patch"
						value {
							string_value: "{experiment.experiment_patch}"
						}
					}
				]
			}
			graph_creation_module: "chromeperf.pinpoint.bisector"
			cria_readers: [
				'project-pinpoint-api-users'
			]
		}`

func TestValidConfigurations(t *testing.T) {
	ctx := context.Background()
	l := bufconn.Listen(bufSize)
	s := grpc.NewServer()

	// Define the same client to use throughout the test.
	dialer := func(context.Context, string) (net.Conn, error) {
		return l.Dial()
	}
	// Also set-up a "mock" luci-config HTTP service which we will perform requests against.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %v", r)
		// This will just serve the contents of the file.
		fmt.Fprintln(w, configSingleTemplate)
	}))
	defer ts.Close()

	// Set up the in-memory configset for this service.
	configSets := map[config.Set]memory.Files{
		configSetName: map[string]string{},
	}
	mockConfig := func(body string) {
		configSets[configSetName][workflowTemplatesFile] = body
	}
	workflows.RegisterWorkflowTemplatesServer(s, &workflowTemplatesServer{luciConfigClient: memory.New(configSets)})
	go func() {
		if err := s.Serve(l); err != nil {
			log.Fatalf("Server startup failed.")
		}
	}()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed creating a connection: %v", err)
	}
	defer conn.Close()
	client := workflows.NewWorkflowTemplatesClient(conn)

	ftt.Run("Given a valid configuration defined with one template", t, func(t *ftt.Test) {
		mockConfig(configSingleTemplate)

		t.Run("When we attempt to validate the contents", func(t *ftt.Test) {
			resp, err := client.ValidateConfig(
				ctx, &configProto.ValidationRequestMessage{
					ConfigSet: "test-validation",
					Path:      "/test/validation/path",
					Content:   []byte(configSingleTemplate),
				},
			)

			t.Run("Then we get a non-error response", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, resp.Messages, should.BeEmpty)
			})
		})

		t.Run("When we list the templates", func(t *ftt.Test) {
			resp, err := client.ListWorkflowTemplates(
				ctx, &workflows.ListWorkflowTemplatesRequest{
					PageSize: 10,
				},
			)

			t.Run("Then we find that the defined template is in the list", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, resp.WorkflowTemplates, should.NotBeEmpty)
			})
		})

		t.Run("When we get the template by name", func(t *ftt.Test) {
			wt, err := client.GetWorkflowTemplate(ctx, &workflows.GetWorkflowTemplateRequest{
				Name: "/workflow-template/chromium-telemetry-bisect-v1",
			})

			t.Run("Then we get a non-error response", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
			})

			t.Run("And we find that the defined template is retrieved", func(t *ftt.Test) {
				assert.Loosely(t, wt.Name, should.Equal("chromium-telemetry-bisect-v1"))
			})

		})

		t.Run("When we get a template that is not defined", func(t *ftt.Test) {
			wt, err := client.GetWorkflowTemplate(ctx, &workflows.GetWorkflowTemplateRequest{
				Name: "/workflow-template/nonexistent",
			})

			t.Run("Then we get a not-found error response", func(t *ftt.Test) {
				assert.Loosely(t, status.Code(err), should.Equal(codes.NotFound))
				assert.Loosely(t, wt, should.BeNil)
			})
		})
	})

	ftt.Run("Given a valid configuration with more templates", t, func(t *ftt.Test) {
		mockConfig(configMultipleTemplates)

		t.Run("When we attempt to validate the contents", func(t *ftt.Test) {
			resp, err := client.ValidateConfig(
				ctx, &configProto.ValidationRequestMessage{
					ConfigSet: "test-validation",
					Path:      "/test/validation/path",
					Content:   []byte(configMultipleTemplates),
				},
			)

			t.Run("Then we get a non-error response", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, resp.Messages, should.BeEmpty)
			})
		})

		t.Run("When we list the templates", func(t *ftt.Test) {
			resp, err := client.ListWorkflowTemplates(
				ctx, &workflows.ListWorkflowTemplatesRequest{
					PageSize: 10,
				},
			)

			t.Run("Then we find that the defined templates are in the list", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, resp.WorkflowTemplates, should.HaveLength(2))
			})

		})

		t.Run("When we get the templates by name", func(t *ftt.Test) {
			wt, err := client.GetWorkflowTemplate(ctx, &workflows.GetWorkflowTemplateRequest{
				Name: "/workflow-template/chromium-telemetry-ab-v1",
			})

			t.Run("Then we get a non-error response", func(t *ftt.Test) {
				assert.Loosely(t, err, should.BeNil)
			})

			t.Run("And we find that the defined templates are retrieved", func(t *ftt.Test) {
				assert.Loosely(t, wt.Name, should.Equal("chromium-telemetry-ab-v1"))
			})

		})

	})

}

func TestInvalidConfigurations(t *testing.T) {

	ftt.Run("Given a configuration with ill-formed text protobufs", t, func(t *ftt.Test) {

		t.Run("When we attempt to validate the contents", func(t *ftt.Test) {

			t.Run("Then we get a validation response with an ERROR severity", nil)

		})

	})

	ftt.Run("Given a configuration with missing input fields", t, func(t *ftt.Test) {

		t.Run("When we attempt to validate the contents", func(t *ftt.Test) {

			t.Run("Then we get a validation response with an ERROR severity", nil)

		})

	})

}
