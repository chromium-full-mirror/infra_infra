// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/frontend/fake"
)

func TestParseATLTopology(t *testing.T) {
	ftt.Run("Verify ParseATLTopology", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := fake.SheetData("../frontend/fake/sheet_data.json")
			assert.Loosely(t, err, should.BeNil)
			topology, _ := ParseATLTopology(resp)
			assert.Loosely(t, topology, should.HaveLength(2))
			assert.Loosely(t, topology, should.ContainKey("100.115.224.0"))
			assert.Loosely(t, topology, should.ContainKey("100.115.226.0"))
			for k, vlan := range topology {
				switch k {
				case "100.115.224.0":
					assert.Loosely(t, vlan.GetName(), should.Equal("atl:201"))
					assert.Loosely(t, vlan.GetCapacityIp(), should.Equal(510))
					assert.Loosely(t, vlan.GetVlanAddress(), should.Equal("100.115.224.0/23"))
					assert.Loosely(t, vlan.GetDescription(), should.Equal("ATL-DUT-Row1_2"))
				case "100.115.226.0":
					assert.Loosely(t, vlan.GetName(), should.Equal("atl:202"))
					assert.Loosely(t, vlan.GetCapacityIp(), should.Equal(510))
					assert.Loosely(t, vlan.GetVlanAddress(), should.Equal("100.115.226.0/23"))
					assert.Loosely(t, vlan.GetDescription(), should.Equal("ATL-DUT-Row3_4"))
				}
			}

		})
	})
}

func TestParseOSDhcpdConf(t *testing.T) {
	ftt.Run("Verify ParseOSDhcpdConf", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := fake.SheetData("../frontend/fake/sheet_data.json")
			assert.Loosely(t, err, should.BeNil)
			topology, _ := ParseATLTopology(resp)
			b, err := fake.GitData("../frontend/fake/dhcp_test.conf")
			assert.Loosely(t, err, should.BeNil)

			parsed, err := ParseOSDhcpdConf(string(b), topology)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, parsed.ValidVlans, should.HaveLength(2))
			assert.Loosely(t, parsed.ValidIPs, should.HaveLength(510))
			ipMaps := make(map[string]*ufspb.IP, 0)
			for _, ip := range parsed.ValidIPs {
				ipMaps[ip.GetId()] = ip
			}
			assert.Loosely(t, len(ipMaps), should.Equal(510))
			ip, ok := ipMaps["atl:201/100.115.224.1"]
			assert.Loosely(t, ok, should.BeTrue)
			assert.Loosely(t, ip.GetOccupied(), should.BeTrue)
			assert.Loosely(t, ip.GetVlan(), should.Equal("atl:201"))
			ip2, ok := ipMaps["atl:201/100.115.224.2"]
			assert.Loosely(t, ok, should.BeTrue)
			assert.Loosely(t, ip2.GetOccupied(), should.BeTrue)
			assert.Loosely(t, ip2.GetVlan(), should.Equal("atl:201"))
			ip3, ok := ipMaps["atl:201/100.115.224.3"]
			assert.Loosely(t, ok, should.BeTrue)
			assert.Loosely(t, ip3.GetOccupied(), should.BeTrue)
			assert.Loosely(t, ip3.GetVlan(), should.Equal("atl:201"))

			assert.Loosely(t, parsed.ValidDHCPs, should.HaveLength(3))
			for _, dhcp := range parsed.ValidDHCPs {
				assert.Loosely(t, []string{"host1", "host2", "host3"}, should.Contain(dhcp.GetHostname()))
				switch dhcp.GetHostname() {
				case "host1":
					assert.Loosely(t, dhcp.GetIp(), should.Equal("100.115.224.1"))
					assert.Loosely(t, dhcp.GetMacAddress(), should.Equal("aa:00:00:00:00:00"))
				case "host2":
					assert.Loosely(t, dhcp.GetIp(), should.Equal("100.115.224.2"))
					assert.Loosely(t, dhcp.GetMacAddress(), should.BeEmpty)
				case "host3":
					assert.Loosely(t, dhcp.GetIp(), should.Equal("100.115.224.3"))
					assert.Loosely(t, dhcp.GetMacAddress(), should.BeEmpty)
				}
			}
		})
	})
}
