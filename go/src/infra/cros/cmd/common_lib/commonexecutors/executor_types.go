// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commonexecutors contains the definitions of the commonly used
// executors.
package commonexecutors

import (
	"infra/cros/cmd/common_lib/interfaces"
)

// All supported common executor types.
const (
	CtrExecutorType       interfaces.ExecutorType = "CtrExecutor"
	ContainerExecutorType interfaces.ExecutorType = "ContainerExecutor"

	// For testing purpose only
	NoExecutorType interfaces.ExecutorType = "NoExecutor"
)
