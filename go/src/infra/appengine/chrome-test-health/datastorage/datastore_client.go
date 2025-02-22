// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package datastorage

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"cloud.google.com/go/datastore"
)

var (
	ErrInsufficientArgs = errors.New("insufficent arguments")
	ErrConnection       = errors.New("connection error")
	ErrInvalidKey       = errors.New("invalid key")
	ErrInvalidType      = errors.New("invalid type")
	ErrEntityNotFound   = errors.New("entity not found")
	ErrInternal         = errors.New("internal error")
)

type DataStoreClient struct {
	datastoreClient *datastore.Client
}

// NewDataStoreClient function creates a new Datastore client and
// returns it if successful. On failure, returns an error.
// Note that this client is specifically designed to communicate
// with Google's Datastore service.
func NewDataStoreClient(ctx context.Context, cloudProject string) (*DataStoreClient, error) {
	datastoreClient, err := datastore.NewClient(ctx, cloudProject)
	if err != nil {
		return nil, ErrConnection
	}
	c := DataStoreClient{
		datastoreClient: datastoreClient,
	}
	return &c, nil
}

// Get takes in the Entity name, entity key, options (in case the key needs ancestor)
// and an empty struct reference. Returns an error if the fetch was unsuccessful.
// Otherwise copies the entity data to the result argument which should be an
// empty struct reference.
//
// Important Notes:
// 1. This function can accept 4 or 6 arguments depending on whether the entity in question
// requires an ancestor to be found or not.
// 2. The 4th and 6th (if present) arguments can be of type string or number because key
// can either be a NameKey or an IdKey.
// 3. If the result interface has a property called Key of type *datastore.Key,
// it will be autopopulated with the fetched entity's datastore Key.
//
// Example usage:
// str := A{}
// Example 1: dsClient.Get(ctx, &str, "EntityA", "EntityAKey", "AncestorEntity", "AncestorEntityKey")
// Example 2: dsClient.Get(ctx, &str, "EntityA", 123, "AncestorEntity", 345)
// Example 3: dsClient.Get(ctx, &str, "EntityA", "k")
func (c DataStoreClient) Get(ctx context.Context, result interface{}, entityName string, key interface{}, options ...interface{}) error {
	if !(len(options) == 0 || len(options) == 2) {
		return fmt.Errorf("%s: Expected 4 or 6 arguments but got %d", ErrInsufficientArgs, len(options)+4)
	}

	var ancestorKeyLiteral *datastore.Key
	if len(options) == 2 {
		var ancestorEntityName string
		if e, isStringType := options[0].(string); isStringType {
			ancestorEntityName = e
		} else {
			return fmt.Errorf("%s: Ancestor entity name should be of type string", ErrInvalidType.Error())
		}

		switch options[1].(type) {
		case string:
			ancestorKeyLiteral = datastore.NameKey(ancestorEntityName, options[1].(string), nil)
		case int64:
			ancestorKeyLiteral = datastore.IDKey(ancestorEntityName, options[1].(int64), nil)
		default:
			return fmt.Errorf("%s: Ancestor key should be of type string or int64", ErrInvalidType.Error())
		}
	}

	var entityKeyLiteral *datastore.Key
	switch k := key.(type) {
	case string:
		entityKeyLiteral = datastore.NameKey(entityName, k, ancestorKeyLiteral)
	case int64:
		entityKeyLiteral = datastore.IDKey(entityName, k, ancestorKeyLiteral)
	default:
		return fmt.Errorf("%s: Entity key should be a string or int64", ErrInvalidType.Error())
	}

	err := c.datastoreClient.Get(ctx, entityKeyLiteral, result)
	if err != nil {
		if errors.Is(err, datastore.ErrInvalidEntityType) {
			return fmt.Errorf("%s: The result argument is likely an invalid type", ErrInvalidType)
		}
		if errors.Is(err, datastore.ErrInvalidKey) {
			return ErrInvalidKey
		}
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return ErrEntityNotFound
		}
		return ErrInternal
	}

	v := reflect.ValueOf(result).Elem()
	if _, hasKey := v.Type().FieldByName("Key"); hasKey {
		v.FieldByName("Key").Set(reflect.ValueOf(entityKeyLiteral))
	}

	return nil
}

// Query takes in entity name, list of query filters, order, limit and result
// in the format *[]S{} where S is struct's name.
// Returns an error if the fetch was unsuccessful. Otherwise copies the
// entity data to the result argument.
//
// Important Notes:
// 1. The order attribute must either be a string field or nil. If nil, the query
// would use the default order to fetch the entity.
//
// 2. The limit attribute must be present and should be >= 0.
// If 0 the query will bring in all the records.
// A positive number will indicate a limit equal to that number.
// A negative limit will be ignore.
// *** The running time and number of external API calls
// made by this function scale linearly with the the query's limit attribute.
// Unless the result count is expected to be small, it is best to specify a limit;
// otherwise Query will continue until it finishes collecting results or the
// provided context expires.
//
// Example usage:
// str := []A{}
//
//	queryFilters := []QueryFilter{
//			{Field: "Field name", Operator: "=", Value: "Val"},
//	}
//
// dsclient.Query(ctx, &str, "EntityA", queryFilters, "-attribute")
func (c DataStoreClient) Query(
	ctx context.Context,
	result interface{},
	entityName string,
	filters []QueryFilter,
	order interface{},
	limit int,
	options ...interface{}) error {
	q := datastore.NewQuery(entityName)
	for _, filterQuery := range filters {
		q = q.FilterField(filterQuery.Field, filterQuery.Operator, filterQuery.Value)
	}

	if order != nil {
		switch order := order.(type) {
		case string:
			q = q.Order(order)
		default:
			return fmt.Errorf("%s: Argument order should be either a string or nil", ErrInvalidType)
		}
	}

	if limit > 0 {
		q = q.Limit(limit)
	}

	keys, err := c.datastoreClient.GetAll(ctx, q, result)
	if err != nil {
		if errors.Is(err, datastore.ErrInvalidEntityType) {
			return fmt.Errorf("%w: The result argument is likely an invalid type", ErrInvalidType)
		}

		return ErrInternal
	}

	resultSlice := reflect.ValueOf(result)
	for i, key := range keys {
		r := resultSlice.Elem().Index(i)
		if _, hasKey := r.Type().FieldByName("Key"); hasKey {
			r.FieldByName("Key").Set(reflect.ValueOf(key))
		}
	}

	return nil
}

// BatchPut takes in a list of entities in the format []S{} where S is the
// struct's name. It also takes a list of keys in the format []*datastore.Key
//
// The length of keys must match with that of entities which corresponds to a
// 1:1 mapping between them.
// Example usage:
//
//	entities := []A{
//	 { prop1: "p1", prop2: "p2" }
//	 { prop1: "p3", prop2: "p4" }
//	}
//
//	keys := []*datastore.Key {
//	 datastore.NameKey("A", "keyliteral", nil)
//	}
//
// dsclient.BatchPut(ctx, entities, keys)
func (c DataStoreClient) BatchPut(
	ctx context.Context,
	entities interface{},
	keys interface{},
) error {
	datastoreKeys, ok := keys.([]*datastore.Key)
	if !ok {
		return fmt.Errorf("%w: The keys must be of type []*datastore.Key", ErrInvalidKey)
	}

	_, err := c.datastoreClient.PutMulti(ctx, datastoreKeys, entities)
	if err != nil {
		return err
	}

	return nil
}
