// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"

	"github.com/golang/protobuf/proto"

	"go.chromium.org/infra/cmd/shivas/site"
	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cmd/shivas/utils/rename"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// RenameDUTCmd rename dut by given name.
var RenameDUTCmd = rename.GenGenericRenameCmd("dut", renameDUT, printDUT, site.OSLikeNamespaces, ufsUtil.OSNamespace)

// renameDUT calls the RPC that renames the given dut
func renameDUT(ctx context.Context, ic ufsAPI.FleetClient, name, newName string) (interface{}, error) {
	// Change  this  API if you want to reuse the command somewhere else.
	return ic.RenameMachineLSE(ctx, &ufsAPI.RenameMachineLSERequest{
		Name:    ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, name),
		NewName: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, newName),
	})
}

// printDUT prints the result of the operation
func printDUT(dut proto.Message) {
	utils.PrintProtoJSON(dut, !utils.NoEmitMode(false))
}
