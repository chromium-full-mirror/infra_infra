// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockHwidData() *ufspb.HwidData {
	return &ufspb.HwidData{
		Sku:     "test-sku",
		Variant: "test-variant",
		Hwid:    "test",
		DutLabel: &ufspb.DutLabel{
			PossibleLabels: []string{
				"test-possible-1",
				"test-possible-2",
			},
			Labels: []*ufspb.DutLabel_Label{
				{
					Name:  "test-label-1",
					Value: "test-value-1",
				},
				{
					Name:  "Sku",
					Value: "test-sku",
				},
				{
					Name:  "variant",
					Value: "test-variant",
				},
				{
					Name:  "hwid_component",
					Value: "test_component/test_component_value",
				},
			},
		},
	}
}

func mockHwidDataNoServer() *ufspb.HwidData {
	return &ufspb.HwidData{
		Sku:     "test-sku-no-server",
		Variant: "test-variant-no-server",
		Hwid:    "test-no-server",
		DutLabel: &ufspb.DutLabel{
			PossibleLabels: []string{
				"test-possible-1",
				"test-possible-2",
			},
			Labels: []*ufspb.DutLabel_Label{
				{
					Name:  "test-label-1",
					Value: "test-value-1",
				},
				{
					Name:  "Sku",
					Value: "test-sku-no-server",
				},
				{
					Name:  "variant",
					Value: "test-variant-no-server",
				},
			},
		},
	}
}

func mockDutLabel() *ufspb.DutLabel {
	return &ufspb.DutLabel{
		PossibleLabels: []string{
			"test-possible-1",
			"test-possible-2",
		},
		Labels: []*ufspb.DutLabel_Label{
			{
				Name:  "test-label-1",
				Value: "test-value-1",
			},
			{
				Name:  "Sku",
				Value: "test-legacy-sku",
			},
			{
				Name:  "variant",
				Value: "test-legacy-variant",
			},
		},
	}
}

// fakeUpdateHwidData updates HwidDataEntity with HwidData in datastore.
func fakeUpdateHwidData(ctx context.Context, d *ufspb.HwidData, hwid string, updatedTime time.Time) (*configuration.HwidDataEntity, error) {
	hwidData, err := proto.Marshal(d)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal HwidData %s", d).Err()
	}

	if hwid == "" {
		return nil, status.Errorf(codes.Internal, "Empty hwid")
	}

	entity := &configuration.HwidDataEntity{
		ID:       hwid,
		HwidData: hwidData,
		Updated:  updatedTime,
	}

	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// fakeUpdateLegacyHwidData updates HwidDataEntity with DutLabel as HwidData instead of
