// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// General utility related bits for provisioning.
package common_utils

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/longrunning"
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
		return nil, fmt.Errorf("%s", result.Error.Message)
	case *longrunning.Operation_Response:
		return result.Response, nil
	default:
		return nil, fmt.Errorf("unexpected result type for long running operation")
	}
}

// Retry implements a basic retrier that will attempt up to retryCount times
// before returning an error.
func Retry(logger *log.Logger, retryCount int, funcName string, f func() error) error {
	var err error
	for attempt := range retryCount {
		logger.Printf("%s: attempt #%d...\n", funcName, attempt)
		// If f() succeeds then we do not need to retry again.
		err = f()
		if err == nil {
			logger.Printf("%s: attempt #%d...SUCCESS\n", funcName, attempt)
			break
		}

		logger.Printf("%s: attempt #%d...FAILED: %s\n", funcName, attempt, err.Error())
	}

	return err
}
