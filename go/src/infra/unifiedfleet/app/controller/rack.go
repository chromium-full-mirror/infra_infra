// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// RackRegistration creates a new rack, switches, kvms and rpms in datastore.
func RackRegistration(ctx context.Context, rack *ufspb.Rack) (*ufspb.Rack, error) {
	// Older clients of Shivas send OldBrowserLabAdminRealm, which needs to be repalced with BrowserLabAdminRealm
	rack.Realm = ufsUtil.GetValidRealmName(rack.GetRealm())

	switches := rack.GetChromeBrowserRack().GetSwitchObjects()
	kvms := rack.GetChromeBrowserRack().GetKvmObjects()
	rpms := rack.GetChromeBrowserRack().GetRpmObjects()
	f := func(ctx context.Context) error {
		hc := getRackClientHistory(rack)
		// Validate the input
		if err := validateRackRegistration(ctx, rack); err != nil {
			return err
		}
		// Format the input
		rack.Name = strings.ToLower(rack.Name)

		// Create switches
		if switches != nil {
			for _, sw := range switches {
				// Fill the rack/zone to switch OUTPUT only fields for indexing
				sw.Rack = rack.GetName()
				sw.Zone = rack.GetLocation().GetZone().String()
				if sw.GetResourceState() == ufspb.State_STATE_UNSPECIFIED {
					sw.ResourceState = ufspb.State_STATE_SERVING
				}
			}
			if _, err := registration.BatchUpdateSwitches(ctx, switches); err != nil {
				return errors.Annotate(err, "Failed to create switches").Err()
			}
			// We do not save switch objects in rack table
			rack.GetChromeBrowserRack().SwitchObjects = nil
		}

		// Create kvms
		if kvms != nil {
			for _, kvm := range kvms {
				// Fill the rack/zone to kvm OUTPUT only fields for indexing
				kvm.Rack = rack.GetName()
				kvm.Zone = rack.GetLocation().GetZone().String()
				if kvm.GetResourceState() == ufspb.State_STATE_UNSPECIFIED {
					kvm.ResourceState = ufspb.State_STATE_REGISTERED
				}
			}
			if _, err := registration.BatchUpdateKVMs(ctx, kvms); err != nil {
				return errors.Annotate(err, "Failed to create KVMs").Err()
			}
			// We do not save kvm objects in rack table
			rack.GetChromeBrowserRack().KvmObjects = nil
		}

		// Create rpms
		if rpms != nil {
			for _, rpm := range rpms {
				// Fill the rack/zone to rpm OUTPUT only fields for indexing
				rpm.Rack = rack.GetName()
				rpm.Zone = rack.GetLocation().GetZone().String()
				if rpm.GetResourceState() == ufspb.State_STATE_UNSPECIFIED {
					rpm.ResourceState = ufspb.State_STATE_REGISTERED
				}
			}
			if _, err := registration.BatchUpdateRPMs(ctx, rpms); err != nil {
				return errors.Annotate(err, "Failed to create RPMs").Err()
			}
			// we do not save rpm objects in rack table
			rack.GetChromeBrowserRack().RpmObjects = nil
		}

		// Create rack
		// we use this func as it is a non-atomic operation and can be used to
		// run within a transaction to make it atomic. Datastore doesnt allow
		// nested transactions.
		rack.ResourceState = ufspb.State_STATE_REGISTERED
		if _, err := registration.BatchUpdateRacks(ctx, []*ufspb.Rack{rack}); err != nil {
			return errors.Annotate(err, "Failed to create rack %s", rack.Name).Err()
		}
		if rack.GetChromeBrowserRack() != nil {
			rack.GetChromeBrowserRack().SwitchObjects = switches
			rack.GetChromeBrowserRack().KvmObjects = kvms
			rack.GetChromeBrowserRack().RpmObjects = rpms
		}

		hc.LogAddRackChanges(rack, switches, kvms, rpms)
		hc.stUdt.addRackStateHelper(ctx, rack)
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to register rack: %s", err)
		return nil, err
	}
	return rack, nil
}

