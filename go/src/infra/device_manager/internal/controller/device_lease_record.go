// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/chromiumos/config/go/test/api"
	lucierr "go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"infra/device_manager/internal/external"
	"infra/device_manager/internal/model"
	"infra/libs/fleet/device"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// ExpirerOpts struct holds configuration options for the Expirer service
type ExpirerOpts struct {
	ExpirationWorkersN *int
}

// LeaseDevice leases a device specified by the request.
//
// The function executes as a transaction. It attempts to create a lease record
// with an available device. Then it updates the Device's state to LEASED
// and publishes to a PubSub stream. The transaction is then committed.
func LeaseDevice(ctx context.Context, db *sql.DB, r *api.LeaseDeviceRequest, deviceID string, idType model.DeviceIDType) (*api.LeaseDeviceResponse, error) {
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

	newRecord := model.DeviceLeaseRecord{
		ID:             uuid.New().String(),
		IdempotencyKey: r.GetIdempotencyKey(),
		DutID:          updatedDevice.DutID,
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

	// log success after commit success
	logging.Debugf(ctx, "LeaseDevice: marked Device %s as leased successfully: %v", updatedDevice.ID, updatedDevice)
	logging.Debugf(ctx, "LeaseDevice: created DeviceLeaseRecord %v", newRecord)

	return &api.LeaseDeviceResponse{
		DeviceLease: &api.DeviceLeaseRecord{
			Id:             createdRecord.ID,
			IdempotencyKey: createdRecord.IdempotencyKey,
			DutId:          createdRecord.DutID,
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

// BulkLeaseDevices leases multiple Devices specified by the request.
//
// The function executes as a transaction. It attempts to create lease records
// on available Devices. Then it updates the Devices' state to LEASED and
// publishes to a PubSub stream. The transaction is then committed.
//
// All calls to this RPC return 200 (except for panics), with global errors
// returned in the bulk response, or individual errors per-device returned
// in the list of per-device responses.
func BulkLeaseDevices(ctx context.Context, db *sql.DB, r *api.BulkLeaseDevicesRequest) (*api.BulkLeaseDevicesResponse, error) {
	reqs := r.GetLeaseDeviceRequests()
	bulkResp := &api.BulkLeaseDevicesResponse{
		LeaseDeviceResponses: make([]*api.LeaseDeviceResponse, len(reqs)),
	}
	var deviceIDs []string
	reqMap := map[string]*api.LeaseDeviceRequest{}
	respMap := map[string]*api.LeaseDeviceResponse{}

	// Extract device IDs for bulk leasing, and construct the bulk lease response
	// in the same order as the bulk lease request.
	logging.Debugf(ctx, "BulkLeaseDevices: extracting DUT IDs from requests")
	for i, req := range reqs {
		resp := &api.LeaseDeviceResponse{}
		bulkResp.LeaseDeviceResponses[i] = resp
		deviceLabels := req.GetHardwareDeviceReqs().GetSchedulableLabels()
		if len(deviceLabels) == 0 {
			resp.ErrorType = api.LeaseDeviceResponseErrorType_LEASE_ERROR_TYPE_DEVICE_NOT_FOUND
			resp.ErrorString = "schedulable labels are empty"
			continue
		}
		deviceID, err := ExtractSingleValuedDimension(ctx, deviceLabels, string(model.IDTypeDutID))
		if err != nil {
			resp.ErrorType = api.LeaseDeviceResponseErrorType_LEASE_ERROR_TYPE_DEVICE_NOT_FOUND
			resp.ErrorString = err.Error()
			logging.Debugf(ctx, err.Error())
			continue
		}
		deviceIDs = append(deviceIDs, deviceID)
		reqMap[deviceID] = req
		respMap[deviceID] = resp
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logging.Errorf(ctx, "BulkLeaseDevices: failed to start database transaction: %w", err)
		return &api.BulkLeaseDevicesResponse{
			ErrorType:   api.BulkLeaseDevicesResponseErrorType_BULK_LEASE_ERROR_TYPE_INTERNAL_DATABASE_ERR,
			ErrorString: fmt.Sprintf("BulkLeaseDevices: failed to start database transaction: %v", err),
		}, nil
	}
	defer tx.Rollback()

	// Update DB to reflect that the devices are leased.
	logging.Debugf(ctx, "BulkLeaseDevices: bulk updating Devices to leased")
	updatedDevices, updateDeviceErrs, err := model.BulkUpdateDevicesToLeased(ctx, tx, deviceIDs, model.IDTypeDutID)
	if err != nil {
		logging.Errorf(ctx, "BulkLeaseDevices: %w. Failed to lease Devices %v", err, deviceIDs)
		return &api.BulkLeaseDevicesResponse{
			ErrorType:   api.BulkLeaseDevicesResponseErrorType_BULK_LEASE_ERROR_TYPE_INTERNAL_DATABASE_ERR,
			ErrorString: fmt.Sprintf("Database error: %s. Could not lease Devices %v", err, deviceIDs),
		}, nil
	}
	newRecords := make([]model.DeviceLeaseRecord, 0, len(updatedDevices))
	leaseDursMap := make([]time.Duration, 0, len(updatedDevices))

	// Update the bulk response.
	for deviceID, d := range updatedDevices {
		newRecords = append(newRecords, model.DeviceLeaseRecord{
			ID:             uuid.New().String(),
			IdempotencyKey: reqMap[deviceID].GetIdempotencyKey(),
			DutID:          d.DutID,
			DeviceID:       d.ID,
			DeviceAddress:  d.DeviceAddress,
			DeviceType:     d.DeviceType,
		})
		leaseDursMap = append(leaseDursMap, reqMap[deviceID].GetLeaseDuration().AsDuration())
	}
	for deviceID, err := range updateDeviceErrs {
		if err != nil {
			respMap[deviceID].ErrorType = api.LeaseDeviceResponseErrorType_LEASE_ERROR_TYPE_DEVICE_ALREADY_LEASED
			respMap[deviceID].ErrorString = err.Error()
		}
	}

	// If all individual responses have errors at this point, no need to continue.
	if allDevicesHaveErrors(bulkResp.GetLeaseDeviceResponses()) {
		return bulkResp, nil
	}

	// Create lease records for each device.
	logging.Debugf(ctx, "BulkLeaseDevices: bulk creating lease records for Devices")
	createdRecords, createRecordErrs, err := model.BulkCreateDeviceLeaseRecords(ctx, tx, newRecords, leaseDursMap)
	if err != nil {
		logging.Errorf(ctx, "BulkLeaseDevices: failed to bulk create DeviceLeaseRecords: %w", err)
		return &api.BulkLeaseDevicesResponse{
			ErrorType:   api.BulkLeaseDevicesResponseErrorType_BULK_LEASE_ERROR_TYPE_INTERNAL_DATABASE_ERR,
			ErrorString: fmt.Sprintf("BulkLeaseDevices: database error: %s. Could not lease Devices %v", err, deviceIDs),
		}, nil
	}

	// Cannot commit transaction because there are failed leases. This is so that
	// we don't mark Devices as leased without creating an actual lease.
	leaseErrCnt := 0
	for deviceID, err := range createRecordErrs {
		if err != nil {
			respMap[deviceID].ErrorType = api.LeaseDeviceResponseErrorType_LEASE_ERROR_TYPE_DEVICE_ALREADY_LEASED
			respMap[deviceID].ErrorString = err.Error()
			leaseErrCnt += 1
		}
	}

	// Return errored request if there is one failed leasing request.
	if leaseErrCnt > 0 {
		logging.Errorf(ctx, "BulkLeaseDevices: lease record errors detected; abort transaction")
		bulkResp.ErrorType = api.BulkLeaseDevicesResponseErrorType_BULK_LEASE_ERROR_TYPE_PARTIAL_LEASE_FAILURE
		bulkResp.ErrorString = fmt.Sprintf("BulkLeaseDevices: lease record error detected; aborting bulk operation: %v; Devices: %v", err, deviceIDs)
		return bulkResp, nil
	}

	// Commit transaction.
	if err = tx.Commit(); err != nil {
		logging.Errorf(ctx, "BulkLeaseDevices: failed to commit database transaction: %w", err)
		return &api.BulkLeaseDevicesResponse{
			ErrorType:   api.BulkLeaseDevicesResponseErrorType_BULK_LEASE_ERROR_TYPE_INTERNAL_DATABASE_ERR,
			ErrorString: fmt.Sprintf("BulkLeaseDevices: failed to commit database transaction: %v", err),
		}, nil
	}
	logging.Debugf(ctx, "BulkLeaseDevices: successfully bulk created lease records for Devices %+v", deviceIDs)

	// Update the bulk response.
	for deviceID, r := range createdRecords {
		respMap[deviceID].DeviceLease = &api.DeviceLeaseRecord{
			Id:             r.ID,
			IdempotencyKey: r.IdempotencyKey,
			DutId:          r.DutID,
			DeviceId:       r.DeviceID,
			DeviceAddress: &api.DeviceAddress{
				Host: r.DeviceAddress,
			},
			DeviceType:      stringToDeviceType(ctx, r.DeviceType),
			LeasedTime:      timestamppb.New(r.LeasedTime),
			ReleasedTime:    timestamppb.New(r.ReleasedTime),
			ExpirationTime:  timestamppb.New(r.ExpirationTime),
			LastUpdatedTime: timestamppb.New(r.LastUpdatedTime),
		}
	}
	return bulkResp, nil
}

func allDevicesHaveErrors(resps []*api.LeaseDeviceResponse) bool {
	for _, r := range resps {
		if r.ErrorType == api.LeaseDeviceResponseErrorType_LEASE_ERROR_TYPE_NONE {
			return false
		}
	}
	return true
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
	defer func() {
		if err := tx.Rollback(); err != nil {
			logging.Debugf(ctx, "ExtendLease: unable to rollback: %s", err)
		}
	}()

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

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	// log success after commit success
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
func ReleaseDevice(ctx context.Context, db *sql.DB, r *api.ReleaseDeviceRequest) (*api.ReleaseDeviceResponse, error) {
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
	err = model.ReleaseLease(ctx, tx, &releaseRec)
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
	toReleaseDevice := model.Device{
		ID:       record.DeviceID,
		DutID:    record.DutID,
		IsActive: true,
	}

	// Try to pull dimensions from Device. Mark as inactive if not found.
	reportFunc := func(e error) { logging.Debugf(ctx, "sanitize dimensions: %s\n", e) }
	dims, err := device.GetOSResourceDims(ctx, client, reportFunc, record.DeviceID)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			toReleaseDevice.IsActive = false
		default:
			return nil, err
		}
	}

	if dims != nil {
		toReleaseDevice.SchedulableLabels = SwarmingDimsToLabels(ctx, dims)
	}

	d, err := model.UpdateDeviceToAvailable(ctx, tx, toReleaseDevice)
	if err != nil {
		logging.Errorf(ctx, "ReleaseDevice: failed to release device %s dut_id %s: %s", record.DeviceID, record.DutID, err)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			logging.Errorf(ctx, "UpdateDeviceToAvailable: unable to rollback: %v", rollbackErr)
		}
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	// log success after commit success
	logging.Debugf(ctx, "ReleaseDevice: released lease %s for device %s dut_id %s", r.GetLeaseId(), d.ID, d.DutID)

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
					DutId:           existingRecord.DutID,
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

// ExpireLeases marks expired leases as released and releases Devices in the DB.
func ExpireLeases(ctx context.Context, db *sql.DB, opts *ExpirerOpts) error {
	var (
		// queryTime is what will be used as expiration time. It is important to
		// get this before sending the query to guard against lease updates during
		// this expiry op.
		queryTime = time.Now()
	)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		err = lucierr.Annotate(err, "ExpireLeases: starting database transaction").Err()
		logging.Errorf(ctx, err.Error())
		return err
	}

	releasedLeases, err := model.ExpireLeases(ctx, tx, queryTime)
	if err != nil {
		return fmt.Errorf("expireLeases: %w", err)
	}

	// Pull device data from UFS
	ctx = external.SetupContext(ctx, ufsUtil.OSNamespace)
	ufsClient, err := external.NewUFSClient(ctx, external.UFSServiceURI)
	if err != nil {
		return fmt.Errorf("expireLeases: %w", err)
	}

	var (
		releaseDeviceChan = make(chan *model.Device, *opts.ExpirationWorkersN)
		successChan       = make(chan *model.Device, len(releasedLeases))
		failureChan       = make(chan *model.Device, len(releasedLeases))
		wg                sync.WaitGroup
	)
	defer close(successChan)
	defer close(failureChan)

	wg.Add(*opts.ExpirationWorkersN)
	for range *opts.ExpirationWorkersN {
		go getDeviceUFSDataWorker(ctx, &wg, tx, ufsClient, releaseDeviceChan, successChan, failureChan)
	}

	for _, lease := range releasedLeases {
		// Update device and device lease state to available after release
		toReleaseDevice := model.Device{
			ID:       lease.DeviceID,
			DutID:    lease.DutID,
			IsActive: true,
		}
		releaseDeviceChan <- &toReleaseDevice
		logging.Debugf(ctx, "Queued to pull UFS data for Device %s", toReleaseDevice.ID)
	}
	close(releaseDeviceChan)

	// Wait for all Devices to be processed before committing transaction.
	wg.Wait()
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("expireLeases: %w", err)
	}

	if len(successChan) > 0 || len(failureChan) > 0 {
		logging.Debugf(ctx, "ExpireLeases: successfully released %d expired Devices: %v", len(successChan), successChan)
		logging.Debugf(ctx, "ExpireLeases: failed to release %d expired Devices: %v", len(failureChan), failureChan)
	}
	return nil
}

// getDeviceUFSDataWorker takes a queue of Devices and pulls UFS data for them
// one by one. Devices are queued and dequeued continuously.
func getDeviceUFSDataWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	tx *sql.Tx,
	ufsClient ufsAPI.FleetClient,
	devices <-chan *model.Device,
	success chan<- *model.Device,
	failure chan<- *model.Device,
) {
	for d := range devices {
		// Try to pull dimensions from Device. Mark as inactive if not found.
		reportFunc := func(e error) { logging.Debugf(ctx, "sanitize dimensions: %s\n", e) }
		dims, err := device.GetOSResourceDims(ctx, ufsClient, reportFunc, d.ID)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				// Not found indicates that the Device no longer exists in UFS meaning
				// it is inactive i.e. decommed
				d.IsActive = false
			}
			logging.Warningf(ctx, "Failed to find dimensions for Device %s: %v", d.ID, err)
			failure <- d
			continue
		}

		if dims != nil {
			d.SchedulableLabels = SwarmingDimsToLabels(ctx, dims)
		}

		updatedDevice, err := model.UpdateDeviceToAvailable(ctx, tx, *d)
		if err != nil {
			logging.Errorf(ctx, "ExpireLeases: failed to release Device %s dut_id %s: %s", d.ID, d.DutID, err)
			failure <- d
			continue
		}
		success <- &updatedDevice

		logging.Debugf(ctx, "getDeviceUFSDataWorker: pending release of Device %s dut_id %s", updatedDevice.ID, updatedDevice.DutID)
	}
	wg.Done()
}