// HwidData proto in datastore.
func fakeUpdateLegacyHwidData(ctx context.Context, d *ufspb.DutLabel, hwid string, updatedTime time.Time) (*configuration.HwidDataEntity, error) {
	dutLabel, err := proto.Marshal(d)
	if err != nil {
		return nil, errors.Annotate(err, "failed to marshal DutLabel %s", d).Err()
	}

	entity := &configuration.HwidDataEntity{
		ID:       hwid,
		HwidData: dutLabel,
		Updated:  updatedTime,
	}
	if err := datastore.Put(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

func TestGetHwidData(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	ctx = external.WithTestingContext(ctx)
	ctx = useTestingCfg(ctx)
	datastore.GetTestable(ctx).Consistent(true)

	es, err := external.GetServerInterface(ctx)
	if err != nil {
		t.Fatalf("Failed to get server interface: %s", err)
	}
	client, err := es.NewHwidClientInterface(ctx)
	if err != nil {
		t.Fatalf("Failed to get fake hwid client interface: %s", err)
	}

	t.Run("happy path - get cached data from datastore", func(t *testing.T) {
		// Server should respond but since cache is within range, cache should be
		// returned and not updated.
		const id = "test"

		// Test if server is responding.
		serverRsp, err := client.QueryHwid(ctx, id)
		if err != nil {
			t.Fatalf("Fake hwid server responded with error: %s", err)
		}
		if diff := cmp.Diff(mockHwidData().GetDutLabel(), serverRsp, protocmp.Transform()); diff != "" {
			t.Errorf("Fake hwid server returned unexpected diff (-want +got):\n%s", diff)
		}

		// Test getting data from datastore.
		cacheTime := time.Now().UTC().Add(-30 * time.Minute)
		_, err = fakeUpdateHwidData(ctx, mockHwidData(), id, cacheTime)
		if err != nil {
			t.Fatalf("fakeUpdateHwidData failed: %s", err)
		}
		want := mockHwidData()
		got, err := GetHwidData(ctx, client, id)
		if err != nil {
			t.Fatalf("GetHwidData failed: %s", err)
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}
		hwidEnt, _ := configuration.GetHwidData(ctx, id)
		if diff := cmp.Diff(cacheTime, hwidEnt.Updated, cmpopts.EquateApproxTime(1*time.Millisecond)); diff != "" {
			t.Errorf("Cache time has unexpected diff (-want +got):\n%s", diff)
		}
		datastore.Delete(ctx, hwidEnt)
	})

	t.Run("happy path - get legacy cached data from datastore", func(t *testing.T) {
		// Cache should be returned and not updated. Server should not be called.
		const id = "test-legacy"

		// Test getting data from datastore.
		cacheTime := time.Now().UTC().Add(-30 * time.Minute)
		_, err = fakeUpdateLegacyHwidData(ctx, mockDutLabel(), id, cacheTime)
		if err != nil {
			t.Fatalf("fakeUpdateHwidData failed: %s", err)
		}
		want := &ufspb.HwidData{
			Sku:     "test-legacy-sku",
			Variant: "test-legacy-variant",
			Hwid:    "test-legacy",
			DutLabel: &ufspb.DutLabel{
				PossibleLabels: []string{
					"test-possible-1",
					"test-possible-2",
				},
				Labels: []*ufspb.DutLabel_Label{
					{
						Name:  "test-label-1",
						Value: "test-value-1",
					},
					{
						Name:  "Sku",
						Value: "test-legacy-sku",
					},
					{
						Name:  "variant",
						Value: "test-legacy-variant",
					},
				},
			},
		}
		got, err := GetHwidData(ctx, client, id)
		if err != nil {
			t.Fatalf("GetHwidData failed: %s", err)
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}
		hwidEnt, _ := configuration.GetHwidData(ctx, id)
		if diff := cmp.Diff(cacheTime, hwidEnt.Updated, cmpopts.EquateApproxTime(1*time.Millisecond)); diff != "" {
			t.Errorf("Cache time has unexpected diff (-want +got):\n%s", diff)
		}
		datastore.Delete(ctx, hwidEnt)
	})

	t.Run("get cached data from datastore; hwid server errors", func(t *testing.T) {
		// Server should respond nil so method should return last cached entity from
		// the datastore.
		const id = "test-no-server"

		// Test if server is responding nil.
		serverRsp, err := client.QueryHwid(ctx, id)
		if err == nil {
			t.Fatalf("Fake hwid server responded without error")
		}
		if diff := cmp.Diff(&ufspb.DutLabel{}, serverRsp, protocmp.Transform()); diff != "" {
			t.Errorf("Fake hwid server returned unexpected diff (-want +got):\n%s", diff)
		}

		// Test getting data from datastore.
		hwidEnt, err := configuration.UpdateHwidData(ctx, mockHwidDataNoServer(), id)
		if err != nil {
			t.Fatalf("UpdateHwidData failed: %s", err)
		}
		want := mockHwidDataNoServer()
		got, err := GetHwidData(ctx, client, id)
		if err != nil {
			t.Fatalf("GetHwidData failed: %s", err)
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}
		datastore.Delete(ctx, hwidEnt)
	})

	t.Run("empty datastore; get data from hwid server and update cache", func(t *testing.T) {
		// Datastore is empty so query hwid server. Server should respond with
		// DutLabel data and cache in datastore.
		const id = "test"

		// No data should exist in datastore for id.
		_, err := configuration.GetHwidData(ctx, id)
		if err != nil && !util.IsNotFoundError(err) {
			t.Fatalf("Datastore already contains data for %s: %s", id, err)
		}

		// Test method and get data from server.
		want := mockHwidData()
		got, err := GetHwidData(ctx, client, id)
		if err != nil {
			t.Fatalf("GetHwidData failed: %s", err)
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}

		// Test if results were cached into datastore.
		hwidEnt, err := configuration.GetHwidData(ctx, id)
		if err != nil {
			if util.IsNotFoundError(err) {
				t.Fatalf("GetHwidData did not cache hwid server result")
			}
			t.Fatalf("GetHwidData unknown error: %s", err)
		}
		data, err := configuration.ParseHwidData(hwidEnt)
		if err != nil {
			t.Fatalf("Failed to parse hwid data: %s", err)
		}
		if diff := cmp.Diff(want, data, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(time.Now().UTC(), hwidEnt.Updated, cmpopts.EquateApproxTime(2*time.Second)); diff != "" {
			t.Errorf("New cache time is outside margin of error; unexpected diff (-want +got):\n%s", diff)
		}
		datastore.Delete(ctx, hwidEnt)
	})

	t.Run("datastore data expired; update cache with hwid server", func(t *testing.T) {
		// Datastore data is expired so query hwid server. Server should respond
		// with DutLabel data and cache in datastore.
		const id = "test"

		// Add expired data to datastore.
		expHwidData := &ufspb.HwidData{
			Sku:     "test-sku-expired",
			Variant: "test-variant-expired",
			Hwid:    "test",
			DutLabel: &ufspb.DutLabel{
				PossibleLabels: []string{
					"test-possible-1",
					"test-possible-2",
				},
				Labels: []*ufspb.DutLabel_Label{
					{
						Name:  "test-label-1",
						Value: "test-value-1",
					},
					{
						Name:  "Sku",
						Value: "test-sku-expired",
					},
					{
						Name:  "variant",
						Value: "test-variant-expired",
					},
				},
			},
		}
		expiredTime := time.Now().Add(-2 * time.Hour).UTC()
		fakeUpdateHwidData(ctx, expHwidData, "test", expiredTime)
		hwidEntExp, _ := configuration.GetHwidData(ctx, id)
		parsedExpData, _ := configuration.ParseHwidData(hwidEntExp)
		if diff := cmp.Diff(expHwidData, parsedExpData, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}

		// Calling GetHwidData should immediately cache new data into datastore
		// and return the new data.
		want := mockHwidData()
		got, err := GetHwidData(ctx, client, id)
		if err != nil {
			t.Fatalf("GetHwidData failed: %s", err)
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}

		// Test if results were cached into datastore.
		hwidEnt, err := configuration.GetHwidData(ctx, id)
		if err != nil {
			t.Fatalf("GetHwidData unknown error: %s", err)
		}
		data, err := configuration.ParseHwidData(hwidEnt)
		if err != nil {
			t.Fatalf("Failed to parse hwid data: %s", err)
		}
		if diff := cmp.Diff(want, data, protocmp.Transform()); diff != "" {
			t.Errorf("GetHwidData returned unexpected diff (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(time.Now().UTC(), hwidEnt.Updated, cmpopts.EquateApproxTime(2*time.Second)); diff != "" {
			t.Errorf("New cache time is outside margin of error; unexpected diff (-want +got):\n%s", diff)
		}
		datastore.Delete(ctx, hwidEnt)
	})

	t.Run("no data in datastore and hwid server errors", func(t *testing.T) {
		got, err := GetHwidData(ctx, client, "test-err")
		if err != nil {
			t.Fatalf("GetHwidData unknown error: %s", err)
		}
		if got != nil {
			t.Errorf("GetHwidData is not nil: %s", got)
		}
	})

	t.Run("no data in datastore and throttle hwid server", func(t *testing.T) {
		cfgLst := &config.Config{
			HwidServiceTrafficRatio: 0,
		}
		trafficCtx := config.Use(ctx, cfgLst)

		got, err := GetHwidData(trafficCtx, client, "test-no-data")
		if err != nil {
			t.Fatalf("GetHwidData unknown error: %s", err)
		}
		if got != nil {
			t.Errorf("GetHwidData is not nil: %s", got)
		}
	})

	t.Run("making request from partner namespace", func(t *testing.T) {
		const id = "test"

		partnerCtx, err := util.SetupDatastoreNamespace(ctx, util.OSPartnerNamespace)
		if err != nil {
			t.Errorf("error setting up context")
		}

		got, err := GetHwidData(partnerCtx, client, id)
		if err == nil {
			t.Fatalf("GetHwidData was expecting error: %s", err)
		}
		if got != nil {
			t.Errorf("GetHwidData is not nil: %s", got)
		}
	})
}

// TestListHwidData tests the ListHwidData RPC method.
func TestListHwidData(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)

	hds := make([]*ufspb.HwidData, 0, 4)
	for i := range 4 {
		hdId := fmt.Sprintf("test-hwid-%d", i)
		hd := mockHwidData()
		resp, err := configuration.UpdateHwidData(ctx, hd, hdId)
		if err != nil {
			t.Fatalf("UpdateHwidData failed: %s", err)
		}
		respProto, err := resp.GetProto()
		if err != nil {
			t.Fatalf("GetProto failed: %s", err)
		}
		hds = append(hds, respProto.(*ufspb.HwidData))
	}
	ftt.Run("ListHwidData", t, func(t *ftt.Test) {
		t.Run("ListHwidData - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListHwidData(ctx, 5, "abc", "", false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsds.InvalidPageToken))
		})

		t.Run("ListHwidData - full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListHwidData(ctx, 4, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(hds))
		})

		t.Run("ListHwidData - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListHwidData(ctx, 3, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(hds[:3]))

			resp, _, err = ListHwidData(ctx, 2, nextPageToken, "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(hds[3:]))
		})
	})
}
