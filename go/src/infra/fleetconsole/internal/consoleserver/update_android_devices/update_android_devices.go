// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"maps"

	"go.chromium.org/luci/common/errors"

	omnilab_pubsub "go.chromium.org/infra/fleetconsole/omnilab/omnilab-pubsub"
)

func UpdateAndroidDevices(ctx context.Context, tx *sql.Tx, labResource *omnilab_pubsub.MonitoredRecord) (err error) {
	if labResource.HostEntry == nil || labResource.HostEntry.Identifier == nil {
		return errors.Reason("invalid lab resource: host or identifier is nil").Err()
	}
	host, devices := parseHostAndDevice(labResource)

	err = updateAndroidDevicesTable(ctx, tx, devices)
	if err != nil {
		return err
	}
	//TODO (pietroscutta): Delete devices in the same host not in the devices list

	err = updateAndroidHostsTable(ctx, tx, host)
	if err != nil {
		return err
	}

	for runTargetLabNameHostGroup := range uniqueRunTargetsLabNamesHostGroups(devices) {
		err = updateLatestDailyMaxTable(ctx, tx, runTargetLabNameHostGroup)
		if err != nil {
			return err
		}

		err = updateAndroidRepairMetricsTable(ctx, tx, runTargetLabNameHostGroup)
		if err != nil {
			return err
		}
	}

	fmt.Println("DONE")
	return nil
}

type host struct {
	hostname   string
	host_group string
	state      string
}

type device struct {
	id        string
	labName   string
	hostGroup string
	runTarget string
	state     string
}

func parseHostAndDevice(labResource *omnilab_pubsub.MonitoredRecord) (host, []device) {
	hostname := labResource.HostEntry.Identifier["host_name"]
	host_group := ""
	for _, attr := range labResource.HostEntry.Attribute {
		if attr.Name == "host_group" {
			host_group = attr.Value
		}
	}
	hostState := "" //TODO (pietroscutta): parse the state into an enum
	for _, res := range labResource.HostEntry.Attribute {
		if res.Name == "status" {
			hostState = res.Value // TODO: double check if this is right
		}
	}

	host := host{
		hostname:   hostname,
		host_group: host_group,
		state:      hostState,
	}

	devices := make([]device, len(labResource.DeviceEntry))
	for i, d := range labResource.DeviceEntry {
		runTarget := ""
		state := "" //TODO (pietroscutta): parse the state into an enum
		for _, attr := range d.Attribute {
			if attr.Name == "run_target" {
				runTarget = attr.Value
			}
			if attr.Name == "status" {
				state = attr.Value // TODO: double check if this is right
			}

		}

		devices[i] = device{
			id:        d.Identifier["device_serial"],
			labName:   labResource.HostEntry.Identifier["lab_name"],
			hostGroup: host_group,
			runTarget: runTarget,
			state:     state,
		}
	}

	return host, devices
}

type runTargetsLabNamesHostGroups struct {
	runTarget string
	hostGroup string
	labName   string
}

func uniqueRunTargetsLabNamesHostGroups(devices []device) iter.Seq[runTargetsLabNamesHostGroups] {
	out := make(map[string]runTargetsLabNamesHostGroups)
	for _, device := range devices {
		key := device.runTarget + device.labName + device.hostGroup

		out[key] = runTargetsLabNamesHostGroups{
			runTarget: device.runTarget,
			hostGroup: device.hostGroup,
			labName:   device.labName,
		}
	}
	return maps.Values(out)
}
