// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"time"

	schedukepb "go.chromium.org/chromiumos/config/go/test/scheduling"
	"go.chromium.org/luci/auth"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"

	ufspb "infra/unifiedfleet/api/v1/models"
)

const (
	leasePriority           = 1
	leasesSchedulingAccount = "leases"
	leaseSchedulingWindow   = 2 * time.Hour
	pollingInterval         = 30 * time.Second
	// SchedukeTaskKey is the key used for a singleton Scheduke task request in
	// request/response protos.
	SchedukeTaskKey = 1
)

// DeviceInfo contains details about the physical lab setup and machine of a
// particular Swarming device.
type DeviceInfo struct {
	Name     string
	LabSetup *ufspb.MachineLSE
	Machine  *ufspb.Machine
}

// LeaseInfo contains details about a particular lease of a Swarming device.
type LeaseInfo struct {
	Device *DeviceInfo
	Build  *buildbucketpb.Build
}

// Abandon sends a cancellation request to Scheduke for the given device names,
// releasing all leased devices for the current user if no devices are
// specified.
func Abandon(ctx context.Context, authOpts auth.Options, deviceNames []string, taskStateIDs []int64, dev bool) error {
	user, err := getUserEmail(ctx, authOpts)
	if err != nil {
		return err
	}

	sc, err := NewSchedukeClientForCLI(ctx, dev, authOpts)
	if err != nil {
		return err
	}

	return sc.CancelTasks(taskStateIDs, []string{user}, deviceNames)
}

// UFSDeviceInfo returns device information from UFS for the device with the
// given name.
func UFSDeviceInfo(ctx context.Context, deviceName string, authOpts auth.Options) (*DeviceInfo, error) {
	info := &DeviceInfo{Name: deviceName}
	err := addDeviceInfo(ctx, info, authOpts)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Lease leases a device from Scheduke and returns information about the device,
// and a bool indicating whether full device information was retrieved.
func Lease(ctx context.Context, authOpts auth.Options, dims map[string][]string, mins int64, leaseIDChan chan int64) (*LeaseInfo, bool, error) {
	deviceName, err := leaseDeviceFromScheduke(ctx, authOpts, dims, mins, leaseIDChan)
	if err != nil {
		return nil, false, err
	}
	info := &LeaseInfo{Device: &DeviceInfo{Name: deviceName}}
	// Swallow any UFS errors since the lease has been secured at this point.
	err = addDeviceInfo(ctx, info.Device, authOpts)
	fullInfoRetrieved := err == nil
	return info, fullInfoRetrieved, nil
}

// Leases retrieves device information for each in-flight lease for the current
// user, and a bool indicating whether full device information was retrieved.
func Leases(ctx context.Context, authOpts auth.Options, dev bool) ([]*LeaseInfo, bool, error) {
	leaseStates, err := listLeasesFromScheduke(ctx, authOpts, dev)
	if err != nil {
		return nil, false, err
	}

	var info []*LeaseInfo
	fullInfoRetrieved := true
	for _, ls := range leaseStates {
		// Leases returned from listLeasesFromScheduke include still-pending leases;
		// we only want to return active ones here.
		if ls.State != schedukepb.TaskState_LAUNCHED {
			continue
		}
		li := &LeaseInfo{Device: &DeviceInfo{Name: ls.DeviceName}}
		// Swallow any UFS errors since at least the device name has been retrieved
		// at this point.
		err = addDeviceInfo(ctx, li.Device, authOpts)
		fullInfoRetrieved = fullInfoRetrieved && (err == nil)
		info = append(info, li)
	}

	return info, fullInfoRetrieved, nil
}

// ShouldUseScheduke returns a bool indicating whether a lease request in the
// pool should use this Scheduke API.
func ShouldUseScheduke(ctx context.Context, pool string, authOpts auth.Options) (bool, error) {
	gc, err := SilentLoginHTTPClient(ctx, authOpts)
	if err != nil {
		return false, errors.Annotate(err, "initializing Gerrit client to read Scheduke pools allowlist").Err()
	}
	return AnyStringInGerritList(ctx, gc, []string{pool}, schedukePoolsURL)
}
