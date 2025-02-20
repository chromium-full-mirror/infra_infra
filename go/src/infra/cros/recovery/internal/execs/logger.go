// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package execs

import (
	"context"

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
func (ei *ExecInfo) CopyFrom(ctx context.Context, resourceName, srcFile, destDir string) error {
	return ei.runArgs.Access.CopyFileFrom(ctx, &tlw.CopyRequest{
		Resource:        resourceName,
		PathSource:      srcFile,
		PathDestination: destDir,
	})
}

// CopyDirectoryFrom copies a directory from resource to localhost.
func (ei *ExecInfo) CopyDirectoryFrom(ctx context.Context, resourceName, srcDir, destDir string) error {
	return ei.runArgs.Access.CopyDirectoryFrom(ctx, &tlw.CopyRequest{
		Resource:        resourceName,
		PathSource:      srcDir,
		PathDestination: destDir,
	})
}
