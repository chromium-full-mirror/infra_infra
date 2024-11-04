// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdut contains methods to work with an cros-dut-base container.
package crosdut

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/components/cft"
	"infra/cros/recovery/tlw"
)

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.DutServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("cros-dut service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("cros-dut service client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.CrosDUTName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "cros-dut service client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "cros-dut service client").Err()
	}
	client := api.NewDutServiceClient(conn)
	if client == nil {
		return nil, errors.Reason("cros-dut service client: fail to create client").Err()
	}
	return client, nil
}