// UpdateRack updates rack in datastore.
func UpdateRack(ctx context.Context, rack *ufspb.Rack, mask *field_mask.FieldMask) (*ufspb.Rack, error) {
	// Older clients of Shivas send OldBrowserLabAdminRealm, which needs to be replaced by BrowserLabAdminRealm
	rack.Realm = ufsUtil.GetValidRealmName(rack.GetRealm())

	var oldRack *ufspb.Rack
	var err error
	f := func(ctx context.Context) error {
		hc := getRackClientHistory(rack)

		// Get the existing/old rack
		oldRack, err = registration.GetRack(ctx, rack.GetName())
		if err != nil {
			return errors.Annotate(err, "UpdateRack - get rack %s failed", rack.GetName()).Err()
		}

		// Validate input
		if err := validateUpdateRack(ctx, oldRack, rack, mask); err != nil {
			return errors.Annotate(err, "UpdateRack - validation failed").Err()
		}

		// Copy for logging
		oldRackCopy := proto.Clone(oldRack).(*ufspb.Rack)

		// Make sure OUTPUT_ONLY fields are set to empty
		// for OS rack we dont do anything as of now.
		if rack.GetChromeBrowserRack() != nil {
			// switches/kvms/rpms is not allowed to be updated in UpdateRack call.
			// We dont store switches/kvms/rpms object inside Rack object in Rack table.
			// switches/kvms/rpms objects are stored in their separate tables
			// user has to use switch/kvm/rpm CRUD apis to update switch/kvm/rpm
			rack.GetChromeBrowserRack().RpmObjects = nil
			rack.GetChromeBrowserRack().KvmObjects = nil
			rack.GetChromeBrowserRack().SwitchObjects = nil
		}

		// Do not let updating from browser to os or vice versa change for rack.
		if oldRack.GetChromeBrowserRack() != nil && rack.GetChromeosRack() != nil {
			return status.Error(codes.InvalidArgument, "UpdateRack - cannot update a browser rack to os rack. Please delete the browser rack and create a new os rack")
		}
		if oldRack.GetChromeosRack() != nil && rack.GetChromeBrowserRack() != nil {
			return status.Error(codes.InvalidArgument, "UpdateRack - cannot update an os rack to browser rack. Please delete the os rack and create a new browser rack")
		}

		// Partial update by field mask
		if mask != nil && len(mask.Paths) > 0 {
			rack, err = processRackUpdateMask(ctx, oldRack, rack, mask, hc)
			if err != nil {
				return errors.Annotate(err, "UpdateRack - processing update mask failed").Err()
			}
		} else if rack.GetLocation().GetZone() != oldRack.GetLocation().GetZone() {
			// this check is for json input with complete update rack
			// Check if rack zone information is changed/updated
			indexMap := map[string]string{"zone": rack.GetLocation().GetZone().String()}
			oldIndexMap := map[string]string{"zone": oldRack.GetLocation().GetZone().String()}
			if err = updateIndexingForRackResources(ctx, rack.GetName(), indexMap, oldIndexMap, hc); err != nil {
				return errors.Annotate(err, "UpdateRack - update zone indexing failed").Err()
			}
		}

		// Update state
		if err := hc.stUdt.updateStateHelper(ctx, rack.GetResourceState()); err != nil {
			return errors.Annotate(err, "Fail to update state of machine %s", rack.GetName()).Err()
		}

		// Update the rack
		if _, err := registration.BatchUpdateRacks(ctx, []*ufspb.Rack{rack}); err != nil {
			return errors.Annotate(err, "UpdateRack - unable to batch update rack %s", rack.Name).Err()
		}
		hc.LogRackChanges(oldRackCopy, rack)
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "UpdateRack - failed to update rack %s in datastore", rack.Name).Err()
	}
	// We fill the rack object with its switches/kvms/rpms from switch/kvm/rpm table
	setRack(ctx, rack)
	return rack, nil
}

// processRackUpdateMask process update field mask to get only specific update
// fields and return a complete rack object with updated and existing fields
func processRackUpdateMask(ctx context.Context, oldRack *ufspb.Rack, rack *ufspb.Rack, mask *field_mask.FieldMask, hc *HistoryClient) (*ufspb.Rack, error) {
	// update the fields in the existing rack
	for _, path := range mask.Paths {
		switch path {
		case ufsUtil.LocationZonePath, ufsUtil.ZonePath:
			indexMap := map[string]string{"zone": rack.GetLocation().GetZone().String()}
			oldIndexMap := map[string]string{"zone": oldRack.GetLocation().GetZone().String()}
			if err := updateIndexingForRackResources(ctx, rack.GetName(), indexMap, oldIndexMap, hc); err != nil {
				return nil, errors.Annotate(err, "processRackUpdateMask - failed to update zone indexing").Err()
			}
			if oldRack.GetLocation() == nil {
				oldRack.Location = &ufspb.Location{}
			}
			oldRack.GetLocation().Zone = rack.GetLocation().GetZone()
			oldRack.Realm = rack.GetRealm()
		case ufsUtil.CapacityRuPath, ufsUtil.CapacityPath:
			oldRack.CapacityRu = rack.GetCapacityRu()
		case ufsUtil.BbnumPath:
			oldRack.Bbnum = rack.GetBbnum()
		case ufsUtil.TagsPath:
			oldRack.Tags = mergeTags(oldRack.GetTags(), rack.GetTags())
		case ufsUtil.ResourceStatePath, ufsUtil.ResourceStateCamelPath:
			oldRack.ResourceState = rack.GetResourceState()
		}
	}
	// return existing/old rack with new updated values
	return oldRack, nil
}

