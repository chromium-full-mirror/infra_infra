// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package datastore

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/grpc/grpcutil"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/realms"

	"go.chromium.org/infra/unifiedfleet/app/util"
)

// Error messages for datastore operations
const (
	InvalidPageToken string = "Invalid Page Token."
	AlreadyExists    string = "Entity already exists."
	NotFound         string = "Entity not found"
	InternalError    string = "Internal Server Error."
	CannotDelete     string = "cannot be deleted"
	InvalidArgument  string = "Invalid argument"
	PermissionDenied string = "PermissionDenied"
)

// FleetEntity represents the interface of entity in datastore.
type FleetEntity interface {
	GetProto() (proto.Message, error)

	// Validate performs a shallow check to make sure that the record can
	// be written to datastore.
	//
	// We don't validate when information is read, but we do validate when
	// it's written.
	//
	// This function is the last line of defense preventing us from writing
	// bad data to UFS. It should ONLY be used for enforcing constraints
	// that can be defined using only this record.
	//
	// For example, hostname not being an empty string is a valid
	// condition, but hostname being unique is not because the latter
	// forces us to consider what other entities exist, which is too much
	// complexity for this layer.  (Cross-entity consistency should be part
	// of request validation here)
	//
	// Validate should be a pure function. It should not modify the record.
	Validate() error
}

// RealmEntity represents the interface of an entity with a way to associate
// with LUCI realms.
type RealmEntity interface {
	FleetEntity
	GetRealm() string
}

// NewFunc creates a new fleet entity.
type NewFunc func(context.Context, proto.Message) (FleetEntity, error)

// NewRealmEntityFunc creates a new realmed fleet entity
type NewRealmEntityFunc func(context.Context, proto.Message) (RealmEntity, error)

// QueryAllFunc queries all entities for a given table.
type QueryAllFunc func(context.Context) ([]FleetEntity, error)

// GetIDFunc gets the id of a fleet entity
type GetIDFunc func(pm proto.Message) string

// Exists checks if a list of fleet entities exist in datastore.
func Exists(ctx context.Context, entities []FleetEntity) ([]bool, error) {
	res, err := datastore.Exists(ctx, entities)
	if err != nil {
		return nil, err
	}
	return res.List(0), nil
}

// ExistsACL checks if a list of fleet entities exist in datastore and is visible
// to the user with their current permissions.
func ExistsACL(ctx context.Context, entities []RealmEntity, neededPerm realms.Permission) ([]bool, error) {
	existsArr := make([]bool, len(entities))
	dsErr := datastore.Get(ctx, entities)

	for i := range entities {
		switch {
		// If no datastore error at all, or no datastore error for the i-th
		// element, it exists and we just need to make a realm check.
		case dsErr == nil || dsErr.(errors.MultiError)[i] == nil:
			has, authErr := auth.HasPermission(ctx, neededPerm, entities[i].GetRealm(), nil)
			if authErr != nil {
				logging.Errorf(ctx, "Failed to fetch auth permissions: %s", authErr)
				return nil, status.Errorf(codes.Internal, InternalError)
			}
			if has {
				// Object exists and user has permission to view it.
				existsArr[i] = true
			}
		case datastore.IsErrNoSuchEntity(dsErr.(errors.MultiError)[i]):
			continue
		// We got a non-DNE error, which means we should just fail.
		default:
			return nil, dsErr
		}
	}

	return existsArr, nil
}

// Put either creates or updates an entity in the datastore
func Put(ctx context.Context, pm proto.Message, nf NewFunc, update bool) (proto.Message, error) {
	entity, err := nf(ctx, pm)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
		return nil, status.Errorf(codes.Internal, InternalError)
	}
	if err := entity.Validate(); err != nil {
		logging.Errorf(ctx, "Entity did not validate: %s", err)
		return nil, status.Errorf(codes.Internal, "entity did not validate: %s", err)
	}
	f := func(ctx context.Context) error {
		existsResults, err := datastore.Exists(ctx, entity)
		if err == nil {
			if !existsResults.All() && update {
				errorMsg := fmt.Sprintf("Entity not found %+v", entity)
				return status.Errorf(codes.NotFound, errorMsg)
			}
			if existsResults.All() && !update {
				return status.Errorf(codes.AlreadyExists, AlreadyExists)
			}
		} else {
			logging.Debugf(ctx, "Failed to check existence: %s", err)
		}
		if err := datastore.Put(ctx, entity); err != nil {
			logging.Errorf(ctx, "Failed to put in datastore: %s", err)
			return status.Errorf(codes.Internal, InternalError)
		}
		return nil
	}

	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, err
	}
	return pm, nil
}

