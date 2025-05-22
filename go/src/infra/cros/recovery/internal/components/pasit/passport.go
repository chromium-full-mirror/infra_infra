// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pasit

import (
	"context"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/lab/api/passport"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// ContainerClient opens a client to the cros-passport container.
func ContainerClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (*grpc.ClientConn, error) {
	if dut == nil {
		return nil, errors.Reason("container client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("container client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.PassportName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "container client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "container client").Err()
	}
	return conn, nil
}

// InitSwitchClient creates and caches a client to the SwitchService on the passport container.
func InitSwitchClient(ctx context.Context, dut *tlw.Dut, conn *grpc.ClientConn) error {
	client := passport.NewSwitchServiceClient(conn)
	if client == nil {
		return errors.Reason("switch service client: fail to create client").Err()
	}

	if err := cft.ClientToScope(ctx, dut, client, cft.PassportSwitchName(dut)); err != nil {
		return errors.Annotate(err, "store cros-passport switch service client").Err()
	}
	return nil
}

// InitClients initializes clients for all passport services.
//
// Note: Passport container serves multiple services on single grpc endpoint.
func InitClients(ctx context.Context, conn *grpc.ClientConn, dut *tlw.Dut) error {
	if err := InitSwitchClient(ctx, dut, conn); err != nil {
		return err
	}

	return nil
}
