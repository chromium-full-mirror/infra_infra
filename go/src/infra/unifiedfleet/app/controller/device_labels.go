// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"slices"

	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/cros/dutstate"
	"infra/libs/fleet/device/attacheddevice"
	"infra/libs/fleet/device/dut"
	"infra/libs/fleet/device/schedulingunit"
	"infra/libs/skylab/inventory/swarming"
	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/util"
)

// CreateDeviceLabels creates a new DeviceLabel in datastore.
func CreateDeviceLabels(ctx context.Context, deviceLabels *ufspb.DeviceLabels, realm string) (*ufspb.DeviceLabels, error) {
	f := func(ctx context.Context) error {
		hc := getDeviceLabelsHistoryClient(deviceLabels)

		if err := validateCreateDeviceLabels(ctx, deviceLabels, realm); err != nil {
			return errors.Annotate(err, "Validation error - Failed to create Device Labels").Err()
		}

		if _, err := inventory.BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels}); err != nil {
			return errors.Annotate(err, "Failed to create device labels %q", deviceLabels.GetName()).Err()
		}

		hc.LogDeviceLabelsChanges(nil, deviceLabels)
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to create device labels in datastore: %s", err)
		return nil, err
	}
	return deviceLabels, nil
}

// UpdateDeviceLabels updates an existing device label in datastore.
func UpdateDeviceLabels(ctx context.Context, deviceLabels *ufspb.DeviceLabels, mask *field_mask.FieldMask) (*ufspb.DeviceLabels, error) {
	f := func(ctx context.Context) error {
		hc := getDeviceLabelsHistoryClient(deviceLabels)

		// Get old/existing device labels
		oldDeviceLabels, err := inventory.GetDeviceLabels(ctx, deviceLabels.GetName())
		if err != nil {
			return errors.Annotate(err, "Failed to get existing device labels by %s", deviceLabels.GetName()).Err()
		}

		// Validate input
		if err := validateUpdateDeviceLabels(ctx, oldDeviceLabels, deviceLabels, mask); err != nil {
			return errors.Annotate(err, "UpdateDeviceLabels - validation failed").Err()
		}

		if _, err := inventory.BatchUpdateDeviceLabels(ctx, []*ufspb.DeviceLabels{deviceLabels}); err != nil {
			return errors.Annotate(err, "Failed to update device label %q", deviceLabels.GetName()).Err()
		}
		hc.LogDeviceLabelsChanges(oldDeviceLabels, deviceLabels)
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to update device labels in datastore: %s", err)
		return nil, err
	}
	return deviceLabels, nil
}

// DeleteDeviceLabels deletes a device label in datastore.
func DeleteDeviceLabels(ctx context.Context, id string) error {
	f := func(ctx context.Context) error {
		hc := getDeviceLabelsHistoryClient(&ufspb.DeviceLabels{Name: id})

		// Get device labels
		deviceLabels, err := inventory.GetDeviceLabels(ctx, id)
		if err != nil {
			return err
		}

		// Validate the input
		if err := validateDeleteDeviceLabels(ctx, deviceLabels); err != nil {
			return err
		}

		if err := inventory.DeleteDeviceLabels(ctx, id); err != nil {
			return errors.Annotate(err, "Failed to delete device labels %s", id).Err()
		}

		hc.LogDeviceLabelsChanges(deviceLabels, nil)
		return hc.SaveChangeEvents(ctx)
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to delete device labels in datastore: %s", err)
		return err
	}
	return nil
}

// ListDeviceLabels lists the device labels
// TODO(echoyang@): Add realm permission checks for reading device labels
func ListDeviceLabels(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly bool) ([]*ufspb.DeviceLabels, string, error) {
	var filterMap map[string][]interface{}
	var err error
	if filter != "" {
		filterMap, err = getFilterMap(filter, inventory.GetDeviceLabelsIndexedFieldName)
		if err != nil {
			return nil, "", errors.Annotate(err, "Failed to read filter for listing device labels").Err()
		}
	}
	filterMap = resetResourceTypeFilter(filterMap, inventory.GetDeviceLabelsIndexedFieldName)
	return inventory.ListDeviceLabels(ctx, pageSize, pageToken, filterMap, keysOnly)
}

// GetDeviceLabels returns device label for the given id from datastore.
func GetDeviceLabels(ctx context.Context, id string) (*ufspb.DeviceLabels, error) {
	return inventory.GetDeviceLabels(ctx, id)
}

// BatchGetDeviceLabels returns a batch of device labels from datastore.
func BatchGetDeviceLabels(ctx context.Context, ids []string) ([]*ufspb.DeviceLabels, error) {
	return inventory.BatchGetDeviceLabels(ctx, ids)
}