// PutSingle upserts a single entity in the datastore.
//
// If you have a clean intention to create or update an entity, please use Put().
// This function doesn't need to be called in a transaction.
func PutSingle(ctx context.Context, pm proto.Message, nf NewFunc) (proto.Message, error) {
	entity, err := nf(ctx, pm)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
		return nil, status.Errorf(codes.Internal, InternalError)
	}
	if err := datastore.Put(ctx, entity); err != nil {
		logging.Errorf(ctx, "Failed to put in datastore: %s", err)
		return nil, status.Errorf(codes.Internal, InternalError)
	}
	return pm, nil
}

// PutAll Upserts entities in the datastore.
// This is a non-atomic operation and doesnt check if the object already exists before insert/update.
// Returns error even if partial insert/updates succeeds.
// Must be used within a Transaction where objects are checked for existence before update/insert.
// Using it in a Transaction will rollback the partial insert/updates and propagate correct error message.
func PutAll(ctx context.Context, pms []proto.Message, nf NewFunc, update bool) ([]proto.Message, error) {
	entities := make([]FleetEntity, 0, len(pms))
	for _, pm := range pms {
		entity, err := nf(ctx, pm)
		if err != nil {
			logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
			return nil, status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", InternalError, err.Error()))
		}
		if err := entity.Validate(); err != nil {
			logging.Errorf(ctx, "Entity did not validate: %s", err)
			return nil, status.Errorf(codes.Internal, "entity did not validate: %s", err)
		}
		entities = append(entities, entity)
	}
	if err := datastore.Put(ctx, entities); err != nil {
		logging.Errorf(ctx, "Failed to put in datastore: %s", err)
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", InternalError, err.Error()))
	}
	return pms, nil
}

// Get retrieves entity from the datastore.
func Get(ctx context.Context, pm proto.Message, nf NewFunc) (proto.Message, error) {
	entity, err := nf(ctx, pm)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
		return nil, status.Errorf(codes.Internal, "%s Failed to marshal new entity: %s", InternalError, err)
	}
	if err = datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			errorMsg := fmt.Sprintf("Entity not found %+v", entity)
			return nil, status.Errorf(codes.NotFound, errorMsg)
		}
		logging.Errorf(ctx, "Failed to get entity from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, InternalError)
	}
	pm, perr := entity.GetProto()
	if perr != nil {
		logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
		return nil, status.Errorf(codes.Internal, InternalError)
	}
	return pm, nil
}

// GetACL retrieves entity from the datastore and applies a realm check before
// returning the entity to a user.
func GetACL(ctx context.Context, pm proto.Message, nf NewRealmEntityFunc, neededPerm realms.Permission) (proto.Message, error) {
	entity, err := nf(ctx, pm)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
		return nil, status.Errorf(codes.Internal, "%s Failed to marshal new entity: %s", InternalError, err)
	}
	if err = datastore.Get(ctx, entity); err != nil {
		if datastore.IsErrNoSuchEntity(err) {
			errorMsg := fmt.Sprintf("Entity not found %+v", entity)
			return nil, status.Errorf(codes.NotFound, errorMsg)
		}
		logging.Errorf(ctx, "Failed to get entity from datastore: %s", err)
		return nil, status.Errorf(codes.Internal, InternalError)
	}

	if !util.SkipRealmsCheck {
		has, err := auth.HasPermission(ctx, neededPerm, entity.GetRealm(), nil)
		if err != nil {
			logging.Errorf(ctx, "Failed to fetch auth permissions: %s", err)
			return nil, status.Errorf(codes.Internal, InternalError)
		}
		if !has {
			logging.Infof(ctx, "User %s does not have permission %s in realm %s", auth.CurrentIdentity(ctx), neededPerm.String(), entity.GetRealm())
			return nil, status.Errorf(codes.PermissionDenied, "Permission denied")
		}
	}

	pm, perr := entity.GetProto()
	if perr != nil {
		logging.Errorf(ctx, "Failed to unmarshal proto: %s", perr)
		return nil, status.Errorf(codes.Internal, InternalError)
	}
	return pm, nil
}

// ListQuery constructs a query to list entities with pagination
func ListQuery(ctx context.Context, entityKind string, pageSize int32, pageToken string, filterMap map[string][]any, keysOnly bool) (q *datastore.Query, err error) {
	var cursor datastore.Cursor
	if pageToken != "" {
		cursor, err = datastore.DecodeCursor(ctx, pageToken)
		if err != nil {
			logging.Errorf(ctx, "Failed to DecodeCursor from pageToken: %s", err)
			return nil, status.Errorf(codes.InvalidArgument, "%s: %s", InvalidPageToken, err.Error())
		}
	}
	q = datastore.NewQuery(entityKind).Limit(pageSize).KeysOnly(keysOnly).FirestoreMode(true)
	for field, values := range filterMap {
		for _, id := range values {
			q = q.Eq(field, id)
		}
	}
	if cursor != nil {
		q = q.Start(cursor)
	}
	return q, nil
}

