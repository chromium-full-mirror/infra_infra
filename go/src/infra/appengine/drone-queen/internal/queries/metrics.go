// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queries

import (
	"context"
	"sync"

	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/drone-queen/internal/entities"
)

var (
	retryUniqueUUID = metric.NewCounter(
		"chromeos/drone-queen/irregular-event/retry-unique-uuid",
		"retry when attempting to generate a unique drone UUID",
		nil,
		field.String("instance"),
	)
	agentCount = metric.NewInt(
		"chromeos/drone-queen/agent/count",
		"count of drone agents",
		nil,
		field.String("hive"),
		field.String("version"),
	)
	agentCapacity = metric.NewInt(
		"chromeos/drone-queen/agent/capacity",
		"capacity of drone agents",
		nil,
		field.String("hive"),
		field.String("type"),
	)
	// agentLoadTracker tracks the load of all agents.
	agentLoadTracker     = make(map[entities.DroneID]agentLoad)
	agentLoadTrackerLock = sync.Mutex{}

	suCount = metric.NewInt(
		"chromeos/drone-queen/scheduling-unit/count",
		"count of scheduling units reported to the drone queen",
		nil,
		field.Bool("assigned"),
		field.String("hive"),
	)
)

// agentLoad is a struct to record drone agent load info.
type agentLoad struct {
	hive          string
	version       string
	totalCapacity int
	usedCapacity  int
}

// agentCounter is a map of versions by hives.
type agentCounter map[string]map[string]int

// increment is used to initialize agentCounter.
func (a agentCounter) increment(hive string, version string) {
	if _, ok := a[hive]; !ok {
		a[hive] = make(map[string]int)
	}
	a[hive][version]++
}

func init() {
	tsmon.RegisterCallback(func(ctx context.Context) {
		freeCapacityByHive := make(map[string]int)
		usedCapacityByHive := make(map[string]int)
		ac := make(agentCounter)

		agentLoadTrackerLock.Lock()
		for _, al := range agentLoadTracker {
			freeCapacityByHive[al.hive] += al.totalCapacity - al.usedCapacity
			usedCapacityByHive[al.hive] += al.usedCapacity
			ac.increment(al.hive, al.version)
		}
		agentLoadTrackerLock.Unlock()

		for k, v := range freeCapacityByHive {
			agentCapacity.Set(ctx, int64(v), k, "free")
		}
		for k, v := range usedCapacityByHive {
			agentCapacity.Set(ctx, int64(v), k, "used")
		}
		for hive, versions := range ac {
			for version, v := range versions {
				agentCount.Set(ctx, int64(v), hive, version)
			}
		}
	})

	tsmon.RegisterCallback(setSchedulingUnitCountMetrics)
}

func updateAgentLoad(d entities.DroneID, l agentLoad) {
	agentLoadTrackerLock.Lock()
	defer agentLoadTrackerLock.Unlock()
	agentLoadTracker[d] = l
}

func deleteAgentLoad(d entities.DroneID) {
	agentLoadTrackerLock.Lock()
	defer agentLoadTrackerLock.Unlock()
	delete(agentLoadTracker, d)
}

// setSchedulingUnitCountMetrics collects the metric of scheduling units
// reported to the queen.
// For historical reason, we still use the term "DUT" here which has the
// equivalent meaning to "scheduling unit" in the function scope.
func setSchedulingUnitCountMetrics(ctx context.Context) {
	dutGroupKey := entities.DUTGroupKey(ctx)
	q := datastore.NewQuery(entities.DUTKind).Ancestor(dutGroupKey)
	var duts []entities.DUT
	if err := datastore.GetAll(ctx, q, &duts); err != nil {
		return
	}
	assignedSUByHive := make(map[string]int)
	unassignedSUByHive := make(map[string]int)
	for _, d := range duts {
		if d.AssignedDrone != "" {
			assignedSUByHive[d.Hive]++
		} else {
			unassignedSUByHive[d.Hive]++
		}
	}

	for k, v := range assignedSUByHive {
		suCount.Set(ctx, int64(v), true, k)
	}

	for k, v := range unassignedSUByHive {
		suCount.Set(ctx, int64(v), false, k)
	}
}
