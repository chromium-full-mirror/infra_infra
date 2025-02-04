// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/luci/common/logging"
)

// ProcessDoneLro process a long running operation that is done and parses the result to a proto.
// Don't use this, use WaitLro instead.
func ProcessDoneLro(ctx context.Context, lro *longrunning.Operation) (*anypb.Any, error) {
	if lro == nil {
		return nil, fmt.Errorf("provided lro is nil")
	}

	// Wait for the operation to be done
	for !lro.Done {
		return nil, fmt.Errorf("lro is not done")
	}

	// Check operation result
	switch x := lro.Result.(type) {
	case *longrunning.Operation_Error:
		logging.Infof(ctx, "LRO ERROR: %s", x.Error.Message)
		return nil, fmt.Errorf("%s", x.Error.Message)
	case *longrunning.Operation_Response:
		logging.Infof(ctx, "LRO RESPONSE: %s", x.Response)
		return x.Response, nil
	default:
		return nil, fmt.Errorf("unexpected lro result type")
	}
}

// WaitLro waits for a long running operation to complete and parses the result to a proto
func WaitLro(ctx context.Context, lroClient longrunning.OperationsClient, lro *longrunning.Operation) (*anypb.Any, error) {
	if lro == nil {
		return nil, fmt.Errorf("provided lro is nil")
	}

	// Wait for the operation to be done
	for !lro.Done {
		var err error
		if err = ctx.Err(); err != nil {
			return nil, fmt.Errorf("context deadline: %w", err)
		}
		logging.Infof(ctx, "POLLING LRO: %s", lro.GetName())
		lro, err = lroClient.WaitOperation(ctx, &longrunning.WaitOperationRequest{
			Name:    lro.GetName(),
			Timeout: durationpb.New(LroTimeout),
		})
		if err != nil {
			return nil, fmt.Errorf("WaitOperation failed: %w", err)
		}
	}

	// Check operation result
	switch x := lro.Result.(type) {
	case *longrunning.Operation_Error:
		logging.Infof(ctx, "LRO ERROR: %s", x.Error.Message)
		return nil, fmt.Errorf("%s", x.Error.Message)
	case *longrunning.Operation_Response:
		logging.Infof(ctx, "LRO RESPONSE: %s", x.Response)
		return x.Response, nil
	default:
		return nil, fmt.Errorf("unexpected lro result type")
	}
}
