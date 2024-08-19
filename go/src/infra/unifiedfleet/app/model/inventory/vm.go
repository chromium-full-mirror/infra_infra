// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"context"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsds "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/util"
)

// VMKind is the datastore entity kind for vm.
const VMKind string = "ChromeVM"

// VMEntity is a datastore entity that tracks VM.
type VMEntity struct {
	_kind          string                `gae:"$kind,ChromeVM"`
	Extra          datastore.PropertyMap `gae:",extra"`
	ID             string                `gae:"$id"`
	OSVersion      string                `gae:"os_version"`
	Vlan           string                `gae:"vlan_id"`
	HostID         string                `gae:"host_id"`
	State          string                `gae:"state"`
	Lab            string                `gae:"lab"` // deprecated
	Zone           string                `gae:"zone"`
	Tags           []string              `gae:"tags"`
	OS             []string              `gae:"os"`
	MacAddress     string                `gae:"mac_address"`
	CpuCores       int32                 `gae:"cpu_cores"`
	Memory         int64                 `gae:"memory"`
	Storage        int64                 `gae:"storage"`
	Pool           string                `gae:"pool"`
	SwarmingServer string                `gae:"swarming_server"`
	Customer       string                `gae:"customer"`
	SecurityLevel  string                `gae:"security_level"`
	MibaRealm      string                `gae:"miba_realm,noindex"` // deprecated
	// Follow others entities, store ufspb.VM bytes.
	VM []byte `gae:",noindex"`
}

// GetProto returns the unmarshaled VM.
func (e *VMEntity) GetProto() (proto.Message, error) {
	var p ufspb.VM
	if err := proto.Unmarshal(e.VM, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether a VMEntity is valid
func (e *VMEntity) Validate() error {
	return nil
}

func newVMEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	p := pm.(*ufspb.VM)
	if p.GetName() == "" {
		return nil, errors.Reason("Empty VM ID").Err()
	}
	vm, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal VM %s", p).Err()
	}

	poolName := ""
	swarmingInstance := ""
	customer := ""
	securityLevel := ""
	if p.GetOwnership() != nil {
		poolName = p.GetOwnership().PoolName
		swarmingInstance = p.GetOwnership().SwarmingInstance
		customer = p.GetOwnership().Customer
		securityLevel = p.GetOwnership().SecurityLevel
	}
	return &VMEntity{
		ID:             p.GetName(),
		OSVersion:      p.GetOsVersion().GetValue(),
		Vlan:           p.GetVlan(),
		HostID:         p.GetMachineLseId(),
		State:          p.GetResourceState().String(),
		Zone:           p.GetZone(),
		Tags:           p.GetTags(),
		OS:             ufsds.GetOSIndex(p.GetOsVersion().GetValue()),
		MacAddress:     p.GetMacAddress(),
		CpuCores:       p.GetCpuCores(),
		Memory:         p.GetMemory(),
		Storage:        p.GetStorage(),
		Pool:           poolName,
		SwarmingServer: swarmingInstance,
		Customer:       customer,
		SecurityLevel:  securityLevel,
		VM:             vm,
	}, nil
}

// QueryVMByPropertyName queries VM Entity in the datastore
// If keysOnly is true, then only key field is populated in returned machinelses
func QueryVMByPropertyName(ctx context.Context, propertyName, id string, keysOnly bool) ([]*ufspb.VM, error) {
	q := datastore.NewQuery(VMKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*VMEntity
	if err := datastore.GetAll(ctx, q.Eq(propertyName, id), &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Infof(ctx, "No vms found for the query: %s=%s", propertyName, id)
		return nil, nil
	}
	vms := make([]*ufspb.VM, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			vm := &ufspb.VM{
				Name: entity.ID,
			}
			vms = append(vms, vm)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			vms = append(vms, pm.(*ufspb.VM))
		}
	}
	return vms, nil
}

func queryAllVM(ctx context.Context) ([]ufsds.FleetEntity, error) {
	var entities []*VMEntity
	q := datastore.NewQuery(VMKind)
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	fe := make([]ufsds.FleetEntity, len(entities))
	for i, e := range entities {
		fe[i] = e
	}
	return fe, nil
}

// GetAllVMs returns all vms in datastore.
func GetAllVMs(ctx context.Context) (*ufsds.OpResults, error) {
	return ufsds.GetAll(ctx, queryAllVM)
}

// DeleteVM deletes the vm in datastore
func DeleteVM(ctx context.Context, id string) error {
	return ufsds.Delete(ctx, &ufspb.VM{Name: id}, newVMEntity)
}

// DeleteVMs deletes a batch of vms
//
// Can be used in a transaction
func DeleteVMs(ctx context.Context, resourceNames []string) *ufsds.OpResults {
	protos := make([]proto.Message, len(resourceNames))
	for i, m := range resourceNames {
		protos[i] = &ufspb.VM{
			Name: m,
		}
	}
	return ufsds.DeleteAll(ctx, protos, newVMEntity)
}

// BatchDeleteVMs deletes vms in datastore.
//
// This is a non-atomic operation. Must be used within a transaction.
// Will lead to partial deletes if not used in a transaction.
func BatchDeleteVMs(ctx context.Context, ids []string) error {
	protos := make([]proto.Message, len(ids))
	for i, id := range ids {
		protos[i] = &ufspb.VM{Name: id}
	}
	return ufsds.BatchDelete(ctx, protos, newVMEntity)
}

