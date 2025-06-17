// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"context"
	"math/rand"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// MachineKind is the datastore entity kind Machine.
const MachineKind string = "Machine"

// MachineEntity is a datastore entity that tracks Machine.
type MachineEntity struct {
	_kind string                `gae:"$kind,Machine"`
	Extra datastore.PropertyMap `gae:",extra"`
	// ufspb.Machine.Name
	ID               string   `gae:"$id"`
	SerialNumber     string   `gae:"serial_number"`
	AssetTag         string   `gae:"asset_tag"`
	KVMID            string   `gae:"kvm_id"`
	KVMPort          string   `gae:"kvm_port"`
	RPMID            string   `gae:"rpm_id"`
	NicIDs           []string `gae:"nic_ids"` // deprecated. Do not use.
	DracID           string   `gae:"drac_id"` // deprecated. Do not use.
	ChromePlatformID string   `gae:"chrome_platform_id"`
	Rack             string   `gae:"rack"`
	Lab              string   `gae:"lab"` // deprecated
	Zone             string   `gae:"zone"`
	Tags             []string `gae:"tags"`
	State            string   `gae:"state"`
	Model            string   `gae:"model"`
	BuildTarget      string   `gae:"build_target"`
	DeviceType       string   `gae:"device_type"`
	Phase            string   `gae:"phase"`
	Pool             string   `gae:"pool"`
	SwarmingServer   string   `gae:"swarming_server"`
	Customer         string   `gae:"customer"`
	SecurityLevel    string   `gae:"security_level"`
	MibaRealm        string   `gae:"miba_realm,noindex"` // deprecated
	GPN              string   `gae:"gpn"`
	Realm            string   `gae:"realm"`
	Manufacturer     string   `gae:"manufacturer"`
	// ufspb.Machine cannot be directly used as it contains pointer.
	Machine []byte `gae:",noindex"`
}

// GetProto returns the unmarshaled Machine.
func (e *MachineEntity) GetProto() (proto.Message, error) {
	var p ufspb.Machine
	if err := proto.Unmarshal(e.Machine, &p); err != nil {
		return nil, err
	}
	// Assign read only realm field
	p.Realm = e.Realm
	return &p, nil
}

// Validate returns whether a MachineEntity is valid.
func (e *MachineEntity) Validate() error {
	return nil
}

func (e *MachineEntity) GetRealm() string {
	return e.Realm
}

func newMachineEntityRealm(ctx context.Context, pm proto.Message) (ufsds.RealmEntity, error) {
	p := pm.(*ufspb.Machine)
	if p.GetName() == "" {
		return nil, errors.Reason("Empty Machine ID").Err()
	}
	// Assign the realm to the proto. This will help us use this with BQ
	realm := util.ToUFSRealm(p.GetLocation().GetZone().String())
	p.Realm = realm

	machine, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal Machine %s", p).Err()
	}

	var buildTarget string
	var deviceType string
	var model string
	var phase string
	if p.GetChromeosMachine() != nil {
		buildTarget = p.GetChromeosMachine().GetBuildTarget()
		deviceType = p.GetChromeosMachine().GetDeviceType().String()
		model = strings.ToLower(p.GetChromeosMachine().GetModel())
		phase = p.GetChromeosMachine().GetPhase()
	} else if p.GetAttachedDevice() != nil {
		buildTarget = p.GetAttachedDevice().GetBuildTarget()
		deviceType = p.GetAttachedDevice().GetDeviceType().String()
		model = strings.ToLower(p.GetAttachedDevice().GetModel())
		phase = strings.ToLower(p.GetAttachedDevice().GetPhase())
	} else if p.GetServerMachine() != nil {
		model = strings.ToLower(p.GetServerMachine().GetModel())
	} else if p.GetDevboard() != nil {
		deviceType = util.GetDevboardType(p.GetDevboard())
	}

	var poolName string
	var swarmingInstance string
	var customer string
	var securityLevel string
	if p.GetOwnership() != nil {
		poolName = p.GetOwnership().PoolName
		swarmingInstance = p.GetOwnership().SwarmingInstance
		customer = p.GetOwnership().Customer
		securityLevel = p.GetOwnership().SecurityLevel
	}

	return &MachineEntity{
		ID:               p.GetName(),
		SerialNumber:     p.GetSerialNumber(),
		AssetTag:         p.GetAssetTag(),
		KVMID:            p.GetChromeBrowserMachine().GetKvmInterface().GetKvm(),
		KVMPort:          p.GetChromeBrowserMachine().GetKvmInterface().GetPortName(),
		RPMID:            p.GetChromeBrowserMachine().GetRpmInterface().GetRpm(),
		ChromePlatformID: p.GetChromeBrowserMachine().GetChromePlatform(),
		Rack:             p.GetLocation().GetRack(),
		Zone:             p.GetLocation().GetZone().String(),
		Tags:             p.GetTags(),
		Machine:          machine,
		State:            p.GetResourceState().String(),
		Model:            model,
		BuildTarget:      buildTarget,
		DeviceType:       deviceType,
		Phase:            phase,
		Pool:             poolName,
		SwarmingServer:   swarmingInstance,
		Customer:         customer,
		SecurityLevel:    securityLevel,
		GPN:              p.GetChromeosMachine().GetGpn(),
		Realm:            realm,
		Manufacturer:     p.GetAttachedDevice().GetManufacturer(),
	}, nil
}

func newMachineEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	return newMachineEntityRealm(ctx, pm)
}

// QueryMachineByPropertyName queries Machine Entity in the datastore
// If keysOnly is true, then only key field is populated in returned machines.
// Note that read realm ACLs are not enforced so this is not appropriate for
// use without some other ACL being enforced beforehand.
func QueryMachineByPropertyName(ctx context.Context, propertyName, id string, keysOnly bool) ([]*ufspb.Machine, error) {
	q := datastore.NewQuery(MachineKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*MachineEntity
	if err := datastore.GetAll(ctx, q.Eq(propertyName, id), &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Debugf(ctx, "No machines found for the query: %s", id)
		return nil, nil
	}
	machines := make([]*ufspb.Machine, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			machine := &ufspb.Machine{
				Name: entity.ID,
			}
			machines = append(machines, machine)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			machines = append(machines, pm.(*ufspb.Machine))
		}
	}
	return machines, nil
}

// CreateMachine creates a new machine in datastore.
func CreateMachine(ctx context.Context, machine *ufspb.Machine) (*ufspb.Machine, error) {
	return putMachine(ctx, machine, false)
}

// UpdateMachine updates machine in datastore.
func UpdateMachine(ctx context.Context, machine *ufspb.Machine) (*ufspb.Machine, error) {
	return putMachine(ctx, machine, true)
}

// UpdateMachineOwnership updates machine ownership in datastore.
func UpdateMachineOwnership(ctx context.Context, id string, ownership *ufspb.OwnershipData) (*ufspb.Machine, error) {
	return putMachineOwnership(ctx, id, ownership, true)
}

// GetMachine returns machine for the given id from datastore.
func GetMachine(ctx context.Context, id string) (*ufspb.Machine, error) {
	pm, err := ufsds.Get(ctx, &ufspb.Machine{Name: id}, newMachineEntity)
	if err == nil {
		return pm.(*ufspb.Machine), err
	}
	return nil, err
}

// GetMachineACL routes the request to either the ACLed or
// unACLed method depending on the rollout status.
func GetMachineACL(ctx context.Context, id string) (*ufspb.Machine, error) {
	cutoff := config.Get(ctx).GetExperimentalAPI().GetGetMachineACL()
	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "GetMachine --- Running in experimental API")
			return getMachineACL(ctx, id)
		}
	}

	return GetMachine(ctx, id)
}

// GetMachineACL returns a machine for the given ID after verifying the user
// has permission.
func getMachineACL(ctx context.Context, id string) (*ufspb.Machine, error) {
	pm, err := ufsds.GetACL(ctx, &ufspb.Machine{Name: id}, newMachineEntityRealm, util.RegistrationsGet)
	if err == nil {
		return pm.(*ufspb.Machine), err
	}
	return nil, err
}

func getMachineID(pm proto.Message) string {
	p := pm.(*ufspb.Machine)
	return p.GetName()
}

// batchGetMachines returns a batch of machines from datastore.
func batchGetMachines(ctx context.Context, ids []string) ([]*ufspb.Machine, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.Machine{Name: n}
	}
	pms, err := ufsds.BatchGet(ctx, protos, newMachineEntity, getMachineID)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.Machine, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.Machine)
	}
	return res, nil
}

