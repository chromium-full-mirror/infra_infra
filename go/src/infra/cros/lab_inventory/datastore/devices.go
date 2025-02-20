// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package datastore contains datastore-related logic.
package datastore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"

	"go.chromium.org/chromiumos/infra/proto/go/lab"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/cros/lab_inventory/utils"
)

const (
	// UUIDPrefix is the prefix we used to identify the system generated ID.
	UUIDPrefix       = "UUID"
	dutIDPlaceholder = "IGNORED"
)

// A query in transaction requires to have Ancestor filter, see
// https://cloud.google.com/appengine/docs/standard/python/datastore/query-restrictions#queries_inside_transactions_must_include_ancestor_filters
func fakeAcestorKey(ctx context.Context) *datastore.Key {
	return datastore.MakeKey(ctx, DeviceKind, "key")
}

func getDutServo(ctx context.Context, d *lab.ChromeOSDevice) (*lab.Servo, error) {
	id := d.GetId().GetValue()
	entity := &DeviceEntity{
		ID:     DeviceEntityID(id),
		Parent: fakeAcestorKey(ctx),
	}
	if err := datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			// hiding error, device not exist and cannot provide old servo
			logging.Errorf(ctx, "device with ID: %s not exits", id)
			return nil, nil
		}
		logging.Errorf(ctx, "Failed to get entity from datastore: %s", err)
		return nil, errors.Annotate(err, "Internal error when try to find the device with id: %s", id).Err()
	}
	var labConfig lab.ChromeOSDevice
	if err := entity.GetCrosDeviceProto(&labConfig); err != nil {
		return nil, errors.Annotate(err, "failed to unmarshal lab config data for %s", id).Err()
	}
	dut := labConfig.GetDut()
	if dut == nil {
		return nil, nil
	}
	return dut.GetPeripherals().GetServo(), nil
}

func addMissingID(devices []*lab.ChromeOSDevice) {
	// Use uuid as the device ID if asset id is not present.
	for _, d := range devices {
		// TODO (guocb) Erase the id passed in as long as it's not asset id to
		// ensure the ID is unique.
		if d.GetId() == nil || d.GetId().GetValue() == "" || d.GetId().GetValue() == dutIDPlaceholder {
			d.Id = &lab.ChromeOSDeviceID{
				Value: fmt.Sprintf("%s:%s", UUIDPrefix, uuid.New().String()),
			}
		}
	}
}

func sanityCheckForAdding(ctx context.Context, d *lab.ChromeOSDevice, q *datastore.Query) error {
	id := d.GetId().GetValue()
	hostname := utils.GetHostname(d)
	var devs []*DeviceEntity
	if err := datastore.GetAll(ctx, q.Eq("Hostname", hostname), &devs); err != nil {
		return errors.Annotate(err, "failed to get host by hostname %s", hostname).Err()
	}
	if len(devs) > 0 {
		return errors.Reason("fail to add device <%s:%s> due to hostname confliction", hostname, id).Err()
	}
	newEntity := DeviceEntity{
		ID:     DeviceEntityID(id),
		Parent: fakeAcestorKey(ctx),
	}
	if !strings.HasPrefix(id, UUIDPrefix) {
		if err := datastore.Get(ctx, &newEntity); err != datastore.ErrNoSuchEntity {
			return errors.Reason("failed to add device %s due to ID confliction", newEntity).Err()
		}
	}
	return nil
}

// GetDevicesByHostnames returns entities by specified hostnames.
func GetDevicesByHostnames(ctx context.Context, hostnames []string) DeviceOpResults {
	q := datastore.NewQuery(DeviceKind).Ancestor(fakeAcestorKey(ctx))
	retrievingResults := make(DeviceOpResults, len(hostnames))

	// Filter out invalid input hostnames.
	for i, hostname := range hostnames {
		retrievingResults[i].Entity = &DeviceEntity{Hostname: hostname}
		var devs []*DeviceEntity
		if err := datastore.GetAll(ctx, q.Eq("Hostname", hostname), &devs); err != nil {
			retrievingResults[i].logError(errors.Annotate(err, "failed to get host by hostname %s", hostname).Err())
			continue
		}
		if len(devs) == 0 {
			retrievingResults[i].logError(errors.Reason("No such host: %s", hostname).Err())
			continue
		}
		if len(devs) > 1 {
			retrievingResults[i].logError(errors.Reason("multiple hosts found with hostname %s: %v", hostname, devs).Err())
			continue
		}
		retrievingResults[i].Entity = devs[0]
	}
	return retrievingResults
}

