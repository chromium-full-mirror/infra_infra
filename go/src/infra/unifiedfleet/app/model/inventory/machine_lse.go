// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inventory

import (
	"context"
	"math/rand"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
	"infra/unifiedfleet/app/config"
	ufsds "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/util"
)

// MachineLSEKind is the datastore entity kind MachineLSE.
const MachineLSEKind string = "MachineLSE"

// MachineLSEEntity is a datastore entity that tracks MachineLSE.
type MachineLSEEntity struct {
	_kind                 string                `gae:"$kind,MachineLSE"`
	Extra                 datastore.PropertyMap `gae:",extra"`
	ID                    string                `gae:"$id"`
	MachineIDs            []string              `gae:"machine_ids"`
	MachineLSEProtoTypeID string                `gae:"machinelse_prototype_id"`
	SwitchID              string                `gae:"switch_id"`
	RPMID                 string                `gae:"rpm_id"`
	RPMPort               string                `gae:"rpm_port"`
	RPMType               string                `gae:"rpm_type"`
	VlanID                string                `gae:"vlan_id"`
	ServoID               string                `gae:"servo_id"`
	ServoType             string                `gae:"servo_type"`
	Rack                  string                `gae:"rack"`
	Lab                   string                `gae:"lab"` // deprecated
	Zone                  string                `gae:"zone"`
	Manufacturer          string                `gae:"manufacturer"`
	Tags                  []string              `gae:"tags"`
	State                 string                `gae:"state"`
	OS                    []string              `gae:"os"`
	VirtualDatacenter     string                `gae:"virtualdatacenter"`
	Nic                   string                `gae:"nic"`
	Pools                 []string              `gae:"pools"`
	AssociatedHostname    string                `gae:"associated_hostname"`
	AssociatedHostPort    string                `gae:"associated_host_port"`
	Pool                  string                `gae:"pool"`
	SwarmingServer        string                `gae:"swarming_server"`
	Customer              string                `gae:"customer"`
	SecurityLevel         string                `gae:"security_level"`
	MibaRealm             string                `gae:"miba_realm,noindex"` // deprecated
	LogicalZone           string                `gae:"logical_zone"`
	Realm                 string                `gae:"realm"`
	Hive                  string                `gae:"hive"`
	// ufspb.MachineLSE cannot be directly used as it contains pointer.
	MachineLSE []byte `gae:",noindex"`
}

