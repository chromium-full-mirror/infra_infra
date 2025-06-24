// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/logger"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// NewLogger returns logger.
func (ei *ExecInfo) NewLogger() logger.Logger {
	return ei.runArgs.Logger
}

// GetLogRoot returns path to logs directory.
func (ei *ExecInfo) GetLogRoot() string {
	return ei.runArgs.LogRoot
}

// CopyFrom copies files from resource to localhost.
func (ei *ExecInfo) CopyFrom(ctx context.Context, runner components.Runner, resourceName, srcFile, destDir string, timeout time.Duration) error {
	return ei.runArgs.Access.CopyFileFrom(ctx, &tlw.CopyRequest{
		Resource:        resourceName,
		PathSource:      srcFile,
		PathDestination: destDir,
		Timeout:         durationpb.New(timeout),
	}, runner)
}

// CopyDirectoryFrom copies a directory from resource to localhost.
func (ei *ExecInfo) CopyDirectoryFrom(ctx context.Context, runner components.Runner, resourceName, srcDir, destDir string, timeout time.Duration) error {
	return ei.runArgs.Access.CopyDirectoryFrom(ctx, &tlw.CopyRequest{
		Resource:        resourceName,
		PathSource:      srcDir,
		PathDestination: destDir,
		Timeout:         durationpb.New(timeout),
	}, runner)
}