// ListQueryIdPrefixSearch constructs a query by searching for the name/id prefix to list entities with pagination
func ListQueryIdPrefixSearch(ctx context.Context, entityKind string, pageSize int32, pageToken string, prefix string, keysOnly bool) (q *datastore.Query, err error) {
	var cursor datastore.Cursor
	if pageToken != "" {
		cursor, err = datastore.DecodeCursor(ctx, pageToken)
		if err != nil {
			logging.Errorf(ctx, "Failed to DecodeCursor from pageToken: %s", err)
			return nil, status.Errorf(codes.InvalidArgument, "%s: %s", InvalidPageToken, err.Error())
		}
	}
	q = datastore.NewQuery(entityKind).Limit(pageSize).KeysOnly(keysOnly).FirestoreMode(true)
	startKey := datastore.NewKey(ctx, entityKind, prefix, 0, nil)
	endKey := datastore.NewKey(ctx, entityKind, fmt.Sprintf("%s\uFFFD", prefix), 0, nil)
	q = q.Gte("__key__", startKey)
	q = q.Lt("__key__", endKey)
	if cursor != nil {
		q = q.Start(cursor)
	}
	return q, nil
}

// Delete deletes the entity from the datastore.
func Delete(ctx context.Context, pm proto.Message, nf NewFunc) error {
	entity, err := nf(ctx, pm)
	if err != nil {
		logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
		return status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", InternalError, err.Error()))
	}
	// Datastore doesn't throw an error if the record doesn't exist.
	// Check and return err if there is no such entity in the datastore.
	existsResults, err := datastore.Exists(ctx, entity)
	if err == nil {
		if !existsResults.All() {
			errorMsg := fmt.Sprintf("Entity not found %+v", entity)
			return status.Errorf(codes.NotFound, errorMsg)
		}
	} else {
		logging.Debugf(ctx, "Failed to check existence: %s", err)
	}
	if err = datastore.Delete(ctx, entity); err != nil {
		logging.Errorf(ctx, "Failed to delete entity from datastore: %s", err)
		return status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", InternalError, err.Error()))
	}
	return nil
}

// Insert inserts the fleet objects.
func Insert(ctx context.Context, es []proto.Message, nf NewFunc, update, upsert bool) (*OpResults, error) {
	allRes := make(OpResults, len(es))
	checkEntities := make([]FleetEntity, 0, len(es))
	checkRes := make(OpResults, 0, len(es))
	for i, e := range es {
		allRes[i] = &OpResult{
			Data: e,
		}
		entity, err := nf(ctx, e)
		if err != nil {
			allRes[i].LogError(err)
			continue
		}
		checkEntities = append(checkEntities, entity)
		checkRes = append(checkRes, allRes[i])
	}

	f := func(ctx context.Context) error {
		toAddEntities := make([]FleetEntity, 0, len(checkEntities))
		toAddRes := make(OpResults, 0, len(checkEntities))
		if upsert {
			toAddEntities = checkEntities
			toAddRes = checkRes
		} else {
			exists, err := Exists(ctx, checkEntities)
			if err == nil {
				for i, e := range checkEntities {
					if !exists[i] && update {
						checkRes[i].LogError(errors.Reason("No such Object in the datastore").Err())
						continue
					}
					if exists[i] && !update {
						checkRes[i].LogError(errors.Reason("Object exists in the datastore").Err())
						continue
					}
					toAddEntities = append(toAddEntities, e)
					toAddRes = append(toAddRes, checkRes[i])
				}
			} else {
				logging.Debugf(ctx, "Failed to check existence: %s", err)
				toAddEntities = checkEntities
				toAddRes = checkRes
			}
		}
		if err := datastore.Put(ctx, toAddEntities); err != nil {
			for i, e := range err.(errors.MultiError) {
				toAddRes[i].LogError(e)
			}
			return err
		}
		return nil
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return &allRes, err
	}
	return &allRes, nil
}

// GetAll returns all entities in table.
func GetAll(ctx context.Context, qf QueryAllFunc) (*OpResults, error) {
	entities, err := qf(ctx)
	if err != nil {
		return nil, err
	}
	res := make(OpResults, len(entities))
	for i, e := range entities {
		res[i] = &OpResult{}
		pm, err := e.GetProto()
		if err != nil {
			res[i].LogError(err)
			continue
		}
		res[i].Data = pm
	}
	return &res, nil
}

