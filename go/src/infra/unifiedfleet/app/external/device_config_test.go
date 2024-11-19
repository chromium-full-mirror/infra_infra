// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/util"
)

// makeDevCfgForTesting creates a basic DeviceConfig. These configs have no
// guarantee to make sense at a domain level, but can be used to verify code
// behavior.
func makeDevCfgForTesting(board, model, variant string, tams []string) *deviceconfig.Config {
	return &deviceconfig.Config{
		Id:  configuration.GetConfigID(board, model, variant),
		Tam: tams,
	}
}

// TestGetDeviceConfig tests behavior of the dual read client.

// The testing environment is seeded with a single device config in datastore.
// It also has a flexible inventory client which can return device configs. This allows
// all combinations of device config existence to be tested
func TestGetDeviceConfig(t *testing.T) {
	tests := []struct {
		name    string
		inUFS   bool
		cfgID   *deviceconfig.ConfigId
		want    *deviceconfig.Config
		wantErr bool
	}{
		{
			name:    "empty config",
			inUFS:   true,
			cfgID:   configuration.GetConfigID("", "", ""),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "config in UFS",
			inUFS:   true,
			cfgID:   configuration.GetConfigID("zork", "gumboz", ""),
			want:    makeDevCfgForTesting("zork", "gumboz", "", []string{"test@google.com"}),
			wantErr: false,
		},
		{
			name:    "fallback config in UFS",
			inUFS:   true,
			cfgID:   configuration.GetConfigID("zork", "gumboz", "12345"),
			want:    makeDevCfgForTesting("zork", "gumboz", "", []string{"test@google.com"}),
			wantErr: false,
		},
		{
			name:    "config nowhere",
			inUFS:   false,
			cfgID:   configuration.GetConfigID("other", "device", ""),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// setup datastore + populate it
			ctx := memory.UseWithAppID(context.Background(), ("gae-test"))
			// grant user permission in the appropriate realm
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:root@lab.com",
				IdentityPermissions: []authtest.RealmPermission{
					{
						Realm:      "chromeos:zork-gumboz",
						Permission: util.ConfigurationsGet,
					},
				},
			})
			datastore.GetTestable(ctx).Consistent(true)
			if tt.inUFS {
				devCfg := makeDevCfgForTesting("zork", "gumboz", "", []string{"test@google.com"})
				_, err := configuration.BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{devCfg}, configuration.BoardModelRealmAssigner)
				if err != nil {
					t.Errorf("error setting up test data")
				}
			}

			// setup inventory and dual read clients
			c := &DualDeviceConfigClient{}

			got, err := c.GetDeviceConfig(ctx, tt.cfgID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DualDeviceConfigClient.GetDeviceConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(deviceconfig.Config{}, deviceconfig.ConfigId{}, deviceconfig.PlatformId{}, deviceconfig.ModelId{}, deviceconfig.VariantId{})); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}

// TestDeviceConfigExists tests functionality of dual reading inventory and UFS
// for determining config existence. It populates a fake datastore with a
// device config entry and allows the test to set the response from inventory.
func TestDeviceConfigExists(t *testing.T) {
	tests := []struct {
		name    string
		cfgIDs  []*deviceconfig.ConfigId
		want    []bool
		wantErr bool
	}{
		{
			name:    "UFS has some configs",
			cfgIDs:  []*deviceconfig.ConfigId{configuration.GetConfigID("other", "device", ""), configuration.GetConfigID("zork", "gumboz", "")},
			want:    []bool{false, true},
			wantErr: false,
		},
		{
			name:    "UFS has all configs",
			cfgIDs:  []*deviceconfig.ConfigId{configuration.GetConfigID("zork", "gumboz", ""), configuration.GetConfigID("zork", "gumboz2", "")},
			want:    []bool{true, true},
			wantErr: false,
		},
		{
			name:    "neither have configs",
			cfgIDs:  []*deviceconfig.ConfigId{configuration.GetConfigID("other", "device", ""), configuration.GetConfigID("other", "device2", "")},
			want:    []bool{false, false},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// setup datastore + populate it
			ctx := memory.UseWithAppID(context.Background(), ("gae-test"))
			// grant user appropriate realm permission
			ctx = auth.WithState(ctx, &authtest.FakeState{
				Identity: "user:root@lab.com",
				IdentityPermissions: []authtest.RealmPermission{
					{
						Realm:      "chromeos:zork-gumboz",
						Permission: util.ConfigurationsGet,
					},
					{
						Realm:      "chromeos:zork-gumboz2",
						Permission: util.ConfigurationsGet,
					},
				},
			})
			datastore.GetTestable(ctx).Consistent(true)
			devCfg := makeDevCfgForTesting("zork", "gumboz", "", []string{"test@google.com"})
			devCfg2 := makeDevCfgForTesting("zork", "gumboz2", "", []string{"test@google.com"})
			_, err := configuration.BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{devCfg, devCfg2}, configuration.BoardModelRealmAssigner)
			if err != nil {
				t.Errorf("error setting up test data")
			}

			// setup dual read clients
			c := &DualDeviceConfigClient{}

			got, err := c.DeviceConfigsExists(ctx, tt.cfgIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("DualDeviceConfigClient.DeviceConfigExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