func getDeviceLabelsHistoryClient(m *ufspb.DeviceLabels) *HistoryClient {
	return &HistoryClient{
		stUdt: &stateUpdater{
			ResourceName: util.AddPrefix(util.DeviceLabelsCollection, m.Name),
		},
		netUdt: &networkUpdater{
			Hostname: m.Name,
		},
	}
}

var ResourceTypeToPrefix = map[ufspb.ResourceType][]string{
	ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT: {util.SchedulingUnitCollection},
	ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE: {util.MachineLSECollection},
	ufspb.ResourceType_RESOURCE_TYPE_ATTACHED_DEVICE: {util.MachineLSECollection},
	ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE:  {util.MachineLSECollection, util.VMCollection},
}

// validateCreateDeviceLabels validates if a device labels entity can be created
func validateCreateDeviceLabels(ctx context.Context, deviceLabels *ufspb.DeviceLabels, realm string) error {
	namePrefix := util.GetPrefix(deviceLabels.GetName())
	expectedPrefixes, ok := ResourceTypeToPrefix[deviceLabels.GetResourceType()]
	if !ok {
		return fmt.Errorf("validateCreateDeviceLabels - Resource type %v not found", deviceLabels.GetResourceType())
	}
	if !slices.Contains(expectedPrefixes, namePrefix) {
		return fmt.Errorf("validateCreateDeviceLabels - Name prefix %s does not match resource type %v. Should be one of %v", namePrefix, deviceLabels.GetResourceType(), expectedPrefixes)
	}
	if err := util.CheckPermission(ctx, util.InventoriesCreate, realm); err != nil {
		return err
	}
	if err := resourceAlreadyExists(ctx, []*Resource{GetDeviceLabelsResource(deviceLabels.Name)}, nil); err != nil {
		return err
	}
	return nil
}

// validateUpdateDeviceLabels validates if a device labels entity can be updated
func validateUpdateDeviceLabels(ctx context.Context, oldDeviceLabels *ufspb.DeviceLabels, deviceLabels *ufspb.DeviceLabels, mask *field_mask.FieldMask) error {
	namePrefix := util.GetPrefix(oldDeviceLabels.GetName())
	entityName := util.RemovePrefix(oldDeviceLabels.GetName())
	switch namePrefix {
	case util.MachineLSECollection, util.VMCollection:
		machine, err := getMachineForHost(ctx, entityName)
		if err != nil {
			return err
		}
		if err := util.CheckPermission(ctx, util.InventoriesUpdate, machine.GetRealm()); err != nil {
			return err
		}
	case util.SchedulingUnitCollection:
		// succeeds
	default:
		logging.Warningf(ctx, "Unrecognized device labels prefix for %s", oldDeviceLabels.GetName())
	}

	// check if resources does not exist
	if err := ResourceExist(ctx, []*Resource{GetDeviceLabelsResource(deviceLabels.Name)}, nil); err != nil {
		return err
	}

	return validateDeviceLabelsUpdateMask(deviceLabels, mask)
}

// validateDeviceLabelsUpdateMask validates the update mask for device labels update
func validateDeviceLabelsUpdateMask(deviceLabels *ufspb.DeviceLabels, mask *field_mask.FieldMask) error {
	if mask != nil {
		// validate the give field mask
		for _, path := range mask.Paths {
			switch path {
			case "name":
				return status.Error(codes.InvalidArgument, "validateDeviceLabelsUpdateMask - name cannot be updated, delete and create a new device labels instead")
			case "resource_type":
				return status.Error(codes.InvalidArgument, "validateDeviceLabelsUpdateMask - resource_type cannot be updated, delete and recreate the device labels instead")
			case "labels":
				// valid fields, nothing to validate.
			default:
				return status.Errorf(codes.InvalidArgument, "validateDeviceLabelsUpdateMask - unsupported update mask path %q", path)
			}
		}
	}
	return nil
}

func validateDeleteDeviceLabels(ctx context.Context, deviceLabels *ufspb.DeviceLabels) error {
	namePrefix := util.GetPrefix(deviceLabels.GetName())
	entityName := util.RemovePrefix(deviceLabels.GetName())
	switch namePrefix {
	case util.MachineLSECollection, util.VMCollection:
		machine, err := getMachineForHost(ctx, entityName)
		if err != nil {
			return err
		}
		if err := util.CheckPermission(ctx, util.InventoriesDelete, machine.GetRealm()); err != nil {
			return err
		}
	case util.SchedulingUnitCollection:
		// succeeds
	default:
		logging.Warningf(ctx, "Unrecognized device labels prefix for %s", deviceLabels.GetName())
	}

	return nil
}

