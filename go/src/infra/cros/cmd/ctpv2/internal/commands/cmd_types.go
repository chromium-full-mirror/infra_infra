// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"infra/cros/cmd/common_lib/interfaces"
)

// All supported command types.
const (
	TranslateV1toV2RequestType     interfaces.CommandType = "TranslateV1toV2Request"
	PrepareFilterContainersCmdType interfaces.CommandType = "PrepareFilterContainers"
	FilterExecutionCmdType         interfaces.CommandType = "FilterExecution"
	FilterStopCmdType              interfaces.CommandType = "FilterStop"
	TranslateRequestType           interfaces.CommandType = "TranslateRequest"
	MiddleoutExecutionType         interfaces.CommandType = "MiddleOutExecution"
	ScheduleTasksCmdType           interfaces.CommandType = "ScheduleTasks"
	GenerateTrv2RequestsCmdType    interfaces.CommandType = "GenerateTrv2Requests"
	SummarizeCmdType               interfaces.CommandType = "Summarize"
	AlStatusUpdateCmdType          interfaces.CommandType = "AlStatusUpdate"
)
