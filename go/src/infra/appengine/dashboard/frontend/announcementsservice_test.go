// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
)

func TestAnnouncementsPrelude(t *testing.T) {
	anon := identity.AnonymousIdentity
	someone := identity.Identity("user:chicken@example.com")
	trooper := identity.Identity("user:trooper@example.com")
	state := &authtest.FakeState{
		FakeDB: authtest.NewFakeDB(
			authtest.MockMembership(trooper, announcementGroup),
		),
	}
	ctx := auth.WithState(context.Background(), state)

	var testCases = []struct {
		methodName string
		caller     identity.Identity
		code       codes.Code
	}{
		{"CreateLiveAnnouncement", anon, codes.Unauthenticated},
		{"CreateLiveAnnouncement", someone, codes.PermissionDenied},
		{"CreateLiveAnnouncement", trooper, 0},
		{"RetireAnnouncement", anon, codes.Unauthenticated},
		{"RetireAnnouncement", someone, codes.PermissionDenied},
		{"RetireAnnouncement", trooper, 0},
		{"UpdateAnnouncementPlatforms", anon, codes.Unauthenticated},
		{"UpdateAnnouncementPlatforms", someone, codes.PermissionDenied},
		{"SearchAnnouncements", anon, 0},
		{"SearchAnnouncements", someone, 0},
	}

	for i, tc := range testCases {
		ftt.Run(fmt.Sprintf("%d - %s by %s", i, tc.methodName, tc.caller), t, func(t *ftt.Test) {
			state.Identity = tc.caller
			_, err := announcementsPrelude(ctx, tc.methodName, nil)
			if tc.code == 0 {
				assert.Loosely(t, err, should.BeNil)
			} else {
				assert.Loosely(t, status.Code(err), should.Equal(tc.code))
			}
		})
	}
	ftt.Run("unrecognized method", t, func(t *ftt.Test) {
		state.Identity = trooper
		assert.Loosely(t, func() { announcementsPrelude(ctx, "melemele", nil) }, should.Panic)
	})
}
