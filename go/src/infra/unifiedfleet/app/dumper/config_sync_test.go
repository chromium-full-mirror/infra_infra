// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"

	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/util"
)

// grantRealmPerms grants `configurations.get` permissions on all realms to the
// existing context.
func grantRealmPerms(ctx context.Context, realms ...string) context.Context {
	perms := []authtest.RealmPermission{}

	for _, r := range realms {
		perms = append(perms, authtest.RealmPermission{
			Realm:      r,
			Permission: util.ConfigurationsGet,
		})
		perms = append(perms, authtest.RealmPermission{
			Realm:      r,
			Permission: util.ConfigurationsDelete,
		})
	}

	newCtx := auth.WithState(ctx, &authtest.FakeState{
		Identity:            "user:root@lab.com",
		IdentityPermissions: perms,
	})

	return newCtx
}

// ConstantRealmAssigner assigns a single realm to all device configs.
func ConstantRealmAssigner(d *deviceconfig.Config) string {
	return "chromeos:realm"
}

// TestSyncDeviceConfigs verifies the e2e sync works as expected
func TestSyncDeviceConfigs(t *testing.T) {
	t.Parallel()

	ctx := memory.UseWithAppID(context.Background(), ("dev~infra-unified-fleet-system"))
	ctx = external.WithTestingContext(ctx)
	ctx = grantRealmPerms(ctx, "chromeos:board1-model1", "chromeos:board2-model2", "chromeos:realm")

	ftt.Run("When sync is run with a valid config", t, func(t *ftt.Test) {
		namespaceToRealmAssignerMap = map[string]configuration.RealmAssignerFunc{
			"random-ns":             configuration.BoardModelRealmAssigner,
			util.OSPartnerNamespace: ConstantRealmAssigner,
		}
		ctx = config.Use(ctx, &config.Config{
			DeviceConfigsPushConfigs: &config.DeviceConfigPushConfigs{
				ConfigsPath: "test_device_config",
				Enabled:     true,
			},
		})

		devCfg := &deviceconfig.Config{
			Id: configuration.GetConfigID("board3", "model3", ""),
		}
		_, err := configuration.BatchUpdateDeviceConfigs(ctx, []*deviceconfig.Config{devCfg}, ConstantRealmAssigner)
		assert.Loosely(t, err, should.BeNil)

		cfg, err := configuration.GetDeviceConfigACL(ctx, configuration.GetConfigID("board3", "model3", ""))
		assert.Loosely(t, cfg, should.Match(devCfg))
		assert.Loosely(t, err, should.BeNil)

		err = syncDeviceConfigs(ctx)
		assert.Loosely(t, err, should.BeNil)

		t.Run("DeviceConfigs should be fetchable in all namespaces specified", func(t *ftt.Test) {
			for ns := range namespaceToRealmAssignerMap {
				ctx, err := util.SetupDatastoreNamespace(ctx, ns)
				assert.Loosely(t, err, should.BeNil)

				cfg, err := configuration.GetDeviceConfigACL(ctx, configuration.GetConfigID("board1", "model1", ""))
				assert.Loosely(t, cfg, should.Match(expectedConfigs[0]))
				assert.Loosely(t, err, should.BeNil)
				cfg2, err := configuration.GetDeviceConfigACL(ctx, configuration.GetConfigID("board2", "model2", ""))
				assert.Loosely(t, cfg2, should.Match(expectedConfigs[1]))
				assert.Loosely(t, err, should.BeNil)
			}
		})
		t.Run("DeviceConfigs should only be fetchable in namespaces specified", func(t *ftt.Test) {
			ctx, err := util.SetupDatastoreNamespace(ctx, "fake")
			assert.Loosely(t, err, should.BeNil)

			cfg, err := configuration.GetDeviceConfigACL(ctx, configuration.GetConfigID("board1", "model1", ""))
			assert.Loosely(t, cfg, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			cfg2, err := configuration.GetDeviceConfigACL(ctx, configuration.GetConfigID("board2", "model2", ""))
			assert.Loosely(t, cfg2, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
		t.Run("Non-existing device config shouldn't be fetchable", func(t *ftt.Test) {
			for ns := range namespaceToRealmAssignerMap {
				ctx, err := util.SetupDatastoreNamespace(ctx, ns)
				assert.Loosely(t, err, should.BeNil)

				cfg, err := configuration.GetDeviceConfigACL(ctx, configuration.GetConfigID("board3", "model3", ""))
				assert.Loosely(t, cfg, should.BeNil)
				assert.Loosely(t, err, should.NotBeNil)
			}
		})
	})
}

// expectedConfigs contain the configs we read from the fake configs
// will be blank on any issue with the configs
var expectedConfigs = getExpectedConfigs()

func getExpectedConfigs() []*deviceconfig.Config {
	cfgs := &deviceconfig.AllConfigs{}
	content, err := os.ReadFile("../frontend/fake/device_config.cfg")
	if err != nil {
		return cfgs.Configs
	}
	if err := protojson.Unmarshal(content, cfgs); err != nil {
		return cfgs.Configs
	}
	return cfgs.Configs
}
