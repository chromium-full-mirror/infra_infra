// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumper

import (
	"context"

	"go.chromium.org/luci/common/logging"

	api "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	frontend "go.chromium.org/infra/unifiedfleet/app/frontend"
)

func importCrosNetwork(ctx context.Context) error {
	sv := &frontend.FleetServerImpl{}
	logging.Debugf(ctx, "Importing ChromeOS networks")
	_, err := sv.ImportOSVlans(ctx, &api.ImportOSVlansRequest{
		Source: &api.ImportOSVlansRequest_MachineDbSource{
			MachineDbSource: &api.MachineDBSource{
				Host: "",
			},
		},
	})
	if err == nil {
		logging.Debugf(ctx, "Finish importing CrOS network configs")
	}
	return err
}
