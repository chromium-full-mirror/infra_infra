// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clients

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/appengine/crosskylabadmin/internal/tq"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
)

const repairBotsQueue = "repair-bots"
const repairLabstationQueue = "repair-labstations"
const auditBotsQueue = "audit-bots"

// PushRepairLabstations pushes BOT ids to taskqueue repairLabstationQueue for
// upcoming repair jobs.
func PushRepairLabstations(ctx context.Context, botIDs []string, swarmingPool string) error {
	return pushDUTs(ctx, repairLabstationQueue, createTasks(botIDs, "", swarmingPool, labstationRepairTask))
}

// PushRepairDUTs pushes BOT ids to taskqueue repairBotsQueue for upcoming repair
// jobs.
func PushRepairDUTs(ctx context.Context, botIDs []string, expectedState string, swarmingPool string) error {
	return pushDUTs(ctx, repairBotsQueue, createTasks(botIDs, expectedState, swarmingPool, crosRepairTask))
}

// PushAuditDUTs pushes BOT ids to taskqueue auditBotsQueue for upcoming audit jobs.
func PushAuditDUTs(ctx context.Context, botIDs, actions []string, taskname, swarmingPool string) error {
	actionsCSV := strings.Join(actions, ",")
	actionsStr := strings.Join(actions, "-")
	tasks := make([]*tq.Task, 0, len(botIDs))
	for _, id := range botIDs {
		if heuristics.LooksLikeSatlabDevice(id) {
			logging.Infof(ctx, fmt.Sprintf("Skipping audit for satlab device %q", id))
			continue
		}
		tasks = append(tasks, crosAuditTask(id, taskname, actionsCSV, actionsStr, swarmingPool))
	}
	return pushDUTs(ctx, auditBotsQueue, tasks)
}

func validateCrosRepairTask(botID string, swarmingPool string) {
	if botID == "" {
		panic("internal error in .../app/clients/taskqueue.go: botID cannot be empty")
	}
	if swarmingPool == "" {
		panic("internal error in .../app/clients/taskqueue.go: swarmingPool cannot be empty")
	}
}

func crosRepairTask(botID string, expectedState string, swarmingPool string) *tq.Task {
	validateCrosRepairTask(botID, swarmingPool)
	values := url.Values{}
	values.Set("botID", botID)
	if expectedState != "" {
		values.Set("expectedState", expectedState)
	}
	values.Set("swarmingPool", swarmingPool)
	return tq.NewPOSTTask(fmt.Sprintf("/internal/task/cros_repair/%s", botID), values)
}

func labstationRepairTask(botID, expectedState string, swarmingPool string) *tq.Task {
	values := url.Values{}
	values.Set("botID", botID)
	values.Set("swarmingPool", swarmingPool)
	return tq.NewPOSTTask(fmt.Sprintf("/internal/task/labstation_repair/%s", botID), values)
}

func crosAuditTask(botID, taskname, actionsCSV, actionsStr, swarmingPool string) *tq.Task {
	values := url.Values{}
	values.Set("botID", botID)
	values.Set("taskname", taskname)
	values.Set("actions", actionsCSV)
	values.Set("swarmingPool", swarmingPool)
	return tq.NewPOSTTask(fmt.Sprintf("/internal/task/audit/%s/%s", botID, actionsStr), values)
}

func createTasks(botIDs []string, expectedState string, swarmingPool string, taskGenerator func(string, string, string) *tq.Task) []*tq.Task {
	tasks := make([]*tq.Task, 0, len(botIDs))
	for _, id := range botIDs {
		tasks = append(tasks, taskGenerator(id, expectedState, swarmingPool))
	}
	return tasks
}

func pushDUTs(ctx context.Context, queueName string, tasks []*tq.Task) error {
	if err := tq.Add(ctx, queueName, tasks...); err != nil {
		return err
	}
	logging.Infof(ctx, "enqueued %d tasks", len(tasks))
	return nil
}
