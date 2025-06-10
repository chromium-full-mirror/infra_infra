// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package topology

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/dut"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

type GetTopology struct {
	SatlabID string

	Hostname string
}

// TriggerRun runs shivas to get PASIT topology for given DUT.
func (c *GetTopology) TriggerRun(ctx context.Context, executor executor.IExecCommander) (*ufspb.Pasit, error) {
	if err := c.validateArgs(); err != nil {
		return &ufspb.Pasit{}, errors.Annotate(err, "validate arguments").Err()
	}
	if c.SatlabID == "" {
		var err error
		if c.SatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, executor); err != nil {
			return &ufspb.Pasit{}, err
		}
	}
	qualifiedHostname := site.MaybePrepend(site.Satlab, c.SatlabID, c.Hostname)
	topologyJson, err := c.getDUTTopology(ctx, executor, qualifiedHostname)
	if err != nil {
		return &ufspb.Pasit{}, errors.Annotate(err, "get topology from dut").Err()
	}
	return topologyJson, nil
}

// validateArgs checks if all required parameters are provided.
func (c *GetTopology) validateArgs() error {
	if c.Hostname == "" {
		return errors.New("DUT hostname (-hostname) is not specified")
	}
	return nil
}

// getDUTTopology extracts PASIT topology information from "shivas get dut" command.
func (c *GetTopology) getDUTTopology(ctx context.Context, executor executor.IExecCommander, dutName string) (*ufspb.Pasit, error) {
	d := dut.GetDUT{
		SatlabID: c.SatlabID,
	}
	dut, err := d.TriggerRun(ctx, executor, []string{dutName})
	if err != nil {
		return &ufspb.Pasit{}, errors.Annotate(err, "get dut information").Err()
	}
	if len(dut) != 1 {
		return &ufspb.Pasit{}, errors.New("number of returned DUTs is different than 1")
	}
	topologyJson := dut[0].GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetPasit()
	return topologyJson, nil
}
