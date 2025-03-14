// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Executor defines a state initializer for each state. Usable by server to start.
// Under normal conditions this would be part of service, but go being go, it
// would create an impossible cycle
package server

import (
	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	common_utils "go.chromium.org/chromiumos/test/provision/v2/common-utils"
)

type ProvisionExecutor interface {
	GetFirstState(dut *lab_api.Dut, dutClient api.DutServiceClient, servoNexusAddr string, req *api.InstallRequest) (common_utils.ServiceState, error)
	Validate(req *api.ProvisionStartupRequest) error
}
