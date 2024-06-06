// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"

	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/controller"
	"infra/device_manager/internal/model"
)

const (
	publishWorkersN   = 50
	updateBatchSize   = 1000
	maxUpdateWaitTime = 500 * time.Millisecond
)

// TODO: b/343293714 - Write unit tests and manually test this. Create a job that calls SendNotifications.
// TODO: b/328662436 - Collect metrics.

var wg sync.WaitGroup

// SendNotifications selects Devices for which no notifications have been sent
// since last_updated_time. It then sets up a worker pool to start publishing a
// message per device to Pub/Sub. Successful sending results in the Device row
// being updated to note the last notification time. The time this function is
// first called is what is used for this to avoid missing updates that happen as
// we go this batch of updates. These final updates are done in batches with a
// max wait time between updates.
func SendNotifications(ctx context.Context, db *sql.DB, psClient *pubsub.Client) error {
	var (
		// queryTime is what will be used as notification time. It is important
		// to get this before sending the query to avoid missing notifications
		// in case devices do get updated by while we are sending notifications.
		queryTime = time.Now()
		query     = `
			SELECT
				id,
				device_address,
				device_type,
				device_state,
				schedulable_labels,
				is_active,
				last_updated_time
			FROM "Devices"
			WHERE
				is_active = true
			  AND (
					last_updated_time > last_notification_time
					OR last_notification_time IS NULL
				);`
		lastUpdatedTime sql.NullTime
	)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("jobs/notifier: failed to get devices to notify on: [%w]", err)
	}
	defer rows.Close()

	var (
		// Each worker gets a spot in input and output channels
		publishDevice = make(chan *model.Device, publishWorkersN)
		updateDevice  = make(chan *model.Device, publishWorkersN)
	)

	for range publishWorkersN {
		go publishDeviceWorker(ctx, psClient, publishDevice, updateDevice)
	}
	go updateWorker(ctx, db, queryTime, updateDevice)

	for rows.Next() {
		var device model.Device
		err = rows.Scan(
			&device.ID,
			&device.DeviceAddress,
			&device.DeviceType,
			&device.DeviceState,
			&device.SchedulableLabels,
			&device.IsActive,
			&lastUpdatedTime,
		)
		if err != nil {
			return fmt.Errorf("jobs/notifier: failed to get scan row of devices to notify on: [%w]", err)
		}

		if lastUpdatedTime.Valid {
			device.LastUpdatedTime = lastUpdatedTime.Time
		}

		wg.Add(1)
		publishDevice <- &device
		logging.Debugf(ctx, "Queued Device %s for publishing event to Pub/Sub", device.ID)
	}

	wg.Wait()
	return nil
}

func publishDeviceWorker(
	ctx context.Context,
	psClient *pubsub.Client,
	devices <-chan *model.Device,
	successes chan<- *model.Device,
) {
	for device := range devices {
		err := controller.PublishDeviceEvent(ctx, psClient, device)
		if err == nil {
			successes <- device
			continue
		}
		// On success updateWorker will handle updating wg.
		wg.Done()
		logging.Errorf(ctx, "Failed to publish notification for device %s: %v", device.ID, err)
	}
}

func updateWorker(ctx context.Context, db *sql.DB, updateTime time.Time, devices <-chan *model.Device) {
	var (
		// pendingUpdates is a quoted list of device IDs
		pendingUpdates = make([]string, 0, updateBatchSize)
		timer          = time.NewTimer(maxUpdateWaitTime)
	)

	updateDevices := func() {
		if len(pendingUpdates) == 0 {
			return
		}
		query := `
			UPDATE "Devices"
			SET
				last_notification_time = $1
			WHERE
				id IN (%s);`
		query = fmt.Sprintf(query, strings.Join(pendingUpdates, ", "))
		_, err := db.QueryContext(ctx, query, updateTime)
		if err != nil {
			logging.Errorf(ctx, "Failed to update notification time for devices with query %s: %v", query, err)
		}
		wg.Add(-len(pendingUpdates))
		pendingUpdates = pendingUpdates[:0]
	}

	for {
		select {
		case device := <-devices:
			pendingUpdates = append(pendingUpdates, fmt.Sprintf("'%s'", device.ID))
			if len(pendingUpdates) == updateBatchSize {
				updateDevices()
			}
		case <-timer.C:
			updateDevices()
		}
	}
}
