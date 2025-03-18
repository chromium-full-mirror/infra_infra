// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Provides functionality to connect to other CFT services.
package cli

import (
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
)

func connectToDutServer(dutServAddr string) (api.DutServiceClient, error) {
	dutConn, err := grpc.Dial(dutServAddr, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Dut Server")
	}
	return api.NewDutServiceClient(dutConn), nil
}

func connectToCrosServod(crosServodAddr string) (api.ServodServiceClient, error) {
	servodConn, err := grpc.Dial(crosServodAddr, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to cros-servod")
	}
	servodServiceClient := api.NewServodServiceClient(servodConn)
	return servodServiceClient, nil
}
