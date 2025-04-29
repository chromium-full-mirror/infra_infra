// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package adb contains methods to work with an ADB-base container.
package adb

import (
	"context"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.ADBServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("adb service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("adb service client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.ADBName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "adb service client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "adb service client").Err()
	}
	client := api.NewADBServiceClient(conn)
	if client == nil {
		return nil, errors.Reason("adb service client: fail to create client").Err()
	}
	return client, nil
}
