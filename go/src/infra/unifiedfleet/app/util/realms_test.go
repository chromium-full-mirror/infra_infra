// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"testing"

	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
	"go.chromium.org/luci/server/auth/realms"
	"go.chromium.org/luci/server/auth/service/protocol"
)

func TestCheckPermission(t *testing.T) {
	t.Parallel()

	realmID := "project:some-project"
	admin := identity.Identity("user:admin@example.com")
	reader := identity.Identity("user:reader@example.com")
	writer := identity.Identity("user:writer@example.com")
	readPermission := realms.RegisterPermission("testing.resource.read")
	writePermission := realms.RegisterPermission("testing.resource.write")
	fakeDB := authtest.NewFakeDB(
		authtest.MockMembership(admin, "admins"),
		authtest.MockMembership(reader, "readers"),
		authtest.MockMembership(writer, "writers"),
		authtest.MockPermission(admin, realmID, readPermission),
		authtest.MockPermission(admin, realmID, writePermission),
		authtest.MockPermission(reader, realmID, readPermission),
		authtest.MockPermission(writer, realmID, writePermission),
		authtest.MockRealmData(realmID, &protocol.RealmData{}),
	)
	check := func(id identity.Identity, permission realms.Permission, realm string, expected bool) {
		ctx := auth.WithState(context.Background(), &authtest.FakeState{
			Identity: id,
			FakeDB:   fakeDB,
		})
		err := CheckPermission(ctx, permission, realm)
		if expected {
			assert.Loosely(t, err, should.BeNil)
		} else {
			assert.Loosely(t, err, should.NotBeNil)
		}
	}
	ftt.Run("TestCheckPermission - Read/Write permission check admin", t, func(t *ftt.Test) {
		check(admin, readPermission, realmID, true)
		check(admin, writePermission, realmID, true)
	})
	ftt.Run("TestCheckPermission - Read only permission check for reader", t, func(t *ftt.Test) {
		check(reader, readPermission, realmID, true)
		check(reader, writePermission, realmID, false)
	})
	ftt.Run("TestCheckPermission - Write only permission check for writer", t, func(t *ftt.Test) {
		check(writer, readPermission, realmID, false)
		check(writer, writePermission, realmID, true)
	})
	ftt.Run("TestCheckPermission - Empty realm", t, func(t *ftt.Test) {
		check(writer, readPermission, "", true)
		check(writer, writePermission, "", true)
	})
}

// TestToUFSRealm checks zone->UFS realm conversions
func TestToUFSRealm(t *testing.T) {
	tests := []struct {
		name      string
		inZone    string
		wantRealm string
	}{
		{
			name:      "unspecified",
			inZone:    "",
			wantRealm: "",
		},
		{
			name:      "browser",
			inZone:    "ZONE_ATLANTA",
			wantRealm: "@internal:ufs/browser",
		},
		{
			name:      "browser no prefix",
			inZone:    "atlanta",
			wantRealm: "@internal:ufs/browser",
		},
		{
			name:      "acs",
			inZone:    "ZONE_CHROMEOS3",
			wantRealm: "@internal:ufs/os-acs",
		},
		{
			name:      "acs no prefix",
			inZone:    "chromeos3",
			wantRealm: "@internal:ufs/os-acs",
		},
		{
			name:      "satlab",
			inZone:    "ZONE_SATLAB",
			wantRealm: "@internal:ufs/satlab-internal-users",
		},
		{
			name:      "satlab no prefix",
			inZone:    "satlab",
			wantRealm: "@internal:ufs/satlab-internal-users",
		},
		{
			name:      "os-atl-chromium",
			inZone:    "ZONE_SFO36_OS_CHROMIUM",
			wantRealm: "@internal:ufs/os-atl-chromium",
		},
		{
			name:      "os-atl no prefix",
			inZone:    "sfo36_os_chromium",
			wantRealm: "@internal:ufs/os-atl-chromium",
		},
		{
			name:      "sfp",
			inZone:    "ZONE_SFP_SFPTEST",
			wantRealm: "chromeos:ufs/sfp_sfptest",
		},
		{
			name:      "sfp no prefix",
			inZone:    "sfp_sfptest",
			wantRealm: "chromeos:ufs/sfp_sfptest",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ToUFSRealm(tt.inZone); got != tt.wantRealm {
				t.Errorf("ToUFSRealm() = %v, want %v", got, tt.wantRealm)
			}
		})
	}
}
