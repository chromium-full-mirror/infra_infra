// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// General utility related bits for provisioning.
package common_utils

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	KvmDevicePath           = "/dev/kvm"
	longrunningWaitInterval = time.Second * 1
)

func WaitLongRunningOp(ctx context.Context, log *log.Logger, operation *longrunning.Operation) (*anypb.Any, error) {
	for !operation.GetDone() {
		var err error
		if err = ctx.Err(); err != nil {
			return nil, fmt.Errorf("context deadline: %w", err)
		}
		log.Printf("\nPolling long running operation %v.", operation.GetName())
		time.Sleep(longrunningWaitInterval)
	}
	switch result := operation.GetResult().(type) {
	case *longrunning.Operation_Error:
		return nil, fmt.Errorf(result.Error.Message)
	case *longrunning.Operation_Response:
		return result.Response, nil
	default:
		return nil, fmt.Errorf("unexpected result type for long running operation")
	}
}
