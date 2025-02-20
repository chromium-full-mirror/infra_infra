// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package platform

import (
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type Chromeos struct {
	execCommander executor.IExecCommander
}

func NewChromeosPlatform() IPlatform {
	return &Chromeos{
		execCommander: &executor.ExecCommander{},
	}
}
