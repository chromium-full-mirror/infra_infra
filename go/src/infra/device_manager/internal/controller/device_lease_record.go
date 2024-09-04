// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/external"
	"infra/device_manager/internal/model"
	"infra/libs/fleet/device"
	ufsUtil "infra/unifiedfleet/app/util"
)

// LeaseDevice leases a device specified by the request.
//
// The function executes as a transaction. It attempts to create a lease record
// with an available device. Then it updates the Device's state to LEASED
// and publishes to a PubSub stream. The transaction is then committed.
func LeaseDevice(ctx context.Context, db *sql.DB, psClient *pubsub.Client, r *api.LeaseDeviceRequest, deviceID string, idType model.DeviceIDType) (*api.LeaseDeviceResponse, error) {
	// TODO (b/328662436): Collect metrics
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.New("LeaseDevice: failed to start database transaction")
	}

	deviceToLease := model.Device{
		ID: deviceID,
	}
	updatedDevice, err := model.UpdateDeviceToLeased(ctx, tx, deviceToLease, idType)
	if err != nil {
		logging.Errorf(ctx, "LeaseDevice: failed to update device state to leased: %s", err)

		// Handle error if Device is already leased
		if errors.Is(err, model.ErrDeviceAlreadyLeased) {
			return &api.LeaseDeviceResponse{
				ErrorType:   api.LeaseDeviceResponseErrorType_LEASE_ERROR_TYPE_DEVICE_ALREADY_LEASED,
				ErrorString: fmt.Sprintf("Device %s was already leased", deviceID),
			}, nil
		}

		return nil, err
	}
	logging.Debugf(ctx, "LeaseDevice: marked Device %s as leased successfully: %v", updatedDevice.ID, updatedDevice)

	newRecord := model.DeviceLeaseRecord{
		ID:             uuid.New().String(),
		IdempotencyKey: r.GetIdempotencyKey(),
		DeviceID:       updatedDevice.ID,
		DeviceAddress:  updatedDevice.DeviceAddress,
		DeviceType:     updatedDevice.DeviceType,
	}
	createdRecord, err := model.CreateDeviceLeaseRecord(ctx, tx, newRecord, r.GetLeaseDuration().AsDuration())
	if err != nil {
		logging.Errorf(ctx, "LeaseDevice: failed to create DeviceLeaseRecord %s", err)
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	logging.Debugf(ctx, "LeaseDevice: created DeviceLeaseRecord %v", newRecord)
	return &api.LeaseDeviceResponse{
		DeviceLease: &api.DeviceLeaseRecord{
			Id:             createdRecord.ID,
			IdempotencyKey: createdRecord.IdempotencyKey,
			DeviceId:       createdRecord.DeviceID,
			DeviceAddress: &api.DeviceAddress{
				Host: createdRecord.DeviceAddress,
			},
			DeviceType:      stringToDeviceType(ctx, createdRecord.DeviceType),
			LeasedTime:      timestamppb.New(createdRecord.LeasedTime),
			ReleasedTime:    timestamppb.New(createdRecord.ReleasedTime),
			ExpirationTime:  timestamppb.New(createdRecord.ExpirationTime),
			LastUpdatedTime: timestamppb.New(createdRecord.LastUpdatedTime),
		},
	}, nil
}

// ExtendLease attempts to extend the lease on a device.
//
// ExtendLease checks the requested lease to verify that it is unexpired. If
// unexpired, it will extend the lease by the requested duration. This maintains
// the leased state on a device.
func ExtendLease(ctx context.Context, db *sql.DB, r *api.ExtendLeaseRequest) (*api.ExtendLeaseResponse, error) {
	// TODO (b/328662436): Collect metrics
	record, err := model.GetDeviceLeaseRecordByID(ctx, db, r.GetLeaseId())
	if err != nil {
		return &api.ExtendLeaseResponse{}, err
	}

	timeNow := time.Now()
	if record.ExpirationTime.Before(timeNow) {
		return &api.ExtendLeaseResponse{
			LeaseId:        r.GetLeaseId(),
			ExpirationTime: timestamppb.New(record.ExpirationTime),
		}, errors.New("ExtendLease: lease is already expired")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.New("ExtendLease: failed to start database transaction")
	}

	// Record ExtendLeaseRequest in DB
	extendDur := r.GetExtendDuration().GetSeconds()
	newExpirationTime := record.ExpirationTime.Add(time.Second * time.Duration(extendDur))
	newRequest := model.ExtendLeaseRequest{
		ID:             uuid.New().String(),
		LeaseID:        r.GetLeaseId(),
		IdempotencyKey: r.GetIdempotencyKey(),
		ExtendDuration: extendDur,
		ExpirationTime: newExpirationTime,
	}

	err = model.CreateExtendLeaseRequest(ctx, tx, newRequest)
	if err != nil {
		logging.Errorf(ctx, "ExtendLease: failed to create ExtendLeaseRequest %s", err)
		return nil, err
	}

	// Update DeviceLeaseRecord with new expiration time
	updatedRec := model.DeviceLeaseRecord{
		ID:             r.GetLeaseId(),
		ExpirationTime: newExpirationTime,
	}

	err = model.ExtendLease(ctx, tx, updatedRec)
	if err != nil {
		logging.Errorf(ctx, "ExtendLease: failed to update DeviceLeaseRecord %s: %s", updatedRec.ID, err)
		return nil, err
	}
	logging.Debugf(ctx, "ExtendLease: updated Device %s successfully", updatedRec.ID)

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	logging.Debugf(ctx, "ExtendLease: created ExtendLeaseRequest %v", newRequest)
	return &api.ExtendLeaseResponse{
		LeaseId:        r.GetLeaseId(),
		ExpirationTime: timestamppb.New(newRequest.ExpirationTime),
	}, nil
}

