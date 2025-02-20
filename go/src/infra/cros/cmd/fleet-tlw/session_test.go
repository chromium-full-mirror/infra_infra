// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/api/test/tls"

	"go.chromium.org/infra/cros/cmd/fleet-tlw/internal/cache"
	"go.chromium.org/infra/cros/fleet/access"
	ufsmodels "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

type fakeTLWServer struct{}

func (t fakeTLWServer) Close() {}

func (t fakeTLWServer) registerWith(*grpc.Server) {}

type fakeTLWBuilder struct{}

func (b fakeTLWBuilder) build() (wiringServer, error) {
	return fakeTLWServer{}, nil
}

// TestSessionServer does a basic test of the session RPCs by using
// them how a simple client might.
func TestSessionServer(t *testing.T) {
	t.Parallel()
	limit := 10 * time.Second
	if deadline, ok := t.Deadline(); ok {
		if newLim := time.Until(deadline); newLim < limit {
			limit = newLim
		}
	}
	ctx, cf := context.WithTimeout(context.Background(), limit)
	t.Cleanup(cf)

	s := newSessionServer(fakeTLWBuilder{})
	expire := tsAfter(time.Minute)

	got, err := s.CreateSession(ctx, &access.CreateSessionRequest{
		Session: &access.Session{
			ExpireTime: expire,
		},
	})
	if err != nil {
		t.Fatalf("failed to create session: %s", err)
	}
	t.Run("check created session", func(t *testing.T) {
		want := proto.Clone(got).(*access.Session)
		want.ExpireTime = expire
		if !proto.Equal(want, got) {
			t.Errorf("session mismatch (-want +got):\n%s\n%s", want, got)
		}
	})
	t.Run("connect to TLW", func(t *testing.T) {
		conn, err := grpc.Dial(got.GetTlwAddress(), grpc.WithInsecure())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		c := tls.NewWiringClient(conn)
		_, err = c.GetDut(ctx, &tls.GetDutRequest{Name: "placeholder"})
		if c := status.Code(err); c != codes.Unimplemented && c != codes.NotFound && c != codes.OK && c != codes.FailedPrecondition {
			t.Errorf("Unexpected error when calling TLW API: %s", err)
		}
	})
	name := got.GetName()
	expire = tsAfter(2 * time.Minute)
	mask, err := fieldmaskpb.New((*access.Session)(nil), "expire_time")
	if err != nil {
		t.Fatalf("failed to make mask: %s", err)
	}
	got, err = s.UpdateSession(ctx, &access.UpdateSessionRequest{
		Session: &access.Session{
			Name:       name,
			ExpireTime: expire,
		},
		UpdateMask: mask,
	})
	if err != nil {
		t.Fatalf("failed to update session: %s", err)
	}
	t.Run("check updated session", func(t *testing.T) {
		want := proto.Clone(got).(*access.Session)
		want.ExpireTime = expire
		if !proto.Equal(want, got) {
			t.Errorf("session mismatch (-want +got):\n%s\n%s", want, got)
		}
	})
	_, err = s.DeleteSession(ctx, &access.DeleteSessionRequest{
		Name: name,
	})
	if err != nil {
		t.Fatalf("failed to delete session: %s", err)
	}
}

// tsAfter returns a proto timestamp some time from now.
// This is a test helper.
func tsAfter(d time.Duration) *timestamppb.Timestamp {
	return timestamppb.New(time.Now().Add(time.Minute))
}

var _ cache.Environment = fakeEnv{}

// fakeEnv is a fake implementation of cache.Environment.
type fakeEnv struct{}

func (fakeEnv) Subnets() []cache.Subnet {
	return nil
}

func (fakeEnv) CacheZones() map[ufsmodels.Zone][]cache.CachingService { return nil }

func (fakeEnv) GetZoneForServer(string) (ufsmodels.Zone, error) {
	return ufsmodels.Zone_ZONE_UNSPECIFIED, nil
}

func (fakeEnv) GetZoneForDUT(string) (ufsmodels.Zone, error) {
	return ufsmodels.Zone_ZONE_UNSPECIFIED, nil
}
