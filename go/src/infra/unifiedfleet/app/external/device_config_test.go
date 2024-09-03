// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package external

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	invV2Api "infra/appengine/cros/lab_inventory/api/v1"
	ufsdevice "infra/unifiedfleet/api/v1/models/chromeos/device"
	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/util"
)

type fakeInventoryClient struct {
	GetDeviceConfigResp    *device.Config
	GetDeviceConfigErr     bool
	DeviceConfigExistsResp *invV2Api.DeviceConfigsExistsResponse
	DeviceConfigExistsErr  bool
}

func (ic *fakeInventoryClient) DeviceConfigsExists(ctx context.Context, in *invV2Api.DeviceConfigsExistsRequest, opts ...grpc.CallOption) (*invV2Api.DeviceConfigsExistsResponse, error) {
	if ic.DeviceConfigExistsErr {
		return nil, errors.New("error fetching device configs")
	}
	return ic.DeviceConfigExistsResp, nil
}

func (ic *fakeInventoryClient) GetDeviceConfig(ctx context.Context, in *invV2Api.GetDeviceConfigRequest, opts ...grpc.CallOption) (*device.Config, error) {
	if ic.GetDeviceConfigErr {
		return nil, errors.New("error fetching device config")
	}
	return ic.GetDeviceConfigResp, nil
}

// makeDevCfgForTesting creates a basic DeviceConfig. These configs have no
// guarantee to make sense at a domain level, but can be used to verify code
// behavior.
func makeDevCfgForTesting(board, model, variant string, tams []string) *ufsdevice.Config {
	return &ufsdevice.Config{
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
		invResp *device.Config
		invErr  bool
		cfgID   *ufsdevice.ConfigId
		want    *ufsdevice.Config
		wantErr bool
	}{
		{
			name:    "config in UFS",
			inUFS:   true,
			invResp: nil,
			invErr:  true,
			cfgID:   configuration.GetConfigID("zork", "gumboz", ""),
			want:    makeDevCfgForTesting("zork", "gumboz", "", []string{"test@google.com"}),
			wantErr: false,
		},
		{
			name:  "config in inventoryv2",
			inUFS: false,
			invResp: &device.Config{
				Id: &device.ConfigId{
					PlatformId: &device.PlatformId{Value: "zork"},
					ModelId:    &device.ModelId{Value: "gumboz"},
					VariantId:  &device.VariantId{Value: ""},
				},
				Tam: []string{"inventory@google.com"},
			},
			invErr:  false,
			cfgID:   configuration.GetConfigID("zork", "gumboz", ""),
			want:    makeDevCfgForTesting("zork", "gumboz", "", []string{"inventory@google.com"}),
			wantErr: false,
		},
		{
			name:  "config in inventoryv2 and UFS", // same board/model but inventoryv2 has different TAM
			inUFS: true,
			invResp: &device.Config{
				Id: &device.ConfigId{
					PlatformId: &device.PlatformId{Value: "zork"},
					ModelId:    &device.ModelId{Value: "gumboz"},
					VariantId:  &device.VariantId{Value: ""},
				},
				Tam: []string{"inventory@google.com"},
			},
			invErr:  false,
			cfgID:   configuration.GetConfigID("zork", "gumboz", ""),
			want:    makeDevCfgForTesting("zork", "gumboz", "", []string{"test@google.com"}),
			wantErr: false,
		},
		{
			name:    "config nowhere",
			inUFS:   false,
			invResp: nil,
			invErr:  true,
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
				_, err := configuration.BatchUpdateDeviceConfigs(ctx, []*ufsdevice.Config{devCfg}, configuration.BoardModelRealmAssigner)
				if err != nil {
					t.Errorf("error setting up test data")
				}
			}

			// setup inventory and dual read clients
			ic := &fakeInventoryClient{
				GetDeviceConfigResp: tt.invResp,
				GetDeviceConfigErr:  tt.invErr,
			}
			c := &DualDeviceConfigClient{
				inventoryClient: ic,
			}

			got, err := c.GetDeviceConfig(ctx, tt.cfgID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DualDeviceConfigClient.GetDeviceConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(ufsdevice.Config{}, ufsdevice.ConfigId{}, ufsdevice.PlatformId{}, ufsdevice.ModelId{}, ufsdevice.VariantId{})); diff != "" {
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
		invResp *invV2Api.DeviceConfigsExistsResponse
		invErr  bool
		cfgIDs  []*ufsdevice.ConfigId
		want    []bool
		wantErr bool
	}{
		{
			name:    "only UFS has some configs",
			invResp: nil,
			invErr:  true,
			cfgIDs:  []*ufsdevice.ConfigId{configuration.GetConfigID("other", "device", ""), configuration.GetConfigID("zork", "gumboz", "")},
			want:    []bool{false, true},
			wantErr: false,
		},
		{
			name:    "UFS has all configs",
			invResp: nil,
			invErr:  true,
			cfgIDs:  []*ufsdevice.ConfigId{configuration.GetConfigID("zork", "gumboz", ""), configuration.GetConfigID("zork", "gumboz2", "")},
			want:    []bool{true, true},
			wantErr: false,
		},
		{
			name:    "only inventory has some configs",
			invResp: &invV2Api.DeviceConfigsExistsResponse{Exists: map[int32]bool{1: true}},
			invErr:  false,
			cfgIDs:  []*ufsdevice.ConfigId{configuration.GetConfigID("other", "device", ""), configuration.GetConfigID("other", "device2", "")},
			want:    []bool{false, true},
			wantErr: false,
		},
		{
			name:    "inventory has all configs",
			invResp: &invV2Api.DeviceConfigsExistsResponse{Exists: map[int32]bool{0: true, 1: true}},
			invErr:  false,
			cfgIDs:  []*ufsdevice.ConfigId{configuration.GetConfigID("other", "device", ""), configuration.GetConfigID("other", "device2", "")},
			want:    []bool{true, true},
			wantErr: false,
		},
		{
			name:    "neither have configs",
			invResp: nil,
			invErr:  true,
			cfgIDs:  []*ufsdevice.ConfigId{configuration.GetConfigID("other", "device", ""), configuration.GetConfigID("other", "device2", "")},
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
			_, err := configuration.BatchUpdateDeviceConfigs(ctx, []*ufsdevice.Config{devCfg, devCfg2}, configuration.BoardModelRealmAssigner)
			if err != nil {
				t.Errorf("error setting up test data")
			}

			// setup inventory and dual read clients
			ic := &fakeInventoryClient{
				DeviceConfigExistsResp: tt.invResp,
				DeviceConfigExistsErr:  tt.invErr,
			}
			c := &DualDeviceConfigClient{
				inventoryClient: ic,
			}

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