// updateIndexingForRackResources updates indexing for kvm/rpm/switch tables
// can be used inside a transaction
func updateIndexingForRackResources(ctx context.Context, rackName string, indexMap, oldIndexMap map[string]string, hc *HistoryClient) error {
	// get KVMs for indexing
	kvms, err := registration.QueryKVMByPropertyName(ctx, "rack", rackName, false)
	if err != nil {
		return errors.Annotate(err, "updateIndexingForRackResources - Failed to query kvms for rack %s", rackName).Err()
	}
	// get RPMs for indexing
	rpms, err := registration.QueryRPMByPropertyName(ctx, "rack", rackName, false)
	if err != nil {
		return errors.Annotate(err, "updateIndexingForRackResources - Failed to query rpms for rack %s", rackName).Err()
	}
	// get Switches for indexing
	switches, err := registration.QuerySwitchByPropertyName(ctx, "rack", rackName, false)
	if err != nil {
		return errors.Annotate(err, "updateIndexingForRackResources - Failed to query switches for rack %s", rackName).Err()
	}
	for k, v := range indexMap {
		// These are output only fields used for indexing kvm/rpm/switch table
		switch k {
		case "zone":
			for _, kvm := range kvms {
				kvm.Zone = v
			}
			for _, rpm := range rpms {
				rpm.Zone = v
			}
			for _, s := range switches {
				s.Zone = v
			}
		}
	}
	if _, err := registration.BatchUpdateKVMs(ctx, kvms); err != nil {
		return errors.Annotate(err, "updateIndexingForRackResources - Unable to update kvms").Err()
	}
	if _, err := registration.BatchUpdateRPMs(ctx, rpms); err != nil {
		return errors.Annotate(err, "updateIndexingForRackResources - Unable to update rpms").Err()
	}
	if _, err := registration.BatchUpdateSwitches(ctx, switches); err != nil {
		return errors.Annotate(err, "updateIndexingForRackResources - Unable to update switches").Err()
	}
	hc.LogRackLocationChanges(kvms, switches, rpms, indexMap, oldIndexMap)
	return nil
}

// GetRack returns rack for the given id from datastore.
func GetRack(ctx context.Context, id string) (*ufspb.Rack, error) {
	rack, err := registration.GetRackACL(ctx, id)
	if err != nil {
		return nil, err
	}
	setRack(ctx, rack)
	return rack, nil
}

// BatchGetRacks returns a batch of racks from datastore.
func BatchGetRacks(ctx context.Context, ids []string) ([]*ufspb.Rack, error) {
	return registration.BatchGetRacksACL(ctx, ids)
}

// ListRacks lists the racks
func ListRacks(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]*ufspb.Rack, string, error) {
	var filterMap map[string][]any
	var err error
	if filter != "" {
		filterMap, err = getFilterMap(filter, registration.GetRackIndexedFieldName)
		if err != nil {
			return nil, "", errors.Annotate(err, "Failed to read filter for listing Racks").Err()
		}
	}
	filterMap = resetStateFilter(filterMap, registration.GetRackIndexedFieldName)
	filterMap = resetZoneFilter(filterMap, registration.GetRackIndexedFieldName)
	filterMap, err = parseIntTypeFilter(filterMap, "bbnum")
	if err != nil {
		return nil, "", errors.Annotate(err, "filter value for bbnum cannot be parsed").Err()
	}
	racks, nextPageToken, err := listRacksWithExperimentalACLs(ctx, pageSize, pageToken, filterMap, keysOnly)
	if full && !keysOnly {
		for _, rack := range racks {
			setRack(ctx, rack)
		}
	}
	return racks, nextPageToken, err
}

