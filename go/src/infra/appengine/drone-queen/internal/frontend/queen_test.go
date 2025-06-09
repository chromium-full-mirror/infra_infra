// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/gae/service/datastore"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"go.chromium.org/infra/appengine/drone-queen/api"
	"go.chromium.org/infra/appengine/drone-queen/internal/clients"
	"go.chromium.org/infra/appengine/drone-queen/internal/entities"
)

func TestDroneQueenImpl_DeclareDuts(t *testing.T) {
	t.Parallel()
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "nelo"},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Group: k},
			{ID: "nelo", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare invalid DUTs", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "nelo"},
			{Name: ""},
			{Name: ""},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Group: k},
			{ID: "nelo", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare more DUTs", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "nelo"},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		availableDuts = []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "nelo"},
			{Name: "casty"},
		}
		_, err = d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Group: k},
			{ID: "nelo", Group: k},
			{ID: "casty", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare fewer DUTs", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "nelo"},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		availableDuts = []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
		}
		_, err = d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Group: k},
			{ID: "nelo", Group: k, Draining: true},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare new and remove old DUTs", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "nelo"},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		availableDuts = []*api.DeclareDutsRequest_Dut{
			{Name: "ion"},
			{Name: "casty"},
		}
		_, err = d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Group: k},
			{ID: "nelo", Group: k, Draining: true},
			{ID: "casty", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare new DUTs with hive", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion", Hive: "hive-A"},
			{Name: "nelo", Hive: "hive-B"},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Hive: "hive-A", Group: k},
			{ID: "nelo", Hive: "hive-B", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare DUTs with updated hive", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion", Hive: "hive-A"},
			{Name: "nelo", Hive: "hive-B"},
		}
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		availableDuts = []*api.DeclareDutsRequest_Dut{
			{Name: "ion", Hive: "hive-C"},
			{Name: "nelo", Hive: "hive-B"},
		}
		_, err = d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "ion", Hive: "hive-C", Group: k},
			{ID: "nelo", Hive: "hive-B", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("declare DUTs running on cloudbots", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		availableDuts := []*api.DeclareDutsRequest_Dut{
			{Name: "ion", Hive: "hive-A"},
			{Name: "test", Hive: "hive-A"},
			{Name: "nelo", Hive: "hive-B"},
			{Name: "test2", Hive: "hive-C"},
		}
		ctx = mockSwarmingListBots(ctx, t)
		_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
		if err != nil {
			t.Fatal(err)
		}
		k := entities.DUTGroupKey(ctx)
		want := []*entities.DUT{
			{ID: "test", Hive: "hive-A", Group: k},
			{ID: "nelo", Hive: "hive-B", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)

	})

}