// GetProto returns the unmarshaled MachineLSE.
func (e *MachineLSEEntity) GetProto() (proto.Message, error) {
	var p ufspb.MachineLSE
	if err := proto.Unmarshal(e.MachineLSE, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether a MachineLSEEntity is valid
func (e *MachineLSEEntity) Validate() error {
	return nil
}

// GetRealm returns the realm for the MachineLSE.
func (e *MachineLSEEntity) GetRealm() string {
	return e.Realm
}

func newMachineLSERealmEntity(ctx context.Context, pm proto.Message) (ufsds.RealmEntity, error) {
	mlse, err := newMachineLSEEntity(ctx, pm)
	if err != nil {
		return nil, err
	}
	return mlse.(*MachineLSEEntity), nil
}

func newMachineLSEEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	p := pm.(*ufspb.MachineLSE)
	if p.GetName() == "" {
		return nil, errors.Reason("Empty MachineLSE ID").Err()
	}
	machineLSE, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal MachineLSE %s", p).Err()
	}
	servo := p.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetServo()
	servoID := ufsds.GetServoID(servo.GetServoHostname(), servo.GetServoPort())
	var rpmID string
	var rpmPort string
	var rpmType chromeosLab.OSRPM_Type
	var pools []string
	var hive string
	if p.GetChromeosMachineLse().GetDeviceLse().GetDut() != nil {
		rpmID = p.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().GetPowerunitName()
		rpmPort = p.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().GetPowerunitOutlet()
		rpmType = p.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetRpm().GetPowerunitType()
		pools = p.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools()
		hive = p.GetChromeosMachineLse().GetDeviceLse().GetDut().GetHive()
	} else if p.GetChromeosMachineLse().GetDeviceLse().GetLabstation() != nil {
		rpmID = p.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().GetPowerunitOutlet()
		rpmPort = p.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().GetPowerunitOutlet()
		rpmType = p.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetRpm().GetPowerunitType()
		pools = p.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetPools()
		hive = p.GetChromeosMachineLse().GetDeviceLse().GetLabstation().GetHive()
	}

	var os []string
	if p.GetChromeBrowserMachineLse() != nil {
		os = ufsds.GetOSIndex(p.GetChromeBrowserMachineLse().GetOsVersion().GetValue())
	} else if p.GetAttachedDeviceLse() != nil {
		os = ufsds.GetOSIndex(p.GetAttachedDeviceLse().GetOsVersion().GetValue())
	}

	// Ownership config for browser bots
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

	return &MachineLSEEntity{
		ID:                    p.GetName(),
		MachineIDs:            p.GetMachines(),
		MachineLSEProtoTypeID: p.GetMachineLsePrototype(),
		SwitchID:              p.GetChromeosMachineLse().GetDeviceLse().GetNetworkDeviceInterface().GetSwitch(),
		RPMID:                 rpmID,
		RPMPort:               rpmPort,
		RPMType:               rpmType.String(),
		VlanID:                p.GetVlan(),
		ServoID:               servoID,
		ServoType:             servo.GetServoType(),
		Rack:                  p.GetRack(),
		Zone:                  p.GetZone(),
		Manufacturer:          strings.ToLower(p.GetManufacturer()),
		State:                 p.GetResourceState().String(),
		OS:                    os,
		VirtualDatacenter:     p.GetChromeBrowserMachineLse().GetVirtualDatacenter(),
		Nic:                   p.GetNic(),
		Tags:                  p.GetTags(),
		Pools:                 pools,
		AssociatedHostname:    p.GetAttachedDeviceLse().GetAssociatedHostname(),
		AssociatedHostPort:    p.GetAttachedDeviceLse().GetAssociatedHostPort(),
		Pool:                  poolName,
		SwarmingServer:        swarmingInstance,
		Customer:              customer,
		SecurityLevel:         securityLevel,
		MachineLSE:            machineLSE,
		LogicalZone:           p.GetLogicalZone().String(),
		Realm:                 p.GetRealm(),
		Hive:                  hive,
	}, nil
}

// QueryMachineLSEByPropertyName queries MachineLSE Entity in the datastore
// If keysOnly is true, then only key field is populated in returned machinelses
func QueryMachineLSEByPropertyName(ctx context.Context, propertyName, id string, keysOnly bool) ([]*ufspb.MachineLSE, error) {
	return QueryMachineLSEByPropertyNames(ctx, map[string]string{propertyName: id}, keysOnly)
}

// QueryMachineLSEByPropertyNames queries MachineLSE Entity in the datastore
// If keysOnly is true, then only key field is populated in returned machinelses
func QueryMachineLSEByPropertyNames(ctx context.Context, propertyMap map[string]string, keysOnly bool) ([]*ufspb.MachineLSE, error) {
	q := datastore.NewQuery(MachineLSEKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*MachineLSEEntity
	for propertyName, id := range propertyMap {
		q = q.Eq(propertyName, id)
	}
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Infof(ctx, "No machineLSEs found for the query: %s", q.String())
		return nil, nil
	}
	machineLSEs := make([]*ufspb.MachineLSE, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			machineLSE := &ufspb.MachineLSE{
				Name: entity.ID,
			}
			machineLSEs = append(machineLSEs, machineLSE)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			machineLSEs = append(machineLSEs, pm.(*ufspb.MachineLSE))
		}
	}
	return machineLSEs, nil
}

// RangedQueryMachineLSEByPropertyName queries MachineLSE entity in the datastore. The query run here is given by
// `SELECT * FROM MachineLSE WHERE propertyName >= gtVal AND propertyName <= ltVal`
func RangedQueryMachineLSEByPropertyName(ctx context.Context, propertyName string, gtVal, ltVal interface{}, keysOnly bool) ([]*ufspb.MachineLSE, error) {
	return RangedQueryMachineLSEByPropertyNames(ctx, map[string][]interface{}{propertyName: {gtVal, ltVal}}, keysOnly)
}

