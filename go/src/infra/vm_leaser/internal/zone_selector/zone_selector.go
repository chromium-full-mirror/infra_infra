// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package zone_selector

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/vm_leaser/internal/constants"
)

var (
	googleApiZoneUriFormat = regexp.MustCompile(`^https:\/\/www\.googleapis\.com\/.*zones\/(?P<zone>[\w]+\-[\w]+\-[\w]+){1}?`)
	zoneFormat             = regexp.MustCompile(`^(?P<zone>[\w]+\-[\w]+\-[\w]+){1}$`)
)

// SelectZone selects a random zone based on the specified testing client.
func SelectZone(ctx context.Context, r *api.LeaseVMRequest, seed int64) string {
	// Call Seed once to seed any subsequent rand calls.
	rand.Seed(seed)

	if r.GetHostReqs().GetGceRegion() != "" {
		return r.GetHostReqs().GetGceRegion()
	}
	switch r.GetTestingClient() {
	case api.VMTestingClient_VM_TESTING_CLIENT_CHROMEOS:
		logging.Infof(ctx, "selecting random zone for ChromeOS testing client")
		return getRandomZone(ctx, constants.ChromeOSZones)
	default:
		logging.Infof(ctx, "selecting random zone for unspecified testing client")
		return getRandomZone(ctx, constants.ChromeOSZones)
	}
}

// getRandomZone takes an array of arrays of zones and returns a random one.
func getRandomZone(ctx context.Context, zones [][]string) string {
	mainIdx := rand.Intn(len(zones))
	subIdx := rand.Intn(len(zones[mainIdx]))
	logging.Infof(ctx, "selected zone for VM creation: %v", zones[mainIdx][subIdx])
	return zones[mainIdx][subIdx]
}

// GetZoneSubnet uses the selected zone to return the correct subnet.
//
// GetZoneSubnet expects zone to be in the format `xxx-yyy-zzz`. `xxx-yyy`
// represents the region while `zzz` represents the subzone. For example,
// `us-central1-a` means the region is `us-central1` and the subzone is `a`.
func GetZoneSubnet(ctx context.Context, zone string) (string, error) {
	if err := validateZone(zone); err != nil {
		return "", err
	}

	network := strings.Join(strings.Split(zone, "-")[:2], "-")
	subnet := fmt.Sprintf("regions/%s/subnetworks/%s", network, network)

	logging.Debugf(ctx, "zone: %s - subnet: %s", zone, subnet)
	return subnet, nil
}

// ExtractGoogleApiZone takes a Google API zone string and returns the zone.
func ExtractGoogleApiZone(uri string) (string, error) {
	if err := validateGoogleApiZoneUri(uri); err != nil {
		return "", err
	}
	matches := googleApiZoneUriFormat.FindStringSubmatch(uri)
	return matches[googleApiZoneUriFormat.SubexpIndex("zone")], nil
}

// validateZone validates the zone format to be xxx-yyy-zzz.
func validateZone(zone string) error {
	if !zoneFormat.MatchString(zone) {
		return errors.New("zone is malformed; needs to be xxx-yyy-zzz")
	}
	return nil
}

// validateGoogleApiZoneUri validates the uri field to be a Google API zone URI.
func validateGoogleApiZoneUri(uri string) error {
	if !googleApiZoneUriFormat.MatchString(uri) {
		return errors.New("google api zone uri is malformed")
	}
	return nil
}
