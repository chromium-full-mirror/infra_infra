// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"context"
	"fmt"
	"testing"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/util"
)

// makeDevCfgForTesting creates a basic DeviceConfig. These configs have no
// guarantee to make sense at a domain level, but can be used to verify code
// behavior.
func makeDevCfgForTesting(board, model, variant string, tams []string) *deviceconfig.Config {
	return &deviceconfig.Config{
		Id:  GetConfigID(board, model, variant),
		Tam: tams,
	}
}

// boardRealmAssigner just sets the realm to be equal to the board.
func boardRealmAssigner(c *deviceconfig.Config) string {
	return c.Id.PlatformId.Value
}

// grantRealmPerms grants `configuration.get` permissions in specified realms.
func grantRealmPerms(ctx context.Context, realms ...string) context.Context {
	perms := []authtest.RealmPermission{}

	for _, r := range realms {
		perms = append(perms, authtest.RealmPermission{
			Realm:      r,
			Permission: util.ConfigurationsGet,
		})
	}

	newCtx := auth.WithState(ctx, &authtest.FakeState{
		Identity:            "user:root@lab.com",
		IdentityPermissions: perms,
	})

	return newCtx
}

// constantRealmAssigner assigns all devices to a specific realm.
func constantRealmAssigner(d *deviceconfig.Config) string {
	return "chromeos:realm"
}

func TestBatchUpdateDeviceConfig(t *testing.T) {
	t.Parallel()
	baseCtx := memory.UseWithAppID(context.Background(), ("gae-test"))
	ctx := grantRealmPerms(baseCtx, "chromeos:realm")

	datastore.GetTestable(ctx).Consistent(true)

	ftt.Run("When a valid config is added", t, func(t *ftt.Test) {
		cfgs := make([]*deviceconfig.Config, 2)
		for i := 0; i < 2; i++ {
			cfgs[i] = makeDevCfgForTesting(fmt.Sprintf("board%d", i), fmt.Sprintf("model%d", i), fmt.Sprintf("variant%d", i), []string{fmt.Sprintf("test-%d", i)})
		}
		resp, err := BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{cfgs[0]}, constantRealmAssigner)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Resemble([]*deviceconfig.Config{cfgs[0]}))
		t.Run("That config is written to datastore", func(t *ftt.Test) {
			cfg0, err := GetDeviceConfigACL(ctx, GetConfigID("board0", "model0", "variant0"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cfg0, should.Resemble(cfgs[0]))
		})
		t.Run("When both that config and another config is added", func(t *ftt.Test) {
			resp, err := BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{cfgs[0], cfgs[1]}, constantRealmAssigner)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble([]*deviceconfig.Config{cfgs[0], cfgs[1]}))
			t.Run("Both configs are accessible", func(t *ftt.Test) {
				cfg0, err := GetDeviceConfigACL(ctx, GetConfigID("board0", "model0", "variant0"))
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, cfg0, should.Resemble(cfgs[0]))
				cfg1, err := GetDeviceConfigACL(ctx, GetConfigID("board1", "model1", "variant1"))
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, cfg1, should.Resemble(cfgs[1]))
			})
		})
	})

	ftt.Run("When an invalid config is added in a batch request", t, func(t *ftt.Test) {
		badCfg := &deviceconfig.Config{}
		goodCfg := makeDevCfgForTesting("board0", "model0", "variant", []string{"email"})

		resp, err := BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{badCfg, goodCfg}, constantRealmAssigner)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, resp, should.BeNil)
		t.Run("No configs from that request are added", func(t *ftt.Test) {
			_, err := GetDeviceConfigACL(ctx, GetConfigID("board0", "model0", "variant"))
			assert.Loosely(t, err, should.NotBeNil)
		})
	})

	ftt.Run("When inserting a config with a specific realm", t, func(t *ftt.Test) {
		cfg := makeDevCfgForTesting("board", "model", "variant", []string{"email"})

		// note boardRealmAssigner
		resp, err := BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{cfg}, constantRealmAssigner)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Resemble([]*deviceconfig.Config{cfg}))
		t.Run("Entity in datastore has correct realm", func(t *ftt.Test) {
			entity := &DeviceConfigEntity{
				ID: GetDeviceConfigIDStr(GetConfigID("board", "model", "variant")),
			}
			err := datastore.Get(ctx, entity)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, entity.Realm, should.Equal("chromeos:realm"))
		})
	})
}

