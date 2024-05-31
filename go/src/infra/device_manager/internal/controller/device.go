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
func UpdateDevice(ctx context.Context, tx *sql.Tx, psClient *pubsub.Client, device model.Device) error {
	// TODO (b/328662436): Collect metrics
	updatedDevice, err := model.UpdateDevice(ctx, tx, device)
	if err != nil {
		logging.Errorf(ctx, "UpdateDevice: failed to update Device %s: %s", updatedDevice.ID, err)
		return err
	}
	return PublishDeviceEvent(ctx, psClient, &updatedDevice)
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
		DeviceReady:      device.IsActive && IsDeviceAvailable(ctx, stringToDeviceState(ctx, device.DeviceState)),
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
	}
	logging.Debugf(ctx, "PublishDeviceEvent: successfully published DeviceEvent %v", deviceEvent)
	return nil
}

// IsDeviceAvailable checks if a device state is available.
func IsDeviceAvailable(ctx context.Context, state api.DeviceState) bool {
	return state == api.DeviceState_DEVICE_STATE_AVAILABLE
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
