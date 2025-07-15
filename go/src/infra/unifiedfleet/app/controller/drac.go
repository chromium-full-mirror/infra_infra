// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/libs/skylab/common/heuristics"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// CreateDrac creates a new drac in datastore.
//
// Checks if the resources referenced by the Drac input already exists
// in the system before creating a new Drac
func CreateDrac(ctx context.Context, drac *ufspb.Drac) (*ufspb.Drac, error) {
	f := func(ctx context.Context) error {
		hc := getDracHistoryClient(drac)
		hc.LogDracChanges(nil, drac)

		// Get browser machine to associate the drac
		machine, err := getBrowserMachine(ctx, drac.GetMachine())
		if err != nil {
			return errors.Annotate(err, "CreateDrac - failed to get machine %s", drac.GetMachine()).Err()
		}

		// Validate input
		if err := validateCreateDrac(ctx, drac, machine); err != nil {
			return errors.Annotate(err, "CreateDrac - validation failed").Err()
		}

		// Fill the rack/zone to drac OUTPUT only fields for drac table indexing
		drac.Rack = machine.GetLocation().GetRack()
		drac.Zone = machine.GetLocation().GetZone().String()
		drac.ResourceState = ufspb.State_STATE_SERVING

		// Create a drac entry
		// we use this func as it is a non-atomic operation and can be used to
		// run within a transaction to make it atomic. Datastore doesnt allow
		// nested transactions.
		if _, err = registration.BatchUpdateDracs(ctx, []*ufspb.Drac{drac}); err != nil {
			return errors.Annotate(err, "CreateDrac - unable to batch update drac %s", drac.Name).Err()
		}

		// Update state
		if err := hc.stUdt.updateStateHelper(ctx, ufspb.State_STATE_SERVING); err != nil {
			return err
		}
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "CreateDrac - unable to create drac %s", drac.Name).Err()
	}
	return drac, nil
}

// UpdateDrac updates drac in datastore.
//
// Checks if the resources referenced by the Drac input already exists
// in the system before updating a Drac
func UpdateDrac(ctx context.Context, drac *ufspb.Drac, mask *field_mask.FieldMask) (*ufspb.Drac, error) {
	for i, f := range mask.GetPaths() {
		if !heuristics.LooksLikeFieldMask(f) {
			return nil, errors.Reason("UpdateDrac - field #%d %q is not valid", i, f).Err()
		}
	}
	f := func(ctx context.Context) error {
		hc := getDracHistoryClient(drac)

		// Get old/existing drac
		oldDrac, err := registration.GetDrac(ctx, drac.GetName())
		if err != nil {
			return errors.Annotate(err, "UpdateDrac - get drac %s failed", drac.GetName()).Err()
		}

		// Validate the input
		if err := validateUpdateDrac(ctx, oldDrac, drac, mask); err != nil {
			return errors.Annotate(err, "UpdateDrac - validation failed").Err()
		}

		oldDracCopy := proto.Clone(oldDrac).(*ufspb.Drac)
		// Copy the rack/zone to drac OUTPUT only fields from already existing drac
		drac.Rack = oldDrac.GetRack()
		drac.Zone = oldDrac.GetZone()

		// Partial update by field mask
		if mask != nil && len(mask.Paths) > 0 {
			drac, err = processDracUpdateMask(ctx, oldDrac, drac, mask)
			if err != nil {
				return errors.Annotate(err, "UpdateDrac - processing update mask failed").Err()
			}
		} else {
			// This is for complete object input
			if drac.GetMachine() == "" {
				return status.Error(codes.InvalidArgument, "Machine cannot be empty for updating a drac")
			}
			// Check if user provided new machine to associate the drac
			if drac.GetMachine() != oldDrac.GetMachine() {
				// A machine can have only one drac. If there is a old drac associated with this machine already, error out.
				// A drac cannot exist in the system without being associated to a machine.
				dracs, err := registration.QueryDracByPropertyName(ctx, "machine", drac.GetMachine(), true)
				if err != nil {
					return errors.Annotate(err, "UpdateDrac - failed to query old drac for machine %s", drac.GetMachine()).Err()
				}
				if len(dracs) > 0 {
					return status.Error(codes.InvalidArgument, fmt.Sprintf("validateUpdateDrac - There is already a drac %s associated with machine %s.\n"+
						"Please delete that drac and then associate this drac to the machine.", dracs[0].GetName(), drac.GetMachine()))
				}

				// Get new browser machine to associate the drac
				machine, err := getBrowserMachine(ctx, drac.GetMachine())
				if err != nil {
					return errors.Annotate(err, "UpdateDrac - get browser machine %s failed", drac.GetMachine()).Err()
				}

				// check permission for the new machine realm
				if err := util.CheckPermission(ctx, util.RegistrationsUpdate, machine.GetRealm()); err != nil {
					return err
				}
				// Fill the rack/zone to drac OUTPUT only fields
				drac.Rack = machine.GetLocation().GetRack()
				drac.Zone = machine.GetLocation().GetZone().String()
			}
		}

		// Update state
		if err := hc.stUdt.updateStateHelper(ctx, drac.GetResourceState()); err != nil {
			return errors.Annotate(err, "UpdateDrac - Failed to update state of drac %s", drac.GetName()).Err()
		}

		// Update drac entry
		// we use this func as it is a non-atomic operation and can be used to
		// run within a transaction. Datastore doesnt allow nested transactions.
		if _, err := registration.BatchUpdateDracs(ctx, []*ufspb.Drac{drac}); err != nil {
			return errors.Annotate(err, "UpdateDrac - unable to batch update drac %s", drac.Name).Err()
		}
		hc.LogDracChanges(oldDracCopy, drac)
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "UpdateDrac - failed to update drac %s in datastore", drac.Name).Err()
	}
	return drac, nil
}

