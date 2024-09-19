// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/protoadapt"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/tsmon/distribution"

	shivasUtil "infra/cmd/shivas/utils"
	"infra/device_manager/internal/controller"
	"infra/device_manager/internal/database"
	"infra/device_manager/internal/external"
	"infra/device_manager/internal/frontend"
	"infra/device_manager/internal/metrics"
	"infra/device_manager/internal/model"
	"infra/libs/fleet/device"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	ufsUtil "infra/unifiedfleet/app/util"
)

// TODO: b/328662436 - Collect metrics and replace logging with metrics.
// TODO: b/331644796 - Import non-OS device data.

// maxUFSImportJobs sets the maximum concurrent jobs for this cron.
const maxUFSImportJobs = 100

var (
	updatedDevicesN   = 0
	getDeviceErrN     = 0
	upsertDeviceErrN  = 0
	refetchDeviceErrN = 0
)

// ImportUFSDevices registers the cron to trigger import for all Device
// information from UFS.
func ImportUFSDevices(ctx context.Context, serviceClients frontend.ServiceClients, project string) error {
	start := time.Now()
	ctx = external.SetupContext(ctx, ufsUtil.OSNamespace)
	ufsClient, err := external.NewUFSClient(ctx, external.UFSServiceURI)
	if err != nil {
		return err
	}
	serviceClients.UFSClient = ufsClient
	lses, err := getAllMachineLSEs(ctx, serviceClients.UFSClient)
	if err != nil {
		return err
	}
	logging.Debugf(ctx, "ImportUFSDevices: found %d DUTs in UFS OS namespace", len(lses))

	sUnits, err := getAllSchedulingUnits(ctx, serviceClients.UFSClient)
	if err != nil {
		return err
	}
	logging.Debugf(ctx, "ImportUFSDevices: found %d SUs in UFS OS namespace", len(sUnits))

	// Contains active hostnames of SchedulingUnits and individual MachineLSEs.
	// A device marked inactive simply means they should not be exposed or be used
	// by the scheduling layer. For example, the components of a SchedulingUnit
	// should not be individually schedulable.
	var activeDUTs []string

	// map for MachineLSEs associated with SchedulingUnit for easy search
	lseInSUnitMap := make(map[string]bool)
	for _, su := range sUnits {
		if len(su.GetMachineLSEs()) > 0 {
			activeDUTs = append(activeDUTs, su.GetName())
			for _, lseName := range su.GetMachineLSEs() {
				lseInSUnitMap[lseName] = true
			}
		}
	}
	logging.Debugf(ctx, "ImportUFSDevices: %d SchedulingUnit DUTs to be marked inactive", len(lseInSUnitMap))

	// add all individual MachineLSEs as active DUTs
	for _, lse := range lses {
		if !lseInSUnitMap[ufsUtil.RemovePrefix(lse.GetName())] {
			activeDUTs = append(activeDUTs, lse.GetName())
		}
	}
	logging.Debugf(ctx, "ImportUFSDevices: found %d active devices to update", len(activeDUTs))

	// get inactive DUTs
	inactiveDUTs, err := getInactiveDevices(ctx, serviceClients, activeDUTs)
	if err != nil {
		return err
	}

	// loop through all active and inactive MachineLSEs and upsert as Devices
	waitQueue := make(chan struct{}, maxUFSImportJobs)
	wg := sync.WaitGroup{}
	for _, d := range inactiveDUTs {
		waitQueue <- struct{}{}
		wg.Add(1)
		go upsertDeviceData(ctx, waitQueue, &wg, serviceClients, d, false)
	}

	for _, dutName := range activeDUTs {
		waitQueue <- struct{}{}
		wg.Add(1)
		go upsertDeviceData(ctx, waitQueue, &wg, serviceClients, dutName, true)
	}
	wg.Wait()
	close(waitQueue)

	// Publish metrics.
	publishJobMetrics(ctx, start, len(lses), project)

	return nil
}

