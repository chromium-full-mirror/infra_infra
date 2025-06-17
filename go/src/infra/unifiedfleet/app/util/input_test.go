// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsmfg "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/manufacturing"
)

func TestUFSStateCoverage(t *testing.T) {
	ftt.Run("test the ufs state mapping covers all UFS state enum", t, func(t *ftt.Test) {
		got := make(map[string]bool, len(StrToUFSState))
		for _, v := range StrToUFSState {
			got[v] = true
		}
		for l := range ufspb.State_value {
			if l == ufspb.State_STATE_UNSPECIFIED.String() {
				continue
			}
			_, ok := got[l]
			assert.Loosely(t, ok, should.BeTrue)
		}
	})

	ftt.Run("test the ufs state mapping doesn't cover any non-UFS state enum", t, func(t *ftt.Test) {
		for _, v := range StrToUFSState {
			_, ok := ufspb.State_value[v]
			assert.Loosely(t, ok, should.BeTrue)
		}
	})
}

func TestGetResourcePrefix(t *testing.T) {
	ftt.Run("Test various proto message", t, func(t *ftt.Test) {
		t.Run("Test machine proto", func(t *ftt.Test) {
			machine := &ufspb.Machine{}
			res, err := GetResourcePrefix(machine)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("machines"))
		})
		t.Run("Test asset proto", func(t *ftt.Test) {
			asset := &ufspb.Asset{}
			res, err := GetResourcePrefix(asset)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("assets"))
		})
		t.Run("Test event proto", func(t *ftt.Test) {
			event := &ufspb.ChangeEvent{}
			res, err := GetResourcePrefix(event)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("events"))
		})
		t.Run("Test chrome platform proto", func(t *ftt.Test) {
			platform := &ufspb.ChromePlatform{}
			res, err := GetResourcePrefix(platform)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("chromePlatforms"))
		})
		t.Run("Test nic proto", func(t *ftt.Test) {
			nic := &ufspb.Nic{}
			res, err := GetResourcePrefix(nic)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("nics"))
		})
		t.Run("Test vlan proto", func(t *ftt.Test) {
			vlan := &ufspb.Vlan{}
			res, err := GetResourcePrefix(vlan)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("vlans"))
		})
		t.Run("Test kvm proto", func(t *ftt.Test) {
			kvm := &ufspb.KVM{}
			res, err := GetResourcePrefix(kvm)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("kvms"))
		})
		t.Run("Test rpm proto", func(t *ftt.Test) {
			rpm := &ufspb.RPM{}
			res, err := GetResourcePrefix(rpm)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("rpms"))
		})
		t.Run("Test switch proto", func(t *ftt.Test) {
			swch := &ufspb.Switch{}
			res, err := GetResourcePrefix(swch)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("switches"))
		})
		t.Run("Test rack proto", func(t *ftt.Test) {
			rack := &ufspb.Rack{}
			res, err := GetResourcePrefix(rack)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Equal("racks"))
		})
	})
}

func TestGetIncomingCtxNamespace(t *testing.T) {
	ctx := context.Background()
	ftt.Run("Test no metadata set up", t, func(t *ftt.Test) {
		assert.Loosely(t, GetIncomingCtxNamespace(ctx), should.Equal(BrowserNamespace))
	})
	ftt.Run("Test no namespace is setup", t, func(t *ftt.Test) {
		md := metadata.Pairs("is_test", "true")
		assert.Loosely(t, GetIncomingCtxNamespace(metadata.NewIncomingContext(ctx, md)), should.Equal(BrowserNamespace))
	})
	ftt.Run("Test OSNamespace is set up", t, func(t *ftt.Test) {
		md := metadata.Pairs(Namespace, OSNamespace)
		assert.Loosely(t, GetIncomingCtxNamespace(metadata.NewIncomingContext(ctx, md)), should.Equal(OSNamespace))
	})
	ftt.Run("Test PartnerNamespace is set up", t, func(t *ftt.Test) {
		md := metadata.Pairs(Namespace, OSPartnerNamespace)
		assert.Loosely(t, GetIncomingCtxNamespace(metadata.NewIncomingContext(ctx, md)), should.Equal(OSPartnerNamespace))
	})
	ftt.Run("Test AndroidNamespace is set up", t, func(t *ftt.Test) {
		md := metadata.Pairs(Namespace, AndroidNamespace)
		assert.Loosely(t, GetIncomingCtxNamespace(metadata.NewIncomingContext(ctx, md)), should.Equal(AndroidNamespace))
	})
}

func TestDevicePhaseCoverage(t *testing.T) {
	ftt.Run("test the UFS ManufacturingConfig Phase mapping covers all UFS ManufacturingConfig Phase enum", t, func(t *ftt.Test) {
		got := make(map[string]bool, len(StrToDevicePhase))
		for _, v := range StrToDevicePhase {
			got[v] = true
		}
		for l := range ufsmfg.ManufacturingConfig_Phase_value {
			if l == ufsmfg.ManufacturingConfig_PHASE_INVALID.String() {
				continue
			}
			_, ok := got[l]
			assert.Loosely(t, ok, should.BeTrue)
		}
	})

	ftt.Run("test the UFS ManufacturingConfig Phase mapping doesn't cover any non-UFS ManufacturingConfig Phase enum", t, func(t *ftt.Test) {
		for _, v := range StrToDevicePhase {
			_, ok := ufsmfg.ManufacturingConfig_Phase_value[v]
			assert.Loosely(t, ok, should.BeTrue)
		}
	})

	ftt.Run("test ToUFSDevicePhase", t, func(t *ftt.Test) {
		t.Run("Test lowercase conversion", func(t *ftt.Test) {
			phase := ToUFSDevicePhase("evt")
			assert.Loosely(t, phase, should.Equal(ufsmfg.ManufacturingConfig_PHASE_EVT))
		})

		t.Run("Test uppercase conversion", func(t *ftt.Test) {
			phase := ToUFSDevicePhase("PVT")
			assert.Loosely(t, phase, should.Equal(ufsmfg.ManufacturingConfig_PHASE_PVT))
		})

		t.Run("Test phase with extended name", func(t *ftt.Test) {
			phase := ToUFSDevicePhase("PVT_EXTENDED")
			assert.Loosely(t, phase, should.Equal(ufsmfg.ManufacturingConfig_PHASE_PVT))
		})

		t.Run("Test phase with actual value in the middle", func(t *ftt.Test) {
			phase := ToUFSDevicePhase("IN_THE_MID_PVT_PHASE")
			assert.Loosely(t, phase, should.Equal(ufsmfg.ManufacturingConfig_PHASE_PVT))
		})

		t.Run("Test multiple phases matched - take first matching phase", func(t *ftt.Test) {
			phase := ToUFSDevicePhase("PVT_DVT2")
			assert.Loosely(t, phase, should.Equal(ufsmfg.ManufacturingConfig_PHASE_PVT))
		})

		t.Run("Test invalid conversion", func(t *ftt.Test) {
			phase := ToUFSDevicePhase("something-wrong")
			assert.Loosely(t, phase, should.Equal(ufsmfg.ManufacturingConfig_PHASE_INVALID))
		})
	})
}