// processDracUpdateMask process update field mask to get only specific update
// fields and return a complete drac object with updated and existing fields
func processDracUpdateMask(ctx context.Context, oldDrac *ufspb.Drac, drac *ufspb.Drac, mask *field_mask.FieldMask) (*ufspb.Drac, error) {
	// update the fields in the existing drac
	for _, path := range mask.Paths {
		switch path {
		case util.MachinePath:
			// Check if user provided new machine to associate the drac
			if drac.GetMachine() != oldDrac.GetMachine() {
				// A machine can have only one drac. If there is a old drac associated with this machine already, error out.
				// A drac cannot exist in the system without being associated to a machine.
				dracs, err := registration.QueryDracByPropertyName(ctx, "machine", drac.GetMachine(), true)
				if err != nil {
					return oldDrac, errors.Annotate(err, "UpdateDrac - failed to query old drac for machine %s", drac.GetMachine()).Err()
				}
				if len(dracs) > 0 {
					return oldDrac, status.Error(codes.InvalidArgument, fmt.Sprintf("validateUpdateDrac - There is already a drac %s associated with machine %s.\n"+
						"Please delete that drac and then associate this drac to the machine.", dracs[0].GetName(), drac.GetMachine()))
				}

				// Get new browser machine to associate the drac
				machine, err := getBrowserMachine(ctx, drac.GetMachine())
				if err != nil {
					return oldDrac, errors.Annotate(err, "UpdateDrac - get browser machine %s failed", drac.GetMachine()).Err()
				}
				// check permission for the new machine realm
				if err := util.CheckPermission(ctx, util.RegistrationsUpdate, machine.GetRealm()); err != nil {
					return oldDrac, err
				}
				oldDrac.Machine = drac.GetMachine()
				// Fill the rack/zone to drac OUTPUT only fields
				oldDrac.Rack = machine.GetLocation().GetRack()
				oldDrac.Zone = machine.GetLocation().GetZone().String()
			}
		case util.DisplayNamePath, util.DisplayNameCamelPath:
			oldDrac.DisplayName = drac.GetDisplayName()
		case util.MacAddressPath, util.MacAddressCamelPath:
			oldDrac.MacAddress = drac.GetMacAddress()
		case util.PasswordPath:
			oldDrac.Password = drac.GetPassword()
		case util.SwitchInterfaceSwitchPath, util.SwitchPath:
			if oldDrac.GetSwitchInterface() == nil {
				oldDrac.SwitchInterface = &ufspb.SwitchInterface{
					Switch: drac.GetSwitchInterface().GetSwitch(),
				}
			} else {
				oldDrac.GetSwitchInterface().Switch = drac.GetSwitchInterface().GetSwitch()
			}
		case util.SwitchInterfacePortNamePath, util.PortNameCamelPath:
			if oldDrac.GetSwitchInterface() == nil {
				oldDrac.SwitchInterface = &ufspb.SwitchInterface{
					PortName: drac.GetSwitchInterface().GetPortName(),
				}
			} else {
				oldDrac.GetSwitchInterface().PortName = drac.GetSwitchInterface().GetPortName()
			}
		case util.TagsPath:
			oldDrac.Tags = mergeTags(oldDrac.GetTags(), drac.GetTags())
		case util.ResourceStatePath, util.ResourceStateCamelPath:
			oldDrac.ResourceState = drac.GetResourceState()
		}
	}
	// For partial update, validate switch interface just before updating in case
	// before we checks the incompleted interface.
	if err := validateDracSwitchPort(ctx, oldDrac.GetName(), oldDrac.GetMachine(), oldDrac.GetSwitchInterface()); err != nil {
		return oldDrac, err
	}
	// return existing/old drac with new updated values
	return oldDrac, nil
}

