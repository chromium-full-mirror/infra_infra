// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package foilprovision contains methods to work with an foil-provision container.
package foilprovision

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/tlw"
)

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.GenericProvisionServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("foil-provision service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("foil-provision service client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.FoilProvisionName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "foil-provision service client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "foil-provision service client").Err()
	}
	client := api.NewGenericProvisionServiceClient(conn)
	if client == nil {
		return nil, errors.Reason("foil-provision service client: fail to create client").Err()
	}
	return client, nil
}
