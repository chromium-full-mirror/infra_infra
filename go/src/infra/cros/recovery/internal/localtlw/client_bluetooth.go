// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"

	"go.chromium.org/infra/cros/recovery/internal/localtlw/localproxy"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/servod"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	defaultBluetoothPeerServerPort = 9992
)

// CallBluetoothPeer executes a command on bluetooth-peer service.
func (c *tlwClient) CallBluetoothPeer(ctx context.Context, req *tlw.CallBluetoothPeerRequest) *tlw.CallBluetoothPeerResponse {
	// Translator to convert error to response structure.
	fail := func(err error) *tlw.CallBluetoothPeerResponse {
		return &tlw.CallBluetoothPeerResponse{
			Value: &xmlrpc.Value{
				ScalarOneof: &xmlrpc.Value_String_{
					String_: fmt.Sprintf("call servod %q: %s", req.GetResource(), err),
				},
			},
			Fault: true,
		}
	}
	// Check if the name was detected by loaded device.
	_, err := c.getDevice(ctx, req.GetResource())
	if err != nil {
		return fail(err)
	}
	val, err := servod.CallServod(ctx, &servod.ServodCallRequest{
		Host:        localproxy.BuildAddr(req.GetResource()),
		SSHProvider: c.sshProvider,
		Options: &tlw.ServodOptions{
			ServodPort: int32(defaultBluetoothPeerServerPort),
		},
		CallMethod:    req.GetMethod(),
		CallArguments: req.GetArgs(),
		// TODO(otabek): Change bluetooth peer's CallBluetoothPeerRequest to include timeout.
		CallTimeout: 30 * time.Second,
	})
	if err != nil {
		return fail(err)
	}
	return &tlw.CallBluetoothPeerResponse{
		Value: val,
		Fault: false,
	}
}