// DeleteDracHost deletes the host of a drac in datastore.
func DeleteDracHost(ctx context.Context, dracName string) error {
	f := func(ctx context.Context) error {
		// Get drac
		drac, err := registration.GetDrac(ctx, dracName)
		if err != nil {
			return errors.Annotate(err, "DeleteDracHost - Unable to get drac").Err()
		}
		// Validate the input
		if err := validateDeleteDrac(ctx, drac); err != nil {
			return errors.Annotate(err, "DeleteDracHost - validation failed").Err()
		}
		hc := &HistoryClient{
			netUdt: &networkUpdater{
				Hostname: dracName,
			},
		}
		if err := hc.netUdt.deleteDHCPHelper(ctx); err != nil {
			return err
		}
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to delete the drac host: %s", err)
		return err
	}
	return nil
}

// UpdateDracHost updates the drac host in datastore.
func UpdateDracHost(ctx context.Context, drac *ufspb.Drac, nwOpt *ufsAPI.NetworkOption) error {
	f := func(ctx context.Context) error {
		hc := getDracHistoryClient(drac)

		// 1. Validate the input
		if err := validateUpdateDracHost(ctx, drac, nwOpt.GetVlan(), nwOpt.GetIp()); err != nil {
			return err
		}

		// 2. Verify if the hostname is already set with IP. if yes, remove the current dhcp.
		if err := hc.netUdt.deleteDHCPHelper(ctx); err != nil {
			return err
		}

		// 3. Find free ip, set IP and DHCP config
		if _, err := hc.netUdt.addHostHelper(ctx, nwOpt.GetVlan(), nwOpt.GetIp(), drac.GetMacAddress()); err != nil {
			return err
		}

		// 4. Update drac entry to update the update_time
		if _, err := registration.BatchUpdateDracs(ctx, []*ufspb.Drac{drac}); err != nil {
			return errors.Annotate(err, "UpdateDracHost - unable to batch update drac %s", drac.Name).Err()
		}
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to assign IP to the drac: %s", err)
		return err
	}
	return nil
}

// GetDrac returns drac for the given id from datastore.
func GetDrac(ctx context.Context, id string) (*ufspb.Drac, error) {
	return registration.GetDrac(ctx, id)
}

