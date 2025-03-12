// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Common helper methods
package common_utils

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.chromium.org/chromiumos/test/execution/errors"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// BucketJoin is equivalent to Path.Join(), but for gs buckets.
// This is necessary as the BUF removes double-slashes, which mangles the URI.
// Returns the bucket and append as a formed bucket URI.
func BucketJoin(bucket string, append string) string {
	if strings.HasPrefix(bucket, "gs://") {
		bucket = bucket[5:]
	}
	return fmt.Sprintf("gs://%s", path.Join(bucket, append))
}

func WrapStringInAny(s string) *anypb.Any {
	stringValue := wrapperspb.String(s)

	// Pack the StringValue into an Any
	anyValue, err := anypb.New(stringValue)
	if err != nil {
		return nil
	}
	return anyValue
}

// ExecuteStateMachine runs a specific state machine
func ExecuteStateMachine(ctx context.Context, cs ServiceState, log *log.Logger) (api.InstallResponse_Status, *anypb.Any, error) {
	if cs == nil {
		msg := "input State cannot be empty in ExecuteStateMachine"
		return api.InstallResponse_STATUS_PROVISIONING_FAILED, nil, stderrors.New(msg)
	}
	for {
		md, status, err := cs.Execute(ctx, log)
		if err != nil {
			return status, md, fmt.Errorf("failed provisioning on %s step, %w", cs.Name(), err)
		}
		cs = cs.Next()
		if cs == nil {
			return api.InstallResponse_STATUS_SUCCESS, md, nil
		}
	}
}

// CreateLogFile creates a file and its parent directory for logging purpose.
func CreateLogFile(fullPath string) (*os.File, error) {
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create directory %v: %w", fullPath, err))
	}

	logFullPathName := filepath.Join(fullPath, "log.txt")

	// Log the full output of the command to disk.
	logFile, err := os.Create(logFullPathName)
	if err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create file %v: %w", fullPath, err))
	}
	return logFile, nil
}

// NewLogger creates a logger. Using go default logger for now.
func NewLogger(logFile *os.File) *log.Logger {
	mw := io.MultiWriter(logFile, os.Stderr)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}
