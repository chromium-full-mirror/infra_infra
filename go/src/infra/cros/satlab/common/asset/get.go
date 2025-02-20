// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package asset

import (
	"context"
	"encoding/json"
	"os/exec"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/commands"
	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/site"
	e "go.chromium.org/infra/cros/satlab/common/utils/errors"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

type Flagmap = map[string][]string

// GetAsset contains fields used to control behaviour when fetching Assets
type GetAsset struct {
	Namespace string
	// Zone value Name(s) of a zone to filter by.
	Zones []string
	// Rack Name(s) of a rack to filter by.
	Racks []string
	// Tags Name(s) of a tag to filter by.
	Tags []string
	// boards Name(s) of a build target/board to filter by.
	Boards []string
	// Models Name(s) of a model to filter by..
	Models []string
	// AssetTypes Name(s) of a assettype to filter by.
	AssetTypes []string
}

func makeGetAssetShivasFlags(in *GetAsset) Flagmap {
	out := make(Flagmap)
	if len(in.Zones) != 0 {
		out["zone"] = in.Zones
	}
	if len(in.Racks) != 0 {
		out["rack"] = in.Racks
	}
	if len(in.Tags) != 0 {
		out["tag"] = in.Tags
	}
	if len(in.Boards) != 0 {
		out["board"] = in.Boards
	}
	if len(in.Models) != 0 {
		out["model"] = in.Models
	}
	if len(in.AssetTypes) != 0 {
		out["assettype"] = in.AssetTypes
	}
	// Default flags
	out["namespace"] = []string{site.GetNamespace(in.Namespace)}
	out["json"] = []string{}
	return out
}

// TriggerRun trigger `shivas get asset` CLI to get the asset information.
func (g *GetAsset) TriggerRun(
	ctx context.Context,
	executor executor.IExecCommander,
) ([]*ufsModels.Asset, error) {
	var err error
	flags := makeGetAssetShivasFlags(g)
	args := (&commands.CommandWithFlags{
		Commands:     []string{paths.ShivasCLI, "get", "asset"},
		Flags:        flags,
		AuthRequired: true,
	}).ToCommand()
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := executor.CombinedOutput(command)
	if err != nil {
		return nil, errors.Annotate(e.HandleExitError(err), "get asset").Err()
	}

	var res []*ufsModels.Asset

	var listOfJson []json.RawMessage
	err = json.Unmarshal(out, &listOfJson)
	if err != nil {
		return nil, errors.Annotate(err, "get asset - decode json list failed").Err()
	}

	for _, j := range listOfJson {
		var data ufsModels.Asset
		// ignore mismatches due to shivas using a different version of the proto.
		err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(j, &data)
		if err != nil {
			return nil, errors.Annotate(err, "get asset - decode json list failed").Err()
		}
		res = append(res, &data)
	}

	return res, nil
}