// BatchGetDracs returns a batch of dracs from datastore.
func BatchGetDracs(ctx context.Context, ids []string) ([]*ufspb.Drac, error) {
	return registration.BatchGetDracs(ctx, ids)
}

// ListDracs lists the dracs
func ListDracs(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly bool) ([]*ufspb.Drac, string, error) {
	var filterMap map[string][]any
	var err error
	if filter != "" {
		filterMap, err = getFilterMap(filter, registration.GetDracIndexedFieldName)
		if err != nil {
			return nil, "", errors.Annotate(err, "Failed to read filter for listing dracs").Err()
		}
	}
	filterMap = resetZoneFilter(filterMap, registration.GetDracIndexedFieldName)
	filterMap = resetStateFilter(filterMap, registration.GetDracIndexedFieldName)
	return registration.ListDracs(ctx, pageSize, pageToken, filterMap, keysOnly)
}

// DeleteDrac deletes the drac in datastore
func DeleteDrac(ctx context.Context, id string) error {
	return deleteDracHelper(ctx, id, true)
}

func deleteDracHelper(ctx context.Context, id string, inTransaction bool) error {
	f := func(ctx context.Context) error {
		hc := getDracHistoryClient(&ufspb.Drac{Name: id})

		// Get drac
		drac, err := registration.GetDrac(ctx, id)
		if err != nil {
			return errors.Annotate(err, "DeleteDrac - Unable to get drac").Err()
		}

		// Validate the input
		if err := validateDeleteDrac(ctx, drac); err != nil {
			return errors.Annotate(err, "DeleteDrac - validation failed").Err()
		}

		// Delete the drac
		if err := registration.DeleteDrac(ctx, id); err != nil {
			return errors.Annotate(err, "DeleteDrac - unable to delete drac %s", id).Err()
		}

		// Delete its ip configs
		if err := hc.netUdt.deleteDHCPHelper(ctx); err != nil {
			return errors.Annotate(err, "DeleteDrac - unable to delete ip configs for drac %s", id).Err()
		}

		// Update state
		hc.stUdt.deleteStateHelper(ctx)

		hc.LogDracChanges(drac, nil)
		return hc.SaveChangeEvents(ctx)
	}
	if inTransaction {
		if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
			return errors.Annotate(err, "DeleteDrac - failed to delete drac in datastore: %s", id).Err()
		}
		return nil
	}
	return f(ctx)
}

// ReplaceDrac replaces an old Drac with new Drac in datastore
//
// It does a delete of old drac and create of new Drac.
// All the steps are in done in a transaction to preserve consistency on failure.
// Before deleting the old Drac, it will get all the resources referencing
// the old Drac. It will update all the resources which were referencing
// the old Drac(got in the last step) with new Drac.
// Deletes the old Drac.
// Creates the new Drac.
// This will preserve data integrity in the system.
func ReplaceDrac(ctx context.Context, oldDrac *ufspb.Drac, newDrac *ufspb.Drac) (*ufspb.Drac, error) {
	// TODO(eshwarn) : implement replace after user testing the tool
	return nil, nil
}

// validateCreateDrac validates if a drac can be created
//
// check if the drac already exists
// checks if the machine and resources referenced by the drac does not exist
func validateCreateDrac(ctx context.Context, drac *ufspb.Drac, machine *ufspb.Machine) error {
	// Check permission
	if err := util.CheckPermission(ctx, util.RegistrationsCreate, machine.GetRealm()); err != nil {
		return err
	}
	// Check if Drac already exists
	if err := resourceAlreadyExists(ctx, []*Resource{GetDracResource(drac.Name)}, nil); err != nil {
		return err
	}

	// A machine can have only one drac. If there is a old drac associated with this machine already, error out.
	// A drac cannot exist in the system without being associated to a machine.
	dracs, err := registration.QueryDracByPropertyName(ctx, "machine", drac.GetMachine(), true)
	if err != nil {
		return errors.Annotate(err, "validateCreateDrac - failed to query old drac for machine %s", drac.GetMachine()).Err()
	}
	if len(dracs) > 0 {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("validateCreateDrac - There is already a drac %s associated with machine %s.\n"+
			"Please delete that drac and then add the new drac to the machine.", dracs[0].GetName(), drac.GetMachine()))
	}

	if err := validateMacAddress(ctx, drac.GetName(), drac.GetMacAddress()); err != nil {
		return err
	}
	if err := validateDracSwitchPort(ctx, drac.GetName(), drac.GetMachine(), drac.GetSwitchInterface()); err != nil {
		return err
	}

	// Aggregate resource to check if resources referenced by the drac does not exist
	if switchID := drac.GetSwitchInterface().GetSwitch(); switchID != "" {
		if err := ResourceExist(ctx, []*Resource{GetSwitchResource(switchID)}, nil); err != nil {
			return err
		}
	}
	return nil
}