// publishJobMetrics publishes metrics for this job.
func publishJobMetrics(ctx context.Context, startTime time.Time, lseN int, project string) {
	// Job actions.
	uad := distribution.New(metrics.UFSActionsPerJob.Bucketer())
	uad.Add(float64(updatedDevicesN))
	metrics.UFSActionsPerJob.Set(ctx, uad, project, "updated_devices")

	lad := distribution.New(metrics.UFSActionsPerJob.Bucketer())
	lad.Add(float64(lseN))
	metrics.UFSActionsPerJob.Set(ctx, lad, project, "lses_processed")

	// Job errors.
	ged := distribution.New(metrics.UFSJobErrorCount.Bucketer())
	ged.Add(float64(getDeviceErrN))
	metrics.UFSJobErrorCount.Set(ctx, ged, project, "get_device")

	ued := distribution.New(metrics.UFSJobErrorCount.Bucketer())
	ued.Add(float64(upsertDeviceErrN))
	metrics.UFSJobErrorCount.Set(ctx, ued, project, "upsert_device")

	red := distribution.New(metrics.UFSJobErrorCount.Bucketer())
	red.Add(float64(refetchDeviceErrN))
	metrics.UFSJobErrorCount.Set(ctx, red, project, "refetch_device")

	// Job runtime.
	rd := distribution.New(metrics.UFSJobRuntime.Bucketer())
	rd.Add(time.Since(startTime).Seconds())
	metrics.UFSJobRuntime.Set(ctx, rd, project)
}

// getAllMachineLSEs gets all MachineLSEs
func getAllMachineLSEs(ctx context.Context, ic ufsAPI.FleetClient) ([]*ufspb.MachineLSE, error) {
	res, err := shivasUtil.BatchList(ctx, ic, listMachineLSEs, []string{}, 0, true, false, nil)
	if err != nil {
		return nil, errors.Annotate(err, "getAllMachineLSEs").Err()
	}
	lses := make([]*ufspb.MachineLSE, len(res))
	for i, r := range res {
		lses[i] = r.(*ufspb.MachineLSE)
	}
	return lses, nil
}

// listMachineLSEs calls ListMachineLSEs in UFS to get a list of MachineLSEs
func listMachineLSEs(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]protoadapt.MessageV1, string, error) {
	req := &ufsAPI.ListMachineLSEsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
		Full:      full,
	}
	res, err := ic.ListMachineLSEs(ctx, req)
	if err != nil {
		return nil, "", errors.Annotate(err, "listMachineLSEs").Err()
	}
	protos := make([]protoadapt.MessageV1, len(res.GetMachineLSEs()))
	for i, kvm := range res.GetMachineLSEs() {
		protos[i] = kvm
	}
	return protos, res.GetNextPageToken(), nil
}

// getAllSchedulingUnits gets all SchedulingUnits
func getAllSchedulingUnits(ctx context.Context, ic ufsAPI.FleetClient) ([]*ufspb.SchedulingUnit, error) {
	res, err := shivasUtil.BatchList(ctx, ic, listSchedulingUnits, []string{}, 0, false, true, nil)
	if err != nil {
		return nil, errors.Annotate(err, "getAllSchedulingUnits").Err()
	}
	sUnits := make([]*ufspb.SchedulingUnit, len(res))
	for i, r := range res {
		sUnits[i] = r.(*ufspb.SchedulingUnit)
	}
	return sUnits, nil
}

// listSchedulingUnits calls ListSchedulingUnits in UFS to get a list of SchedulingUnits
func listSchedulingUnits(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]protoadapt.MessageV1, string, error) {
	req := &ufsAPI.ListSchedulingUnitsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
	}
	res, err := ic.ListSchedulingUnits(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]protoadapt.MessageV1, len(res.GetSchedulingUnits()))
	for i, m := range res.GetSchedulingUnits() {
		protos[i] = m
	}
	return protos, res.GetNextPageToken(), nil
}

// getAllDMDevices gets all Devices in the Device Manager DB
func getAllDMDevices(ctx context.Context, db *sql.DB) ([]model.Device, error) {
	var (
		pageToken database.PageToken
		devices   []model.Device
	)

	for {
		res, token, err := model.ListDevices(ctx, db, pageToken, database.DefaultPageSize, "")
		if err != nil {
			return nil, err
		}
		devices = append(devices, res...)

		if token == "" {
			break
		}
		pageToken = token
	}

	logging.Debugf(ctx, "getAllDMDevices: found %d Devices in database", len(devices))
	return devices, nil
}