func listRacksWithExperimentalACLs(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*ufspb.Rack, nextPageToken string, err error) {
	if pageToken != "" {
		// See registration/machine.go.
		// ListMachinesACL runs a different API to compared to ListMachines
		// This results in ListMachinesACL getting a different type of page
		// token (a multicursor) compared to ListMachines. The function
		// IsMultiCursor is used here to tell which API to use.
		// This is required because this function gets called repeatedly
		// (due to limitations in RPC size) by clients (like shivas).
		//
		// The first time there is no page token so we choose which rpc
		// to use at random. But if there is more data to be returned
		// after the first run, datastore returns a page token. As the
		// token is different based on which API was used, we use it to
		// do the remaining transactions.
		//
		// Note: We can't use token from ListMachinesACL on ListMachines,
		// this doesn't always throw an error but the results are
		// undefined. We do get an error(with high accuracy) if we use
		// token from ListMachines on ListMachinesACL
		if datastore.IsMultiCursorString(pageToken) {
			logging.Infof(ctx, "ListMachines --- Continue Running in experimental API")
			// If we have a multicursor in our hand. Then we got to do the ACLs
			return registration.ListRacksACL(ctx, pageSize, pageToken, filterMap, keysOnly)
		} else {
			return registration.ListRacks(ctx, pageSize, pageToken, filterMap, keysOnly)
		}
	}

	cutoff := config.Get(ctx).GetExperimentalAPI().GetListRacksACL()

	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "ListMachines --- Running in experimental API")
			return registration.ListRacksACL(ctx, pageSize, pageToken, filterMap, keysOnly)
		}
	}

	// default to old API
	return registration.ListRacks(ctx, pageSize, pageToken, filterMap, keysOnly)
}

// DeleteRack deletes the rack in datastore
//
// For referential data intergrity,
// Delete if this Rack is not referenced by other resources in the datastore.
// If there are any references, delete will be rejected and an error will be returned.
func DeleteRack(ctx context.Context, id string) error {
	// [TODO]: Add logic for Chrome OS
	f := func(ctx context.Context) error {
		// Get the rack
		rack, err := registration.GetRack(ctx, id)
		if status.Code(err) == codes.Internal {
			return err
		}
		if rack == nil {
			return status.Errorf(codes.NotFound, ufsds.NotFound)
		}

		// Check if any other resource references this rack.
		if err = validateDeleteRack(ctx, rack); err != nil {
			return err
		}

		var switchIDs []string
		var kvmIDs []string
		var rpmIDs []string
		//Only for a browser rack
		if rack.GetChromeBrowserRack() != nil {
			switchIDs, err = getDeleteSwitchIDs(ctx, rack.GetName())
			if err != nil {
				return err
			}
			kvmIDs, err = getDeleteKVMIDs(ctx, rack.GetName())
			if err != nil {
				return err
			}
			rpmIDs, err = getDeleteRPMIDs(ctx, rack.GetName())
			if err != nil {
				return err
			}
			for _, switchID := range switchIDs {
				if err := deleteSwitchHelper(ctx, switchID, false); err != nil {
					return errors.Annotate(err, "Failed to delete associated switch %s", switchID).Err()
				}
			}
			for _, kvmID := range kvmIDs {
				if err := deleteKVMHelper(ctx, kvmID, false); err != nil {
					return errors.Annotate(err, "Failed to delete associated kvm %s", kvmID).Err()
				}
			}
			for _, rpmID := range rpmIDs {
				if err := deleteRPMHelper(ctx, rpmID, false); err != nil {
					return errors.Annotate(err, "Failed to delete associated rpm %s", rpmID).Err()
				}
			}
		}

		// 6. Delete the rack
		if err := registration.DeleteRack(ctx, id); err != nil {
			return err
		}
		hc := getRackClientHistory(&ufspb.Rack{Name: id})
		hc.LogRackChanges(&ufspb.Rack{Name: id}, nil)
		hc.stUdt.deleteStateHelper(ctx)
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to delete rack and its associated switches, rpms and kvms in datastore: %s", err)
		return err
	}
	return nil
}