func GetMachineLSELabels(ctx context.Context, lse *ufspb.MachineLSE) (*ufspb.DeviceLabels, error) {
	// Get data based on device type
	if lse.GetChromeBrowserMachineLse() != nil {
		return getBrowserHostLabels(lse), nil
	} else if lse.GetChromeosMachineLse() != nil {
		device, err := getChromeOSDeviceDataWithLSEAndMachine(ctx, lse, nil)
		if err != nil {
			return nil, errors.Annotate(err, "failed to get chromeos device data").Err()
		}
		return getChromeOSDeviceLabels(ctx, device), nil
	} else if lse.GetAttachedDeviceLse() != nil {
		device, err := GetAttachedDeviceData(ctx, lse)
		if err != nil {
			return nil, errors.Annotate(err, "failed to get attached device data").Err()
		}
		return getAttachedDeviceLabels(ctx, device), nil

	}
	// Ignore other LSEs (eg. labstation)
	return nil, nil
}

func getChromeOSDeviceLabelsWithLSEAndMachine(ctx context.Context, lse *ufspb.MachineLSE, machine *ufspb.Machine) (*ufspb.DeviceLabels, error) {
	if lse.GetChromeosMachineLse() == nil {
		// Ignore other LSEs (eg. labstation)
		return nil, nil
	}

	deviceData, err := getChromeOSDeviceDataWithLSEAndMachine(ctx, lse, machine)
	if err != nil {
		return nil, errors.Annotate(err, "failed to get chromeos device data").Err()
	}
	return getChromeOSDeviceLabels(ctx, deviceData), nil
}

func GetBrowserVMLabels(vm *ufspb.VM) *ufspb.DeviceLabels {
	name := util.AddPrefix(util.VMCollection, vm.GetName())
	state := dutstate.ConvertFromUFSState(vm.GetResourceState()).String()
	zone := vm.GetZone()
	return getBrowserLabelsResponse(name, state, zone)
}

func getBrowserHostLabels(lse *ufspb.MachineLSE) *ufspb.DeviceLabels {
	name := util.AddPrefix(util.MachineLSECollection, lse.GetName())
	state := dutstate.ConvertFromUFSState(lse.GetResourceState()).String()
	zone := lse.GetZone()
	labels := getBrowserLabelsResponse(name, state, zone)
	labels.Realm = lse.Realm
	return labels
}

func getBrowserLabelsResponse(name, state, zone string) *ufspb.DeviceLabels {
	return &ufspb.DeviceLabels{
		Name:         name,
		ResourceType: ufspb.ResourceType_RESOURCE_TYPE_BROWSER_DEVICE,
		Labels: map[string]*ufspb.DeviceLabelValues{
			"ufs_state": {LabelValues: []string{state}},
			// Duplicate state to dut_state to reuse analytics logic built for ChromeOS lab
			"dut_state": {LabelValues: []string{state}},
			"ufs_zone":  {LabelValues: []string{zone}},
		},
		Realm: util.ToUFSRealm(zone),
	}
}

func GetSchedulingUnitLabels(ctx context.Context, su *ufspb.SchedulingUnit, lses []*ufspb.MachineLSE) (*ufspb.DeviceLabels, error) {
	dims, err := getSchedulingUnitSwarmingDimensions(ctx, su, lses)
	if err != nil {
		return nil, err
	}
	deviceLabels := convertSwarmingDimensionsToDeviceLabels(dims)
	deviceLabels.Name = util.AddPrefix(util.SchedulingUnitCollection, su.GetName())
	deviceLabels.ResourceType = ufspb.ResourceType_RESOURCE_TYPE_SCHEDULING_UNIT

	if len(lses) > 0 {
		deviceLabels.Realm = lses[0].GetRealm()
	}

	return deviceLabels, nil
}