// RangedQueryMachineLSEByPropertyNames queries MachineLSE Entity in the datastore
// The propertyMap contains a slice of size 2 containing [greaterthanval, lesserthanval],
// if slice has only one string it's used as greaterthanval, use ["", lesserthanval] for
// less than queries. For any other size of propertyMap error is thrown.
// If keysOnly is true, then only key field is populated in returned machinelses
// The query run here is given by:
// `SELECT * FROM MachineLSE WHERE (propertyKey >= propertyVal[0] AND propertyKey <= propertyVal[1]) AND (pr...)`

func RangedQueryMachineLSEByPropertyNames(ctx context.Context, propertyMap map[string][]interface{}, keysOnly bool) ([]*ufspb.MachineLSE, error) {
	q := datastore.NewQuery(MachineLSEKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*MachineLSEEntity
	for propertyName, val := range propertyMap {
		var gt, lt interface{}
		if len(val) == 0 || len(val) > 2 {
			return nil, status.Errorf(codes.Internal, "Cannot determine range for %s [%v]", propertyName, val)
		}
		gt = val[0]
		if len(val) == 2 {
			lt = val[1]
		}

		if gt != nil {
			q = q.Gte(propertyName, gt)
		}
		if lt != nil {
			q = q.Lte(propertyName, lt)
		}
	}
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Infof(ctx, "No machineLSEs found for the query: %s", q.String())
		return nil, nil
	}
	machineLSEs := make([]*ufspb.MachineLSE, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			machineLSE := &ufspb.MachineLSE{
				Name: entity.ID,
			}
			machineLSEs = append(machineLSEs, machineLSE)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			machineLSEs = append(machineLSEs, pm.(*ufspb.MachineLSE))
		}
	}
	return machineLSEs, nil
}

// CreateMachineLSE creates a new machineLSE in datastore.
func CreateMachineLSE(ctx context.Context, machineLSE *ufspb.MachineLSE) (*ufspb.MachineLSE, error) {
	return putMachineLSE(ctx, machineLSE, false)
}

// UpdateMachineLSE updates machineLSE in datastore.
func UpdateMachineLSE(ctx context.Context, machineLSE *ufspb.MachineLSE) (*ufspb.MachineLSE, error) {
	return putMachineLSE(ctx, machineLSE, true)
}

// UpdateMachineLSEOwnership updates machineLSE ownership in datastore.
func UpdateMachineLSEOwnership(ctx context.Context, id string, ownership *ufspb.OwnershipData) (*ufspb.MachineLSE, error) {
	return putMachineLSEOwnership(ctx, id, ownership, true)
}

// GetMachineLSE returns machine for the given id from datastore.
func GetMachineLSE(ctx context.Context, id string) (*ufspb.MachineLSE, error) {
	pm, err := ufsds.Get(ctx, &ufspb.MachineLSE{Name: id}, newMachineLSEEntity)
	if err == nil {
		return pm.(*ufspb.MachineLSE), err
	}
	return nil, err
}

// GetMachineLSEACL returns the machineLSE for the requested id if the user
// has permissions to do so.
func GetMachineLSEACL(ctx context.Context, id string) (*ufspb.MachineLSE, error) {
	// TODO(b/285603337): Remove the cutoff logic once we migrate to using
	// ACLs everywhere
	cutoff := config.Get(ctx).GetExperimentalAPI().GetGetMachineLSEACL()
	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "GetAsset --- Running in experimental API")
			return getMachineLSEACL(ctx, id)
		}
	}

	return GetMachineLSE(ctx, id)
}

// getMachineLSEACL returns the machineLSE for the requested id if the user has
// permissions to do so.
func getMachineLSEACL(ctx context.Context, id string) (*ufspb.MachineLSE, error) {
	pm, err := ufsds.GetACL(ctx, &ufspb.MachineLSE{Name: id}, newMachineLSERealmEntity, util.InventoriesGet)
	if err != nil {
		return nil, err
	}
	return pm.(*ufspb.MachineLSE), nil
}

