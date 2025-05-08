// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// commontypes contains types common to many infra projects.
package commontypes

import (
	"sync/atomic"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// ContainerManagementRequest passes along any necessary information
// needed from the ContainerManager.
type ContainerManagementRequest struct {
	Container *api.ContainerRequest
	// Carry the instruction enum for the manager to process.
	ContainerInstruction ContainerInstruction
	// Channel for indicating the instruction is done being processed.
	ResponseChannel chan *ContainerManagementResponse
}

type ContainerManagementResponse struct {
	Address     *labapi.IpEndpoint
	LogLocation string
}

type RunningContainerInfo struct {
	ContainerInstance *interfaces.ContainerInterface
	Address           *labapi.IpEndpoint
	LogLocation       string
	CountUsers        atomic.Int64
}

type ContainerLogInfo struct {
	Name        string
	LogLocation string
}

type ContainerInstruction = int64

const (
	FinishedUsingContainer ContainerInstruction = 0
	ProvideContainer       ContainerInstruction = 1
	FinishedUsingManager   ContainerInstruction = 2
)
