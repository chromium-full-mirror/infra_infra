// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execution_test

// This file contains tests to ensure that testrunner builds are created with
// the right arguments.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.chromium.org/chromiumos/infra/proto/go/chromite/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/steps"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	trservice "go.chromium.org/infra/cmd/cros_test_platform/internal/execution/testrunner/service"
	"go.chromium.org/infra/libs/skylab/inventory"
)

func TestRequestArguments(t *testing.T) {
	ftt.Run("Given a server test with autotest labels", t, func(t *ftt.Test) {
		trClient := &trservice.ArgsCollectingClientWrapper{
			Client: trservice.StubClient{},
		}

		inv := serverTestInvocation("name1", "foo-arg1 foo-arg2")
		addAutotestDependency(inv, "cr50:pvt")
		addAutotestDependency(inv, "cleanup-reboot")
		inv.DisplayName = "given_name"
		invs := []*steps.EnumerationResponse_AutotestInvocation{inv}

		_, err := runWithDefaults(
			context.Background(),
			trClient,
			invs,
		)
		assert.Loosely(t, err, should.BeNil)

		t.Run("the launched task request should have correct parameters.", func(t *ftt.Test) {
			assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
			launchArgs := trClient.Calls.LaunchTask[0].Args

			assert.Loosely(t, launchArgs.SwarmingTags, should.Contain("parent_buildbucket_id:42"))
			assert.Loosely(t, launchArgs.SwarmingTags, should.Contain("luci_project:foo-luci-project"))
			assert.Loosely(t, launchArgs.SwarmingTags, should.Contain("foo-tag1"))
			assert.Loosely(t, launchArgs.SwarmingTags, should.Contain("foo-tag2"))
			assert.Loosely(t, launchArgs.ParentTaskID, should.Equal("foo-parent-task-id"))
			assert.Loosely(t, launchArgs.ParentRequestUID, should.Equal("TestPlanRuns/42/12345678/foo"))

			assert.Loosely(t, launchArgs.Priority, should.Equal(79))

			prefix := "log_location:"
			var logdogURL string
			matchingTags := 0
			for _, tag := range launchArgs.SwarmingTags {
				if strings.HasPrefix(tag, prefix) {
					matchingTags++
					assert.Loosely(t, tag, should.HaveSuffix("+/annotations"))

					logdogURL = strings.TrimPrefix(tag, "log_location:")
				}
			}
			assert.Loosely(t, matchingTags, should.Equal(1))
			assert.Loosely(t, logdogURL, should.HavePrefix("logdog://foo-logdog-host/foo-luci-project/skylab/"))
			assert.Loosely(t, logdogURL, should.HaveSuffix("/+/annotations"))

			assert.Loosely(t, launchArgs.Cmd.TaskName, should.Equal("name1"))
			assert.Loosely(t, launchArgs.Cmd.ClientTest, should.BeFalse)

			// Logdog annotation url argument should match the associated tag's url.
			assert.Loosely(t, launchArgs.Cmd.LogDogAnnotationURL, should.Equal(logdogURL))

			assert.Loosely(t, launchArgs.Cmd.TestArgs, should.Equal("foo-arg1 foo-arg2"))

			assert.Loosely(t, launchArgs.Cmd.Keyvals["k1"], should.Equal("v1"))
			assert.Loosely(t, launchArgs.Cmd.Keyvals["parent_job_id"], should.Equal("foo-parent-task-id"))
			assert.Loosely(t, launchArgs.Cmd.Keyvals["label"], should.Equal("given_name"))

			assert.Loosely(t, launchArgs.ProvisionableDimensions, should.HaveLength(3))
			assert.Loosely(t, launchArgs.ProvisionableDimensions, should.Contain("provisionable-cros-version:foo-build"))
			assert.Loosely(t, launchArgs.ProvisionableDimensions, should.Contain("provisionable-fwro-version:foo-ro-firmware"))
			assert.Loosely(t, launchArgs.ProvisionableDimensions, should.Contain("provisionable-fwrw-version:foo-rw-firmware"))

			assert.Loosely(t, launchArgs.ProvisionableDimensionExpiration, should.Equal(time.Minute))

			assert.Loosely(t, launchArgs.SchedulableLabels.GetModel(), should.Equal("foo-model"))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetBoard(), should.Equal("foo-board"))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetCriticalPools(), should.HaveLength(1))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetCriticalPools()[0], should.Equal(inventory.SchedulableLabels_DUT_POOL_CQ))

			assert.Loosely(t, launchArgs.Dimensions, should.HaveLength(4))
			assert.Loosely(t, launchArgs.Dimensions, should.Contain("freeform-key:freeform-value"))
			assert.Loosely(t, launchArgs.Dimensions, should.Contain("dut_state:ready"))
		})
	})
}

