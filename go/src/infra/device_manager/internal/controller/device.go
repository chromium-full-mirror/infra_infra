// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/test/api"
	schedulingAPI "go.chromium.org/chromiumos/config/go/test/scheduling"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/database"
	"infra/device_manager/internal/external"
	"infra/device_manager/internal/model"
	"infra/libs/skylab/inventory/swarming"
)

// TODO: b/343293714 - Write unit tests and manually test this. Create a job that calls SendNotifications.
// TODO: b/328662436 - Collect metrics.

// NotifierOpts struct holds configuration options for the Notifier service
type NotifierOpts struct {
	PublishWorkersN   *int
	UpdateBatchSize   *int
	MaxUpdateWaitTime time.Duration
}

// GetDevice gets a Device from the database based on a deviceID.
func GetDevice(ctx context.Context, db *sql.DB, idType model.DeviceIDType, deviceID string) (*api.Device, error) {
	device, err := model.GetDeviceByID(ctx, db, idType, deviceID)
	if err != nil {
		return &api.Device{}, err
	}
	return deviceModelToAPIDevice(ctx, device), nil
}

// ListDevices lists Devices from the db based on filters.
func ListDevices(ctx context.Context, db *sql.DB, r *api.ListDevicesRequest) (*api.ListDevicesResponse, error) {
	devices, nextPageToken, err := model.ListDevices(ctx, db, database.PageToken(r.GetPageToken()), int(r.GetPageSize()), r.GetFilter())
	if err != nil {
		return nil, err
	}

	devicesProtos := make([]*api.Device, len(devices))
	for i, d := range devices {
		devicesProtos[i] = deviceModelToAPIDevice(ctx, d)
	}

	return &api.ListDevicesResponse{
		Devices:       devicesProtos,
		NextPageToken: string(nextPageToken),
	}, nil
}

// UpdateDevice updates a Device in a transaction.
func UpdateDevice(ctx context.Context, tx *sql.Tx, device model.Device) error {
	_, err := model.UpdateDevice(ctx, tx, device)
	return err
}

// PublishDeviceEvent takes a Device and publishes an event to PubSub.
func PublishDeviceEvent(ctx context.Context, psClient *pubsub.Client, device *model.Device) error {
	// Send message to PubSub Device events stream
	topic := psClient.Topic(external.DeviceEventsPubSubTopic)
	defer topic.Stop()

	dutID, err := device.DUTID()
	if err != nil {
		return errors.Annotate(err, "PublishDeviceEvent").Err()
	}

	marshalOpts := protojson.MarshalOptions{EmitUnpopulated: true}

	deviceEvent := &schedulingAPI.DeviceEvent{
		EventTime:        time.Now().Unix(),
		DeviceId:         dutID,
		DeviceReady:      device.IsActive && IsDeviceAvailable(ctx, device.DeviceState),
		DeviceDimensions: labelsToSwarmingDims(ctx, device.SchedulableLabels),
		DeviceName:       device.ID,
	}

	var msg []byte
	msg, err = marshalOpts.Marshal(deviceEvent)
	if err != nil {
		return fmt.Errorf("protojson.Marshal err: %w", err)
	}

	rsp := topic.Publish(ctx, &pubsub.Message{
		Data: msg,
	})

	_, err = rsp.Get(ctx)
	if err != nil {
		logging.Debugf(ctx, "PublishDeviceEvent: failed to publish to PubSub %s", err)
		return err
	}
	logging.Debugf(ctx, "PublishDeviceEvent: successfully published DeviceEvent %v", deviceEvent)
	return nil
}

