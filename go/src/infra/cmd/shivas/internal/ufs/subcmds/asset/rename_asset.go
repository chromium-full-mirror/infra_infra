// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package asset

import (
	"context"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmd/shivas/utils/rename"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// RenameAssetCmd rename asset by given name.
var RenameAssetCmd = rename.GenGenericRenameCmd("asset", renameAsset, printAsset, site.OSLikeNamespaces, ufsUtil.OSNamespace)

// renameAsset calls the RPC that renames the given asset
func renameAsset(ctx context.Context, ic ufsAPI.FleetClient, name, newName string) (interface{}, error) {
	// Change  this  API if you want to reuse the command somewhere else.
	return ic.RenameAsset(ctx, &ufsAPI.RenameAssetRequest{
		Name:    ufsUtil.AddPrefix(ufsUtil.AssetCollection, name),
		NewName: ufsUtil.AddPrefix(ufsUtil.AssetCollection, newName),
	})
}

// printAsset prints the result of the operation
func printAsset(asset proto.Message) {
	utils.PrintProtoJSON(asset, !utils.NoEmitMode(false))
}