func TestInvocationKeyvals(t *testing.T) {
	ftt.Run("Given an enumeration with a suite keyval", t, func(t *ftt.Test) {
		trClient := &trservice.ArgsCollectingClientWrapper{
			Client: trservice.StubClient{},
		}
		invs := []*steps.EnumerationResponse_AutotestInvocation{
			{
				Test: &api.AutotestTest{
					Name:                 "someTest",
					ExecutionEnvironment: api.AutotestTest_EXECUTION_ENVIRONMENT_CLIENT,
				},
				ResultKeyvals: map[string]string{
					"suite": "someSuite",
				},
			},
		}

		t.Run("and a request without keyvals", func(t *ftt.Test) {
			p := basicParams()
			p.Decorations = nil
			_, err := runWithDefaults(context.Background(), trClient, invs)
			assert.Loosely(t, err, should.BeNil)

			t.Run("created command includes invocation suite keyval", func(t *ftt.Test) {
				assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
				launchArgs := trClient.Calls.LaunchTask[0].Args
				flatCommand := strings.Join(launchArgs.Cmd.Args(), " ")
				keyvals := extractKeyvalsArgument(t, flatCommand)
				assert.Loosely(t, keyvals, should.ContainSubstring(`"suite":"someSuite"`))
				assert.Loosely(t, keyvals, should.ContainSubstring(`"label":"foo-build/someSuite/someTest"`))
			})
		})

		t.Run("and a request with different suite keyvals", func(t *ftt.Test) {
			p := basicParams()
			p.Decorations = &test_platform.Request_Params_Decorations{
				AutotestKeyvals: map[string]string{
					"suite": "someOtherSuite",
				},
			}

			_, err := runWithParams(context.Background(), trClient, p, invs, "")
			assert.Loosely(t, err, should.BeNil)

			t.Run("created command includes request suite keyval", func(t *ftt.Test) {
				assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
				launchArgs := trClient.Calls.LaunchTask[0].Args
				flatCommand := strings.Join(launchArgs.Cmd.Args(), " ")
				keyvals := extractKeyvalsArgument(t, flatCommand)
				assert.Loosely(t, keyvals, should.ContainSubstring(`"suite":"someOtherSuite"`))
				assert.Loosely(t, keyvals, should.ContainSubstring(`"label":"foo-build/someOtherSuite/someTest"`))
			})
		})
	})
}

func TestKeyvalsAcrossTestRuns(t *testing.T) {
	ftt.Run("Given a request with a suite keyval", t, func(t *ftt.Test) {
		p := basicParams()
		p.Decorations = &test_platform.Request_Params_Decorations{
			AutotestKeyvals: map[string]string{
				"suite": "someSuite",
			},
		}

		t.Run("and two enumerations with different test names", func(t *ftt.Test) {

			invs := []*steps.EnumerationResponse_AutotestInvocation{
				{
					Test: &api.AutotestTest{
						Name:                 "firstTest",
						ExecutionEnvironment: api.AutotestTest_EXECUTION_ENVIRONMENT_CLIENT,
					},
				},
				{
					Test: &api.AutotestTest{
						Name:                 "secondTest",
						ExecutionEnvironment: api.AutotestTest_EXECUTION_ENVIRONMENT_CLIENT,
					},
				},
			}

			t.Run("created commands include common suite keyval and different label keyvals", func(t *ftt.Test) {
				trClient := &trservice.ArgsCollectingClientWrapper{
					Client: trservice.StubClient{},
				}
				_, err := runWithParams(context.Background(), trClient, p, invs, "")
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(2))
				cmd := make([]string, 2)
				for i, lt := range trClient.Calls.LaunchTask {
					cmd[i] = strings.Join(lt.Args.Cmd.Args(), " ")
				}
				kv0 := extractKeyvalsArgument(t, cmd[0])
				assert.Loosely(t, kv0, should.ContainSubstring(`"suite":"someSuite"`))
				assert.Loosely(t, kv0, should.ContainSubstring(`"label":"foo-build/someSuite/firstTest"`))
				kv1 := extractKeyvalsArgument(t, cmd[1])
				assert.Loosely(t, kv1, should.ContainSubstring(`"suite":"someSuite"`))
				assert.Loosely(t, kv1, should.ContainSubstring(`"label":"foo-build/someSuite/secondTest"`))
			})
		})
	})
}

func TestClientTestArg(t *testing.T) {
	ftt.Run("Given a client test", t, func(t *ftt.Test) {
		trClient := &trservice.ArgsCollectingClientWrapper{
			Client: trservice.StubClient{},
		}
		_, err := runWithDefaults(
			context.Background(),
			trClient,
			[]*steps.EnumerationResponse_AutotestInvocation{
				clientTestInvocation("name1", ""),
			},
		)
		assert.Loosely(t, err, should.BeNil)

		t.Run("the launched task request should have correct parameters.", func(t *ftt.Test) {
			assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
			assert.Loosely(t, trClient.Calls.LaunchTask[0].Args.Cmd.ClientTest, should.BeTrue)
		})
	})
}