// validateUpdateDrac validates if a drac can be updated
//
// checks if drac, machine and resources referecned by the drac does not exist
func validateUpdateDrac(ctx context.Context, oldDrac *ufspb.Drac, drac *ufspb.Drac, mask *field_mask.FieldMask) error {
	machine, err := registration.GetMachine(ctx, oldDrac.GetMachine())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "machine %s not found", oldDrac.GetMachine())
	}
	// Check permission
	if err := util.CheckPermission(ctx, util.RegistrationsUpdate, machine.GetRealm()); err != nil {
		return err
	}
	// Aggregate resource to check if does not exist
	var resourcesNotFound []*Resource
	// Aggregate resource to check if machine does not exist
	if drac.GetMachine() != "" {
		resourcesNotFound = append(resourcesNotFound, GetMachineResource(drac.GetMachine()))
	}
	// Aggregate resource to check if resources referenced by the drac does not exist
	if switchID := drac.GetSwitchInterface().GetSwitch(); switchID != "" {
		resourcesNotFound = append(resourcesNotFound, GetSwitchResource(switchID))
	}

	// check if resources does not exist
	if err := ResourceExist(ctx, resourcesNotFound, nil); err != nil {
		return err
	}

	// Check partial update first to avoid unnecessary validations
	if err := validateDracUpdateMask(ctx, drac, mask); err != nil {
		return err
	}
	if err := validateDracSwitchPort(ctx, drac.GetName(), drac.GetMachine(), drac.GetSwitchInterface()); err != nil {
		return err
	}
	if err := validateMacAddress(ctx, drac.GetName(), drac.GetMacAddress()); err != nil {
		return err
	}
	return nil
}

// validateUpdateDracHost validates if a host can be added to a drac
func validateUpdateDracHost(ctx context.Context, drac *ufspb.Drac, vlanName, ipv4Str string) error {
	// during partial update, drac object may not have machine info, so we get the old drac to get the machine
	// to check the permission
	oldDrac, err := registration.GetDrac(ctx, drac.GetName())
	if err != nil {
		return err
	}
	machine, err := registration.GetMachine(ctx, oldDrac.GetMachine())
	if err != nil {
		return errors.Annotate(err, "unable to get machine %s", oldDrac.GetMachine()).Err()
	}
	// Check permission
	if err := util.CheckPermission(ctx, util.RegistrationsUpdate, machine.GetRealm()); err != nil {
		return err
	}
	if drac.GetMacAddress() == "" {
		return errors.New("mac address of drac hasn't been specified")
	}
	if ipv4Str != "" {
		return nil
	}
	// Check if resources does not exist
	return ResourceExist(ctx, []*Resource{GetVlanResource(vlanName)}, nil)
}