// BatchGet returns all entities in table for given IDs.
func BatchGet(ctx context.Context, es []proto.Message, nf NewFunc, getID GetIDFunc) ([]proto.Message, error) {
	if len(es) == 0 {
		return nil, nil
	}
	res := make([]proto.Message, 0)
	entities := make([]FleetEntity, len(es))
	ids := make([]string, len(es))
	for i, e := range es {
		entity, err := nf(ctx, e)
		if err != nil {
			return nil, err
		}
		entities[i] = entity
		ids[i] = getID(e)
	}

	if err := datastore.Get(ctx, entities); err != nil {
		for i, e := range err.(errors.MultiError) {
			if e != nil {
				logging.Debugf(ctx, "BatchGet for %s: %s", ids[i], e.Error())
				return nil, errors.Annotate(e, "Fail to get asset %q", ids[i]).Tag(grpcutil.FailedPreconditionTag).Err()
			}
		}
	}

	for _, e := range entities {
		pm, err := e.GetProto()
		if err != nil {
			return nil, err
		}
		res = append(res, pm)
	}
	return res, nil
}

// BatchGetACL returns all entities in table for given IDs after ensuring the
// user can access them. If any entity is not accessible to the user, this will
// return no data and an error.
func BatchGetACL(ctx context.Context, es []proto.Message, nf NewRealmEntityFunc, getID GetIDFunc, neededPerm realms.Permission) ([]proto.Message, error) {
	if len(es) == 0 {
		return nil, nil
	}
	res := make([]proto.Message, 0)
	entities := make([]RealmEntity, len(es))
	ids := make([]string, len(es))
	for i, e := range es {
		entity, err := nf(ctx, e)
		if err != nil {
			return nil, err
		}
		entities[i] = entity
		ids[i] = getID(e)
	}

	if err := datastore.Get(ctx, entities); err != nil {
		for i, e := range err.(errors.MultiError) {
			if e != nil {
				logging.Debugf(ctx, "BatchGet for %s: %s", ids[i], e.Error())
				return nil, errors.Annotate(e, "Fail to get asset %q", ids[i]).Tag(grpcutil.FailedPreconditionTag).Err()
			}
		}
	}

	for _, e := range entities {
		if !util.SkipRealmsCheck {
			has, err := auth.HasPermission(ctx, neededPerm, e.GetRealm(), nil)
			if err != nil {
				logging.Errorf(ctx, "Failed to fetch auth permissions: %s", err)
				return nil, status.Errorf(codes.Internal, InternalError)
			}
			if !has {
				logging.Infof(ctx, "User %s does not have permission %s in realm %s", auth.CurrentIdentity(ctx), neededPerm.String(), e.GetRealm())
				return nil, status.Errorf(codes.PermissionDenied, "Permission denied")
			}
		}

		pm, err := e.GetProto()
		if err != nil {
			return nil, err
		}
		res = append(res, pm)
	}
	return res, nil
}

// BatchDelete removes the entities from the datastore
//
// This is a non-atomic operation
// Returns error even if partial delete succeeds.
// Must be used within a Transaction so that partial deletes are rolled back.
// Using it in a Transaction will rollback the partial deletes and propagate correct error message.
func BatchDelete(ctx context.Context, es []proto.Message, nf NewFunc) error {
	checkEntities := make([]FleetEntity, 0, len(es))
	for _, e := range es {
		entity, err := nf(ctx, e)
		if err != nil {
			logging.Errorf(ctx, "Failed to marshal new entity: %s", err)
			return status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", InternalError, err.Error()))
		}
		checkEntities = append(checkEntities, entity)
	}
	// Datastore doesn't throw an error if the record doesn't exist.
	// Check and return err if there is no such entity in the datastore.
	exists, err := Exists(ctx, checkEntities)
	if err == nil {
		for i, entity := range checkEntities {
			if !exists[i] {
				errorMsg := fmt.Sprintf("Entity not found: %+v", entity)
				logging.Errorf(ctx, errorMsg)
				return status.Errorf(codes.NotFound, errorMsg)
			}
		}
	}
	if err := datastore.Delete(ctx, checkEntities); err != nil {
		logging.Errorf(ctx, "Failed to delete entities from datastore: %s", err)
		return status.Errorf(codes.Internal, fmt.Sprintf("%s: %s", InternalError, err.Error()))
	}
	return nil
}