// ReleaseDevice releases the leased device.
//
// ReleaseDevice takes a lease ID and releases the device associated. In a
// transaction, the RPC will update the lease and set the device to be
// available.
func ReleaseDevice(ctx context.Context, db *sql.DB, psClient *pubsub.Client, r *api.ReleaseDeviceRequest) (*api.ReleaseDeviceResponse, error) {
	// TODO (b/328662436): Collect metrics
	record, err := model.GetDeviceLeaseRecordByID(ctx, db, r.GetLeaseId())
	if err != nil {
		return nil, err
	}

	timeNow := time.Now()
	if !record.ReleasedTime.IsZero() && record.ReleasedTime.Before(timeNow) {
		logging.Debugf(ctx, "ReleaseDevice: leased device was already released")
		return &api.ReleaseDeviceResponse{
			LeaseId:     r.GetLeaseId(),
			ErrorType:   api.ReleaseDeviceResponseErrorType_ERROR_TYPE_DEVICE_ALREADY_RELEASED,
			ErrorString: fmt.Sprintf("Lease %s for device %s was already released", r.GetLeaseId(), record.DeviceID),
		}, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.New("ReleaseDevice: failed to start database transaction")
	}

	// Update lease record to mark released time
	releaseRec := model.DeviceLeaseRecord{
		ID: r.GetLeaseId(),
	}
	err = model.ReleaseLease(ctx, tx, releaseRec)
	if err != nil {
		logging.Errorf(ctx, "ReleaseDevice: failed to release lease %s: %s", releaseRec.ID, err)
		return nil, err
	}

	// Pull device data from UFS
	ctx = external.SetupContext(ctx, ufsUtil.OSNamespace)
	client, err := external.NewUFSClient(ctx, external.UFSServiceURI)
	if err != nil {
		return nil, err
	}

	// Update device and device lease state to available after release
	updatedDevice := model.Device{
		ID:          record.DeviceID,
		DeviceState: "DEVICE_STATE_AVAILABLE",
		IsActive:    true,
	}

	// Try to pull dimensions from Device. Mark as inactive if not found.
	reportFunc := func(e error) { logging.Debugf(ctx, "sanitize dimensions: %s\n", e) }
	dims, err := device.GetOSResourceDims(ctx, client, reportFunc, record.DeviceID)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			updatedDevice.IsActive = false
		default:
			return nil, err
		}
	}

	if dims != nil {
		updatedDevice.SchedulableLabels = SwarmingDimsToLabels(ctx, dims)
	}

	err = UpdateDevice(ctx, tx, updatedDevice)
	if err != nil {
		logging.Errorf(ctx, "ReleaseDevice: failed to release device %s: %s", record.DeviceID, err)
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	logging.Debugf(ctx, "ReleaseDevice: released lease %s for device %s", r.GetLeaseId(), record.DeviceID)
	return &api.ReleaseDeviceResponse{
		LeaseId: r.GetLeaseId(),
	}, nil
}

// CheckLeaseIdempotency checks if there is a record with the same idempotency key.
//
// If there is an unexpired record, it will return the record. If it is expired,
// it will error. If there is no record, it will return an empty response and no
// error.
func CheckLeaseIdempotency(ctx context.Context, db *sql.DB, idemKey string) (*api.LeaseDeviceResponse, error) {
	timeNow := time.Now()
	existingRecord, err := model.GetDeviceLeaseRecordByIdemKey(ctx, db, idemKey)
	if err == nil {
		if existingRecord.ExpirationTime.After(timeNow) {
			addr, err := stringToDeviceAddress(ctx, existingRecord.DeviceAddress)
			if err != nil {
				addr = &api.DeviceAddress{}
			}

			return &api.LeaseDeviceResponse{
				DeviceLease: &api.DeviceLeaseRecord{
					Id:              existingRecord.ID,
					IdempotencyKey:  existingRecord.IdempotencyKey,
					DeviceId:        existingRecord.DeviceID,
					DeviceAddress:   addr,
					DeviceType:      api.DeviceType_DEVICE_TYPE_PHYSICAL,
					LeasedTime:      timestamppb.New(existingRecord.LeasedTime),
					ReleasedTime:    timestamppb.New(existingRecord.ReleasedTime),
					ExpirationTime:  timestamppb.New(existingRecord.ExpirationTime),
					LastUpdatedTime: timestamppb.New(existingRecord.LastUpdatedTime),
				},
			}, nil
		} else {
			return &api.LeaseDeviceResponse{}, errors.New("CheckLeaseIdempotency: DeviceLeaseRecord found with same idempotency key but is already expired")
		}
	}
	return &api.LeaseDeviceResponse{}, nil
}

// CheckExtensionIdempotency checks if there is a extend request with the same
// idempotency key.
//
// If there is a duplicate request, it will return the request. If there is no
// record, it will return an empty response and no error.
func CheckExtensionIdempotency(ctx context.Context, db *sql.DB, idemKey string) (*api.ExtendLeaseResponse, error) {
	existingRecord, err := model.GetExtendLeaseRequestByIdemKey(ctx, db, idemKey)
	if err == nil {
		return &api.ExtendLeaseResponse{
			LeaseId:        existingRecord.LeaseID,
			ExpirationTime: timestamppb.New(existingRecord.ExpirationTime),
		}, nil
	}
	return &api.ExtendLeaseResponse{}, nil
}