func getLSEID(pm proto.Message) string {
	p := pm.(*ufspb.MachineLSE)
	return p.GetName()
}

// BatchGetMachineLSEs returns a batch of machine lses from datastore.
func BatchGetMachineLSEs(ctx context.Context, ids []string) ([]*ufspb.MachineLSE, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.MachineLSE{Name: n}
	}
	pms, err := ufsds.BatchGet(ctx, protos, newMachineLSEEntity, getLSEID)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.MachineLSE, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.MachineLSE)
	}
	return res, nil
}

// ListMachineLSEs lists the machine lses
// Does a query over MachineLSE entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListMachineLSEs(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []*ufspb.MachineLSE, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, MachineLSEKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListQuery(ctx, q, pageSize, pageToken, keysOnly)
}

// ListMachineLSEsACL lists the machine lses that user has access to
// Does a query over MachineLSE entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListMachineLSEsACL(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]interface{}, keysOnly bool) (res []*ufspb.MachineLSE, nextPageToken string, err error) {
	err = validateListMachineLSEFilters(filterMap)
	if err != nil {
		return nil, "", errors.Annotate(err, "ListMachineLSEsACL -- cannot validate query").Err()
	}

	userRealms, err := auth.QueryRealms(ctx, util.InventoriesList, "", nil)
	if err != nil {
		return nil, "", err
	}

	q, err := ufsds.ListQuery(ctx, MachineLSEKind, pageSize, "", filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}

	// Create a list of queries each checking for a realm assignment
	queries := ufsds.AssignRealms(q, userRealms)

	return runListQueries(ctx, queries, pageSize, pageToken, keysOnly)
}

