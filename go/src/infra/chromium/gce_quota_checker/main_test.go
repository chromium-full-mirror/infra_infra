// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	gceproviderpb "go.chromium.org/luci/gce/api/config/v1"
)

func generateVMConfig(project string, zone string, amountMax int32, network string, machineType string) *gceproviderpb.Configs {
	attributes := &gceproviderpb.VM{
		Project: project,
		NetworkInterface: []*gceproviderpb.NetworkInterface{
			{Network: network},
		},
		Zone:        zone,
		MachineType: machineType,
	}
	amount := &gceproviderpb.Amount{
		Max: amountMax,
	}
	config := &gceproviderpb.Config{
		Attributes: attributes,
		Amount:     amount,
	}
	configs := &gceproviderpb.Configs{
		Vms: []*gceproviderpb.Config{config},
	}
	return configs
}

func addExternalIP(configs ...*gceproviderpb.Configs) {
	for _, config := range configs {
		network := config.Vms[0].Attributes.NetworkInterface[0]
		network.AccessConfig = []*gceproviderpb.AccessConfig{
			{},
		}
	}
}

func addDisks(configs *gceproviderpb.Configs, hddGB int64, remoteSSDGB int64, localSSDGB int64) {
	attributes := configs.Vms[0].Attributes
	if hddGB > 0 {
		newDisk := &gceproviderpb.Disk{
			Size: hddGB,
		}
		attributes.Disk = append(attributes.Disk, newDisk)
	}
	if remoteSSDGB > 0 {
		newDisk := &gceproviderpb.Disk{
			Size: remoteSSDGB,
			Type: "zones/{{.Zone}}/diskTypes/pd-ssd",
		}
		attributes.Disk = append(attributes.Disk, newDisk)
	}
	if localSSDGB > 0 {
		newDisk := &gceproviderpb.Disk{
			Size: localSSDGB,
			Type: "zones/{{.Zone}}/diskTypes/local-ssd",
		}
		attributes.Disk = append(attributes.Disk, newDisk)
	}
}

func writeConfigs(t *ftt.Test, configs ...*gceproviderpb.Configs) []string {
	tmpDir := t.TempDir()
	var configPaths []string
	for i, config := range configs {
		blob, err := prototext.Marshal(config)
		assert.Loosely(t, err, should.BeNil)
		configPath := path.Join(tmpDir, fmt.Sprintf("config%d.cfg", i))
		err = os.WriteFile(configPath, blob, 0666)
		assert.Loosely(t, err, should.BeNil)
		configPaths = append(configPaths, configPath)
	}
	return configPaths
}

// initMaps creates and returns all the quota maps, with empty quota entries
// created for all possible regions and networks. This would normally be done
// for us via the get*Quotas() functions, but those aren't covered in tests
// here, so need to do this ourselves.
func initMaps(possibleRegions []string, possibleNetworks []string, possibleFamilies []string) (map[string]*regionQuotas, map[string]*quotaVals) {
	quotasPerRegion := make(map[string]*regionQuotas)
	quotasPerNetwork := make(map[string]*quotaVals)
	for _, region := range possibleRegions {
		quotasPerRegion[region] = &regionQuotas{
			localSSDPerFamilyQuota: make(map[string]*quotaVals),
			cpusPerFamilyQuota:     make(map[string]*quotaVals),
		}
		for _, family := range possibleFamilies {
			quotasPerRegion[region].localSSDPerFamilyQuota[family] = &quotaVals{}
			if family != "n1" && family != "e2" {
				quotasPerRegion[region].cpusPerFamilyQuota[family] = &quotaVals{}
			}
		}
	}
	for _, network := range possibleNetworks {
		quotasPerNetwork[network] = &quotaVals{}
	}
	return quotasPerRegion, quotasPerNetwork
}