// BatchGetMachinesACL routes the request to either the ACLed or
// unACLed method depending on the rollout status.
func BatchGetMachinesACL(ctx context.Context, ids []string) ([]*ufspb.Machine, error) {
	cutoff := config.Get(ctx).GetExperimentalAPI().GetGetMachineACL()
	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "GetMachine --- Running in experimental API")
			return batchGetMachinesACL(ctx, ids)
		}
	}

	return batchGetMachines(ctx, ids)
}

// batchGetMachines returns a batch of machines from datastore after making an
// ACL check.
func batchGetMachinesACL(ctx context.Context, ids []string) ([]*ufspb.Machine, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.Machine{Name: n}
	}
	pms, err := ufsds.BatchGetACL(ctx, protos, newMachineEntityRealm, getMachineID, util.RegistrationsGet)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.Machine, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.Machine)
	}
	return res, nil
}

// ListMachines lists the machines
// Does a query over Machine entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListMachines(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*ufspb.Machine, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, MachineKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListQuery(ctx, q, pageSize, pageToken, keysOnly)
}

// ListMachinesACL lists the machines in a realm the user has permission to view.
//
// Does a query over Machine entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListMachinesACL(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*ufspb.Machine, nextPageToken string, err error) {
	err = validateListMachineFilters(filterMap)
	if err != nil {
		return nil, "", errors.Annotate(err, "ListMachinesACL --- cannot validate query").Err()
	}

	userRealms, err := auth.QueryRealms(ctx, util.RegistrationsList, "", nil)
	if err != nil {
		return nil, "", err
	}

	q, err := ufsds.ListQuery(ctx, MachineKind, pageSize, "", filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}

	// Create a list of queries each checking for a realm assignment
	queries := ufsds.AssignRealms(q, userRealms)

	return runListQueries(ctx, queries, pageSize, pageToken, keysOnly)
}

