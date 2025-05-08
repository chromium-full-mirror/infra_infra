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

// RackKind is the datastore entity kind Rack.
const RackKind string = "Rack"

// RackEntity is a datastore entity that tracks Rack.
type RackEntity struct {
	_kind     string                `gae:"$kind,Rack"`
	Extra     datastore.PropertyMap `gae:",extra"`
	ID        string                `gae:"$id"`
	Bbnum     int32                 `gae:"bbnum"`
	SwitchIDs []string              `gae:"switch_ids"` // deprecated. Do not use.
	KVMIDs    []string              `gae:"kvm_ids"`    // deprecated. Do not use.
	RPMIDs    []string              `gae:"rpm_ids"`    // deprecated. Do not use.
	Lab       string                `gae:"lab"`        // deprecated
	Zone      string                `gae:"zone"`
	Tags      []string              `gae:"tags"`
	State     string                `gae:"state"`
	Realm     string                `gae:"realm"`
	// ufspb.Rack cannot be directly used as it contains pointer.
	Rack []byte `gae:",noindex"`
}

// GetProto returns the unmarshaled Rack.
func (e *RackEntity) GetProto() (proto.Message, error) {
	var p ufspb.Rack
	if err := proto.Unmarshal(e.Rack, &p); err != nil {
		return nil, err
	}
	// Realm is read only and not guaranteed to be in the proto bytes.
	p.Realm = e.Realm
	return &p, nil
}

// Validate returns whether a RackEntity is valid.
func (e *RackEntity) Validate() error {
	return nil
}

func (e *RackEntity) GetRealm() string {
	return e.Realm
}

func newRackEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	p := pm.(*ufspb.Rack)
	if p.GetName() == "" {
		return nil, errors.Reason("Empty Rack ID").Err()
	}
	// Assign the realm to the proto.
	p.Realm = util.ToUFSRealm(p.GetLocation().GetZone().String())

	rack, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal Rack %s", p).Err()
	}
	return &RackEntity{
		ID:    p.GetName(),
		Bbnum: p.GetBbnum(),
		Zone:  p.GetLocation().GetZone().String(),
		Tags:  p.GetTags(),
		Rack:  rack,
		State: p.GetResourceState().String(),
		Realm: p.GetRealm(),
	}, nil
}

func newRackRealmEntity(ctx context.Context, pm proto.Message) (ufsds.RealmEntity, error) {
	r, err := newRackEntity(ctx, pm)
	if err != nil {
		return nil, err
	}
	return r.(*RackEntity), nil
}

// QueryRackByPropertyName queries Rack Entity in the datastore
//
// If keysOnly is true, then only key field is populated in returned racks.
// Note that this is not ACLed and should not be used to return results
// directly to users without ACLs being checked somewhere upstream.
func QueryRackByPropertyName(ctx context.Context, propertyName string, id any, keysOnly bool) ([]*ufspb.Rack, error) {
	q := datastore.NewQuery(RackKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*RackEntity
	if err := datastore.GetAll(ctx, q.Eq(propertyName, id), &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Infof(ctx, "No racks found for the query: %s", id)
		return nil, nil
	}
	racks := make([]*ufspb.Rack, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			rack := &ufspb.Rack{
				Name: entity.ID,
			}
			racks = append(racks, rack)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			racks = append(racks, pm.(*ufspb.Rack))
		}
	}
	return racks, nil
}

// CreateRack creates a new rack in datastore.
func CreateRack(ctx context.Context, rack *ufspb.Rack) (*ufspb.Rack, error) {
	return putRack(ctx, rack, false)
}

// UpdateRack updates rack in datastore.
func UpdateRack(ctx context.Context, rack *ufspb.Rack) (*ufspb.Rack, error) {
	return putRack(ctx, rack, true)
}

// GetRack returns rack for the given id from datastore.
func GetRack(ctx context.Context, id string) (*ufspb.Rack, error) {
	pm, err := ufsds.Get(ctx, &ufspb.Rack{Name: id}, newRackEntity)
	if err == nil {
		return pm.(*ufspb.Rack), err
	}
	return nil, err
}

// GetRackACL returns a rack for the given id from datastore if the context
// contains a user who has registration.get perms in the rack's realm.
//
// Temporarily may return a result without checking realm permissions if the
// service is not configured to always check ACLs.
func GetRackACL(ctx context.Context, id string) (*ufspb.Rack, error) {
	// TODO(b/285605480): Clean up cutoff once read ACLs fully rolled out.
	cutoff := config.Get(ctx).GetExperimentalAPI().GetGetRackACL()
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "GetRack --- Running in experimental API")
			return getRackACL(ctx, id)
		}
	}

	return GetRack(ctx, id)
}

// getRackACL returns a rack for the given id from datastore if the context
// contains a user who has registration.get perms in the rack's realm.
func getRackACL(ctx context.Context, id string) (*ufspb.Rack, error) {
	pm, err := ufsds.GetACL(ctx, &ufspb.Rack{Name: id}, newRackRealmEntity, util.RegistrationsGet)
	if err == nil {
		return pm.(*ufspb.Rack), err
	}
	return nil, err
}

func getRackID(pm proto.Message) string {
	p := pm.(*ufspb.Rack)
	return p.GetName()
}

// BatchGetRacks returns a batch of racks from datastore.
func BatchGetRacks(ctx context.Context, ids []string) ([]*ufspb.Rack, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.Rack{Name: n}
	}
	pms, err := ufsds.BatchGet(ctx, protos, newRackEntity, getRackID)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.Rack, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.Rack)
	}
	return res, nil
}