// validateDracUpdateMask validates the update mask for drac update
func validateDracUpdateMask(ctx context.Context, drac *ufspb.Drac, mask *field_mask.FieldMask) error {
	if mask != nil {
		// validate the give field mask
		for _, path := range mask.Paths {
			switch path {
			case util.NamePath:
				return status.Error(codes.InvalidArgument, "validateDracUpdateMask - name cannot be updated, delete and create a new drac instead")
			case util.DisplayNamePath, util.DisplayNameCamelPath:
				if drac.GetDisplayName() == "" {
					return status.Error(codes.InvalidArgument, "validateDracUpdateMask - display name cannot be empty")
				}
			case util.SwitchInterfaceSwitchPath, util.SwitchPath:
				fallthrough
			case util.SwitchInterfacePortNamePath, util.PortNameCamelPath:
				// Check switch interface validity in processDracUpdateMask later.
				if drac.GetSwitchInterface() == nil {
					return status.Error(codes.InvalidArgument, "validateDracUpdateMask - switch interface cannot be empty/nil.")
				}
			case util.MachinePath:
				if drac.GetMachine() == "" {
					return status.Error(codes.InvalidArgument, "validateDracUpdateMask - machine cannot be empty")
				}
			case util.MacAddressPath, util.MacAddressCamelPath:
				if err := validateMacAddress(ctx, drac.GetName(), drac.GetMacAddress()); err != nil {
					return err
				}
			case util.PasswordPath:
			case util.TagsPath:
			case util.ResourceStatePath, util.ResourceStateCamelPath:
				// valid fields, nothing to validate.
			default:
				return status.Errorf(codes.InvalidArgument, "validateDracUpdateMask - unsupported update mask path %q", path)
			}
		}
	}
	return nil
}

func getDracHistoryClient(m *ufspb.Drac) *HistoryClient {
	return &HistoryClient{
		netUdt: &networkUpdater{
			Hostname: m.Name,
		},
		stUdt: &stateUpdater{
			ResourceName: util.AddPrefix(util.DracCollection, m.Name),
		},
	}
}

// validateDeleteDrac validates if a drac can be deleted
func validateDeleteDrac(ctx context.Context, drac *ufspb.Drac) error {
	machine, err := registration.GetMachine(ctx, drac.GetMachine())
	if err != nil {
		return errors.Annotate(err, "validateDeleteDrac - unable to get machine %s", drac.GetMachine()).Err()
	}
	// Check permission
	if err := util.CheckPermission(ctx, util.RegistrationsDelete, machine.GetRealm()); err != nil {
		return err
	}
	return nil
}

// updateIndexInDrac updates indexes in drac objects
//
// This function can be used inside a transaction.
func updateIndexInDrac(ctx context.Context, property, oldValue, newValue string, hc *HistoryClient) error {
	var dracs []*ufspb.Drac
	var err error
	switch property {
	case "machine":
		// get Dracs for indexing
		dracs, err = registration.QueryDracByPropertyName(ctx, "machine", oldValue, false)
		if err != nil {
			return errors.Annotate(err, "failed to query dracs for machine %s", newValue).Err()
		}
		for _, drac := range dracs {
			// Copy for logging
			oldDracCopy := proto.Clone(drac).(*ufspb.Drac)
			drac.Machine = newValue
			hc.LogDracChanges(oldDracCopy, drac)
		}
	case "switch":
		dracs, err = registration.QueryDracByPropertyName(ctx, "switch_id", oldValue, false)
		if err != nil {
			return errors.Annotate(err, "failed to query dracs for switch %s", newValue).Err()
		}
		for _, drac := range dracs {
			oldDracCopy := proto.Clone(drac).(*ufspb.Drac)
			drac.GetSwitchInterface().Switch = newValue
			hc.LogDracChanges(oldDracCopy, drac)
		}
	case "rack":
		dracs, err := registration.QueryDracByPropertyName(ctx, "rack", oldValue, false)
		if err != nil {
			return errors.Annotate(err, "failed to query dracs in rack %s", oldValue).Err()
		}
		for _, r := range dracs {
			oldCopy := proto.Clone(r).(*ufspb.Drac)
			r.Rack = newValue
			hc.LogDracChanges(oldCopy, r)
		}
	}
	if _, err := registration.BatchUpdateDracs(ctx, dracs); err != nil {
		return errors.Annotate(err, "updateIndexing - unable to batch update dracs").Err()
	}
	return nil
}
