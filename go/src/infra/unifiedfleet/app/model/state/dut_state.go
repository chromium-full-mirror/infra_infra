// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package state

import (
	"context"
	"math/rand"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	"go.chromium.org/infra/unifiedfleet/app/config"
	ufsds "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// DutStateKind is the datastore entity kind of dut state.
//
// Dut state is only specific to OS devices for now.
const DutStateKind string = "DutState"

// DutStateEntity is a datastore entity that tracks dut state.
type DutStateEntity struct {
	_kind string                `gae:"$kind,DutState"`
	Extra datastore.PropertyMap `gae:",extra"`
	// refer to the device id
	ID       string `gae:"$id"`
	Hostname string `gae:"hostname"`
	// lab.DutState cannot be directly used as it contains pointer (timestamp).
	DutState []byte `gae:",noindex"`
	Realm    string `gae:"realm"`
}

// GetProto returns the unmarshaled dut state.
func (e *DutStateEntity) GetProto() (proto.Message, error) {
	var p chromeosLab.DutState
	if err := proto.Unmarshal(e.DutState, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate returns whether a DutStateEntity is valid
func (e *DutStateEntity) Validate() error {
	return nil
}

func (e *DutStateEntity) GetRealm() string {
	return e.Realm
}

func newDutStateEntityRealm(ctx context.Context, pm proto.Message) (ufsds.RealmEntity, error) {
	p := pm.(*chromeosLab.DutState)
	if p.GetId().GetValue() == "" {
		return nil, errors.Reason("Empty ID in Dut state").Err()
	}
	s, err := proto.Marshal(p)
	if err != nil {
		return nil, errors.Annotate(err, "fail to marshal DutState %s", p).Err()
	}
	return &DutStateEntity{
		ID:       p.GetId().GetValue(),
		Hostname: p.GetHostname(),
		DutState: s,
		Realm:    p.GetRealm(),
	}, nil
}

func newDutStateEntity(ctx context.Context, pm proto.Message) (ufsds.FleetEntity, error) {
	return newDutStateEntityRealm(ctx, pm)
}

// GetDutState returns dut state for the given id from datastore.
func GetDutState(ctx context.Context, id string) (*chromeosLab.DutState, error) {
	pm, err := ufsds.Get(ctx, &chromeosLab.DutState{Id: &chromeosLab.ChromeOSDeviceID{Value: id}}, newDutStateEntity)
	if err == nil {
		return pm.(*chromeosLab.DutState), err
	}
	return nil, err
}

// GetDutStateACL returns the DutState for the requested id if the user
// has permissions to do so.
func GetDutStateACL(ctx context.Context, id string) (*chromeosLab.DutState, error) {
	// TODO(b/285605478): Remove the cutoff logic once we migrate to using
	// ACLs everywhere
	cutoff := config.Get(ctx).GetExperimentalAPI().GetGetDutStateACL()
	// If cutoff is set attempt to divert the traffic to new API
	if cutoff != 0 {
		// Roll the dice to determine which one to use
		roll := rand.Uint32() % 100
		cutoff := cutoff % 100
		if roll <= cutoff {
			logging.Infof(ctx, "GetDutStateACL --- Running in experimental API")
			return getDutStateACL(ctx, id)
		}
	}

	return GetDutState(ctx, id)
}

// getDutStateACL returns the DutState for the requested id if the user has
// permissions to do so.
func getDutStateACL(ctx context.Context, id string) (*chromeosLab.DutState, error) {
	pm, err := ufsds.GetACL(ctx, &chromeosLab.DutState{Id: &chromeosLab.ChromeOSDeviceID{Value: id}}, newDutStateEntityRealm, util.ConfigurationsGet)
	if err != nil {
		return nil, err
	}
	return pm.(*chromeosLab.DutState), nil
}

// UpdateDutStates updates dut states in datastore.
func UpdateDutStates(ctx context.Context, dutStates []*chromeosLab.DutState) ([]*chromeosLab.DutState, error) {
	protos := make([]proto.Message, len(dutStates))
	utime := timestamppb.Now()
	for i, ds := range dutStates {
		ds.UpdateTime = utime
		protos[i] = ds
	}
	_, err := ufsds.PutAll(ctx, protos, newDutStateEntity, true)
	if err == nil {
		return dutStates, err
	}
	return nil, err
}

func queryAllDutStates(ctx context.Context) ([]ufsds.FleetEntity, error) {
	var entities []*DutStateEntity
	q := datastore.NewQuery(DutStateKind)
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		return nil, err
	}
	fe := make([]ufsds.FleetEntity, len(entities))
	for i, e := range entities {
		fe[i] = e
	}
	return fe, nil
}

// QueryDutStateByPropertyNames queries DutState Entity in the datastore.
// If keysOnly is true, then only key field is populated in returned DutStates.
func QueryDutStateByPropertyNames(ctx context.Context, propertyMap map[string]string, keysOnly bool) ([]*chromeosLab.DutState, error) {
	q := datastore.NewQuery(DutStateKind).KeysOnly(keysOnly).FirestoreMode(true)
	var entities []*DutStateEntity
	for propertyName, id := range propertyMap {
		q = q.Eq(propertyName, id)
	}
	if err := datastore.GetAll(ctx, q, &entities); err != nil {
		logging.Errorf(ctx, "Failed to query from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if len(entities) == 0 {
		logging.Infof(ctx, "No DutStates found for the query: %s", q.String())
		return nil, nil
	}
	dutStates := make([]*chromeosLab.DutState, 0, len(entities))
	for _, entity := range entities {
		if keysOnly {
			dutState := &chromeosLab.DutState{
				Id: &chromeosLab.ChromeOSDeviceID{Value: entity.ID},
			}
			dutStates = append(dutStates, dutState)
		} else {
			pm, perr := entity.GetProto()
			if perr != nil {
				logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
				continue
			}
			dutStates = append(dutStates, pm.(*chromeosLab.DutState))
		}
	}
	return dutStates, nil
}

// ListDutStates lists the DutStates.
//
// Does a query over DutState entities. Returns up to pageSize entities, plus non-nil cursor (if
// there are more results). pageSize must be positive.
func ListDutStates(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*chromeosLab.DutState, nextPageToken string, err error) {
	q, err := ufsds.ListQuery(ctx, DutStateKind, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	var nextCur datastore.Cursor
	err = datastore.Run(ctx, q, func(ent *DutStateEntity, cb datastore.CursorCB) error {
		if keysOnly {
			DutState := &chromeosLab.DutState{
				Id: &chromeosLab.ChromeOSDeviceID{Value: ent.ID},
			}
			res = append(res, DutState)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*chromeosLab.DutState))
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
		logging.Errorf(ctx, "Failed to list DutStates %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	return
}

func ListDutStatesACL(ctx context.Context, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (res []*chromeosLab.DutState, nextPageToken string, err error) {
	err = validateDutStateFilters(filterMap)
	if err != nil {
		return nil, "", errors.Annotate(err, "ListDutStatesACL --- cannot validate query").Err()
	}
	userRealms, err := auth.QueryRealms(ctx, util.ConfigurationsList, "", nil)
	if err != nil {
		return nil, "", err
	}

	q, err := ufsds.ListQuery(ctx, DutStateKind, pageSize, "", filterMap, keysOnly)
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
	err = datastore.RunMulti(ctx, queries, func(ent *DutStateEntity, cb datastore.CursorCB) error {
		if keysOnly {
			DutState := &chromeosLab.DutState{
				Id: &chromeosLab.ChromeOSDeviceID{Value: ent.ID},
			}
			res = append(res, DutState)
		} else {
			pm, err := ent.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Failed to UnMarshal: %s", err)
				return nil
			}
			res = append(res, pm.(*chromeosLab.DutState))
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
		logging.Errorf(ctx, "Failed to list DutStates %s", err)
		return nil, "", status.Errorf(codes.Internal, ufsds.InternalError)
	}
	if nextCur != nil {
		nextPageToken = nextCur.String()
	}
	logging.Debugf(ctx, "ListDutStatesACL --- filtering for %v", userRealms)

	return
}

func validateDutStateFilters(filterMap map[string][]any) error {
	for field := range filterMap {
		if field == "realm" {
			return errors.Reason("cannot filter on %s", field).Err()
		}
	}

	return nil
}

// The below functions (GetAllDutStates, DeleteDutStates)
// cannot be used by anyone outside the fleet team.
// Although we believe these need to be removed that is a task for another day
// TODO: remove usage of these function or just throw an error as an output

// GetAllDutStates returns all dut states in datastore.
func GetAllDutStates(ctx context.Context) (*ufsds.OpResults, error) {
	return ufsds.GetAll(ctx, queryAllDutStates)
}

// DeleteDutStates deletes a batch of dut states
func DeleteDutStates(ctx context.Context, resourceNames []string) *ufsds.OpResults {
	protos := make([]proto.Message, len(resourceNames))
	for i, m := range resourceNames {
		protos[i] = &chromeosLab.DutState{
			Id: &chromeosLab.ChromeOSDeviceID{
				Value: m,
			},
		}
	}
	return ufsds.DeleteAll(ctx, protos, newDutStateEntity)
}