// GetVM returns vms for the given id from datastore.
func GetVM(ctx context.Context, id string) (*ufspb.VM, error) {
	pm, err := ufsds.Get(ctx, &ufspb.VM{Name: id}, newVMEntity)
	if err == nil {
		return pm.(*ufspb.VM), err
	}
	return nil, err
}

func getVMID(pm proto.Message) string {
	p := pm.(*ufspb.VM)
	return p.GetName()
}

// BatchGetVMs returns a batch of vms from datastore.
func BatchGetVMs(ctx context.Context, ids []string) ([]*ufspb.VM, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.VM{Name: n}
	}
	pms, err := ufsds.BatchGet(ctx, protos, newVMEntity, getVMID)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.VM, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.VM)
	}
	return res, nil
}

// BatchUpdateVMs updates vms in datastore.
//
// This is a non-atomic operation and doesnt check if the object already exists before
// update. Must be used within a Transaction where objects are checked before update.
// Will lead to partial updates if not used in a transaction.
func BatchUpdateVMs(ctx context.Context, vms []*ufspb.VM) ([]*ufspb.VM, error) {
	protos := make([]proto.Message, len(vms))
	updateTime := ptypes.TimestampNow()
	for i, v := range vms {
		v.UpdateTime = updateTime

		// Redact ownership data
		redactVMOwnership(ctx, v)
		protos[i] = v
	}
	_, err := ufsds.PutAll(ctx, protos, newVMEntity, true)
	if err == nil {
		return vms, err
	}
	return nil, err
}

// UpdateVMOwnership updates VM ownership in datastore.
func UpdateVMOwnership(ctx context.Context, id string, ownership *ufspb.OwnershipData) (*ufspb.VM, error) {
	return putVMOwnership(ctx, id, ownership, true)
}

// Updates the ownership data for an existing VM.
func putVMOwnership(ctx context.Context, id string, ownership *ufspb.OwnershipData, update bool) (*ufspb.VM, error) {
	vm, err := GetVM(ctx, id)
	if err != nil {
		return vm, err
	}
	vm.Ownership = ownership

	vm.UpdateTime = ptypes.TimestampNow()
	pm, err := ufsds.Put(ctx, vm, newVMEntity, update)
	if err == nil {
		return pm.(*ufspb.VM), err
	}
	return nil, err
}

// Redacts VM ownership for updates by either changing the ownership to existing values or
// for new entities setting the ownership to nil as we don't want to allow user updates to these values.
func redactVMOwnership(ctx context.Context, vm *ufspb.VM) {
	if vm == nil {
		return
	}
	// Redact ownership data
	existingVM, err := GetVM(ctx, vm.Name)
	if err == nil {
		vm.Ownership = existingVM.Ownership
	} else {
		vm.Ownership = nil
	}
}

// ListVMs lists the vms
//
// Does a query over vm entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListVMs(ctx context.Context, pageSize int32, requiredSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool, validFunc func(*ufspb.VM) bool) (res []*ufspb.VM, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, VMKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runVMListQuery(ctx, q, pageSize, requiredSize, pageToken, keysOnly, validFunc)
}

// ListVMsByIdPrefixSearch lists the VMs
//
// Does a query over VM entities using Name/ID prefix. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). PageSize must be positive.
func ListVMsByIdPrefixSearch(ctx context.Context, pageSize int32, requiredSize int32, pageToken string, prefix string, keysOnly bool, validFunc func(*ufspb.VM) bool) (res []*ufspb.VM, nextPageToken string, err error) {
	q, err := ufsds.ListQueryIdPrefixSearch(ctx, VMKind, pageSize, pageToken, prefix, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runVMListQuery(ctx, q, pageSize, requiredSize, pageToken, keysOnly, validFunc)
}

// Runs the query to list VMs and returns the results
func runVMListQuery(ctx context.Context, q *datastore.Query, pageSize int32, requiredSize int32, pageToken string, keysOnly bool, validFunc func(*ufspb.VM) bool) (res []*ufspb.VM, nextPageToken string, err error) {
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *VMEntity, cb datastore.CursorCB) error {
		if keysOnly {
			vm := &ufspb.VM{
				Name: ent.ID,
			}
			if validFunc == nil || (validFunc != nil && validFunc(vm)) {
				res = append(res, vm)
			}
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			vm := pm.(*ufspb.VM)
			if validFunc == nil || (validFunc != nil && validFunc(vm)) {
				res = append(res, vm)
			}
		}
		if len(res) >= int(requiredSize) {
			if nextCur, err = cb(); err != nil {
				return err
			}
			return datastore.Stop
		}
		return nil
	})
	if err != nil {
		logging.Errorf(ctx, "Failed to List VMs: %s", err)
		return nil, "", status.Errorf(codes.Internal, err.Error())
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// GetVMIndexedFieldName returns the index name
func GetVMIndexedFieldName(input string) (string, error) {
	var field string
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case util.VlanFilterName:
		field = "vlan_id"
	case util.StateFilterName:
		field = "state"
	case util.HostFilterName:
		field = "host_id"
	case util.ZoneFilterName:
		field = "zone"
	case util.TagFilterName:
		field = "tags"
	case util.OSFilterName:
		field = "os"
	case util.CpuCoresFilterName:
		field = "cpu_cores"
	case util.MemoryFilterName:
		field = "memory"
	case util.StorageFilterName:
		field = "storage"
	default:
		return "", status.Errorf(codes.InvalidArgument, "Invalid field name %s - field name for host are vlan/state/host/zone/tag/os/cpucores/memory/storage", input)
	}
	return field, nil
}
