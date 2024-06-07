// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testsupport provides the `NewFixture` function, which produces
// a context and a frontend that's suitable for testing.
package testsupport

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"

	fleetcostpb "infra/cros/fleetcost/api/models"
	"infra/cros/fleetcost/internal/costserver"
	"infra/cros/fleetcost/internal/costserver/entities"
	"infra/cros/fleetcost/internal/utils"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	mockufs "infra/unifiedfleet/api/v1/rpc/mock"
)

// Fixture creates a new test fixture for the fleet cost service and the backend services it talks to.
//
// For each of the backend services, we use the best test implementation available.
// Datastore uses the LUCI in-memory representation, so it doesn't get an explicit field.
// UFS uses the go-mock generated stub.
type Fixture struct {
	Ctx      context.Context
	Frontend *costserver.FleetCostFrontend
	MockUFS  *mockufs.MockFleetClient
}

// RegisterDUTsForLabstation registers a GetChromeOSDeviceData request and response.
func (tf *Fixture) RegisterDUTsForLabstation(reqMatcher gomock.Matcher, resp *ufsAPI.GetDUTsForLabstationResponse) {
	tf.MockUFS.EXPECT().GetDUTsForLabstation(gomock.Any(), reqMatcher).Return(resp, nil)
}

// RegisterGetChromeOSDeviceDataCall registers a GetChromeOSDeviceData request and response.
func (tf *Fixture) RegisterGetChromeOSDeviceDataCall(reqMatcher gomock.Matcher, resp *ufspb.ChromeOSDeviceData) {
	tf.MockUFS.EXPECT().GetChromeOSDeviceData(gomock.Any(), reqMatcher).Return(resp, nil)
}

// RegisterGetDeviceDataCall registers a GetDeviceData request and response.
func (tf *Fixture) RegisterGetDeviceDataCall(reqMatcher gomock.Matcher, resp *ufsAPI.GetDeviceDataResponse) {
	tf.MockUFS.EXPECT().GetDeviceData(gomock.Any(), reqMatcher).Return(resp, nil)
}

// RegisterGetDeviceDataFailure registers a failing GetDeviceData response.
//
// No attempt is made to ensure that the error that we get back is realistic.
// TODO(gregorynisbet): Check the error for reasonableness and make sure it resembles a UFS error
// and panic if it does not resemble one.
func (tf *Fixture) RegisterGetDeviceDataFailure(reqMatcher gomock.Matcher, failure error) {
	tf.MockUFS.EXPECT().GetDeviceData(gomock.Any(), reqMatcher).Return(nil, failure)
}

// RegisterListMachineLSEs registers a listMachineLSEs request.
func (tf *Fixture) RegisterListMachineLSEs(reqMatcher gomock.Matcher, resp *ufsAPI.ListMachineLSEsResponse) {
	tf.MockUFS.EXPECT().ListMachineLSEs(gomock.Any(), reqMatcher).Return(resp, nil)
}

// NewFixture creates a basic fixture with fake versions of datastore and UFS with properties
// that are convenient for unit tests.
func NewFixture(ctx context.Context, t *testing.T) *Fixture {
	mc := gomock.NewController(t)
	var out Fixture
	out.Ctx = memory.Use(ctx)
	datastore.GetTestable(out.Ctx).Consistent(true)
	out.Frontend = costserver.NewFleetCostFrontend().(*costserver.FleetCostFrontend)
	out.MockUFS = mockufs.NewMockFleetClient(mc)
	costserver.SetUFSClient(out.Frontend, out.MockUFS)
	return &out
}

// NewFixtureWithData returns a fixture with test data.
func NewFixtureWithData(ctx context.Context, t *testing.T) *Fixture {
	tf := NewFixture(ctx, t)
	err := utils.InsertOneWithoutReplacement(tf.Ctx, &entities.CostIndicatorEntity{
		CostIndicator: &fleetcostpb.CostIndicator{
			Board:       "e",
			BurnoutRate: 44.0,
		},
	}, nil)
	if err != nil {
		panic(err)
	}
	return tf
}

// SimpleMatcher takes an artbirary function and makes it a matcher.
type SimpleMatcher struct {
	Predicate func(any) bool
	Message   string
}

// Matches determines whether an item fulfills the predicate.
func (matcher SimpleMatcher) Matches(item any) bool {
	return matcher.Predicate(item)
}

// String returns the matcher message.
func (matcher SimpleMatcher) String() string {
	return matcher.Message
}

// NewMatcher makes a new predicate matcher.
func NewMatcher(message string, predicate func(any) bool) gomock.Matcher {
	return SimpleMatcher{
		Predicate: predicate,
		Message:   message,
	}
}