// upsertDeviceData upserts to db and publishes a device event with UFS device data
func upsertDeviceData(ctx context.Context, queue <-chan struct{}, wg *sync.WaitGroup, serviceClients frontend.ServiceClients, name string, active bool) {
	// catch panic and continue
	defer func() {
		if err := recover(); err != nil {
			logging.Debugf(ctx, "panic occurred: %s", err)
		}
		<-queue
		wg.Done()
	}()

	// DeviceState will be clobbered by SQL COALESCE statement in
	// UpsertDeviceFromUFS.
	deviceModel := model.Device{
		ID:          ufsUtil.RemovePrefix(name),
		DeviceType:  "DEVICE_TYPE_PHYSICAL",
		DeviceState: "DEVICE_STATE_AVAILABLE",
		IsActive:    active,
	}

	r := func(e error) { logging.Debugf(ctx, "sanitize dimensions: %s\n", e) }
	dims, err := device.GetOSResourceDims(ctx, serviceClients.UFSClient, r, name)
	if err != nil {
		return
	}
	deviceModel.SchedulableLabels = controller.SwarmingDimsToLabels(ctx, dims)
	err = deviceModel.SetDutIDFromLabels(ctx)
	if err != nil {
		logging.Errorf(ctx, "Failed to set DUT ID using schedulable labels for Device %s: %s", deviceModel.ID, err)
		return
	}
	dbDevice, err := model.GetDeviceByID(ctx, serviceClients.DBClient.Conn, model.IDTypeHostname, deviceModel.ID)

	// System error in looking up device
	if err != nil && !errors.Is(err, model.ErrDeviceNotFound) {
		logging.Errorf(ctx, "Failed to get Device %s: %s", deviceModel.ID, err)
		getDeviceErrN++
		return
	}

	// Device found and not different
	if !errors.Is(err, model.ErrDeviceNotFound) && !areLabelsOrActiveStateDifferent(ctx, dbDevice, deviceModel) {
		logging.Debugf(ctx, "Device %s did not change. Did not update Device in database", deviceModel.ID)
		return
	}

	// Either Device was not found and is new or it is different
	logging.Debugf(ctx, "Found changes for Device %s. Upserting to DB", deviceModel.ID)
	err = model.UpsertDeviceFromUFS(ctx, serviceClients.DBClient.Conn, deviceModel)
	if err != nil {
		logging.Errorf(ctx, "Failed to upsert Device %s: %s", deviceModel.ID, err)
		upsertDeviceErrN++
		return
	}
	updatedDevicesN++

	// Re-fetch the Device after the upsert to get the updated Device
	dbDevice, err = model.GetDeviceByID(ctx, serviceClients.DBClient.Conn, model.IDTypeHostname, deviceModel.ID)
	if err != nil {
		logging.Errorf(ctx, "Failed to re-fetch Device %s: %s", deviceModel.ID, err)
		refetchDeviceErrN++
		return
	}
}

// getInactiveDevices marks inactive Devices as inactive and returns the list.
//
// This function takes a list of Devices and compares them with the list of
// Devices managed by Device Manager. It marks Devices that are not in the
// active list as inactive and returns the list.
func getInactiveDevices(ctx context.Context, serviceClients frontend.ServiceClients, activeDevices []string) ([]string, error) {
	dmDevices, err := getAllDMDevices(ctx, serviceClients.DBClient.Conn)
	if err != nil {
		return nil, err
	}

	// create a map of active Device names
	activeMap := make(map[string]struct{}, len(dmDevices))
	for _, activeDevice := range activeDevices {
		activeMap[ufsUtil.RemovePrefix(activeDevice)] = struct{}{}
	}

	// the set difference of active Devices - all DM Devices = inactive Devices
	var inactiveDevices []string
	for _, dmDevice := range dmDevices {
		// only process DM Devices that are currently marked active
		if _, found := activeMap[dmDevice.ID]; !found {
			inactiveDevices = append(inactiveDevices, dmDevice.ID)
		}
	}

	logging.Debugf(ctx, "getInactiveDevices: found %d inactive DUTs", len(inactiveDevices))
	return inactiveDevices, nil
}

// areLabelsOrActiveStateDifferent checks if the labels or the active state of
// two Devices are different from one another.
func areLabelsOrActiveStateDifferent(ctx context.Context, d1, d2 model.Device) bool {
	if d1.ID != d2.ID {
		panic(fmt.Sprintf("comparing two different devices %s and %s", d1.ID, d2.ID))
	}

	// compare Device fields
	if !cmp.Equal(d1.SchedulableLabels, d2.SchedulableLabels) {
		logging.Debugf(ctx, "%s SchedulableLabels is different:\n %s\n %s", d1.ID, d1.SchedulableLabels, d2.SchedulableLabels)
		return true
	}
	if d1.IsActive != d2.IsActive {
		logging.Debugf(ctx, "%s IsActive is different", d1.ID)
		return true
	}
	if d1.DutID != d2.DutID {
		logging.Debugf(ctx, "%s DUT ID is different", d1.ID)
		return true
	}
	return false
}