// SendNotifications selects Devices for which no notifications have been sent
// since last_updated_time. It then sets up a worker pool to start publishing a
// message per device to Pub/Sub. Successfully sending results in the Device row
// updates to note the last notification time. The time used for notification is
// when the function is first called. This avoids missing updates that happen as
// we go this batch of updates. These final updates are done in batches with a
// max wait time between updates.
func SendNotifications(
	ctx context.Context,
	db *sql.DB,
	psClient *pubsub.Client,
	opts *NotifierOpts,
) {
	var (
		// queryTime is what will be used as notification time. It is important to
		// get this before sending the query to avoid missing notifications in case
		// devices do get updated by while we are sending notifications.
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
					OR NOW() - last_notification_time > '6h'
				);`
		lastUpdatedTime sql.NullTime
	)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		panic(fmt.Errorf("notifier_service: failed to get devices to notify on: [%w]", err))
	}
	defer rows.Close()

	var (
		// Each worker gets a spot in input and output channels
		publishDevice = make(chan *model.Device, *opts.PublishWorkersN)
		updateDevice  = make(chan *model.Device, *opts.PublishWorkersN)

		// Control pending updates.
		wg sync.WaitGroup
	)
	defer close(publishDevice)
	defer close(updateDevice)

	for range *opts.PublishWorkersN {
		go publishDeviceWorker(ctx, &wg, psClient, publishDevice, updateDevice)
	}
	go updateWorker(ctx, &wg, db, queryTime, updateDevice, *opts)

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
			panic(fmt.Errorf("notifier_service: failed to get scan row of devices to notify on: [%w]", err))
		}

		if lastUpdatedTime.Valid {
			device.LastUpdatedTime = lastUpdatedTime.Time
		}

		wg.Add(1)
		publishDevice <- &device
		logging.Debugf(ctx, "Queued Device %s for publishing event to Pub/Sub", device.ID)
	}

	wg.Wait()
}

// publishDeviceWorker takes a queue of Devices and process them by publishing a
// DeviceEvent when applicable. Devices are queued and dequeued continuously.
func publishDeviceWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	psClient *pubsub.Client,
	devices <-chan *model.Device,
	successes chan<- *model.Device,
) {
	for device := range devices {
		err := PublishDeviceEvent(ctx, psClient, device)
		if err == nil {
			successes <- device
			continue
		}
		// On success updateWorker will handle updating wg.
		wg.Done()

		logging.Errorf(ctx, "Failed to publish notification for device %s: %v", device.ID, err)
	}
}

// updateWorker queues up Devices to be updated in batches. Once a batch is
// filled or the ticker reaches the max wait time, the Devices will be
// processed.
func updateWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	db *sql.DB,
	updateTime time.Time,
	devices <-chan *model.Device,
	opts NotifierOpts,
) {
	var (
		// pendingUpdates is a quoted list of device IDs
		pendingUpdates = make([]string, 0, *opts.UpdateBatchSize)
		timer          = time.NewTicker(opts.MaxUpdateWaitTime)
	)

	updateDevices := func() {
		if len(pendingUpdates) == 0 {
			return
		}
		defer func() {
			wg.Add(-len(pendingUpdates))
			pendingUpdates = pendingUpdates[:0]
		}()

		query := `
			UPDATE "Devices"
			SET
				last_notification_time = $1
			WHERE
				id IN (%s);`
		query = fmt.Sprintf(query, strings.Join(pendingUpdates, ", "))
		row, err := db.QueryContext(ctx, query, updateTime)
		if row != nil {
			row.Close()
		}
		if err != nil {
			logging.Errorf(ctx, "Failed to update notification time for devices with query %s: %v", query, err)
			return
		}
		logging.Debugf(ctx, "Query ran with %d updates", len(pendingUpdates))
	}
	defer updateDevices()

	for {
		select {
		case device, ok := <-devices:
			if !ok {
				return
			}
			pendingUpdates = append(pendingUpdates, fmt.Sprintf("'%s'", device.ID))
			if len(pendingUpdates) == *opts.UpdateBatchSize {
				updateDevices()
			}
		case <-timer.C:
			updateDevices()
		}
	}
}

// IsDeviceAvailable checks if a device state is available.
func IsDeviceAvailable(ctx context.Context, state string) bool {
	return state == "DEVICE_STATE_AVAILABLE"
}

// stringToDeviceAddress takes a net address string and converts to the
// DeviceAddress in API format.
//
// The format is defined by the DeviceAddress proto. It does a basic split of
// Host and Port and uses the net package. This package supports IPv4 and IPv6.
func stringToDeviceAddress(ctx context.Context, addr string) (*api.DeviceAddress, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return &api.DeviceAddress{}, errors.Annotate(err, "failed to split host and port %s", addr).Err()
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return &api.DeviceAddress{}, errors.Annotate(err, "port %s is not convertible to integer", portStr).Err()
	}

	return &api.DeviceAddress{
		Host: host,
		Port: int32(port),
	}, nil
}

// deviceAddressToString takes a DeviceAddress and converts it to string.
//
// The format is defined by the DeviceAddress proto. It does a basic join of
// Host and Port using the net package.
func deviceAddressToString(ctx context.Context, addr *api.DeviceAddress) string {
	return net.JoinHostPort(addr.GetHost(), fmt.Sprint(addr.GetPort()))
}

// stringToDeviceType takes a string and converts it to DeviceType.
func stringToDeviceType(ctx context.Context, deviceType string) api.DeviceType {
	return api.DeviceType(api.DeviceType_value[deviceType])
}

// stringToDeviceState takes a string and converts it to DeviceState.
func stringToDeviceState(ctx context.Context, state string) api.DeviceState {
	return api.DeviceState(api.DeviceState_value[state])
}

// labelsToHardwareReqs formats SchedulableLabels to be HardwareRequirements.
func labelsToHardwareReqs(ctx context.Context, labels model.SchedulableLabels) *api.HardwareRequirements {
	hardwareReqs := &api.HardwareRequirements{
		SchedulableLabels: map[string]*api.HardwareRequirements_LabelValues{},
	}
	for k, v := range labels {
		hardwareReqs.GetSchedulableLabels()[k] = &api.HardwareRequirements_LabelValues{
			Values: v.Values,
		}
	}
	return hardwareReqs
}

// labelsToSwarmingDims formats the labels to SwarmingDimensions for publishing.
func labelsToSwarmingDims(ctx context.Context, labels model.SchedulableLabels) *schedulingAPI.SwarmingDimensions {
	swarmingDims := &schedulingAPI.SwarmingDimensions{
		DimsMap: map[string]*schedulingAPI.DimValues{},
	}
	for k, v := range labels {
		swarmingDims.GetDimsMap()[k] = &schedulingAPI.DimValues{
			Values: v.Values,
		}
	}
	return swarmingDims
}

// SwarmingDimsToLabels converts SwarmingDimensions to Device Manager
// SchedulableLabels.
func SwarmingDimsToLabels(ctx context.Context, dims swarming.Dimensions) model.SchedulableLabels {
	schedLabels := make(model.SchedulableLabels)
	for k, v := range dims {
		schedLabels[k] = model.LabelValues{
			Values: v,
		}
	}
	return schedLabels
}

// deviceModelToAPIDevice takes a Device model and returns an API Device object.
func deviceModelToAPIDevice(ctx context.Context, device model.Device) *api.Device {
	addr, err := stringToDeviceAddress(ctx, device.DeviceAddress)
	if err != nil {
		logging.Errorf(ctx, err.Error())
		addr = &api.DeviceAddress{}
	}

	return &api.Device{
		Id:           device.ID,
		Address:      addr,
		Type:         stringToDeviceType(ctx, device.DeviceType),
		State:        stringToDeviceState(ctx, device.DeviceState),
		HardwareReqs: labelsToHardwareReqs(ctx, device.SchedulableLabels),
	}
}

// ExtractSingleValuedDimension extracts one specified dimension from a
// dimension slice.
func ExtractSingleValuedDimension(ctx context.Context, dims map[string]*api.HardwareRequirements_LabelValues, key string) (string, error) {
	vs, ok := dims[key]
	if !ok {
		return "", fmt.Errorf("ExtractSingleValuedDimension: failed to find dimension %s", key)
	}
	switch len(vs.GetValues()) {
	case 1:
		return vs.GetValues()[0], nil
	case 0:
		return "", fmt.Errorf("ExtractSingleValuedDimension: no value for dimension %s", key)
	default:
		return "", fmt.Errorf("ExtractSingleValuedDimension: multiple values for dimension %s", key)
	}
}
