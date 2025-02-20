// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/tq"
	"go.chromium.org/infra/cros/recovery/logger/metrics"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// TestPushBotsForAdminTasksImplSmokeTesttests that pushing bots for admin tasks
// calls the ListALiveIdleBotsInPool API.
func TestPushBotsForAdminTasksImplSmokeTest(t *testing.T) {
	tf, validate := newTestFixture(&ftt.Test{T: t})
	defer validate()
	ctx := tf.C
	tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), gomock.Any(), gomock.Any())
	tf.MockKarte.EXPECT().Search(gomock.Any(), gomock.Any()).Return(
		&metrics.QueryResult{
			Actions:   nil,
			PageToken: "",
		},
		nil,
	)
	ctx = config.Use(ctx, &config.Config{
		Swarming: &config.Swarming{
			BotPool: "fake-unused-pool",
			PoolCfgs: []*config.Swarming_PoolCfg{
				{
					PoolName: "fake-bot-pool",
				},
			},
		},
	})
	req := &fleet.PushBotsForAdminTasksRequest{
		TargetDutState: fleet.DutState_NeedsRepair,
	}
	_, err := (&adminTaskBotPusher{
		swarmingClient: tf.MockSwarming,
		metricsClient:  tf.MockKarte,
	}).pushBotsForAdminTasksImpl(ctx, req)

	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestPushBotsForAdminTasksWithUFSClient tests that pushing bots for admin tasks with a UFS client succeeds.
func TestPushBotsForAdminTasksWithUFSClient(t *testing.T) {
	tf, validate := newTestFixture(&ftt.Test{T: t})
	defer validate()
	ctx := tf.C
	tq.GetTestable(ctx).CreateQueue("repair-bots")
	tf.MockUFS.EXPECT().GetDUTsForLabstation(gomock.Any(), gomock.Any())
	tf.MockKarte.EXPECT().Search(gomock.Any(), gomock.Any()).Return(
		&metrics.QueryResult{
			Actions:   nil,
			PageToken: "",
		},
		nil,
	)
	tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*swarmingv2.BotInfo{
		{
			BotId: "fake-bot-a",
			Dimensions: []*swarmingv2.StringListPair{
				{
					Key:   "id",
					Value: []string{"fake-bot-a"},
				},
				{
					Key:   "pool",
					Value: []string{"fake-bot-pool"},
				},
				{
					Key:   "dut_state",
					Value: []string{"needs_repair"},
				},
				{
					Key:   "label-os_type",
					Value: []string{"OS_TYPE_MOBLAB"},
				},
			},
		},
		{
			BotId: "fake-bot-b",
			Dimensions: []*swarmingv2.StringListPair{
				{
					Key:   "id",
					Value: []string{"fake-bot-b"},
				},
				{
					Key:   "pool",
					Value: []string{"fake-bot-pool"},
				},
				{
					Key:   "dut_state",
					Value: []string{"needs_repair"},
				},
				{
					Key:   "label-os_type",
					Value: []string{"OS_TYPE_MOBLAB"},
				},
			},
		},
	}, nil)
	ctx = config.Use(ctx, &config.Config{
		Swarming: &config.Swarming{
			BotPool: "fake-unused-pool",
			PoolCfgs: []*config.Swarming_PoolCfg{
				{
					PoolName: "fake-bot-pool",
				},
			},
		},
	})
	req := &fleet.PushBotsForAdminTasksRequest{
		TargetDutState: fleet.DutState_NeedsRepair,
	}
	_, err := (&adminTaskBotPusher{
		swarmingClient: tf.MockSwarming,
		ufsClient:      tf.MockUFS,
		metricsClient:  tf.MockKarte,
	}).pushBotsForAdminTasksImpl(ctx, req)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	numTasks := len(tq.GetTestable(ctx).GetScheduledTasks()["repair-bots"])
	if numTasks != 2 {
		t.Errorf("unexpected number of tasks %d", numTasks)
	}
}

