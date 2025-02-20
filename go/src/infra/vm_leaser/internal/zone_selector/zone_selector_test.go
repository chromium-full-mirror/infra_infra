// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package zone_selector

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/vm_leaser/internal/constants"
)

func TestSelectZone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	allZones := []string{}
	for _, a := range constants.ChromeOSZones {
		allZones = append(allZones, a...)
	}

	ftt.Run("Test SelectZone", t, func(t *ftt.Test) {
		t.Run("SelectZone - zone provided; return zone", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage:  "test-image",
					GceRegion: "test-region",
				},
			}
			z := SelectZone(ctx, req, 1)
			assert.Loosely(t, z, should.Equal("test-region"))
		})
		t.Run("SelectZone - select single random zone", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage: "test-image",
				},
			}
			z := SelectZone(ctx, req, 1)
			assert.Loosely(t, allZones, should.Contain(z))
		})
		t.Run("SelectZone - select single random zone for ChromeOS testing client", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage: "test-image",
				},
				TestingClient: api.VMTestingClient_VM_TESTING_CLIENT_CHROMEOS,
			}
			z := SelectZone(ctx, req, 1)
			assert.Loosely(t, allZones, should.Contain(z))
		})
		t.Run("SelectZone - check distribution of zones", func(t *ftt.Test) {
			req := &api.LeaseVMRequest{
				HostReqs: &api.VMRequirements{
					GceImage: "test-image",
				},
			}

			zonesDict := make(map[string]int)

			// run select zone for a reasonably large number
			var numZones int64 = 10000
			var c int64 = 0
			for c < numZones {
				z := SelectZone(ctx, req, c)
				zonesDict[z] = zonesDict[z] + 1
				c = c + 1
			}

			// all keys should be valid zones
			for k := range zonesDict {
				assert.Loosely(t, allZones, should.Contain(k))
			}

			// Since we have 4 main zones and 13 total zones, the distribution for
			// each zone must be around 9% per zone. To give the randomness some
			// leeway, the zone distribution should be 4-14% (9±5%) per subzone.
			dist := 1.0 / float64(len(allZones))
			for _, a := range allZones {
				assert.That(t, float64(zonesDict[a]), should.BeLessThan(float64(numZones)*(dist+0.05)))
				assert.That(t, float64(zonesDict[a]), should.BeGreaterThan(float64(numZones)*(dist-0.05)))
			}
		})
	})
}

func TestGetZoneSubnet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ftt.Run("Test GetZoneSubnet", t, func(t *ftt.Test) {
		t.Run("GetZoneSubnet - happy path", func(t *ftt.Test) {
			z, err := GetZoneSubnet(ctx, "test-region-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, z, should.Equal("regions/test-region/subnetworks/test-region"))
		})
		t.Run("GetZoneSubnet - bad zone", func(t *ftt.Test) {
			z, err := GetZoneSubnet(ctx, "test-region")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("zone is malformed; needs to be xxx-yyy-zzz"))
			assert.Loosely(t, z, should.BeEmpty)
		})
	})
}

func TestExtractGoogleApiZone(t *testing.T) {
	t.Parallel()

	ftt.Run("Test ExtractGoogleApiZone", t, func(t *ftt.Test) {
		t.Run("ExtractGoogleApiZone - happy path", func(t *ftt.Test) {
			z, err := ExtractGoogleApiZone("https://www.googleapis.com/compute/v1/projects/chrome-fleet-vm-leaser-dev/zones/us-central1-b")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, z, should.Equal("us-central1-b"))
		})
		t.Run("ExtractGoogleApiZone - bad zone", func(t *ftt.Test) {
			z, err := ExtractGoogleApiZone("https://www.googleapis.com/compute/v1/projects/chrome-fleet-vm-leaser-dev/zones/us-central1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("google api zone uri is malformed"))
			assert.Loosely(t, z, should.BeEmpty)
		})
		t.Run("ExtractGoogleApiZone - no zone", func(t *ftt.Test) {
			z, err := ExtractGoogleApiZone("https://www.googleapis.com/compute/v1/projects/chrome-fleet-vm-leaser-dev/zones")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("google api zone uri is malformed"))
			assert.Loosely(t, z, should.BeEmpty)
		})
	})
}

func TestValidateZone(t *testing.T) {
	t.Parallel()

	ftt.Run("Test validateZone", t, func(t *ftt.Test) {
		t.Run("validateZone - happy path", func(t *ftt.Test) {
			err := validateZone("us-central1-b")
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("validateZone - bad zone", func(t *ftt.Test) {
			err := validateZone("us-central1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("zone is malformed; needs to be xxx-yyy-zzz"))
		})
		t.Run("validateZone - no zone", func(t *ftt.Test) {
			err := validateZone("")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("zone is malformed; needs to be xxx-yyy-zzz"))
		})
	})
}
