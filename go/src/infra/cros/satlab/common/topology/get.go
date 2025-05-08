// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package topology

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/dut"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type GetTopology struct {
	SatlabID string

	Hostname string
}

// TriggerRun runs shivas to get PASIT topology for given DUT.
func (c *GetTopology) TriggerRun(ctx context.Context, executor executor.IExecCommander) (*api.PasitHost, error) {
	if err := c.validateArgs(); err != nil {
		return &api.PasitHost{}, errors.Annotate(err, "validate arguments").Err()
	}
	if c.SatlabID == "" {
		var err error
		if c.SatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, executor); err != nil {
			return &api.PasitHost{}, err
		}
	}
	qualifiedHostname := site.MaybePrepend(site.Satlab, c.SatlabID, c.Hostname)
	topologyJson, err := c.getDUTTopology(ctx, executor, qualifiedHostname)
	if err != nil {
		return &api.PasitHost{}, errors.Annotate(err, "get topology from dut").Err()
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
func (c *GetTopology) getDUTTopology(ctx context.Context, executor executor.IExecCommander, dutName string) (*api.PasitHost, error) {
	d := dut.GetDUT{
		SatlabID: c.SatlabID,
	}
	dut, err := d.TriggerRun(ctx, executor, []string{dutName})
	if err != nil {
		return &api.PasitHost{}, errors.Annotate(err, "get dut information").Err()
	}
	if len(dut) != 1 {
		return &api.PasitHost{}, errors.New("number of returned DUTs is different than 1")
	}
	topologyJson := dut[0].GetChromeosMachineLse().GetDeviceLse().GetDut().GetPeripherals().GetPasitHost2()
	return topologyJson, nil
}