// TestGetDUTsForLabstations tests that getDUTsForLabstations returns the correct list of bot names given that the GetDUTsForLabstation RPC is functioning correctly.
func TestGetDUTsForLabstations(t *testing.T) {
	tf, validate := newTestFixture(&ftt.Test{T: t})
	defer validate()
	ctx := tf.C
	// Make the UFS call successfully return exactly one fake DUT.
	tf.MockUFS.EXPECT().GetDUTsForLabstation(gomock.Any(), gomock.Any(), gomock.Any()).Return(
		&ufsAPI.GetDUTsForLabstationResponse{
			Items: []*ufsAPI.GetDUTsForLabstationResponse_LabstationMapping{
				{
					Hostname: "fake-labstation-1",
					DutName:  []string{"fake-dut-1"},
				},
			},
		},
		nil,
	)
	duts, err := (&adminTaskBotPusher{ufsClient: tf.MockUFS}).getDUTsForLabstations(ctx, nil)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if diff := cmp.Diff([]string{"fake-dut-1"}, duts); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestGetLabstations tests that we return the hostname of every labstation in the records returned by
// metrics.Search. For this test, we assume that the output of the Karte API is correct.
// We have Search return two records describing the same labstation to test the deduplication logic.
func TestGetLabstations(t *testing.T) {
	var zero time.Time
	tf, validate := newTestFixture(&ftt.Test{T: t})
	defer validate()
	ctx := tf.C
	tf.MockKarte.EXPECT().Search(gomock.Any(), gomock.Any()).Return(
		&metrics.QueryResult{
			Actions: []*metrics.Action{
				{
					Hostname:   "fake-labstation-1",
					ActionKind: labstationRebootKind,
					Status:     metrics.ActionStatusSuccess,
				},
				{
					Hostname:   "fake-labstation-1",
					ActionKind: labstationRebootKind,
					Status:     metrics.ActionStatusSuccess,
				},
			},
		},
		nil,
	)
	labstations, err := (&adminTaskBotPusher{metricsClient: tf.MockKarte}).getLabstations(ctx, zero, zero)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if diff := cmp.Diff([]string{"fake-labstation-1"}, labstations); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestPushBotsForAdminTasksWithPoolCfg tests that pushing bots for admin tasks with Pool
func TestPushBotsForAdminTasksWithPoolCfg(t *testing.T) {
	ftt.Run("Handling PoolCfg bots", t, func(t *ftt.Test) {
		tf, validate := newTestFixture(t)
		defer validate()
		ctx := tf.C
		tqt := tq.GetTestable(ctx)
		qn := "repair-bots"
		tqt.CreateQueue(qn)
		tf.MockUFS.EXPECT().GetDUTsForLabstation(gomock.Any(), gomock.Any())
		tf.MockKarte.EXPECT().Search(gomock.Any(), gomock.Any()).Return(
			&metrics.QueryResult{
				Actions:   nil,
				PageToken: "",
			},
			nil,
		)
		tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), "fake-bot-pool", gomock.Any()).Return([]*swarmingv2.BotInfo{
			{
				BotId: "fake-bot-a",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"fake-bot-a"},
					},
					{
						Key:   "pool",
						Value: []string{"fake-bot-pool"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
			{
				BotId: "fake-bot-b",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"fake-bot-b"},
					},
					{
						Key:   "pool",
						Value: []string{"fake-bot-pool"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
		}, nil)
		tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), "pool-cfg-a", gomock.Any()).Return([]*swarmingv2.BotInfo{
			{
				BotId: "pool-cfg-bot-a",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"pool-cfg-bot-a"},
					},
					{
						Key:   "pool",
						Value: []string{"pool-cfg-a"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
		}, nil)
		tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), "pool-cfg-b", gomock.Any()).Return([]*swarmingv2.BotInfo{
			{
				BotId: "pool-cfg-bot-b",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"pool-cfg-bot-b"},
					},
					{
						Key:   "pool",
						Value: []string{"pool-cfg-b"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
		}, nil)

		ctx = config.Use(ctx, &config.Config{
			Swarming: &config.Swarming{
				BotPool: "fake-unused-pool",
				PoolCfgs: []*config.Swarming_PoolCfg{
					{
						PoolName: "fake-bot-pool",
					},
					{
						PoolName: "pool-cfg-a",
					},
					{
						PoolName:      "pool-cfg-b",
						BuilderBucket: "some_bucket",
					},
				},
			},
		})

		req := &fleet.PushBotsForAdminTasksRequest{
			TargetDutState: fleet.DutState_NeedsRepair,
		}
		_, err := (&adminTaskBotPusher{swarmingClient: tf.MockSwarming, ufsClient: tf.MockUFS, metricsClient: tf.MockKarte}).pushBotsForAdminTasksImpl(ctx, req)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		tasks := tqt.GetScheduledTasks()[qn]
		fmt.Println(tasks)
		numTasks := len(tasks)
		assert.Loosely(t, numTasks, should.Equal(4))
		var taskPaths, taskParams []string
		for _, v := range tasks {
			taskPaths = append(taskPaths, v.Path)
			taskParams = append(taskParams, string(v.Payload))
		}
		sort.Strings(taskPaths)
		sort.Strings(taskParams)
		expectedPaths := []string{"/internal/task/cros_repair/fake-bot-a", "/internal/task/cros_repair/fake-bot-b", "/internal/task/cros_repair/pool-cfg-bot-a", "/internal/task/cros_repair/pool-cfg-bot-b"}
		expectedParams := []string{"botID=fake-bot-a&expectedState=needs_repair&swarmingPool=fake-bot-pool", "botID=fake-bot-b&expectedState=needs_repair&swarmingPool=fake-bot-pool", "botID=pool-cfg-bot-a&expectedState=needs_repair&swarmingPool=pool-cfg-a", "botID=pool-cfg-bot-b&expectedState=needs_repair&swarmingPool=pool-cfg-b"}
		assert.Loosely(t, taskPaths, should.Match(expectedPaths))
		assert.Loosely(t, taskParams, should.Match(expectedParams))
	})
}