// ListMachineLSEsByIdPrefixSearch lists the machineLSEs
// Does a query over MachineLSE entities using Name/ID prefix. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). PageSize must be positive.
func ListMachineLSEsByIdPrefixSearch(ctx context.Context, pageSize int32, pageToken string, prefix string, keysOnly bool) (res []*ufspb.MachineLSE, nextPageToken string, err error) {
	q, err := ufsds.ListQueryIdPrefixSearch(ctx, MachineLSEKind, pageSize, pageToken, prefix, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return runListQuery(ctx, q, pageSize, pageToken, keysOnly)
}

func runListQuery(ctx context.Context, query *datastore.Query, pageSize int32, pageToken string, keysOnly bool) (res []*ufspb.MachineLSE, nextPageToken string, err error) {
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, query, func(ent *MachineLSEEntity, cb datastore.CursorCB) error {
		if keysOnly {
			res = append(res, &ufspb.MachineLSE{
				Name: ent.ID,
			})
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			machineLSE := pm.(*ufspb.MachineLSE)
			res = append(res, machineLSE)
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
		logging.Errorf(ctx, "Failed to List MachineLSEs %s", err)
		return nil, "", status.Errorf(codes.Internal, err.Error())
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

func runListQueries(ctx context.Context, queries []*datastore.Query, pageSize int32, pageToken string, keysOnly bool) (res []*ufspb.MachineLSE, nextPageToken string, err error) {
	if pageToken != "" {
		queries, err = datastore.ApplyCursorString(ctx, queries, pageToken)
		if err != nil {
			return nil, "", err
		}
	}
	var nextCur datastore.Cursor
	err = datastore.RunMulti(ctx, queries, func(ent *MachineLSEEntity, cb datastore.CursorCB) error {
		if keysOnly {
			res = append(res, &ufspb.MachineLSE{
				Name: ent.ID,
			})
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			machineLSE := pm.(*ufspb.MachineLSE)
			res = append(res, machineLSE)
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
		logging.Errorf(ctx, "Failed to List MachineLSEs %s", err)
		return nil, "", status.Errorf(codes.Internal, err.Error())
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// ListFreeMachineLSEs lists the machine lses with vm capacity
func ListFreeMachineLSEs(ctx context.Context, requiredSize int32, filterMap map[string][]interface{}, capacityMap map[string]int) (res []*ufspb.MachineLSE, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, MachineLSEKind, -1, "", filterMap, false)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *MachineLSEEntity, cb datastore.CursorCB) error {
		pm, err := ent.GetProto()
		if err != nil {
			logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
			return nil
		}
		machineLSE := pm.(*ufspb.MachineLSE)
		if machineLSE.GetChromeBrowserMachineLse().GetVmCapacity() > int32(capacityMap[machineLSE.GetName()]) {
			res = append(res, machineLSE)
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
		logging.Errorf(ctx, "Failed to List MachineLSEs %s", err)
		return nil, "", status.Errorf(codes.Internal, err.Error())
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// ListAllMachineLSEs returns all machine lses in datastore.
func ListAllMachineLSEs(ctx context.Context, keysOnly bool) (res []*ufspb.MachineLSE, err error) {
	var entities []*MachineLSEEntity
	q := datastore.NewQuery(MachineLSEKind).KeysOnly(keysOnly).FirestoreMode(true)
	if err = datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	for _, ent := range entities {
		if keysOnly {
			res = append(res, &ufspb.MachineLSE{
				Name: ent.ID,
			})
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil, err
			}
			machineLSE := pm.(*ufspb.MachineLSE)
			res = append(res, machineLSE)
		}
	}
	return
}

// ListAllMachineLSEsNameHive return all machine lses name and hive in datastore.
func ListAllMachineLSEsNameHive(ctx context.Context) (res map[string]string, err error) {
	var entities []*MachineLSEEntity
	res = make(map[string]string)
	q := datastore.NewQuery(MachineLSEKind).Project("hive").FirestoreMode(true)
	if err = datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	for _, ent := range entities {
		res[ent.ID] = ent.Hive
	}
	return
}

// DeleteMachineLSE deletes the machineLSE in datastore
func DeleteMachineLSE(ctx context.Context, id string) error {
	return ufsds.Delete(ctx, &ufspb.MachineLSE{Name: id}, newMachineLSEEntity)
}

// BatchUpdateMachineLSEs updates machineLSEs in datastore.
// This is a non-atomic operation and doesnt check if the object already exists before
// update. Must be used within a Transaction where objects are checked before update.
// Will lead to partial updates if not used in a transaction.
func BatchUpdateMachineLSEs(ctx context.Context, machineLSEs []*ufspb.MachineLSE) ([]*ufspb.MachineLSE, error) {
	return putAllMachineLSE(ctx, machineLSEs, true)
}

func putMachineLSE(ctx context.Context, machineLSE *ufspb.MachineLSE, update bool) (*ufspb.MachineLSE, error) {
	machineLSE.UpdateTime = ptypes.TimestampNow()

	// Redact ownership data
	redactMachineLSEOwnership(ctx, machineLSE)

	pm, err := ufsds.Put(ctx, machineLSE, newMachineLSEEntity, update)
	if err != nil {
		return nil, errors.Annotate(err, "put machine LSE").Err()
	}
	return pm.(*ufspb.MachineLSE), err
}

func putAllMachineLSE(ctx context.Context, machineLSEs []*ufspb.MachineLSE, update bool) ([]*ufspb.MachineLSE, error) {
	protos := make([]proto.Message, len(machineLSEs))
	updateTime := ptypes.TimestampNow()
	for i, machineLSE := range machineLSEs {
		machineLSE.UpdateTime = updateTime

		// Redact ownership data
		redactMachineLSEOwnership(ctx, machineLSE)

		protos[i] = machineLSE
	}
	_, err := ufsds.PutAll(ctx, protos, newMachineLSEEntity, update)
	if err == nil {
		return machineLSEs, err
	}
	return nil, err
}

// Updates the ownership data for an existing machineLSE.
func putMachineLSEOwnership(ctx context.Context, id string, ownership *ufspb.OwnershipData, update bool) (*ufspb.MachineLSE, error) {
	machineLSE, err := GetMachineLSE(ctx, id)
	if err != nil {
		return machineLSE, err
	}
	machineLSE.Ownership = ownership
	machineLSE.UpdateTime = ptypes.TimestampNow()
	pm, err := ufsds.Put(ctx, machineLSE, newMachineLSEEntity, update)
	if err == nil {
		return pm.(*ufspb.MachineLSE), err
	}
	return nil, err
}

// Redacts machineLSE ownership for updates by either changing the ownership to existing values or
// for new entities setting the ownership to nil as we don't want to allow user updates to these values.
func redactMachineLSEOwnership(ctx context.Context, machineLSE *ufspb.MachineLSE) {
	if machineLSE == nil {
		return
	}

	// Redact ownership data
	existingMachineLSE, err := GetMachineLSE(ctx, machineLSE.Name)
	if err == nil {
		machineLSE.Ownership = existingMachineLSE.Ownership
	} else {
		machineLSE.Ownership = nil
	}
}

func queryAllMachineLSE(ctx context.Context) ([]ufsds.FleetEntity, error) {
	var entities []*MachineLSEEntity
	q := datastore.NewQuery(MachineLSEKind)
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	fe := make([]ufsds.FleetEntity, len(entities))
	for i, e := range entities {
		fe[i] = e
	}
	return fe, nil
}

// GetAllMachineLSEs returns all machine lses in datastore.
func GetAllMachineLSEs(ctx context.Context) (*ufsds.OpResults, error) {
	return ufsds.GetAll(ctx, queryAllMachineLSE)
}

// DeleteMachineLSEs deletes a batch of machine LSEs
func DeleteMachineLSEs(ctx context.Context, resourceNames []string) *ufsds.OpResults {
	protos := make([]proto.Message, len(resourceNames))
	for i, m := range resourceNames {
		protos[i] = &ufspb.MachineLSE{
			Name: m,
		}
	}
	return ufsds.DeleteAll(ctx, protos, newMachineLSEEntity)
}

// GetMachineLSEIndexedFieldName returns the index name
func GetMachineLSEIndexedFieldName(input string) (string, error) {
	var field string
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case util.SwitchFilterName:
		field = "switch_id"
	case util.RPMFilterName:
		field = "rpm_id"
	case util.RPMPortFilterName:
		field = "rpm_port"
	case util.RPMTypeFilterName:
		field = "rpm_type"
	case util.VlanFilterName:
		field = "vlan_id"
	case util.ServoFilterName:
		field = "servo_id"
	case util.ServoTypeFilterName:
		field = "servo_type"
	case util.ZoneFilterName:
		field = "zone"
	case util.RackFilterName:
		field = "rack"
	case util.MachineFilterName:
		field = "machine_ids"
	case util.MachinePrototypeFilterName:
		field = "machinelse_prototype_id"
	case util.ManufacturerFilterName:
		field = "manufacturer"
	case util.FreeVMFilterName:
		field = "free"
	case util.TagFilterName:
		field = "tags"
	case util.StateFilterName:
		field = "state"
	case util.OSFilterName:
		field = "os"
	case util.VirtualDatacenterFilterName:
		field = "virtualdatacenter"
	case util.NicFilterName:
		field = "nic"
	case util.PoolsFilterName:
		field = "pools"
	case util.LogicalZoneFilterName:
		field = "logical_zone"
	case util.HiveFilterName:
		field = "hive"
	default:
		return "", status.Errorf(codes.InvalidArgument, "Invalid field name %s - field name for host are nic/machine/machineprototype/rpm/rpmport/rpmtype/vlan/servo/servotype/zone/rack/switch/man/free/tag/state/os/vdc(virtualdatacenter)/pools/logicalzone/hive", input)
	}
	return field, nil
}

// validateListMachineLSEFilters validates that the given filter map is valid
func validateListMachineLSEFilters(filterMap map[string][]interface{}) error {
	for field := range filterMap {
		switch field {
		case "zone":
		case "rack":
		case "switch_id":
		case "rpm_id":
		case "rpm_port":
		case "rpm_type":
		case "vlan_id":
		case "servo_id":
		case "servo_type":
		case "machine_ids":
		case "machinelse_prototype_id":
		case "manufacturer":
		case "tags":
		case "state":
		case "os":
		case "virtualdatacenter":
		case "nic":
		case "pools":
		case "logical_zone":
		case "hive":
			continue
		default:
			return errors.Reason("Cannot filter on %s", field).Err()
		}
	}
	return nil
}
