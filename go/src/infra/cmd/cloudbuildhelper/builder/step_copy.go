// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package builder

import (
	"context"

	"go.chromium.org/infra/cmd/cloudbuildhelper/gitignore"
)

// runCopyBuildStep executes manifest.CopyBuildStep.
func runCopyBuildStep(ctx context.Context, inv *stepRunnerInv) error {
	return inv.addFilesToOutput(ctx,
		inv.BuildStep.CopyBuildStep.Copy,
		inv.BuildStep.Dest,
		gitignore.NewPatternExcluder(inv.BuildStep.CopyBuildStep.Ignore),
	)
}
