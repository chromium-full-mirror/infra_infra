// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
	configpb "go.chromium.org/luci/swarming/proto/config"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
)

var branchNumber uint32 = 0

// encTestingContext creates a testing context which mocks the logging and datastore services.
// Also loads a custom config, which will allow the loading of a dummy bot config file
func encTestingContext() context.Context {
	c := gaetesting.TestingContextWithAppID("dev~infra-unified-fleet-system")
	c = gologger.StdConfig.Use(c)
	c = logging.SetLevel(c, logging.Error)
	c = config.Use(c, &config.Config{})
	c = external.WithTestingContext(c)
	datastore.GetTestable(c).Consistent(true)
	return c
}

// Dummy UFS config with ownership config
func mockOwnershipConfig() *config.Config {
	return &config.Config{
		OwnershipConfig: &config.OwnershipConfig{
			GitilesHost: "test_gitiles",
			Project:     "test_project",
			Branch:      fmt.Sprintf("test_branch_%d", atomic.AddUint32(&branchNumber, 1)),
			SecurityConfig: []*config.OwnershipConfig_ConfigFile{
				{
					Name:       "test_name",
					RemotePath: "test_security_git_path",
				},
			},
		},
	}
}

// Dummy config for bots
func mockBotConfig(botRange string, pool string) *configpb.BotsCfg {
	return &configpb.BotsCfg{
		BotGroup: []*configpb.BotGroup{
			{
				BotId:      []string{botRange},
				Dimensions: []string{"pool:" + pool},
			},
		},
	}
}

// Dummy security config for bots
func mockSecurityConfig(botRange string, pool string, swarmingServerId string, customer string, securityLevel string, builders string) *ufspb.SecurityInfos {
	return &ufspb.SecurityInfos{
		Pools: []*ufspb.SecurityInfo{
			{
				Hosts:            []string{botRange},
				PoolName:         pool,
				SwarmingServerId: swarmingServerId,
				Customer:         customer,
				SecurityLevel:    securityLevel,
				Builders:         []string{builders},
			},
		},
	}
}

// Dummy ChromeBrowserMachine
func mockChromeBrowserMachine(id, name string) *ufspb.Machine {
	return &ufspb.Machine{
		Name: id,
		Device: &ufspb.Machine_ChromeBrowserMachine{
			ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
				Description: name,
			},
		},
	}
}

// Dummy MachineLSE
func MockMachineLSE(id string) *ufspb.MachineLSE {
	return &ufspb.MachineLSE{
		Name: id,
	}
}

// Tests the functionality for fetching the config file and importing the configs
// No t.Parallel(): The happy path test relies on comparing a global variable for sha1 hash testing.
// A race condition arises in which another test can modify this variable before the test finishes.
// In addition, the test suite hangs or panics when it reaches the timestamp deep equal assertion.
// TODO(b/265826661): Fix hang/panic with accessing test context and re-enable parallelization
func TestImportBotConfigs(t *testing.T) {
	ctx := encTestingContext()
	ftt.Run("Import Bot Configs", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		ctx = config.Use(ctx, contextConfig)
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("test1-1", "test1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)

			resp, err = registration.GetMachine(ctx, "test1-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)

			botResp, err := inventory.GetOwnershipData(ctx, "test1-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, botResp, should.NotBeNil)
			assert.Loosely(t, botResp.OwnershipData, should.NotBeNil)
			p, err := botResp.GetProto()
			assert.Loosely(t, err, should.BeNil)
			pm := p.(*ufspb.OwnershipData)
			assert.Loosely(t, pm, should.Match(resp.Ownership))

			// Import Again, should not update the Asset
			err = ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			resp2, err := registration.GetMachine(ctx, "test1-1")
			assert.Loosely(t, resp2, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp2.Ownership, should.NotBeNil)
			assert.Loosely(t, resp2.GetUpdateTime(), should.Match(resp.GetUpdateTime()))
		})
	})
}

// Tests the functionality for fetching the config file and importing the configs at a particular commit hash
func TestGetBotConfigsForCommitSh(t *testing.T) {
	ctx := encTestingContext()
	t.Parallel()
	ftt.Run("Get Bot Configs For CommitSh", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		ctx = config.Use(ctx, contextConfig)
		t.Run("happy path", func(t *ftt.Test) {
			ownerships, err := GetBotConfigsForCommitSh(ctx, "5201756875e0405c5c44d0e6d97de653b0d6cfca")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownerships, should.NotBeNil)
			assert.Loosely(t, len(ownerships), should.NotEqual(0))
		})
		t.Run("unknown commitsh", func(t *ftt.Test) {
			ownerships, err := GetBotConfigsForCommitSh(ctx, "blah")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ownerships, should.BeNil)
		})
		t.Run("empty commitsh", func(t *ftt.Test) {
			ownerships, err := GetBotConfigsForCommitSh(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ownerships, should.BeNil)
		})
	})
}