// RenameRack renames a rack in datastore
//
// All corresponding entities which are related to the old rack will be updated.
func RenameRack(ctx context.Context, oldName, newName string) (rack *ufspb.Rack, err error) {
	oldRack, err := registration.GetRack(ctx, oldName)
	// The error type can only be Internal or NotFound
	if status.Code(err) == codes.NotFound {
		return nil, status.Errorf(codes.NotFound, ufsds.NotFound)
	}
	if err != nil {
		return nil, errors.Annotate(err, "internal error in getting old rack %s", oldName).Err()
	}

	// Check if any other resource references this rack.
	if err = validateRenameRack(ctx, oldRack, newName); err != nil {
		return nil, errors.Annotate(err, "validate old rack %s", oldName).Err()
	}
	hc := getRackClientHistory(oldRack)
	// Move this function out of transaction as there may be 500+ entities attached to the old rack.
	if err := renameRackHelper(ctx, oldName, newName, hc); err != nil {
		return nil, errors.Annotate(err, "update entities attached to old rack").Err()
	}
	f := func(ctx context.Context) error {
		rack = proto.Clone(oldRack).(*ufspb.Rack)

		// Delete the old rack
		if err := registration.DeleteRack(ctx, oldName); err != nil {
			return err
		}
		// Check if new rack name already exists, if not, create the new rack.
		if err := resourceAlreadyExists(ctx, []*Resource{GetRackResource(newName)}, nil); err == nil {
			rack.Name = newName
			if _, err := registration.BatchUpdateRacks(ctx, []*ufspb.Rack{rack}); err != nil {
				return err
			}
		}
		return nil
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to rename rack: %s", err)
		return nil, errors.Annotate(err, "failed to rename rack %s to %s", oldName, newName).Err()
	}
	// Log history change events
	hc.LogRackChanges(&ufspb.Rack{Name: oldName}, &ufspb.Rack{Name: newName})
	if err := hc.SaveChangeEvents(ctx); err != nil {
		return nil, err
	}
	return rack, nil
}