func TestGetDeviceConfig(t *testing.T) {
	t.Parallel()
	baseCtx := memory.UseWithAppID(context.Background(), ("gae-test"))
	ctx := grantRealmPerms(baseCtx, "chromeos:board-model")

	cfg := makeDevCfgForTesting("board", "model", "variant", []string{"email"})

	ftt.Run("When a config is added", t, func(t *ftt.Test) {
		resp, err := BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{cfg}, BoardModelRealmAssigner)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Resemble([]*deviceconfig.Config{cfg}))
		t.Run("That config can be accessed", func(t *ftt.Test) {
			cfg_resp, err := GetDeviceConfigACL(ctx, GetConfigID("board", "model", "variant"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, cfg_resp, should.Resemble(cfg))
		})
		t.Run("Another config cannot be accessed", func(t *ftt.Test) {
			cfg_resp, err := GetDeviceConfigACL(ctx, GetConfigID("board2", "model2", "variant2"))
			assert.Loosely(t, cfg_resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("A config with an invalid ID cannot be accessed", func(t *ftt.Test) {
			cfg_resp, err := GetDeviceConfigACL(ctx, &deviceconfig.ConfigId{})
			assert.Loosely(t, cfg_resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
		t.Run("A config can't be accessed without permissions", func(t *ftt.Test) {
			otherPermsCtx := grantRealmPerms(baseCtx, "chromeos:other-board")
			cfg_resp, err := GetDeviceConfigACL(otherPermsCtx, GetConfigID("board", "model", "variant"))
			assert.Loosely(t, cfg_resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(PermissionDenied))
		})
	})
}

func TestDeviceConfigsExist(t *testing.T) {
	t.Parallel()
	baseCtx := memory.UseWithAppID(context.Background(), ("gae-test"))
	ctx := grantRealmPerms(baseCtx, "chromeos:board-model")

	cfg := makeDevCfgForTesting("board", "model", "variant", []string{"email"})
	cfg1 := makeDevCfgForTesting("board-hidden", "model-hidden", "variant", []string{"email"})

	ftt.Run("When a config is added", t, func(t *ftt.Test) {
		resp, err := BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{cfg, cfg1}, BoardModelRealmAssigner)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Resemble([]*deviceconfig.Config{cfg, cfg1}))
		t.Run("DeviceConfigsExist should correctly report that config exists, and other config does not", func(t *ftt.Test) {
			cfgIDs := []*deviceconfig.ConfigId{GetConfigID("board", "model", "variant"), GetConfigID("non", "existant", "config")}
			exists, err := DeviceConfigsExistACL(ctx, cfgIDs)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, exists, should.Resemble([]bool{true, false}))
		})
		t.Run("DeviceConfigsExist should only report that configs the user can see are returned", func(t *ftt.Test) {
			cfgIDs := []*deviceconfig.ConfigId{GetConfigID("board", "model", "variant"), GetConfigID("board-hidden", "model-hidden", "variant")}
			exists, err := DeviceConfigsExistACL(ctx, cfgIDs)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, exists, should.Resemble([]bool{true, false}))

			fullPermsCtx := grantRealmPerms(baseCtx, "chromeos:board-model", "chromeos:board-hidden-model-hidden")
			exists, err = DeviceConfigsExistACL(fullPermsCtx, cfgIDs)

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, exists, should.Resemble([]bool{true, true}))
		})
	})
}

func TestGetDeviceConfigIDStr(t *testing.T) {
	t.Parallel()

	ftt.Run("test full config", t, func(t *ftt.Test) {
		cfgID := GetConfigID("board", "model", "variant")
		id := GetDeviceConfigIDStr(cfgID)
		assert.Loosely(t, id, should.Equal("board.model.variant"))
	})
	ftt.Run("test board/model", t, func(t *ftt.Test) {
		cfgID := GetConfigID("board", "model", "")
		id := GetDeviceConfigIDStr(cfgID)
		assert.Loosely(t, id, should.Equal("board.model."))
	})
	ftt.Run("test empty config", t, func(t *ftt.Test) {
		cfgID := &deviceconfig.ConfigId{}
		id := GetDeviceConfigIDStr(cfgID)
		assert.Loosely(t, id, should.Equal(".."))
	})
}
