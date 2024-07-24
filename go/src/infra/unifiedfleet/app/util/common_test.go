// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestGetHiveForDut(t *testing.T) {
	ftt.Run("Test GetHiveForDut", t, func(t *ftt.Test) {
		assert.Loosely(t, GetHiveForDut("satlab-xx12sha23-chromeos1-row2-rack3-host4", ""), should.Equal("satlab-xx12sha23"))
		assert.Loosely(t, GetHiveForDut("satlab-12:sha45-em25-desk-noogler", ""), should.Equal("satlab-12:sha45"))
		assert.Loosely(t, GetHiveForDut("satlab-abc123-host1", "satlab-nothash"), should.Equal("satlab-nothash"))
		assert.Loosely(t, GetHiveForDut("chromeos1-row2-rack3-host4", ""), should.BeEmpty)
		assert.Loosely(t, GetHiveForDut("cros-mtv1950-144-rack204-host1", ""), should.Equal(gtransitHive))
		assert.Loosely(t, GetHiveForDut("cros-mtv1950-144-rack204-host2", ""), should.Equal(gtransitHive))
		assert.Loosely(t, GetHiveForDut("cros-mtv1950-144-rack204-labstation1", ""), should.Equal(gtransitHive))
		assert.Loosely(t, GetHiveForDut("chromeos8-foo", ""), should.Equal(sfo36OSHive))
		assert.Loosely(t, GetHiveForDut("chrome-chromeos8-foo", ""), should.Equal(sfo36OSHive))
		assert.Loosely(t, GetHiveForDut(ChromiumNamePrefix, ""), should.Equal(chromiumHive))
		assert.Loosely(t, GetHiveForDut("chromium-bar", ""), should.Equal(chromiumHive))
		assert.Loosely(t, GetHiveForDut("chrome-perf-chromeos8-host2", ""), should.Equal(chromePerfHive))
		assert.Loosely(t, GetHiveForDut("cri12-host8", ""), should.Equal(iad65OSHive))
	})
}

func TestAppendUnique(t *testing.T) {
	ftt.Run("Test AppendUnique", t, func(t *ftt.Test) {
		assert.Loosely(t, AppendUniqueStrings([]string{"eeny", "meeny", "miny", "moe"}, "catch", "a", "tiger", "by", "the", "toe"), should.HaveLength(10))
		assert.Loosely(t, AppendUniqueStrings([]string{"london", "bridge", "is", "falling", "down"}, "falling", "down", "falling", "down"), should.HaveLength(5))
		assert.Loosely(t, AppendUniqueStrings([]string{}, "twinkle", "twinkle", "little", "star"), should.HaveLength(3))
		assert.Loosely(t, AppendUniqueStrings([]string{"humpty", "dumpty", "sat", "on", "a", "wall"}), should.HaveLength(6))
		assert.Loosely(t, AppendUniqueStrings([]string{"row", "row", "row", "your", "boat"}), should.HaveLength(3))
	})
}

func TestIsSFPZone(t *testing.T) {
	t.Parallel()

	ftt.Run("Testing IsSFPZone", t, func(t *ftt.Test) {
		assert.Loosely(t, IsSFPZone("ZONE_SFP_TEST"), should.BeTrue)
		assert.Loosely(t, IsSFPZone("ZONE_SFP"), should.BeFalse)
		assert.Loosely(t, IsSFPZone("FAKE_SFP_TEST"), should.BeFalse)
		assert.Loosely(t, IsSFPZone("ZONE_OTHER_TEST"), should.BeFalse)
	})
}
