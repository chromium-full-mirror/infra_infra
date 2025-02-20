// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commoncommands stores some command commands ran by ctpv2/filters.
package commoncommands

import (
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
)

// All supported common command types.
const (
	// Ctr service related commands
	ContainerStartCmdType       interfaces.CommandType = "ContainerStart"
	ContainerCloseLogsCmdType   interfaces.CommandType = "ContainerCloseLogs"
	ContainerReadLogsCmdType    interfaces.CommandType = "ContainerReadLogs"
	CtrServiceStartAsyncCmdType interfaces.CommandType = "CtrServiceStartAsync"
	CtrServiceStopCmdType       interfaces.CommandType = "CtrServiceStop"
	GcloudAuthCmdType           interfaces.CommandType = "GcloudAuth"

	// For testing purposes only
	UnSupportedCmdType interfaces.CommandType = "UnSupportedCmd"
)