// Tests the functionality for reading config files and fetching git client
// No t.Parallel(): The happy path test relies on comparing a global variable for sha1 hash testing.
// A race condition arises in which another test can modify this variable before the test finishes.
func TestGetConfigAndGitClient(t *testing.T) {
	ctx := encTestingContext()
	ftt.Run("Get Ownership Config and Git Client", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		ctx = config.Use(ctx, contextConfig)
		t.Run("happy path", func(t *ftt.Test) {
			ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownershipConfig, should.NotBeNil)
			assert.Loosely(t, gitClient, should.NotBeNil)

			// Fetch Again, should not return ownership config
			ownershipConfig, gitClient, err = GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownershipConfig, should.BeNil)
			assert.Loosely(t, gitClient, should.BeNil)
		})
		t.Run("No Ownership Config - Ownership not updated", func(t *ftt.Test) {
			ctx = config.Use(ctx, &config.Config{})
			ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, ownershipConfig, should.BeNil)
			assert.Loosely(t, gitClient, should.BeNil)
		})
	})
}

// Tests the functionality for getting all the bot security configs
func TestGetAllOwnershipConfigs(t *testing.T) {
	t.Parallel()
	ctx := encTestingContext()
	ftt.Run("Get all ownership Configs", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		ctx = config.Use(ctx, contextConfig)
		ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
		assert.Loosely(t, err, should.BeNil)
		t.Run("happy path", func(t *ftt.Test) {
			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)
			ownerships, _, err := ListOwnershipConfigs(ctx, 20, "", "", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownerships, should.NotBeNil)
			assert.Loosely(t, len(ownerships), should.Equal(14))
		})
	})
}