// BatchGetRacksACL returns a batch of racks after potentially checking for
// ACLs in the relevant realms.
func BatchGetRacksACL(ctx context.Context, ids []string) ([]*ufspb.Rack, error) {
	cutoff := config.Get(ctx).GetExperimentalAPI().GetGetRackACL()
	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "GetRack --- Running in experimental API")
			return batchGetRacksACL(ctx, ids)
		}
	}

	return BatchGetRacks(ctx, ids)
}

// batchGetRacks returns a batch of racks from datastore after checking ACLs
func batchGetRacksACL(ctx context.Context, ids []string) ([]*ufspb.Rack, error) {
	protos := make([]proto.Message, len(ids))
	for i, n := range ids {
		protos[i] = &ufspb.Rack{Name: n}
	}
	pms, err := ufsds.BatchGetACL(ctx, protos, newRackRealmEntity, getRackID, util.RegistrationsGet)
	if err != nil {
		return nil, err
	}
	res := make([]*ufspb.Rack, len(pms))
	for i, pm := range pms {
		res[i] = pm.(*ufspb.Rack)
	}
	return res, nil
}

// ListRacks lists the racks
// Does a query over Rack entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListRacks(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*ufspb.Rack, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, RackKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *RackEntity, cb datastore.CursorCB) error {
		if keysOnly {
			rack := &ufspb.Rack{
				Name: ent.ID,
			}
			res = append(res, rack)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.Rack))
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
		logging.Errorf(ctx, "Failed to List Racks %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

// ListRacksACL lists the racks in realms user can access.
// Does a query over Rack entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListRacksACL(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*ufspb.Rack, nextPageToken string, err error) {
	err = validateListRackFilters(filterMap)
	if err != nil {
		return nil, "", errors.Annotate(err, "ListRacksACL --- cannot validate query").Err()
	}
	userRealms, err := auth.QueryRealms(ctx, util.RegistrationsList, "", nil)
	if err != nil {
		return nil, "", err
	}

	q, err := ufsds.ListQuery(ctx, RackKind, pageSize, "", filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}

	// Create a list of queries each checking for a realm assignment
	queries := ufsds.AssignRealms(q, userRealms)

	// Apply page token if necessary
	if pageToken != "" {
		queries, err = datastore.ApplyCursorString(ctx, queries, pageToken)
	}

	var nextCur datastore.Cursor
	err = datastore.RunMulti(ctx, queries, func(ent *RackEntity, cb datastore.CursorCB) error {
		if keysOnly {
			rack := &ufspb.Rack{
				Name: ent.ID,
			}
			res = append(res, rack)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*ufspb.Rack))
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
		logging.Errorf(ctx, "Failed to List Racks %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	logging.Debugf(ctx, "ListRacksACL --- filtering for %v", userRealms)
	return
}

// DeleteRack deletes the rack in datastore
func DeleteRack(ctx context.Context, id string) error {
	return ufsds.Delete(ctx, &ufspb.Rack{Name: id}, newRackEntity)
}

// BatchUpdateRacks updates racks in datastore.
//
// This is a non-atomic operation and doesnt check if the object already exists before
// update. Must be used within a Transaction where objects are checked before update.
// Will lead to partial updates if not used in a transaction.
func BatchUpdateRacks(ctx context.Context, racks []*ufspb.Rack) ([]*ufspb.Rack, error) {
	return putAllRack(ctx, racks, true)
}

func putRack(ctx context.Context, rack *ufspb.Rack, update bool) (*ufspb.Rack, error) {
	rack.UpdateTime = timestamppb.Now()
	pm, err := ufsds.Put(ctx, rack, newRackEntity, update)
	if err == nil {
		return pm.(*ufspb.Rack), err
	}
	return nil, err
}

func putAllRack(ctx context.Context, racks []*ufspb.Rack, update bool) ([]*ufspb.Rack, error) {
	protos := make([]proto.Message, len(racks))
	updateTime := timestamppb.Now()
	for i, rack := range racks {
		rack.UpdateTime = updateTime
		protos[i] = rack
	}
	_, err := ufsds.PutAll(ctx, protos, newRackEntity, update)
	if err == nil {
		return racks, err
	}
	return nil, err
}

func queryAllRack(ctx context.Context) ([]ufsds.FleetEntity, error) {
	var entities []*RackEntity
	q := datastore.NewQuery(RackKind)
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	fe := make([]ufsds.FleetEntity, len(entities))
	for i, e := range entities {
		fe[i] = e
	}
	return fe, nil
}

// GetAllRacks returns all racks in datastore.
func GetAllRacks(ctx context.Context) (*ufsds.OpResults, error) {
	return ufsds.GetAll(ctx, queryAllRack)
}

// DeleteRacks deletes a batch of racks
func DeleteRacks(ctx context.Context, resourceNames []string) *ufsds.OpResults {
	protos := make([]proto.Message, len(resourceNames))
	for i, m := range resourceNames {
		protos[i] = &ufspb.Rack{
			Name: m,
		}
	}
	return ufsds.DeleteAll(ctx, protos, newRackEntity)
}

// GetRackIndexedFieldName returns the index name
func GetRackIndexedFieldName(input string) (string, error) {
	var field string
	input = strings.TrimSpace(input)
	switch strings.ToLower(input) {
	case util.ZoneFilterName:
		field = "zone"
	case util.TagFilterName:
		field = "tags"
	case util.StateFilterName:
		field = "state"
	case util.BbnumFilterName:
		field = "bbnum"
	default:
		return "", status.Errorf(codes.InvalidArgument, "Invalid field name %s - field name for rack are zone/tag/state/bbnum", input)
	}
	return field, nil
}

func validateListRackFilters(filterMap map[string][]any) error {
	for field := range filterMap {
		if field == "realm" {
			return errors.Reason("cannot filter on %s", field).Err()
		}
	}

	return nil
}
