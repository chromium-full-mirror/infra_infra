// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"encoding/json"
	"os/exec"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"

	"infra/cros/satlab/common/commands"
	"infra/cros/satlab/common/paths"
	"infra/cros/satlab/common/satlabcommands"
	"infra/cros/satlab/common/site"
	e "infra/cros/satlab/common/utils/errors"
	"infra/cros/satlab/common/utils/executor"
	ufsModels "infra/unifiedfleet/api/v1/models"
)

// GetDUT contains fields used to control behavior when fetching DUTs
type GetDUT struct {
	// SatlabID will be prepended to DUT names if not already prefixing them.
	SatlabID      string
	Namespace     string
	HostInfoStore bool

	// Zone value Name(s) of a zone to filter by.
	Zones []string
	// Rack Name(s) of a rack to filter by.
	Racks []string
	// Machines Name(s) of a machine/asset to filter by.
	Machines []string
	// Prototypes Name(s) of a host prototype to filter by.
	Prototypes []string
	// Tags Name(s) of a tag to filter by.
	Tags []string
	// States Name(s) of a state to filter by.
	States []string
	// Servos Name(s) of a servo:port to filter by.
	Servos []string
	// Servotypes Name(s) of a servo type to filter by.
	Servotypes []string
	// Switches Name(s) of a switch to filter by.
	Switches []string
	// Rpms Name(s) of a rpm to filter by.
	Rpms []string
	// Pools Name(s) of a pool to filter by.
	Pools []string
}

func makeGetDUTShivasFlags(f *GetDUT) Flagmap {
	out := make(Flagmap)

	if len(f.Zones) != 0 {
		out["zone"] = f.Zones
	}
	if len(f.Racks) != 0 {
		out["rack"] = f.Racks
	}
	if len(f.Machines) != 0 {
		out["machine"] = f.Machines
	}
	if len(f.Prototypes) != 0 {
		out["prototype"] = f.Prototypes
	}
	if len(f.Servos) != 0 {
		out["servo"] = f.Servos
	}
	if len(f.Servotypes) != 0 {
		out["servotype"] = f.Servotypes
	}
	if len(f.Switches) != 0 {
		out["switch"] = f.Switches
	}
	if len(f.Rpms) != 0 {
		out["rpms"] = f.Rpms
	}
	if len(f.Pools) != 0 {
		out["pools"] = f.Pools
	}
	if f.HostInfoStore {
		out["host-info-store"] = []string{}
	}

	// Default flags
	out["namespace"] = []string{site.GetNamespace(f.Namespace)}
	out["json"] = []string{}

	return out
}

// TriggerRun trigger `shivas get dut` CLI to get the machines information.
func (g *GetDUT) TriggerRun(
	ctx context.Context,
	executor executor.IExecCommander,
	names []string,
) ([]*ufsModels.MachineLSE, error) {
	var err error
	if g.SatlabID == "" {
		g.SatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
		if err != nil {
			return nil, errors.Annotate(err, "get dut").Err()
		}
	}

	if names == nil {
		names = []string{}
	}

	for idx, name := range names {
		names[idx] = site.MaybePrepend(site.Satlab, g.SatlabID, name)
	}

	flags := makeGetDUTShivasFlags(g)

	args := (&commands.CommandWithFlags{
		Commands:       []string{paths.ShivasCLI, "get", "dut"},
		Flags:          flags,
		PositionalArgs: names,
		AuthRequired:   true,
	}).ToCommand()
	command := exec.CommandContext(ctx, args[0], args[1:]...)

	// Don't use `CombinedOutput` here because it returns
	// `rpc error: code = NotFound` that means the asset doesn't exist.
	// As we check only the length of output.
	out, err := executor.Output(command)

	if err != nil {
		return nil, errors.Annotate(e.HandleExitError(err), "get dut").Err()
	}

	res := []*ufsModels.MachineLSE{}

	var messages []json.RawMessage
	err = json.Unmarshal(out, &messages)
	if err != nil {
		return nil, errors.Annotate(err, "get dut - decode json list failed").Err()
	}

	for _, j := range messages {
		data := &ufsModels.MachineLSE{}
		// ignore mismatches due to shivas using a different version of the proto.
		err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(j, data)
		if err != nil {
			return nil, errors.Annotate(err, "get dut - decode json list failed").Err()
		}
		res = append(res, data)
	}

	return res, nil
}