// Tests the functionality for importing security configs from the config files
func TestImportSecurityConfig(t *testing.T) {
	t.Parallel()
	ftt.Run("Import Security Config", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		t.Run("happy path", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("test1-1", "test1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)

			resp, err = registration.GetMachine(ctx, "test1-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)

			botResp, err := inventory.GetOwnershipData(ctx, "test1-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, botResp, should.NotBeNil)
			assert.Loosely(t, botResp.OwnershipData, should.NotBeNil)
			p, err := botResp.GetProto()
			assert.Loosely(t, err, should.BeNil)
			pm := p.(*ufspb.OwnershipData)
			assert.Loosely(t, pm, should.Match(resp.Ownership))
			assert.Loosely(t, pm.GetPools(), should.Match([]string{"abc"}))

			// Update ownership and Import Again, should update the ownership to the original value
			pm.Pools = []string{"dummy"}
			registration.UpdateMachineOwnership(ctx, "test1-1", pm)
			entity, err := inventory.PutOwnershipData(ctx, pm, "test1-1", inventory.AssetTypeMachine)
			assert.Loosely(t, err, should.BeNil)
			p, err = entity.GetProto()
			assert.Loosely(t, err, should.BeNil)
			pm = p.(*ufspb.OwnershipData)
			assert.Loosely(t, strings.Join(pm.Pools, ";"), should.NotEqual(strings.Join(resp.Ownership.Pools, ";")))

			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)
			resp2, err := registration.GetMachine(ctx, "test1-1")

			assert.Loosely(t, resp2, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp2.Ownership, should.NotBeNil)
			assert.Loosely(t, resp2.Ownership, should.Match(resp.Ownership))
		})
		t.Run("happy path - Bot ID Prefix", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("testing-1", "testing1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			resp, err = registration.CreateMachine(ctx, mockChromeBrowserMachine("tester-1", "tester1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)

			resp, err = registration.GetMachine(ctx, "testing-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)

			resp, err = registration.GetMachine(ctx, "tester-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.BeNil)
		})
		t.Run("happy path - Bot ID Prefix for MachineLSE", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			resp, err := inventory.CreateMachineLSE(ctx, MockMachineLSE("testLSE1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)

			resp, err = inventory.GetMachineLSE(ctx, "testLSE1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)

			// Import Again, should not update the Asset
			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)
			resp2, err := inventory.GetMachineLSE(ctx, "testLSE1")
			assert.Loosely(t, resp2, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp2.Ownership, should.NotBeNil)
			assert.Loosely(t, resp2.GetUpdateTime(), should.Match(resp.GetUpdateTime()))
		})
		t.Run("happy path - Bot ID Prefix for VM", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
			assert.Loosely(t, err, should.BeNil)
			vm1 := &ufspb.VM{
				Name: "vm-1",
			}
			_, err = inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)

			resp, err := inventory.GetVM(ctx, "vm-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)

			// Import Again, should not update the Asset
			err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
			assert.Loosely(t, err, should.BeNil)
			resp2, err := inventory.GetVM(ctx, "vm-1")
			assert.Loosely(t, resp2, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp2.Ownership, should.NotBeNil)
			assert.Loosely(t, resp2.GetUpdateTime(), should.Match(resp.GetUpdateTime()))
		})
	})
}

// Tests the functionality for parsing and storing bot security configs in Datastore
func TestParseSecurityConfig(t *testing.T) {
	t.Parallel()
	ctx := encTestingContext()
	ftt.Run("Parse Security Config", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		ctx = config.Use(ctx, contextConfig)
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("test1-1", "test1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			ParseSecurityConfig(ctx, mockSecurityConfig("test{1,2}-1", "abc", "testSwarming", "customer", "trusted", "builder"))

			resp, err = registration.GetMachine(ctx, "test1-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)
			assert.Loosely(t, resp.Ownership.Pools, should.Contain("abc"))
			assert.Loosely(t, resp.Ownership.SwarmingInstance, should.Equal("testSwarming"))
			assert.Loosely(t, resp.Ownership.Customer, should.Equal("customer"))
			assert.Loosely(t, resp.Ownership.SecurityLevel, should.Equal("trusted"))
			assert.Loosely(t, resp.Ownership.Builders, should.Match([]string{"builder"}))
		})
		t.Run("Does not update non existent bots", func(t *ftt.Test) {
			ParseSecurityConfig(ctx, mockSecurityConfig("test{2,3}-1", "abc", "testSwarming", "customer", "trusted", "builder"))

			resp, err := registration.GetMachine(ctx, "test2-1")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("NotFound"))
		})
		t.Run("Cleans up stale configs", func(t *ftt.Test) {
			ownership := &ufspb.OwnershipData{PoolName: "pool1"}
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("test10-1", "test1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.Ownership, should.BeNil)
			assert.Loosely(t, err, should.BeNil)

			resp, err = registration.UpdateMachineOwnership(ctx, "test10-1", ownership)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)

			_, err = inventory.PutOwnershipData(ctx, ownership, "test10-1", inventory.AssetTypeMachine)
			assert.Loosely(t, err, should.BeNil)

			ParseSecurityConfig(ctx, mockSecurityConfig("test{5,6}-1", "abc", "testSwarming", "customer", "trusted", "builder"))

			configResp, err := inventory.GetOwnershipData(ctx, "test10-1")
			assert.Loosely(t, configResp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("NotFound"))
			resp, err = registration.GetMachine(ctx, "test10-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.BeNil)
		})
		t.Run("updates asset tables on subsequent runs", func(t *ftt.Test) {
			// First parse security configs
			ParseSecurityConfig(ctx, mockSecurityConfig("test{100,102}-1", "abc", "testSwarming", "customer", "trusted", "builder"))

			// Add a machine entry after parsing security configs
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("test100-1", "test100"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			// Machine entry should not have ownership data
			resp, err = registration.GetMachine(ctx, "test100-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.BeNil)

			// Prase security configs again as a subsequent dumper job
			ParseSecurityConfig(ctx, mockSecurityConfig("test{100,102}-1", "abc", "testSwarming", "customer", "trusted", "builder"))

			// Machine entry should now have ownership data
			resp, err = registration.GetMachine(ctx, "test100-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)
			assert.Loosely(t, resp.Ownership.Pools, should.Contain("abc"))
			assert.Loosely(t, resp.Ownership.SwarmingInstance, should.Equal("testSwarming"))
			assert.Loosely(t, resp.Ownership.Customer, should.Equal("customer"))
			assert.Loosely(t, resp.Ownership.SecurityLevel, should.Equal("trusted"))
			assert.Loosely(t, resp.Ownership.Builders, should.Match([]string{"builder"}))

			// Clear machine entry ownership data
			registration.UpdateMachineOwnership(ctx, "test100-1", nil)
			resp, err = registration.GetMachine(ctx, "test100-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.BeNil)

			// Prase security configs again as a subsequent dumper job
			ParseSecurityConfig(ctx, mockSecurityConfig("test{100,102}-1", "abc", "testSwarming", "customer", "trusted", "builder"))

			// Machine entry should again have ownership data
			resp, err = registration.GetMachine(ctx, "test100-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Ownership, should.NotBeNil)
		})
	})
}

// Tests the functionality for parsing botId strings
func TestParseBotIds(t *testing.T) {
	t.Parallel()
	ftt.Run("Parse ENC Bot Config", t, func(t *ftt.Test) {
		t.Run("Parse comma separated and ranges", func(t *ftt.Test) {
			ids := parseBotIds("mac{9,10..11,12}-483")
			assert.Loosely(t, ids, should.Match([]string{"mac9-483", "mac10-483", "mac11-483", "mac12-483"}))
		})
		t.Run("Parse multiple ranges", func(t *ftt.Test) {
			ids := parseBotIds("mac{9,10..11,18..20}-483")
			assert.Loosely(t, ids, should.Match([]string{"mac9-483", "mac10-483", "mac11-483", "mac18-483", "mac19-483", "mac20-483"}))
		})
		t.Run("Parse invalid range - ignores invalid range", func(t *ftt.Test) {
			ids := parseBotIds("mac{9,10..11,22..20}-483")
			assert.Loosely(t, ids, should.Match([]string{"mac9-483", "mac10-483", "mac11-483"}))
		})
		t.Run("Parse mal formed range - ignores malformed range", func(t *ftt.Test) {
			ids := parseBotIds("mac{9,10..11,..20}-483")
			assert.Loosely(t, ids, should.Match([]string{"mac9-483", "mac10-483", "mac11-483"}))
		})
		t.Run("Parse non digit characters in range - ignores", func(t *ftt.Test) {
			ids := parseBotIds("mac{9,10,11..a}-483")
			assert.Loosely(t, ids, should.Match([]string{"mac9-483", "mac10-483"}))
		})
	})
}

// Tests the functionality for getting ownership data for a machine/vm/machineLSE
func TestGetOwnershipData(t *testing.T) {
	t.Parallel()
	ftt.Run("GetOwnership Data", t, func(t *ftt.Test) {
		contextConfig := mockOwnershipConfig()
		t.Run("happy path - machine", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			resp, err := registration.CreateMachine(ctx, mockChromeBrowserMachine("testing-1", "testing1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			ownership, err := GetOwnershipData(ctx, "testing-1")

			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownership, should.NotBeNil)
			assert.Loosely(t, ownership.Pools, should.Contain("test"))
			assert.Loosely(t, ownership.SwarmingInstance, should.Equal("testSwarming"))
		})
		t.Run("happy path - vm", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			vm1 := &ufspb.VM{
				Name: "vm-1",
			}
			_, err := inventory.BatchUpdateVMs(ctx, []*ufspb.VM{vm1})
			assert.Loosely(t, err, should.BeNil)

			err = ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			ownership, err := GetOwnershipData(ctx, "vm-1")

			assert.Loosely(t, ownership, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownership.Pools, should.Contain("test"))
			assert.Loosely(t, ownership.SwarmingInstance, should.Equal("testSwarming"))
		})
		t.Run("happy path - machineLSE", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			resp, err := inventory.CreateMachineLSE(ctx, MockMachineLSE("testLSE1"))
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			err = ImportBotConfigs(ctx)
			assert.Loosely(t, err, should.BeNil)
			ownership, err := GetOwnershipData(ctx, "testLSE1")

			assert.Loosely(t, ownership, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ownership.Pools, should.Contain("test"))
			assert.Loosely(t, ownership.SwarmingInstance, should.Equal("testSwarming"))
		})
		t.Run("missing host in inventory", func(t *ftt.Test) {
			ctx := encTestingContext()
			ctx = config.Use(ctx, contextConfig)
			ParseSecurityConfig(ctx, mockSecurityConfig("test{2,3}-1", "abc", "testSwarming", "trusted", "customer", "builder"))
			ownership, err := GetOwnershipData(ctx, "blah4-1")
			s, _ := status.FromError(err)

			assert.Loosely(t, ownership, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, s.Code(), should.Equal(codes.NotFound))
		})
	})
}