func TestDroneQueenImpl_ReleaseDuts(t *testing.T) {
	t.Parallel()
	t.Run("release all DUTs", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		k := entities.DUTGroupKey(ctx)
		duts := []*entities.DUT{
			{ID: "ionasal", Group: k, AssignedDrone: "earthes"},
			{ID: "nayaflask", Group: k, AssignedDrone: "earthes"},
		}
		if err := datastore.Put(ctx, duts); err != nil {
			t.Fatal(err)
		}
		var d DroneQueenImpl
		_, err := d.ReleaseDuts(ctx, &api.ReleaseDutsRequest{
			DroneUuid: "earthes",
			Duts:      []string{"ionasal", "nayaflask"},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := []*entities.DUT{
			{ID: "ionasal", Group: k},
			{ID: "nayaflask", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("release partial DUTs", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		k := entities.DUTGroupKey(ctx)
		duts := []*entities.DUT{
			{ID: "ionasal", Group: k, AssignedDrone: "earthes"},
			{ID: "nayaflask", Group: k, AssignedDrone: "earthes"},
		}
		if err := datastore.Put(ctx, duts); err != nil {
			t.Fatal(err)
		}
		var d DroneQueenImpl
		_, err := d.ReleaseDuts(ctx, &api.ReleaseDutsRequest{
			DroneUuid: "earthes",
			Duts:      []string{"nayaflask"},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := []*entities.DUT{
			{ID: "ionasal", Group: k, AssignedDrone: "earthes"},
			{ID: "nayaflask", Group: k},
		}
		assertDatastoreDUTs(ctx, t, want)
	})
	t.Run("release unassigned duts", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		k := entities.DUTGroupKey(ctx)
		duts := []*entities.DUT{
			{ID: "ionasal", Group: k},
		}
		if err := datastore.Put(ctx, duts); err != nil {
			t.Fatal(err)
		}
		var d DroneQueenImpl
		_, err := d.ReleaseDuts(ctx, &api.ReleaseDutsRequest{
			DroneUuid: "earthes",
			Duts:      []string{"ionasal"},
		})
		if err != nil {
			t.Fatal(err)
		}
		assertDatastoreDUTs(ctx, t, duts)
	})
	t.Run("release DUTs of another drone", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		k := entities.DUTGroupKey(ctx)
		duts := []*entities.DUT{
			{ID: "casty", Group: k, AssignedDrone: "delta"},
		}
		if err := datastore.Put(ctx, duts); err != nil {
			t.Fatal(err)
		}
		var d DroneQueenImpl
		_, err := d.ReleaseDuts(ctx, &api.ReleaseDutsRequest{
			DroneUuid: "earthes",
			Duts:      []string{"casty"},
		})
		if err != nil {
			t.Fatal(err)
		}
		assertDatastoreDUTs(ctx, t, duts)
	})
	t.Run("release nonexistent DUT", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		k := entities.DUTGroupKey(ctx)
		duts := []*entities.DUT{
			{ID: "casty", Group: k, AssignedDrone: "delta"},
		}
		if err := datastore.Put(ctx, duts); err != nil {
			t.Fatal(err)
		}
		var d DroneQueenImpl
		_, err := d.ReleaseDuts(ctx, &api.ReleaseDutsRequest{
			DroneUuid: "earthes",
			Duts:      []string{"nelo"},
		})
		if err != nil {
			t.Fatal(err)
		}
		assertDatastoreDUTs(ctx, t, duts)
	})
	t.Run("omit drone UUID", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		var d DroneQueenImpl
		_, err := d.ReleaseDuts(ctx, &api.ReleaseDutsRequest{})
		if err == nil {
			t.Errorf("Expected error, got no error")
		}
	})
}

func TestDroneQueenImpl_ReportDrone(t *testing.T) {
	t.Run("unknown UUID", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		d := DroneQueenImpl{}
		res, err := d.ReportDrone(ctx, &api.ReportDroneRequest{
			DroneUuid:        "unicorn",
			DroneDescription: "unicorn",
		})
		if err != nil {
			t.Fatal(err)
		}
		if s := res.Status; s != api.ReportDroneResponse_UNKNOWN_UUID {
			t.Errorf("Got report status %v; want UNKNOWN_UUID", s)
		}
	})
	t.Run("expired drone", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		now := time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC)
		d := DroneQueenImpl{
			nowFunc: staticTime(now),
		}
		dr := entities.Drone{
			ID:          "nelo",
			Expiration:  now.Add(-10 * time.Second),
			Description: "nelo",
		}
		if err := datastore.Put(ctx, &dr); err != nil {
			t.Fatal(err)
		}
		res, err := d.ReportDrone(ctx, &api.ReportDroneRequest{
			DroneUuid:        "nelo",
			DroneDescription: "nelo",
		})
		if err != nil {
			t.Fatal(err)
		}
		if s := res.Status; s != api.ReportDroneResponse_UNKNOWN_UUID {
			t.Errorf("Got report status %v; want UNKNOWN_UUID", s)
		}
	})
}

func TestGetVersionFromContext(t *testing.T) {
	t.Parallel()
	t.Run("No metadata", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		if version := getVersionFromContext(ctx); version != "unknown" {
			t.Errorf("Got %v; want unknown", version)
		}
	})
	t.Run("No drone agent version", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		md := metadata.Pairs("something-unrelated", "12345")
		ctx = metadata.NewIncomingContext(ctx, md)
		if version := getVersionFromContext(ctx); version != "unknown" {
			t.Errorf("Got %v; want unknown", version)
		}
	})
	t.Run("Drone agent in metadata", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		md := metadata.Pairs("drone-agent-version", "12345")
		ctx = metadata.NewIncomingContext(ctx, md)
		if version := getVersionFromContext(ctx); version != "12345" {
			t.Errorf("Got %v; want 12345", version)
		}
	})
}

func TestIsVersionSupported2(t *testing.T) {
	t.Parallel()
	const threshold = 3000
	ctx := gaetesting.TestingContextWithAppID("go-test")
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty", input: "", want: true},
		{name: "unknown", input: "unknown", want: true},
		{name: "supported", input: "4000", want: true},
		{name: "equal (supported)", input: "3000", want: true},
		{name: "unsupported", input: "2000", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if supported := isVersionSupported2(ctx, c.input, threshold); supported != c.want {
				t.Errorf("Got %v; want %v", supported, c.want)
			}
		})
	}
}

func TestDroneQueenImpl_workflows(t *testing.T) {
	t.Parallel()
	t.Run("happy path", testHappyPath)
}