func TestParseCfgFiles(t *testing.T) {
	t.Parallel()

	possibleRegions := []string{"us-east1", "us-west1", "us-central1"}
	possibleNetworks := []string{"networkA", "networkB"}
	possibleFamilies := []string{"g1", "n1", "n2", "e2"}

	ftt.Run("test multiple projects", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		region, zone := possibleRegions[0], fmt.Sprintf("%s-a", possibleRegions[0])
		configs := []*gceproviderpb.Configs{
			generateVMConfig("projectA", zone, 1, possibleNetworks[0], "g1-small"),
			generateVMConfig("projectB", zone, 10, possibleNetworks[0], "g1-small"),
		}
		configPaths := writeConfigs(t, configs...)

		parseCfgFiles("projectA", configPaths, possibleRegions, quotasPerRegion, quotasPerNetwork)

		assert.Loosely(t, quotasPerRegion[region].instancesQuota.starlarkUsage, should.Equal(1))
	})

	ftt.Run("test multiple zones and regions", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		region1, region2 := possibleRegions[0], possibleRegions[1]
		zone1a, zone1b := fmt.Sprintf("%s-a", region1), fmt.Sprintf("%s-b", region1)
		zone2a, zone2b := fmt.Sprintf("%s-a", region2), fmt.Sprintf("%s-b", region2)
		configs := []*gceproviderpb.Configs{
			generateVMConfig("project", zone1a, 1, possibleNetworks[0], "g1-small"),
			generateVMConfig("project", zone1b, 2, possibleNetworks[0], "g1-small"),
			generateVMConfig("project", zone2a, 10, possibleNetworks[0], "g1-small"),
			generateVMConfig("project", zone2b, 20, possibleNetworks[0], "g1-small"),
		}
		configPaths := writeConfigs(t, configs...)

		parseCfgFiles("project", configPaths, possibleRegions, quotasPerRegion, quotasPerNetwork)

		assert.Loosely(t, quotasPerRegion[region1].instancesQuota.starlarkUsage, should.Equal(3))
		assert.Loosely(t, quotasPerRegion[region2].instancesQuota.starlarkUsage, should.Equal(30))
	})

	ftt.Run("test networks", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		zone := fmt.Sprintf("%s-a", possibleRegions[0])
		network1, network2 := possibleNetworks[0], possibleNetworks[1]
		configs := []*gceproviderpb.Configs{
			generateVMConfig("project", zone, 1, network1, "g1-small"),
			generateVMConfig("project", zone, 1, network1, "g1-small"),
			generateVMConfig("project", zone, 1, network1, "g1-small"),
			generateVMConfig("project", zone, 1, network2, "g1-small"),
			// Instances for "random-network" should be gracefully
			// ignored since GCE didn't report a quota for it.
			generateVMConfig("project", zone, 1, "random-network", "g1-small"),
		}
		configPaths := writeConfigs(t, configs...)

		parseCfgFiles("project", configPaths, possibleRegions, quotasPerRegion, quotasPerNetwork)

		assert.Loosely(t, quotasPerNetwork[network1].starlarkUsage, should.Equal(3))
		assert.Loosely(t, quotasPerNetwork[network2].starlarkUsage, should.Equal(1))
	})

	ftt.Run("test IP addresses", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		region1, zone1a := possibleRegions[0], fmt.Sprintf("%s-a", possibleRegions[0])
		region2, zone2a := possibleRegions[1], fmt.Sprintf("%s-a", possibleRegions[1])
		network1, network2 := possibleNetworks[0], possibleNetworks[1]
		configs := []*gceproviderpb.Configs{
			generateVMConfig("project", zone1a, 1, network1, "g1-small"),
			generateVMConfig("project", zone1a, 1, network1, "g1-small"),
			generateVMConfig("project", zone1a, 1, network1, "g1-small"),
			generateVMConfig("project", zone1a, 10, network2, "g1-small"),
			generateVMConfig("project", zone2a, 100, network2, "g1-small"),
		}
		// Add an "external IP" to all instances but one.
		addExternalIP(configs[0], configs[1], configs[3], configs[4])
		configPaths := writeConfigs(t, configs...)

		parseCfgFiles("project", configPaths, possibleRegions, quotasPerRegion, quotasPerNetwork)

		assert.Loosely(t, quotasPerRegion[region1].ipsQuota.starlarkUsage, should.Equal(12))
		assert.Loosely(t, quotasPerRegion[region2].ipsQuota.starlarkUsage, should.Equal(100))
	})

	ftt.Run("test core count", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		region1, zone1a := possibleRegions[0], fmt.Sprintf("%s-a", possibleRegions[0])
		region2, zone2a := possibleRegions[1], fmt.Sprintf("%s-a", possibleRegions[1])
		network := possibleNetworks[0]
		configs := []*gceproviderpb.Configs{
			// N1 in region1: 1x2-core, 2x4-core, 4x8-core = 42 cores
			generateVMConfig("project", zone1a, 1, network, "n1-standard-2"),
			generateVMConfig("project", zone1a, 2, network, "n1-standard-4"),
			generateVMConfig("project", zone1a, 4, network, "n1-standard-8"),
			// N2 in region1: 2x4-core, 4x8-core = 40 cores
			generateVMConfig("project", zone1a, 2, network, "n2-standard-4"),
			generateVMConfig("project", zone1a, 4, network, "n2-standard-8"),
			// E2 in region1: 8x32-core = 256 cores
			generateVMConfig("project", zone1a, 8, network, "e2-standard-32"),
			// G1 in region1: 3x1-core
			generateVMConfig("project", zone1a, 3, network, "g1-small"),
			// In region2: 100x8-core N1, 100x8-core N2, 100x8-core E2.
			generateVMConfig("project", zone2a, 100, network, "n1-standard-8"),
			generateVMConfig("project", zone2a, 100, network, "n2-standard-8"),
			generateVMConfig("project", zone2a, 100, network, "e2-standard-8"),
		}
		configPaths := writeConfigs(t, configs...)

		parseCfgFiles("project", configPaths, possibleRegions, quotasPerRegion, quotasPerNetwork)

		assert.Loosely(t, quotasPerRegion[region1].cpusQuota.starlarkUsage, should.Equal(341))
		assert.Loosely(t, quotasPerRegion[region1].cpusPerFamilyQuota["n2"].starlarkUsage, should.Equal(40))
		assert.Loosely(t, quotasPerRegion[region1].cpusPerFamilyQuota["g1"].starlarkUsage, should.Equal(3))
		assert.Loosely(t, quotasPerRegion[region2].cpusQuota.starlarkUsage, should.Equal(2400))
		assert.Loosely(t, quotasPerRegion[region2].cpusPerFamilyQuota["n2"].starlarkUsage, should.Equal(800))
	})

	ftt.Run("test disks", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		region1, zone1a := possibleRegions[0], fmt.Sprintf("%s-a", possibleRegions[0])
		region2, zone2a := possibleRegions[1], fmt.Sprintf("%s-a", possibleRegions[1])
		network := possibleNetworks[0]
		configs := []*gceproviderpb.Configs{
			generateVMConfig("project", zone1a, 1, network, "g1-small"),
			generateVMConfig("project", zone1a, 10, network, "g1-small"),
			generateVMConfig("project", zone2a, 100, network, "g1-small"),
			generateVMConfig("project", zone2a, 100, network, "g1-small"),
			generateVMConfig("project", zone2a, 100, network, "g1-small"),
		}
		// 1 region1 instance with 10GB HDD + 20GB remote SSD + 30GB local SSD
		addDisks(configs[0], 10, 20, 30)
		// 10 region1 instances with 100GB HDD + 200GB remote SSD + 300GB local SSD
		addDisks(configs[1], 100, 200, 300)
		// 100 region2 instances with 5GB HDD + 0GB remote SSD + 0GB local SSD
		addDisks(configs[2], 5, 0, 0)
		// 100 region2 instances with 0GB HDD + 6GB remote SSD + 0GB local SSD
		addDisks(configs[3], 0, 6, 0)
		// 100 region2 instances with 0GB HDD + 0GB remote SSD + 7GB local SSD
		addDisks(configs[4], 0, 0, 7)
		configPaths := writeConfigs(t, configs...)

		parseCfgFiles("project", configPaths, possibleRegions, quotasPerRegion, quotasPerNetwork)

		assert.Loosely(t, quotasPerRegion[region1].hddQuota.starlarkUsage, should.Equal(1010))
		assert.Loosely(t, quotasPerRegion[region1].remoteSSDQuota.starlarkUsage, should.Equal(2020))
		assert.Loosely(t, quotasPerRegion[region1].localSSDPerFamilyQuota["g1"].starlarkUsage, should.Equal(3030))

		assert.Loosely(t, quotasPerRegion[region2].hddQuota.starlarkUsage, should.Equal(500))
		assert.Loosely(t, quotasPerRegion[region2].remoteSSDQuota.starlarkUsage, should.Equal(600))
		assert.Loosely(t, quotasPerRegion[region2].localSSDPerFamilyQuota["g1"].starlarkUsage, should.Equal(700))
	})

}

func TestFindQuotaErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	possibleRegions := []string{"us-east1"}
	possibleNetworks := []string{"networkA"}
	possibleFamilies := []string{"n1"}

	ftt.Run("cpu cutoff", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)

		// 90 of 100 shouldn't be an error.
		quotasPerRegion["us-east1"].cpusQuota.max = 100
		quotasPerRegion["us-east1"].cpusQuota.starlarkUsage = 90
		quotasPerRegion["us-east1"].cpusQuota.desc = "cpus"
		assert.Loosely(t, findQuotaErrors(ctx, quotasPerRegion, quotasPerNetwork, 100.0, false), should.BeEmpty)

		// 100 of 100 shouldn't be an error.
		quotasPerRegion["us-east1"].cpusQuota.starlarkUsage = 100
		assert.Loosely(t, findQuotaErrors(ctx, quotasPerRegion, quotasPerNetwork, 100.0, false), should.BeEmpty)

		// 101 of 100 should be an error.
		quotasPerRegion["us-east1"].cpusQuota.starlarkUsage = 101
		assert.Loosely(t, findQuotaErrors(ctx, quotasPerRegion, quotasPerNetwork, 100.0, false), should.Match([]string{"cpus at 101.00% (101 of 100)"}))
	})

	ftt.Run("local ssd check", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		quotasPerRegion["us-east1"].localSSDPerFamilyQuota["n1"].max = 1000
		quotasPerRegion["us-east1"].localSSDPerFamilyQuota["n1"].starlarkUsage = 2000
		quotasPerRegion["us-east1"].localSSDPerFamilyQuota["n1"].desc = "local ssd"
		assert.Loosely(t, findQuotaErrors(ctx, quotasPerRegion, quotasPerNetwork, 100.0, false), should.Match([]string{"local ssd at 200.00% (2000 of 1000)"}))
	})

	ftt.Run("network check", t, func(t *ftt.Test) {
		quotasPerRegion, quotasPerNetwork := initMaps(possibleRegions, possibleNetworks, possibleFamilies)
		quotasPerNetwork["networkA"].max = 100
		quotasPerNetwork["networkA"].starlarkUsage = 95
		quotasPerNetwork["networkA"].desc = "networkA"
		// 100% cut off at 95% usage shouldn't be an error.
		assert.Loosely(t, findQuotaErrors(ctx, quotasPerRegion, quotasPerNetwork, 100.0, false), should.BeEmpty)
		// 90% cut off at 95% usage should be an error.
		assert.Loosely(t, findQuotaErrors(ctx, quotasPerRegion, quotasPerNetwork, 90.0, false), should.Match([]string{"networkA at 95.00% (95 of 100)"}))
	})
}