func getSchedulingUnitSwarmingDimensions(ctx context.Context, su *ufspb.SchedulingUnit, lses []*ufspb.MachineLSE) (*swarming.Dimensions, error) {
	var dutsDims []swarming.Dimensions
	var botDimensions swarming.Dimensions
	var lse *ufspb.MachineLSE
	var err error
	for _, hostname := range su.GetMachineLSEs() {
		i := slices.IndexFunc(lses, func(l *ufspb.MachineLSE) bool { return l.GetName() == hostname })
		if i == -1 {
			lse, err = GetMachineLSE(ctx, hostname)
			if err != nil {
				return nil, err
			}
		} else {
			lse = lses[i]
		}

		// Get data based on device type
		if lse.GetChromeosMachineLse() != nil {
			device, err := getChromeOSDeviceDataWithLSEAndMachine(ctx, lse, nil)
			if err != nil {
				return nil, errors.Annotate(err, "getSchedulingUnitSwarmingDimensions: failed to get chromeos device data").Err()
			}
			botDimensions = *getChromeOSBotSwarmingDimensions(ctx, device)
		} else if lse.GetAttachedDeviceLse() != nil {
			device, err := GetAttachedDeviceData(ctx, lse)
			if err != nil {
				return nil, errors.Annotate(err, "getSchedulingUnitSwarmingDimensions: failed to get attached device data").Err()
			}
			botDimensions = *getAttachedDeviceSwarmingDimensions(ctx, device)
		} else {
			return nil, fmt.Errorf("getSchedulingUnitSwarmingDimensions: SchedulingUnit %s MachineLSE %s not recognized", su.GetName(), hostname)
		}
		dutsDims = append(dutsDims, botDimensions)
	}
	dims := swarming.Dimensions(schedulingunit.GetSchedulingUnitDimensions(su, dutsDims))
	return &dims, nil
}

func getChromeOSDeviceLabels(ctx context.Context, deviceData *ufspb.ChromeOSDeviceData) *ufspb.DeviceLabels {
	dims := getChromeOSBotSwarmingDimensions(ctx, deviceData)
	deviceLabels := convertSwarmingDimensionsToDeviceLabels(dims)
	deviceLabels.Name = util.AddPrefix(util.MachineLSECollection, deviceData.GetLabConfig().GetName())
	deviceLabels.ResourceType = ufspb.ResourceType_RESOURCE_TYPE_CHROMEOS_DEVICE
	deviceLabels.Realm = deviceData.GetMachine().GetRealm()
	return deviceLabels
}

func getChromeOSBotSwarmingDimensions(ctx context.Context, deviceData *ufspb.ChromeOSDeviceData) *swarming.Dimensions {
	r := func(e error) {
		logging.Warningf(ctx, "Problem getting device data for DUT %s: %s", deviceData.GetLabConfig().GetName(), e.Error())
	}
	var machine string
	if len(deviceData.GetLabConfig().GetMachines()) > 0 {
		machine = deviceData.GetLabConfig().GetMachines()[0]
	}
	dutState := dutstate.Info{
		State:    dutstate.ConvertFromUFSState(deviceData.GetLabConfig().GetResourceState()),
		Time:     deviceData.GetLabConfig().GetUpdateTime().GetSeconds(),
		DeviceId: machine,
	}
	dims := dut.GetDUTBotDims(ctx, r, dutState, deviceData)
	return &dims
}

func getAttachedDeviceLabels(ctx context.Context, deviceData *ufsAPI.AttachedDeviceData) *ufspb.DeviceLabels {
	dims := getAttachedDeviceSwarmingDimensions(ctx, deviceData)
	deviceLabels := convertSwarmingDimensionsToDeviceLabels(dims)
	deviceLabels.Name = util.AddPrefix(util.MachineLSECollection, deviceData.GetLabConfig().GetName())
	deviceLabels.ResourceType = ufspb.ResourceType_RESOURCE_TYPE_ATTACHED_DEVICE
	deviceLabels.Realm = deviceData.GetMachine().GetRealm()
	return deviceLabels
}

func getAttachedDeviceSwarmingDimensions(ctx context.Context, deviceData *ufsAPI.AttachedDeviceData) *swarming.Dimensions {
	r := func(e error) {
		logging.Warningf(ctx, "Problem getting device data for AttachedDevice %s: %s", deviceData.GetLabConfig().GetName(), e.Error())
	}
	var machine string
	if len(deviceData.GetLabConfig().GetMachines()) > 0 {
		machine = deviceData.GetLabConfig().GetMachines()[0]
	}
	dutState := dutstate.Info{
		State:    dutstate.ConvertFromUFSState(deviceData.GetLabConfig().GetResourceState()),
		Time:     deviceData.GetLabConfig().GetUpdateTime().GetSeconds(),
		DeviceId: machine,
	}
	dims := attacheddevice.GetAttachedDeviceBotDims(ctx, r, dutState, deviceData)
	return &dims
}

func convertSwarmingDimensionsToDeviceLabels(dims *swarming.Dimensions) *ufspb.DeviceLabels {
	deviceLabels := &ufspb.DeviceLabels{
		Name:         "",
		ResourceType: ufspb.ResourceType_RESOURCE_TYPE_UNSPECIFIED,
		Labels:       make(map[string]*ufspb.DeviceLabelValues),
	}
	for k, v := range *dims {
		deviceLabels.Labels[k] = &ufspb.DeviceLabelValues{LabelValues: v}
	}
	return deviceLabels

}
