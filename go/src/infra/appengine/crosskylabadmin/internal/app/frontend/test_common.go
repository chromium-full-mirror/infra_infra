// Copyright 2018 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/gae/service/datastore"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"

	fleet "go.chromium.org/infra/appengine/crosskylabadmin/api/fleet/v1"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/clients"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/clients/mock"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/app/config"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/tq"
	"go.chromium.org/infra/appengine/crosskylabadmin/internal/ufs/mockufs"
	"go.chromium.org/infra/cros/recovery/logger/metrics/mockmetrics"
)

type testFixture struct {
	T testing.TB
	C context.Context

	Tracker   fleet.TrackerServer
	Inventory *ServerImpl

	MockSwarming       *mock.MockSwarmingClient
	MockBotTasksCursor *mock.MockBotTasksCursor
	MockUFS            *mockufs.MockClient
	MockKarte          *mockmetrics.MockMetrics
}

// newTextFixture creates a new testFixture to be used in unittests.
//
// The function returns the created testFixture and a validation function that
// must be deferred by the caller.
func newTestFixture(t *ftt.Test) (testFixture, func()) {
	return newTestFixtureWithContext(testingContext(), t)
}

func newTestFixtureWithContext(c context.Context, t testing.TB) (testFixture, func()) {
	// Configure the tq implementation: confirm that it's testable and set up queues used by CrOSSkylabAdmin.
	if tq.GetTestable(c) == nil {
		panic("internal error in app/frontend/test_common.go: in unit tests, taskqueue must be a testable implementation")
	}

	tf := testFixture{T: t, C: c}

	var testingT *testing.T
	switch v := t.(type) {
	case *testing.T:
		testingT = v
	case *ftt.Test:
		testingT = v.T
	default:
		panic(fmt.Sprintf("invalid type %T for t", t))
	}

	mc := gomock.NewController(testingT)

	tf.MockSwarming = mock.NewMockSwarmingClient(mc)
	tf.MockBotTasksCursor = mock.NewMockBotTasksCursor(mc)
	tf.Inventory = &ServerImpl{}
	tf.MockUFS = mockufs.NewMockClient(mc)
	tf.MockKarte = mockmetrics.NewMockMetrics(mc)
	tf.Tracker = &TrackerServerImpl{
		SwarmingFactory: func(context.Context, string) (clients.SwarmingClient, error) {
			return tf.MockSwarming, nil
		},
		MetricsClient: tf.MockKarte,
	}

	validate := func() {
		mc.Finish()
	}
	return tf, validate
}

// TestingContext returns a context suitable for unit tests.
func testingContext() context.Context {
	c := gaetesting.TestingContextWithAppID("dev~infra-crosskylabadmin")
	c = config.Use(c, &config.Config{
		AccessGroup: "fake-access-group",
		Swarming: &config.Swarming{
			Host:              "https://fake-host.appspot.com",
			BotPool:           "ChromeOSSkylab",
			FleetAdminTaskTag: "fake-tag",
			LuciProjectTag:    "fake-project",
			PoolCfgs: []*config.Swarming_PoolCfg{
				{
					PoolName:     "ChromeOSSkylab",
					AuditEnabled: true,
				},
			},
		},
		StableVersionConfig: &config.StableVersionConfig{
			GerritHost:            "xxx-fake-gerrit-review.googlesource.com",
			GitilesHost:           "xxx-gitiles.googlesource.com",
			Project:               "xxx-project",
			Branch:                "xxx-branch",
			StableVersionDataPath: "xxx-stable_version_data_path",
		},
	})
	datastore.GetTestable(c).Consistent(true)
	c = gologger.StdConfig.Use(c)
	c = logging.SetLevel(c, logging.Debug)
	return c
}

// expectDefaultPerBotRefresh sets up the default expectations for refreshing
// each bot, once the list of bots is known.
//
// This is useful for tests that only target the initial Swarming bot listing
// logic.
func expectDefaultPerBotRefresh(tf testFixture) {
	tf.MockSwarming.EXPECT().ListSortedRecentTasksForBot(
		gomock.Any(), gomock.Any(), gomock.Any(),
	).AnyTimes().Return([]*swarmingv2.TaskResultResponse{}, nil)
	tf.MockSwarming.EXPECT().ListBotTasks(gomock.Any()).AnyTimes().Return(
		tf.MockBotTasksCursor)
	tf.MockBotTasksCursor.EXPECT().Next(gomock.Any(), gomock.Any()).AnyTimes().Return(
		[]*swarmingv2.TaskResultResponse{}, nil)
}

// BotForDUT returns BotInfos for DUTs with the given dut id.
//
// state is the bot's state dimension.
// dims is a convenient way to specify other bot dimensions.
// "a:x,y;b:z" will set the dimensions of the bot to ["a": ["x", "y"], "b":
//
//	["z"]]
func BotForDUT(id string, state string, dims string) *swarmingv2.BotInfo {
	sdims := make([]*swarmingv2.StringListPair, 0, 2)
	if dims != "" {
		ds := strings.Split(dims, ";")
		for _, d := range ds {
			d = strings.Trim(d, " ")
			kvs := strings.Split(d, ":")
			if len(kvs) != 2 {
				panic(fmt.Sprintf("dims string |%s|%s has a non-keyval dimension |%s|", dims, ds, d))
			}
			sdim := &swarmingv2.StringListPair{
				Key:   strings.Trim(kvs[0], " "),
				Value: []string{},
			}
			for _, v := range strings.Split(kvs[1], ",") {
				sdim.Value = append(sdim.Value, strings.Trim(v, " "))
			}
			sdims = append(sdims, sdim)
		}
	}
	sdims = append(sdims, &swarmingv2.StringListPair{
		Key:   "dut_state",
		Value: []string{state},
	})
	sdims = append(sdims, &swarmingv2.StringListPair{
		Key:   "dut_id",
		Value: []string{id},
	})
	sdims = append(sdims, &swarmingv2.StringListPair{
		Key:   "dut_name",
		Value: []string{id + "-host"},
	})
	return &swarmingv2.BotInfo{
		BotId:      fmt.Sprintf("bot_%s", id),
		Dimensions: sdims,
	}
}