func renameRackHelper(ctx context.Context, oldName, newName string, hc *HistoryClient) error {
	// Rename hosts associated with this rack
	if err := updateIndexInMachineLSE(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename machines associated with this rack
	if err := updateIndexInMachine(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename assets associated with this rack
	if err := updateIndexInAsset(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename switches associated with this rack
	if err := updateIndexInSwitch(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename nics associated with this rack
	if err := updateIndexInNic(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename rpms associated with this rack
	if err := updateIndexInRPM(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename kvms associated with this rack
	if err := updateIndexInKVM(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	// Rename dracs associated with this rack
	if err := updateIndexInDrac(ctx, "rack", oldName, newName, hc); err != nil {
		return err
	}

	return nil
}

// validateRenameRack validates if a rack can be renamed
func validateRenameRack(ctx context.Context, oldRack *ufspb.Rack, newName string) error {
	// Check permission
	if err := ufsUtil.CheckPermission(ctx, ufsUtil.RegistrationsUpdate, oldRack.GetRealm()); err != nil {
		return err
	}
	return nil
}

// can be called inside a transaction
func getDeleteSwitchIDs(ctx context.Context, rackName string) ([]string, error) {
	switches, err := registration.QuerySwitchByPropertyName(ctx, "rack", rackName, false)
	if err != nil {
		return nil, errors.Annotate(err, "DeleteRack - failed to query switches for rack %s", rackName).Err()
	}
	switchIDs := make([]string, 0, len(switches))
	for _, s := range switches {
		if err := validateDeleteSwitch(ctx, s); err != nil {
			return nil, errors.Annotate(err, "validation failed - Unable to delete switch %s", s.GetName()).Err()
		}
		switchIDs = append(switchIDs, s.GetName())
	}
	return switchIDs, nil
}

// can be called inside a transaction
func getDeleteKVMIDs(ctx context.Context, rackName string) ([]string, error) {
	kvms, err := registration.QueryKVMByPropertyName(ctx, "rack", rackName, false)
	if err != nil {
		return nil, errors.Annotate(err, "DeleteRack - failed to query kvms for rack %s", rackName).Err()
	}
	var kvmIDs []string
	for _, kvm := range kvms {
		if err := validateDeleteKVM(ctx, kvm); err != nil {
			return nil, errors.Annotate(err, "validation failed - Unable to delete kvm %s", kvm.GetName()).Err()
		}
		kvmIDs = append(kvmIDs, kvm.GetName())
	}
	return kvmIDs, nil
}

// can be called inside a transaction
func getDeleteRPMIDs(ctx context.Context, rackName string) ([]string, error) {
	rpms, err := registration.QueryRPMByPropertyName(ctx, "rack", rackName, false)
	if err != nil {
		return nil, errors.Annotate(err, "DeleteRack - failed to query rpms for rack %s", rackName).Err()
	}
	var rpmIDs []string
	for _, rpm := range rpms {
		if err := validateDeleteRPM(ctx, rpm); err != nil {
			return nil, errors.Annotate(err, "validation failed - Unable to delete rpm %s", rpm.GetName()).Err()
		}
		rpmIDs = append(rpmIDs, rpm.GetName())
	}
	return rpmIDs, nil
}

// ReplaceRack replaces an old Rack with new Rack in datastore
//
// It does a delete of old rack and create of new Rack.
// All the steps are in done in a transaction to preserve consistency on failure.
// Before deleting the old Rack, it will get all the resources referencing
// the old Rack. It will update all the resources which were referencing
// the old Rack(got in the last step) with new Rack.
// Deletes the old Rack.
// Creates the new Rack.
// This will preserve data integrity in the system.
func ReplaceRack(ctx context.Context, oldRack *ufspb.Rack, newRack *ufspb.Rack) (*ufspb.Rack, error) {
	f := func(ctx context.Context) error {
		hc := getRackClientHistory(newRack)
		racklses, err := inventory.QueryRackLSEByPropertyName(ctx, "rack_ids", oldRack.Name, false)
		if err != nil {
			return err
		}
		if racklses != nil {
			for _, racklse := range racklses {
				racks := racklse.GetRacks()
				for i := range racks {
					if racks[i] == oldRack.Name {
						racks[i] = newRack.Name
						break
					}
				}
				racklse.Racks = racks
			}
			_, err := inventory.BatchUpdateRackLSEs(ctx, racklses)
			if err != nil {
				return err
			}
		}

		err = registration.DeleteRack(ctx, oldRack.Name)
		if err != nil {
			return err
		}

		entity := &registration.RackEntity{
			ID: newRack.Name,
		}
		existsResults, err := datastore.Exists(ctx, entity)
		if err == nil {
			if existsResults.All() {
				return status.Errorf(codes.AlreadyExists, ufsds.AlreadyExists)
			}
		} else {
			logging.Errorf(ctx, "Failed to check existence: %s", err)
			return status.Errorf(codes.Internal, ufsds.InternalError)
		}

		_, err = registration.BatchUpdateRacks(ctx, []*ufspb.Rack{newRack})
		if err != nil {
			return err
		}
		hc.LogRackChanges(oldRack, nil)
		hc.LogRackChanges(nil, newRack)
		return hc.SaveChangeEvents(ctx)
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		logging.Errorf(ctx, "Failed to replace entity in datastore: %s", err)
		return nil, err
	}
	return newRack, nil
}

// validateDeleteRack validates if a Rack can be deleted
//
// Checks if this Rack(RackID) is not referenced by other resources in the datastore.
// If there are any other references, delete will be rejected and an error will be returned.
func validateDeleteRack(ctx context.Context, rack *ufspb.Rack) error {
	if err := ufsUtil.CheckPermission(ctx, ufsUtil.RegistrationsDelete, rack.GetRealm()); err != nil {
		return err
	}
	racklses, err := inventory.QueryRackLSEByPropertyName(ctx, "rack_ids", rack.GetName(), true)
	if err != nil {
		return err
	}
	machines, err := registration.QueryMachineByPropertyName(ctx, "rack", rack.GetName(), true)
	if err != nil {
		return err
	}
	assets, err := registration.QueryAssetByPropertyName(ctx, "rack", rack.GetName(), true)
	if err != nil {
		return err
	}
	if len(racklses) > 0 || len(machines) > 0 || len(assets) > 0 {
		var errorMsg strings.Builder
		errorMsg.WriteString(fmt.Sprintf("Rack %s cannot be deleted because there are other resources which are referring this Rack.", rack.GetName()))
		if len(racklses) > 0 {
			errorMsg.WriteString("\nRackLSEs referring the Rack:\n")
			for _, racklse := range racklses {
				errorMsg.WriteString(racklse.Name + ", ")
			}
		}
		if len(machines) > 0 {
			errorMsg.WriteString("\nMachines referring to the Rack:\n")
			for _, machine := range machines {
				errorMsg.WriteString(machine.Name + ", ")
			}
		}
		if len(assets) > 0 {
			errorMsg.WriteString("\nAssets referring to the Rack:\n")
			for _, asset := range assets {
				errorMsg.WriteString(asset.Name + ", ")
			}
		}
		logging.Errorf(ctx, errorMsg.String())
		return status.Errorf(codes.FailedPrecondition, errorMsg.String())
	}
	return nil
}

func validateRackBbnum(ctx context.Context, rackName string, bbnum int32) error {
	if bbnum == 0 {
		// Won't verify bbnum's uniqueness if it's not set
		return nil
	}
	racks, err := registration.QueryRackByPropertyName(ctx, "bbnum", bbnum, true)
	if err != nil {
		return errors.Annotate(err, "validateRackBbnum - failed to query rack for bbnum %d", bbnum).Err()
	}
	if len(racks) > 0 && racks[0].GetName() != rackName {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("validateRackBbnum - Rack %s already has bbnum %d.\n", racks[0].GetName(), bbnum))
	}
	return nil
}

// validateRackRegistration validates if a rack, switches, kvms and rpms can be created in the datastore.
//
// checks if the resources rack/switches/kvms/rpms already exists in the system.
// checks if resources referenced by rack/switches/kvms/rpms does not exist in the system.
func validateRackRegistration(ctx context.Context, rack *ufspb.Rack) error {
	if rack == nil {
		return errors.New("rack cannot be empty")
	}
	if err := ufsUtil.CheckPermission(ctx, ufsUtil.RegistrationsCreate, rack.GetRealm()); err != nil {
		return err
	}
	if err := validateRackBbnum(ctx, rack.GetName(), rack.GetBbnum()); err != nil {
		return err
	}
	if rack.GetChromeBrowserRack() == nil && rack.GetChromeosRack() == nil {
		if rack.GetLocation() == nil || rack.GetLocation().GetZone() == ufspb.Zone_ZONE_UNSPECIFIED {
			return errors.New("zone information in the location object cannot be empty/unspecified for a rack")
		}
		if ufsUtil.IsInBrowserZone(rack.GetLocation().GetZone().String()) {
			rack.Rack = &ufspb.Rack_ChromeBrowserRack{
				ChromeBrowserRack: &ufspb.ChromeBrowserRack{},
			}
		} else {
			rack.Rack = &ufspb.Rack_ChromeosRack{
				ChromeosRack: &ufspb.ChromeOSRack{},
			}
		}
	}

	var resourcesAlreadyExists []*Resource
	var resourcesNotFound []*Resource
	var switches []*ufspb.Switch
	var kvms []*ufspb.KVM
	var rpms []*ufspb.RPM
	if rack.GetChromeBrowserRack() != nil {
		switches = rack.GetChromeBrowserRack().GetSwitchObjects()
		kvms = rack.GetChromeBrowserRack().GetKvmObjects()
		rpms = rack.GetChromeBrowserRack().GetRpmObjects()
	}
	// Aggregate resources to check if rack already exists
	resourcesAlreadyExists = append(resourcesAlreadyExists, GetRackResource(rack.Name))
	// Also validate the corresponding smaller case of rack name
	resourcesAlreadyExists = append(resourcesAlreadyExists, GetRackResource(strings.ToLower(rack.Name)))

	for _, s := range switches {
		// Aggregate resources to check if switch already exists
		resourcesAlreadyExists = append(resourcesAlreadyExists, GetSwitchResource(s.Name))
	}

	for _, kvm := range kvms {
		// Aggregate resources to check if kvm already exists
		resourcesAlreadyExists = append(resourcesAlreadyExists, GetKVMResource(kvm.Name))

		// Aggregate resource to check if resources referenced by the kvm does not exist
		if chromePlatformID := kvm.GetChromePlatform(); chromePlatformID != "" {
			resourcesNotFound = append(resourcesNotFound, GetChromePlatformResource(chromePlatformID))
		}

		if err := validateMacAddress(ctx, kvm.GetName(), kvm.GetMacAddress()); err != nil {
			return err
		}
	}

	for _, rpm := range rpms {
		// Aggregate resources to check if rpm already exists
		resourcesAlreadyExists = append(resourcesAlreadyExists, GetRPMResource(rpm.Name))
	}

	// Check if rack/switches/kvms/rpms already exists
	if err := resourceAlreadyExists(ctx, resourcesAlreadyExists, nil); err != nil {
		return err
	}

	// Check if resources referenced by rack/switches/kvms/rpms does not exist
	return ResourceExist(ctx, resourcesNotFound, nil)
}

// validateUpdateRack validates if a rack can be updated
func validateUpdateRack(ctx context.Context, oldRack *ufspb.Rack, rack *ufspb.Rack, mask *field_mask.FieldMask) error {
	if err := ufsUtil.CheckPermission(ctx, ufsUtil.RegistrationsUpdate, oldRack.GetRealm()); err != nil {
		return err
	}
	if rack.GetRealm() != "" && oldRack.GetRealm() != rack.GetRealm() {
		if err := ufsUtil.CheckPermission(ctx, ufsUtil.RegistrationsUpdate, rack.GetRealm()); err != nil {
			return err
		}
	}
	if mask == nil {
		if err := validateRackBbnum(ctx, rack.GetName(), rack.GetBbnum()); err != nil {
			return err
		}
	}
	return validateRackUpdateMask(ctx, rack, mask)
}

// validateRackUpdateMask validates the update mask for Rack update
func validateRackUpdateMask(ctx context.Context, rack *ufspb.Rack, mask *field_mask.FieldMask) error {
	if mask != nil {
		// validate the give field mask
		for _, path := range mask.Paths {
			switch path {
			case ufsUtil.NamePath:
				return status.Error(codes.InvalidArgument, "validateUpdateRack - name cannot be updated, delete and create a new rack instead")
			case ufsUtil.UpdateTimePath:
				return status.Error(codes.InvalidArgument, "validateUpdateRack - update_time cannot be updated, it is a output only field")
			case ufsUtil.LocationZonePath, ufsUtil.ZonePath:
				if rack.GetLocation() == nil {
					return status.Error(codes.InvalidArgument, "validateUpdateRack - location cannot be empty/nil.")
				}
			case ufsUtil.BbnumPath:
				if err := validateRackBbnum(ctx, rack.GetName(), rack.GetBbnum()); err != nil {
					return err
				}
			case ufsUtil.CapacityRuPath, ufsUtil.CapacityPath:
			case ufsUtil.TagsPath:
			case ufsUtil.ResourceStatePath, ufsUtil.ResourceStateCamelPath:
				// valid fields, nothing to validate.
			default:
				return status.Errorf(codes.InvalidArgument, "validateUpdateRack - unsupported update mask path %q", path)
			}
		}
	}
	return nil
}

func getRackClientHistory(m *ufspb.Rack) *HistoryClient {
	return &HistoryClient{
		stUdt: &stateUpdater{
			ResourceName: ufsUtil.AddPrefix(ufsUtil.RackCollection, m.Name),
		},
		netUdt: &networkUpdater{
			Hostname: m.Name,
		},
	}
}

// setRack populates the rack object with switches and drac
func setRack(ctx context.Context, rack *ufspb.Rack) {
	// get switches for rack
	switches, err := registration.QuerySwitchByPropertyName(ctx, "rack", rack.GetName(), false)
	if err != nil {
		// Just log a warning message and dont fail operation
		logging.Warningf(ctx, "GetRack - failed to query switches for rack %s: %s", rack.GetName(), err)
	}
	setSwitchesToRack(rack, switches)

	// get kvms for rack
	kvms, err := registration.QueryKVMByPropertyName(ctx, "rack", rack.GetName(), false)
	if err != nil {
		// Just log a warning message and dont fail operation
		logging.Warningf(ctx, "GetRack - failed to query kvms for rack %s: %s", rack.GetName(), err)
	}
	setKVMsToRack(rack, kvms)

	// get rpms for rack
	rpms, err := registration.QueryRPMByPropertyName(ctx, "rack", rack.GetName(), false)
	if err != nil {
		// Just log a warning message and dont fail operation
		logging.Warningf(ctx, "GetRack - failed to query rpms for rack %s: %s", rack.GetName(), err)
	}
	setRPMsToRack(rack, rpms)
}

func setSwitchesToRack(rack *ufspb.Rack, switches []*ufspb.Switch) {
	if len(switches) <= 0 {
		return
	}
	if rack.GetChromeBrowserRack() != nil {
		rack.GetChromeBrowserRack().SwitchObjects = switches
	} else if ufsUtil.IsInBrowserZone(rack.GetLocation().GetZone().String()) {
		rack.Rack = &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{
				SwitchObjects: switches,
			},
		}
	}
	if rack.GetChromeosRack() != nil {
		rack.GetChromeosRack().SwitchObjects = switches
	} else if ufsUtil.IsInOSZone(rack.GetLocation().GetZone().String()) {
		rack.Rack = &ufspb.Rack_ChromeosRack{
			ChromeosRack: &ufspb.ChromeOSRack{
				SwitchObjects: switches,
			},
		}
	}
}

func setKVMsToRack(rack *ufspb.Rack, kvms []*ufspb.KVM) {
	if len(kvms) <= 0 {
		return
	}
	if rack.GetChromeBrowserRack() == nil {
		rack.Rack = &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{
				KvmObjects: kvms,
			},
		}
	} else {
		rack.GetChromeBrowserRack().KvmObjects = kvms
	}
}

func setRPMsToRack(rack *ufspb.Rack, rpms []*ufspb.RPM) {
	if len(rpms) <= 0 {
		return
	}
	if rack.GetChromeBrowserRack() == nil {
		rack.Rack = &ufspb.Rack_ChromeBrowserRack{
			ChromeBrowserRack: &ufspb.ChromeBrowserRack{
				RpmObjects: rpms,
			},
		}
	} else {
		rack.GetChromeBrowserRack().RpmObjects = rpms
	}
}