// TestPushBotsForAdminTasksWithPoolCfgSkipError tests that error condition while pushing bots for admin tasks with Pool
func TestPushBotsForAdminTasksWithPoolCfgSkipError(t *testing.T) {
	ftt.Run("Handling Errors for PoolCfg bots ", t, func(t *ftt.Test) {
		tf, validate := newTestFixture(t)
		defer validate()
		ctx := tf.C
		tqt := tq.GetTestable(ctx)
		qn := "repair-bots"
		tqt.CreateQueue(qn)
		tf.MockUFS.EXPECT().GetDUTsForLabstation(gomock.Any(), gomock.Any())
		tf.MockKarte.EXPECT().Search(gomock.Any(), gomock.Any()).Return(
			&metrics.QueryResult{
				Actions:   nil,
				PageToken: "",
			},
			nil,
		)
		tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), "fake-bot-pool", gomock.Any()).Return([]*swarmingv2.BotInfo{
			{
				BotId: "fake-bot-a",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"fake-bot-a"},
					},
					{
						Key:   "pool",
						Value: []string{"fake-bot-pool"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
			{
				BotId: "fake-bot-b",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"fake-bot-b"},
					},
					{
						Key:   "pool",
						Value: []string{"fake-bot-pool"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
		}, nil)

		tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), "pool-cfg-a", gomock.Any()).Return(nil, errors.Reason("Fake Error").Err())

		tf.MockSwarming.EXPECT().ListAliveIdleBotsInPool(gomock.Any(), "pool-cfg-b", gomock.Any()).Return([]*swarmingv2.BotInfo{
			{
				BotId: "pool-cfg-bot-b",
				Dimensions: []*swarmingv2.StringListPair{
					{
						Key:   "id",
						Value: []string{"pool-cfg-bot-b"},
					},
					{
						Key:   "pool",
						Value: []string{"pool-cfg-b"},
					},
					{
						Key:   "dut_state",
						Value: []string{"needs_repair"},
					},
					{
						Key:   "label-os_type",
						Value: []string{"OS_TYPE_MOBLAB"},
					},
				},
			},
		}, nil)

		ctx = config.Use(ctx, &config.Config{
			Swarming: &config.Swarming{
				BotPool: "fake-unused-pool",
				PoolCfgs: []*config.Swarming_PoolCfg{
					{
						PoolName: "fake-bot-pool",
					},
					{
						PoolName: "pool-cfg-a",
					},
					{
						PoolName:      "pool-cfg-b",
						BuilderBucket: "some_bucket",
					},
				},
			},
		})

		req := &fleet.PushBotsForAdminTasksRequest{
			TargetDutState: fleet.DutState_NeedsRepair,
		}
		_, err := (&adminTaskBotPusher{
			swarmingClient: tf.MockSwarming,
			ufsClient:      tf.MockUFS,
			metricsClient:  tf.MockKarte,
		}).pushBotsForAdminTasksImpl(ctx, req)
		if err != nil {
			if !strings.Contains(err.Error(), "Fake Error") {
				t.Errorf("unexpected error: %s", err)
			}
		}
		tasks := tqt.GetScheduledTasks()[qn]
		fmt.Println(tasks)
		numTasks := len(tasks)
		assert.Loosely(t, numTasks, should.Equal(3))
		var taskPaths, taskParams []string
		for _, v := range tasks {
			taskPaths = append(taskPaths, v.Path)
			taskParams = append(taskParams, string(v.Payload))
		}
		sort.Strings(taskPaths)
		sort.Strings(taskParams)
		expectedPaths := []string{"/internal/task/cros_repair/fake-bot-a", "/internal/task/cros_repair/fake-bot-b", "/internal/task/cros_repair/pool-cfg-bot-b"}
		expectedParams := []string{"botID=fake-bot-a&expectedState=needs_repair&swarmingPool=fake-bot-pool", "botID=fake-bot-b&expectedState=needs_repair&swarmingPool=fake-bot-pool", "botID=pool-cfg-bot-b&expectedState=needs_repair&swarmingPool=pool-cfg-b"}
		assert.Loosely(t, taskPaths, should.Match(expectedPaths))
		assert.Loosely(t, taskParams, should.Match(expectedParams))
	})
}