func TestQuotaSchedulerAccountOnQSAccount(t *testing.T) {
	ftt.Run("Given a client test and a quota account", t, func(t *ftt.Test) {
		trClient := &trservice.ArgsCollectingClientWrapper{
			Client: trservice.StubClient{},
		}
		params := basicParams()
		params.Scheduling = &test_platform.Request_Params_Scheduling{
			Pool: &test_platform.Request_Params_Scheduling_UnmanagedPool{
				UnmanagedPool: "foo-pool",
			},
			QsAccount: "foo-account",
		}
		_, err := runWithParams(
			context.Background(),
			trClient,
			params,
			[]*steps.EnumerationResponse_AutotestInvocation{
				serverTestInvocation("name1", ""),
			},
			"",
		)
		assert.Loosely(t, err, should.BeNil)

		t.Run("the launched task request should have a tag specifying the correct quota account and run in foo-pool.", func(t *ftt.Test) {
			assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
			launchArgs := trClient.Calls.LaunchTask[0].Args
			assert.Loosely(t, launchArgs.SwarmingTags, should.Contain("qs_account:foo-account"))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetSelfServePools(), should.HaveLength(1))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetCriticalPools(), should.HaveLength(0))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetSelfServePools()[0], should.Equal("foo-pool"))
		})
	})
}

func TestReservedTagShouldNotBeSetByUsers(t *testing.T) {
	ftt.Run("Given a client test and a fake quota account set by user", t, func(t *ftt.Test) {
		trClient := &trservice.ArgsCollectingClientWrapper{
			Client: trservice.StubClient{},
		}
		params := basicParams()
		params.Scheduling = &test_platform.Request_Params_Scheduling{
			Pool: &test_platform.Request_Params_Scheduling_ManagedPool_{
				ManagedPool: test_platform.Request_Params_Scheduling_MANAGED_POOL_QUOTA,
			},
			QsAccount: "real-account",
		}
		params.Decorations = &test_platform.Request_Params_Decorations{
			Tags: []string{"qs_account:fake-account"},
		}

		_, err := runWithParams(
			context.Background(),
			trClient,
			params,
			[]*steps.EnumerationResponse_AutotestInvocation{
				serverTestInvocation("name1", ""),
			},
			"",
		)
		assert.Loosely(t, err, should.BeNil)

		t.Run("the launched task request should have a tag specifying the correct quota account and run in the quota pool.", func(t *ftt.Test) {
			assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
			launchArgs := trClient.Calls.LaunchTask[0].Args
			assert.Loosely(t, launchArgs.SwarmingTags, should.Contain("qs_account:real-account"))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetSelfServePools(), should.HaveLength(0))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetCriticalPools(), should.HaveLength(1))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetCriticalPools()[0], should.Equal(inventory.SchedulableLabels_DUT_POOL_QUOTA))
		})
	})
}

func TestUnmanagedPool(t *testing.T) {
	ftt.Run("Given a client test and an unmanaged pool.", t, func(t *ftt.Test) {
		trClient := &trservice.ArgsCollectingClientWrapper{
			Client: trservice.StubClient{},
		}
		params := basicParams()
		params.Scheduling.Pool = &test_platform.Request_Params_Scheduling_UnmanagedPool{
			UnmanagedPool: "foo-pool",
		}

		_, err := runWithParams(
			context.Background(),
			trClient,
			params,
			[]*steps.EnumerationResponse_AutotestInvocation{
				serverTestInvocation("name1", ""),
			},
			"",
		)
		assert.Loosely(t, err, should.BeNil)

		t.Run("the launched task request run in the unmanaged pool.", func(t *ftt.Test) {
			assert.Loosely(t, trClient.Calls.LaunchTask, should.HaveLength(1))
			launchArgs := trClient.Calls.LaunchTask[0].Args
			assert.Loosely(t, launchArgs.SchedulableLabels.GetCriticalPools(), should.HaveLength(0))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetSelfServePools(), should.HaveLength(1))
			assert.Loosely(t, launchArgs.SchedulableLabels.GetSelfServePools()[0], should.Equal("foo-pool"))
		})
	})
}

func addAutotestDependency(inv *steps.EnumerationResponse_AutotestInvocation, dep string) {
	inv.Test.Dependencies = append(inv.Test.Dependencies, &api.AutotestTaskDependency{Label: dep})
}

var keyvalsPattern = regexp.MustCompile(`\-keyvals\s*\{([\w\s":,-/]+)\}`)

func extractKeyvalsArgument(t testing.TB, cmd string) string {
	t.Helper()
	ms := keyvalsPattern.FindAllStringSubmatch(cmd, -1)
	assert.Loosely(t, ms, should.HaveLength(1), truth.LineContext())
	m := ms[0]
	// Guaranteed by the constant regexp definition.
	if len(m) != 2 {
		panic(fmt.Sprintf("Match %s of regexp %s has length %d, want 2", m, keyvalsPattern, len(m)))
	}
	return m[1]
}
