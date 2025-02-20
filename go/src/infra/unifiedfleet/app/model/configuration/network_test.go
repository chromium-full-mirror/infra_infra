// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

func TestImportDHCPConfigs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("import nics", t, func(t *ftt.Test) {
		dhcps := []*ufspb.DHCPConfig{
			mockDHCPConfig("hostname1", "ip1"),
			mockDHCPConfig("hostname2", "ip2"),
		}
		t.Run("happy path", func(t *ftt.Test) {
			resp, err := ImportDHCPConfigs(ctx, dhcps)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Passed(), should.HaveLength(len(dhcps)))
			getRes, _, err := ListDHCPConfigs(ctx, 100, "", nil, false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, getRes, should.Match(dhcps))
		})
		t.Run("happy path also for importing existing dhcp configs", func(t *ftt.Test) {
			dhcps1 := []*ufspb.DHCPConfig{
				mockDHCPConfig("hostname1", "ip1-1"),
			}
			resp, err := ImportDHCPConfigs(ctx, dhcps1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Passed(), should.HaveLength(len(dhcps1)))
			s, err := GetDHCPConfig(ctx, "hostname1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.GetIp(), should.Equal("ip1-1"))
		})
	})
}

func mockDHCPConfig(hostname, ipv4 string) *ufspb.DHCPConfig {
	return &ufspb.DHCPConfig{
		Hostname: hostname,
		Ip:       ipv4,
	}
}