func getDUTServoByHostname(ctx context.Context, hostnames []string) ([]*lab.Servo, error) {
	q := datastore.NewQuery(DeviceKind).Ancestor(fakeAcestorKey(ctx))
	var servos []*lab.Servo

	for _, hostname := range hostnames {
		if hostname == "" {
			continue
		}
		var devs []*DeviceEntity
		if err := datastore.GetAll(ctx, q.Eq("Hostname", hostname), &devs); err != nil {
			return nil, errors.Annotate(err, "failed to get DUT by hostname %s", hostname).Err()
		} else if len(devs) == 0 {
			return nil, errors.Reason("No such host: %s", hostname).Err()
		} else if len(devs) > 1 {
			return nil, errors.Reason("multiple entities found with hostname %s: %v", hostname, devs).Err()
		}
		entity := devs[0]
		var devProto lab.ChromeOSDevice
		if err := entity.GetCrosDeviceProto(&devProto); err != nil {
			return nil, errors.Annotate(err, "failed to unmarshal lab config data for %s", hostname).Err()
		}
		dut := devProto.GetDut()
		if dut == nil {
			continue
		}
		servo := dut.GetPeripherals().GetServo()
		if servo == nil {
			continue
		}
		servos = append(servos, servo)
	}
	return servos, nil
}

// AddDevices creates a new Device datastore entity with a unique ID.
func AddDevices(ctx context.Context, devices []*lab.ChromeOSDevice, assignServoPort bool) (*DeviceOpResults, error) {
	updatedTime := time.Now().UTC()

	addMissingID(devices)

	addingResults := make(DeviceOpResults, len(devices))
	for i, d := range devices {
		addingResults[i].Data = d
	}

	f := func(ctx context.Context) error {
		q := datastore.NewQuery(DeviceKind).Ancestor(fakeAcestorKey(ctx))
		entities := make([]*DeviceEntity, 0, len(devices))
		entityResults := make([]*DeviceOpResult, 0, len(devices))
		// Don't use the value returned by `range`. It's a copied value,
		// instead of a reference.
		for i := range addingResults {
			devToAdd := &addingResults[i]
			message := devToAdd.Data.(*lab.ChromeOSDevice)
			hostname := utils.GetHostname(message)
			id := message.GetId().GetValue()

			devToAdd.Entity = &DeviceEntity{
				ID:       DeviceEntityID(id),
				Hostname: hostname,
				Parent:   fakeAcestorKey(ctx),
			}

			if err := sanityCheckForAdding(ctx, message, q); err != nil {
				devToAdd.logError(err)
				continue
			}

			labConfig, err := proto.Marshal(message)
			if err != nil {
				devToAdd.logError(errors.Annotate(err, fmt.Sprintf("fail to marshal device <%s:%s>", hostname, id), err).Err())
				continue
			}
			devToAdd.Entity.Updated = updatedTime
			devToAdd.Entity.LabConfig = labConfig

			entities = append(entities, devToAdd.Entity)
			entityResults = append(entityResults, devToAdd)
		}
		if err := datastore.Put(ctx, entities); err != nil {
			for i, e := range err.(errors.MultiError) {
				entityResults[i].logError(e)
			}
		}
		return nil
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return &addingResults, err
	}
	return &addingResults, nil
}

// GetAllDevices  returns all device entities.
//
// TODO(guocb) optimize for performance if needed.
func GetAllDevices(ctx context.Context) (DeviceOpResults, error) {
	q := datastore.NewQuery(DeviceKind).Ancestor(fakeAcestorKey(ctx))
	var devs []*DeviceEntity
	if err := datastore.GetAll(ctx, q, &devs); err != nil {
		return nil, errors.Annotate(err, "failed to get all devices").Err()
	}
	result := make([]DeviceOpResult, len(devs))
	for i, d := range devs {
		result[i].Entity = d
	}
	return DeviceOpResults(result), nil
}