// BatchDeleteACL removes the entities from the datastore after ensuring the
// user can access them
//
// This is a non-atomic operation, returns error even if partial delete succeeds.
// Must be used within a Transaction so that partial deletes are rolled back.
// If any entity is not accessible to the user, this will return no data and an error.
func BatchDeleteACL(ctx context.Context, es []proto.Message, nf NewRealmEntityFunc, neededPerm realms.Permission) error {
	if len(es) == 0 {
		return nil
	}
	entities := make([]RealmEntity, len(es))
	for i, e := range es {
		entity, err := nf(ctx, e)
		if err != nil {
			return err
		}
		entities[i] = entity
	}

	if err := datastore.Get(ctx, entities); err != nil {
		var valErrs errors.MultiError
		if errors.As(err, &valErrs) {
			for i, e := range err.(errors.MultiError) {
				if e != nil {
					logging.Debugf(ctx, "BatchGet for %s: %s", entities[i], e.Error())
					return errors.Annotate(e, "Fail to get %q", entities[i]).Tag(grpcutil.FailedPreconditionTag).Err()
				}
			}
		} else {
			return errors.Annotate(err, "Fail to get entity").Tag(grpcutil.FailedPreconditionTag).Err()
		}
	}

	if !util.SkipRealmsCheck {
		for _, e := range entities {
			has, err := auth.HasPermission(ctx, neededPerm, e.GetRealm(), nil)
			if err != nil {
				logging.Errorf(ctx, "Failed to fetch auth permissions: %s", err)
				return status.Errorf(codes.Internal, "Fail to fetch auth permissions: %s", err)
			}
			if !has {
				logging.Infof(ctx, "User %s does not have permission %s in realm %s", auth.CurrentIdentity(ctx), neededPerm.String(), e.GetRealm())
				return status.Errorf(codes.PermissionDenied, "Permission denied")
			}
		}
	}

	if err := datastore.Delete(ctx, entities); err != nil {
		logging.Errorf(ctx, "Failed to delete entities from datastore: %s", err)
		return status.Errorf(codes.Internal, fmt.Sprintf("fail to delete entities with ACL: %s", err.Error()))
	}
	return nil
}

// DeleteAll removes the entities from the datastore
func DeleteAll(ctx context.Context, es []proto.Message, nf NewFunc) *OpResults {
	allRes := make(OpResults, len(es))
	checkRes := make(OpResults, 0, len(es))
	checkEntities := make([]FleetEntity, 0, len(es))
	for i, e := range es {
		allRes[i] = &OpResult{
			Data: e,
		}
		entity, err := nf(ctx, e)
		if err != nil {
			allRes[i].LogError(err)
			continue
		}
		checkEntities = append(checkEntities, entity)
		checkRes = append(checkRes, allRes[i])
	}
	if len(checkEntities) == 0 {
		return &allRes
	}
	// Datastore doesn't throw an error if the record doesn't exist.
	// Check and return err if there is no such entity in the datastore.
	exists, err := Exists(ctx, checkEntities)
	if err == nil {
		for i := range checkEntities {
			if !exists[i] {
				checkRes[i].LogError(errors.Reason("Entity not found").Err())
			}
		}
	}
	if err := datastore.Delete(ctx, checkEntities); err != nil {
		for i, e := range err.(errors.MultiError) {
			if e != nil {
				checkRes[i].LogError(e)
			}
		}
	}
	return &allRes
}

// OpResult records the result of datastore operations
type OpResult struct {
	// Operations:
	// Get: record the retrieved proto object.
	// Add: record the proto object to be added.
	// Delete: record the proto object to be deleted.
	// Update: record the proto object to be updated.
	Data proto.Message
	Err  error
}

// LogError logs the error for an operation.
func (op *OpResult) LogError(e error) {
	op.Err = e
}

// OpResults is a list of OpResult.
type OpResults []*OpResult

func (rs OpResults) filter(f func(*OpResult) bool) OpResults {
	result := make(OpResults, 0, len(rs))
	for _, r := range rs {
		if f(r) {
			result = append(result, r)
		}
	}
	return result
}

// Passed generates the list of entities passed the operation.
func (rs OpResults) Passed() OpResults {
	return rs.filter(func(result *OpResult) bool {
		return result.Err == nil
	})
}

// Failed generates the list of entities failed the operation.
func (rs OpResults) Failed() OpResults {
	return rs.filter(func(result *OpResult) bool {
		return result.Err != nil
	})
}

// AssignRealms assigns the realms to the query, and returns a list of queries
// such that each query has an equality condition for one realm
func AssignRealms(query *datastore.Query, realms []string) []*datastore.Query {
	var queries []*datastore.Query
	for _, realm := range realms {
		queries = append(queries, query.Eq("realm", realm))
	}
	return queries
}
