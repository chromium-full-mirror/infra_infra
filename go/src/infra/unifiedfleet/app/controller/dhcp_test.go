// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
)

func TestBatchGetDHCPs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetDHCPs", t, func(t *ftt.Test) {
		t.Run("Batch get dhcps - happy path", func(t *ftt.Test) {
			dhcps := make([]*ufspb.DHCPConfig, 4)
			for i := range 4 {
				dhcp := &ufspb.DHCPConfig{
					Hostname: fmt.Sprintf("dhcp-batchGet-%d", i),
					Ip:       fmt.Sprintf("%d", i),
				}
				dhcps[i] = dhcp
			}
			_, err := configuration.BatchUpdateDHCPs(ctx, dhcps)
			assert.Loosely(t, err, should.BeNil)
			resp, err := configuration.BatchGetDHCPConfigs(ctx, []string{"dhcp-batchGet-0", "dhcp-batchGet-1", "dhcp-batchGet-2", "dhcp-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(dhcps))
		})
		t.Run("Batch get dhcps - missing id", func(t *ftt.Test) {
			resp, err := configuration.BatchGetDHCPConfigs(ctx, []string{"dhcp-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("dhcp-batchGet-non-existing"))
		})
		t.Run("Batch get dhcps - empty input", func(t *ftt.Test) {
			resp, err := configuration.BatchGetDHCPConfigs(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = configuration.BatchGetDHCPConfigs(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}
