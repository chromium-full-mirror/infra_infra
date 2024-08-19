// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package datastore contains datastore-related logic.
package datastore

import (
	"context"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/gae/service/datastore"

	fleet "infra/libs/fleet/protos"
	ufs "infra/libs/fleet/protos/go"
)

// AssetInfoOpRes is return type for AssetInfo related operations
type AssetInfoOpRes struct {
	AssetInfo *ufs.AssetInfo
	Entity    *AssetInfoEntity
	Err       error
}

// GetAllAssets returns all assets from datastore.
//
// If keysOnly is true, then only key field is populated in returned assets
func GetAllAssets(ctx context.Context, keysOnly bool) ([]*fleet.ChopsAsset, error) {
	q := datastore.NewQuery(AssetEntityName).Ancestor(fakeAncestorKey(ctx))
	q = q.KeysOnly(keysOnly)
	var assetEntities []*AssetEntity
	if err := datastore.GetAll(ctx, q, &assetEntities); err != nil {
		return nil, err
	}
	assets := make([]*fleet.ChopsAsset, 0)
	for _, ae := range assetEntities {
		if a, err := ae.ToChopsAsset(); err == nil {
			assets = append(assets, a)
		}
	}
	return assets, nil
}

// A query in transaction requires to have Ancestor filter, see
// https://cloud.google.com/appengine/docs/standard/python/datastore/query-restrictions#queries_inside_transactions_must_include_ancestor_filters
func fakeAncestorKey(ctx context.Context) *datastore.Key {
	return datastore.MakeKey(ctx, AssetEntityName, "key")
}

// GetAssetInfo returns the AssetInfo matching the AssetID
func GetAssetInfo(ctx context.Context, ids []string) []*AssetInfoOpRes {
	queryResults := make([]*AssetInfoOpRes, len(ids))
	qrMap := make(map[string]*AssetInfoOpRes)
	entities := make([]*AssetInfoEntity, 0, len(ids))
	for _, assetID := range ids {
		res := &AssetInfoOpRes{
			Entity: &AssetInfoEntity{
				AssetTag: assetID,
			},
		}
		qrMap[assetID] = res
		// TODO(crbug.com/1074114): Check for "" may not be required
		// depending on how the bug is addressed..
		if assetID != "" {
			entities = append(entities, res.Entity)
		} else {
			res.Err = errors.Reason("Not a valid asset tag").Err()
		}
	}
	if err := datastore.Get(ctx, entities); err != nil {
		for i, e := range err.(errors.MultiError) {
			qrMap[entities[i].AssetTag].Err = e
		}
	}
	for i, assetID := range ids {
		queryResults[i] = qrMap[assetID]
	}
	return queryResults
}

// GetAllAssetInfo returns all AssetInfo from datastore.
//
// If keysOnly is true, then only key field is populated in returned assets
func GetAllAssetInfo(ctx context.Context, keysOnly bool) ([]*ufs.AssetInfo, error) {
	q := datastore.NewQuery(AssetInfoEntityKind)
	q = q.KeysOnly(keysOnly)
	var assetInfoEntities []*AssetInfoEntity
	if err := datastore.GetAll(ctx, q, &assetInfoEntities); err != nil {
		return nil, err
	}
	assetinfo := make([]*ufs.AssetInfo, 0, len(assetInfoEntities))
	for _, ae := range assetInfoEntities {
		assetinfo = append(assetinfo, &ae.Info)
	}
	return assetinfo, nil
}
