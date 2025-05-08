// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file contains a list of interfaces that our data clients want to implement.
package datastorage

import "context"

type QueryFilter struct {
	Field    string
	Operator string
	Value    any
}

// IDataClient interface has necessary data related functions that need
// to be implemented by the struct using it. For example, a datastore
// client struct implementing this interface would need to have datastore
// specific implementation of the functions mentioned in this interface.
type IDataClient interface {
	// Gets a single data item of the specified type, matching the specified key.
	// Optionally takes additional arguments that can be used for specific
	// implementations of this interface.
	//
	// Returns an error if the data item cannot be found.
	Get(
		ctx context.Context,
		result any,
		dataType string,
		key any,
		options ...any,
	) error

	// Query function performs a data storage query with the given filters,
	// order, limit and options. Returns the matching items from the query result.
	//
	// Filters should be tuples of property name, operator, and value.
	//
	// The order argument is used to specify the order of the query results.
	//
	// The limit argument is used to limit the number of items being read from
	// the data storage.
	//
	// The options argument can be used for specific implementations of this
	// function.
	//
	// The result is reflected in the "result" argument.
	Query(
		ctx context.Context,
		result any,
		dataType string,
		queryFilters []QueryFilter,
		order any,
		limit int,
		options ...any,
	) error

	// BatchPut function performs a data storage query to save
	// multiple entities given their corresponding keys into
	// the storage.
	//
	// Returns an error if the operation fails.
	BatchPut(
		ctx context.Context,
		entities any,
		keys any,
	) error
}