// ListMachinesByIdPrefixSearch lists the machines
// Does a query over Machine entities using ID prefix. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). PageSize must be positive.
// Note- currently not ACLed, so should not be used for user-facing tasks
// without other ACLs upstream.
func ListMachinesByIdPrefixSearch(ctx context.Context, pageSize int32, pageToken string, prefix string, keysOnly bool) (res []*ufspb.Machine, nextPageToken string, err error) {
	q, err := ufsds.ListQueryIdPrefixSearch(ctx, MachineKind, pageSize, pageToken, prefix, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListQuery(ctx, q, pageSize, pageToken, keysOnly)
}

func runListQuery(ctx context.Context, query *datastore.Query, pageSize int32, pageToken string, keysOnly bool) (res []*ufspb.Machine, nextPageToken string, err error) {
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, query, func(ent *MachineEntity, cb datastore.CursorCB) error {
		if keysOnly {
			machine := &ufspb.Machine{
				Name: ent.ID,
			}
			res = append(res, machine)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.Machine))
		}
		if len(res) >= int(pageSize) {
			if nextCur, err = cb(); err != nil {
				return err
			}
			return datastore.Stop
		}
		return nil
	})
	if err != nil {
		logging.Errorf(ctx, "Failed to List Machines %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

func runListQueries(ctx context.Context, queries []*datastore.Query, pageSize int32, pageToken string, keysOnly bool) (res []*ufspb.Machine, nextPageToken string, err error) {
	if pageToken != "" {
		queries, err = datastore.ApplyCursorString(ctx, queries, pageToken)
		if err != nil {
			return nil, "", err
		}
	}

	var nextCur datastore.Cursor
	err = datastore.RunMulti(ctx, queries, func(ent *MachineEntity, cb datastore.CursorCB) error {
		if keysOnly {
			machine := &ufspb.Machine{
				Name: ent.ID,
			}
			res = append(res, machine)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.Machine))
		}
		if len(res) >= int(pageSize) {
			if nextCur, err = cb(); err != nil {
				return err
			}
			return datastore.Stop
		}
		return nil
	})
	if err != nil {
		logging.Errorf(ctx, "Failed to List Machines %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// DeleteMachines deletes a batch of machines
func DeleteMachines(ctx context.Context, resourceNames []string) *ufsds.OpResults {
	protos := make([]proto.Message, len(resourceNames))
	for i, m := range resourceNames {
		protos[i] = &ufspb.Machine{
			Name: m,
		}
	}
	return ufsds.DeleteAll(ctx, protos, newMachineEntity)
}

// DeleteMachine deletes the machine in datastore
func DeleteMachine(ctx context.Context, id string) error {
	return ufsds.Delete(ctx, &ufspb.Machine{Name: id}, newMachineEntity)
}

// BatchUpdateMachines updates machines in datastore.
//
// This is a non-atomic operation and doesnt check if the object already exists before
// update. Must be used within a Transaction where objects are checked before update.
// Will lead to partial updates if not used in a transaction.
func BatchUpdateMachines(ctx context.Context, machines []*ufspb.Machine) ([]*ufspb.Machine, error) {
	return putAllMachine(ctx, machines, true)
}

func putMachine(ctx context.Context, machine *ufspb.Machine, update bool) (*ufspb.Machine, error) {
	machine.UpdateTime = timestamppb.Now()

	// Redact ownership data
	redactMachineOwnership(ctx, machine)

	pm, err := ufsds.Put(ctx, machine, newMachineEntity, update)
	if err == nil {
		return pm.(*ufspb.Machine), err
	}
	return nil, err
}

// Updates the ownership data for an existing machine.
func putMachineOwnership(ctx context.Context, id string, ownership *ufspb.OwnershipData, update bool) (*ufspb.Machine, error) {
	machine, err := GetMachine(ctx, id)
	if err != nil {
		return machine, err
	}
	machine.Ownership = ownership
	machine.UpdateTime = timestamppb.Now()
	pm, err := ufsds.Put(ctx, machine, newMachineEntity, update)
	if err == nil {
		return pm.(*ufspb.Machine), err
	}
	return nil, err
}

// Redacts machine ownership for updates by either changing the ownership to existing values or
// for new entities setting the ownership to nil as we don't want to allow user updates to these values.
func redactMachineOwnership(ctx context.Context, machine *ufspb.Machine) {
	if machine == nil {
		return
	}
	// Redact ownership data
	existingMachine, err := GetMachine(ctx, machine.Name)
	if err == nil {
		machine.Ownership = existingMachine.Ownership
	} else {
		machine.Ownership = nil
	}
}

func putAllMachine(ctx context.Context, machines []*ufspb.Machine, update bool) ([]*ufspb.Machine, error) {
	protos := make([]proto.Message, len(machines))
	updateTime := timestamppb.Now()
	for i, machine := range machines {
		machine.UpdateTime = updateTime

		// Redact ownership data
		redactMachineOwnership(ctx, machine)

		protos[i] = machine
	}
	_, err := ufsds.PutAll(ctx, protos, newMachineEntity, update)
	if err == nil {
		return machines, err
	}
	return nil, err
}

// GetMachineIndexedFieldName returns the index name
func GetMachineIndexedFieldName(input string) (string, error) {
	var field string
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case util.SerialNumberFilterName:
		field = "serial_number"
	case util.AssetTagFilterName:
		field = "asset_tag"
	case util.KVMFilterName:
		field = "kvm_id"
	case util.RPMFilterName:
		field = "rpm_id"
	case util.ZoneFilterName:
		field = "zone"
	case util.RackFilterName:
		field = "rack"
	case util.ChromePlatformFilterName:
		field = "chrome_platform_id"
	case util.TagFilterName:
		field = "tags"
	case util.StateFilterName:
		field = "state"
	case util.KVMPortFilterName:
		field = "kvm_port"
	case util.ModelFilterName:
		field = "model"
	case util.BuildTargetFilterName:
		field = "build_target"
	case util.DeviceTypeFilterName:
		field = "device_type"
	case util.PhaseFilterName:
		field = "phase"
	case util.GPNFilterName:
		field = "gpn"
	case util.ManufacturerFilterName:
		field = "manufacturer"
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid field name %s - field name for machine are serialnumber/kvm/kvmport/rpm/zone/rack/platform/tag/state/model/buildtarget(target)/devicetype/phase/gpn/man", input)
	}
	return field, nil
}

// validateListAssetFilters validates that the given filter map is valid
func validateListMachineFilters(filterMap map[string][]any) error {
	for field := range filterMap {
		if field == "realm" {
			return errors.Reason("cannot filter on %s", field).Err()
		}
	}
	return nil
}
