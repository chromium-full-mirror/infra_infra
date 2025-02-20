// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configuration

import (
	"context"
	"fmt"
	"sort"

	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// an ipCallback is a function that takes an ip and returns whether to keep going and what the error is.
// It is used to iterate over IPs.
type ipCallback = func(ip *ufspb.IP) (keepGoing bool, err error)

// RunFreeIPs runs an ipCallback over all the unassigned IPs in a vlan.
//
// Note that there are two kinds of unassigned IPs in a vlan, one of them has an explicit IPEntity object inside datastore
// that is not reserved and not occupied. The other kind doesn't exist (i.e. that particular IP has no corresponding datastore entity).
// It used to be the case that every possible IP in a vlan, whether assigned or unassigned, always got an IPEntity from the moment
// the vlan was created right up until it got deleted.
//
// At some point, though, we decided to start supporting large vlans for which it would be impractical to preallocate all the IPs.
//
// That's where this two-phase function comes from. We first try to find an unassigned-but-existent vlan inside datastore and then
// return that.
// However, in cases where there are no unassigned-but-existent vlans, we don't give up. We instead precompute all the possible IPs,
// put them in a map, remove entries that already exist from the map, and then pick arbitrary IPs.
//
// This method is also deterministic. It uses a map internally, but we iterate over its keys in sorted order.
func RunFreeIPs(ctx context.Context, vlanName string, cb ipCallback) error {
	keepGoing, err := runOverExistingFreeIPs(ctx, vlanName, cb)
	if err != nil {
		return err
	}
	if !keepGoing {
		return nil
	}
	return runOverNonexistentIPs(ctx, vlanName, cb)
}

// runOverExistingFreeIPs runs a callback over free IPs in a vlan that have already been allocated
func runOverExistingFreeIPs(ctx context.Context, vlanName string, cb ipCallback) (bool, error) {
	explicitFreeIPQuery := datastore.NewQuery(IPKind).FirestoreMode(true).Eq("vlan", vlanName).Eq("occupied", false).Eq("reserve", false)
	checkFreeSpaces := true
	if err := datastore.Run(ctx, explicitFreeIPQuery, func(entity *IPEntity) error {
		e, err := entity.GetProto()
		if err != nil {
			return fmt.Errorf("err encountered while parsing proto: %w", err)
		}
		keepGoing, err := cb(e.(*ufspb.IP))
		switch {
		case !keepGoing:
			checkFreeSpaces = false
			return datastore.Stop
		case err != nil:
			return err
		}
		return nil
	}); err != nil {
		return false, err
	}
	return checkFreeSpaces, nil
}

// runOverNonexistentIPs runs a callback over all the IPs inside a vlan that do not exist at all.
func runOverNonexistentIPs(ctx context.Context, vlanName string, cb ipCallback) error {
	vlan, err := GetVlan(ctx, vlanName)
	if err != nil {
		return err
	}
	startFreeIP, err := util.IPv4StrToInt(vlan.GetFreeStartIpv4Str())
	if err != nil {
		return err
	}
	endFreeIP, err := util.IPv4StrToInt(vlan.GetFreeEndIpv4Str())
	if err != nil {
		return err
	}

	allIPs := make(map[uint32]bool)
	if err := util.Uint32Iter(startFreeIP, endFreeIP, func(item uint32) error {
		allIPs[item] = true
		return nil
	}); err != nil {
		return err
	}

	allIPsQuery := datastore.NewQuery(IPKind).FirestoreMode(true).Eq("vlan", vlanName)
	if err := datastore.Run(ctx, allIPsQuery, func(entity *IPEntity) {
		delete(allIPs, entity.IPv4)
	}); err != nil {
		return err
	}

	sortedIPs := []uint32{}
	for k := range allIPs {
		sortedIPs = append(sortedIPs, k)
	}
	sort.Slice(sortedIPs, func(i, j int) bool { return sortedIPs[i] < sortedIPs[j] })

	for _, k := range sortedIPs {
		e := &ufspb.IP{
			Vlan:    vlanName,
			Ipv4:    k,
			Ipv4Str: util.IPv4IntToStr(k),
		}
		keepGoing, err := cb(e)
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
	}
	return nil
}
