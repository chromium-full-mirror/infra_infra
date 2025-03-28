// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// NlyteAssetRegistration registers the given asset to the datastore after validation
func NlyteAssetRegistration(ctx context.Context, asset *ufspb.Asset) (*ufspb.Asset, error) {

	f := func(ctx context.Context) error {
		if err := validateNlyteAssetRegistration(ctx, asset); err != nil {
			return err
		}
		// Nlyte: Skip creating a backing machine; we only want new assets
		_, err := registration.BatchUpdateNlyteAssets(ctx, []*ufspb.Asset{asset})
		if err != nil {
			return err
		}
		return nil
	}
	if err := datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "AddNlyeAsset - unable to update asset %s", asset.GetName()).Err()
	}
	// Nlyte: Don't try to update asset with info from HaRT
	return asset, nil
}

// UpdateNlyteAsset updates the asset record to the datastore after validation
func UpdateNlyteAsset(ctx context.Context, asset *ufspb.Asset, mask *field_mask.FieldMask) (*ufspb.Asset, error) {
	var oldAsset *ufspb.Asset
	var err error
	f := func(ctx context.Context) error {
		// TODO(anushruth): Support validation of DUT/Labstation/Servo
		// created using this asset. And update them accordingly or fail.
		oldAsset, err = registration.GetNlyteAsset(ctx, asset.GetName())
		if err != nil {
			return err
		}

		err := validateNlyteUpdateAsset(ctx, oldAsset, asset, mask)
		if err != nil {
			return err
		}

		// Copy OUTPUT_ONLY fields
		if asset.GetInfo() == nil {
			asset.Info = &ufspb.AssetInfo{}
		}
		asset.GetInfo().SerialNumber = oldAsset.GetInfo().GetSerialNumber()

		// Allow users to modify hwid before we can get authoritative source from HaRT
		// Don't allow users to modify it to empty
		if asset.GetInfo().GetHwid() == "" {
			asset.GetInfo().Hwid = oldAsset.GetInfo().GetHwid()
		}
		asset.GetInfo().Sku = oldAsset.GetInfo().GetSku()

		// updatableAsset will be used to update the asset
		updatableAsset := asset
		if mask != nil && mask.Paths != nil {
			// Construct updatableAsset from mask if given
			updatableAsset = proto.Clone(proto.MessageV1(oldAsset)).(*ufspb.Asset)
			updatableAsset, err = processAssetUpdateMask(asset, updatableAsset, mask)
			if err != nil {
				return err
			}
		}
		a, err := registration.BatchUpdateNlyteAssets(ctx, []*ufspb.Asset{updatableAsset})
		if err != nil {
			return err
		}

		// Nlyte: Don't attempt to create/update machines for assets

		// Return the updated asset
		asset = a[0]
		return nil
	}
	if err = datastore.RunInTransaction(ctx, f, nil); err != nil {
		return nil, errors.Annotate(err, "UpdateNlyteAsset - unable to update asset %s", asset.GetName()).Err()
	}
	return asset, err
}

// GetNlyteAsset returns asset for the given name from datastore
func GetNlyteAsset(ctx context.Context, name string) (*ufspb.Asset, error) {
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "GetNlyteAsset - missing asset name")
	}
	asset, err := registration.GetNlyteAssetACL(ctx, name)
	if err != nil {
		return nil, errors.Annotate(err, "GetNlyteAsset - unable to get asset %s", name).Err()
	}
	return asset, nil
}

func validateNlyteUpdateAsset(ctx context.Context, oldAsset *ufspb.Asset, asset *ufspb.Asset, mask *field_mask.FieldMask) error {
	if err := util.CheckPermission(ctx, util.RegistrationsUpdate, oldAsset.GetRealm()); err != nil {
		return err
	}
	if asset.GetRealm() != "" && oldAsset.GetRealm() != asset.GetRealm() {
		if err := util.CheckPermission(ctx, util.RegistrationsUpdate, asset.GetRealm()); err != nil {
			return err
		}
	}
	if mask == nil || mask.Paths == nil {
		// If mask doesn't exist then validate the given asset
		return validateAsset(ctx, asset)
	}
	// Validate AssetUpdate Mask if it exists
	return validateAssetUpdateMask(ctx, asset, mask)
}

func validateNlyteAssetRegistration(ctx context.Context, asset *ufspb.Asset) error {
	if err := util.CheckPermission(ctx, util.RegistrationsCreate, asset.GetRealm()); err != nil {
		return err
	}
	if err := validateAsset(ctx, asset); err != nil {
		return err
	}
	var errMsg strings.Builder
	errMsg.WriteString("validateAsset - ")
	if err := ResourceExist(ctx, []*Resource{GetNlyteAssetResource(asset.GetName())}, &errMsg); err == nil {
		return status.Errorf(codes.FailedPrecondition, "validateNlyteAssetRegistration - Asset %s exists, cannot create another", asset.GetName())
	}
	return nil
}
