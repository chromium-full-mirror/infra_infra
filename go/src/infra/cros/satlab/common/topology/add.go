// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package topology

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/dut/shivas"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type AddTopology struct {
	SatlabID string

	Hostname     string
	TopologyPath string
}

// TriggerRun runs shivas to add PASIT topology to the DUT.
func (c *AddTopology) TriggerRun(ctx context.Context, executor executor.IExecCommander) error {
	if err := c.validateArgs(); err != nil {
		return errors.Annotate(err, "validate arguments").Err()
	}
	if c.SatlabID == "" {
		var err error
		if c.SatlabID, err = satlabcommands.GetDockerHostBoxIdentifier(ctx, executor); err != nil {
			return err
		}
	}
	qualifiedHostname := site.MaybePrepend(site.Satlab, c.SatlabID, c.Hostname)
	if err := (&shivas.PASITTopology{
		Hostname:     qualifiedHostname,
		TopologyPath: c.TopologyPath,
		Executor:     executor,
	}).Add(ctx); err != nil {
		return errors.Annotate(err, "add PASIT topology").Err()
	}
	return nil
}

// validateArgs checks if all required parameters are provided.
func (c *AddTopology) validateArgs() error {
	if c.Hostname == "" {
		return errors.New("DUT hostname (-hostname) is not specified")
	}
	if c.TopologyPath == "" {
		return errors.New("path to the topology file (-file) is not specified")
	}
	return nil
}