func testHappyPath(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	now := time.Date(2000, 1, 2, 3, 4, 5, 6, time.UTC)
	d := DroneQueenImpl{
		nowFunc: staticTime(now),
	}
	// Declare some DUTs.
	availableDuts := []*api.DeclareDutsRequest_Dut{
		{Name: "ion", Hive: "hive-A"},
		{Name: "casty", Hive: "hive-A"},
		{Name: "nelo", Hive: "hive-B"},
	}
	_, err := d.DeclareDuts(ctx, &api.DeclareDutsRequest{AvailableDuts: availableDuts})
	if err != nil {
		t.Fatal(err)
	}
	k := entities.DUTGroupKey(ctx)
	want := []*entities.DUT{
		{ID: "ion", Hive: "hive-A", Group: k},
		{ID: "casty", Hive: "hive-A", Group: k},
		{ID: "nelo", Hive: "hive-B", Group: k},
	}
	assertDatastoreDUTs(ctx, t, want)
	// Call ReportDrone.
	res, err := d.ReportDrone(ctx, &api.ReportDroneRequest{
		LoadIndicators: &api.ReportDroneRequest_LoadIndicators{
			DutCapacity: 2,
		},
		Hive: "hive-A",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s := res.Status; s != api.ReportDroneResponse_OK {
		t.Errorf("Got report status %v; want OK", s)
	}
	if res.DroneUuid == "" {
		t.Errorf("Got empty drone UUID; expected a new UUID to be assigned")
	}
	if n := len(res.AssignedDuts); n != 2 {
		t.Errorf("Got %v DUTs; expected 2", n)
	}
	assertSameStrings(t, []string{"ion", "casty"}, res.AssignedDuts)
	if len(res.DrainingDuts) != 0 {
		t.Errorf("Got draining DUTs %v; want none", res.DrainingDuts)
	}
	if e := goTime(res.ExpirationTime); !e.After(now) {
		t.Errorf("Got expiration time %v; expected time after %v", e, now)
	}
}

func TestDroneQueenImpl_listCloudbots(t *testing.T) {
	t.Parallel()

	t.Run("listCloudbots", func(t *testing.T) {
		t.Parallel()
		ctx := gaetesting.TestingContextWithAppID("go-test")
		ctx = mockSwarmingListBots(ctx, t)
		bots, _ := listCloudbots(ctx)
		if len(bots) != len(fakeCloudBotsSwarmingBots.Items) {
			t.Errorf("Got %v; want %v", len(bots), len(fakeCloudBotsSwarmingBots.Items))
		}

	})
}

// goTime converts a protobuf timestamp to a Go Time.
func goTime(t *timestamppb.Timestamp) time.Time {
	if err := t.CheckValid(); err != nil {
		panic(err)
	}
	return t.AsTime()
}

// staticTime returns a nowFunc for DroneQueenImpl for a static time.
func staticTime(t time.Time) func() time.Time {
	return func() time.Time {
		return t
	}
}

func assertSameStrings(t *testing.T, want, got []string) {
	t.Helper()
	sort.Strings(want)
	sort.Strings(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected strings (-want +got):\n%s", diff)
	}
}

func assertDatastoreDUTs(ctx context.Context, t *testing.T, want []*entities.DUT) {
	t.Helper()
	q := datastore.NewQuery(entities.DUTKind)
	var got []*entities.DUT
	if err := datastore.GetAll(ctx, q, &got); err != nil {
		t.Fatal(err)
	}
	sortDUTs(got)
	sortDUTs(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected DUTs (-want +got):\n%s", diff)
	}
}

func sortDUTs(d []*entities.DUT) {
	sort.Slice(d, func(i, j int) bool { return d[i].ID < d[j].ID })
}

var fakeCloudBotsSwarmingBots = &apipb.BotInfoListResponse{
	Cursor: "",
	Items: []*apipb.BotInfo{
		{
			Dimensions: []*apipb.StringListPair{
				{
					Key:   "dut_name",
					Value: []string{"ion"},
				},
			},
		},
		{
			Dimensions: []*apipb.StringListPair{
				{
					Key:   "dut_name",
					Value: []string{"test2"},
				},
			},
		},
	},
}

func mockSwarmingListBots(ctx context.Context, t *testing.T) context.Context {
	mc := gomock.NewController(t)
	msc := clients.NewMockSwarmingClient(mc)
	ctx = context.WithValue(ctx, &clients.MockSwarmingClientKey, msc)
	msc.EXPECT().ListBots(gomock.Any(), gomock.Any()).Return(fakeCloudBotsSwarmingBots, nil)
	return ctx
}
