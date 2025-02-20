// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package outputprops sets up the infrastructure to write custom types to the
// LUCIEXE output properties.
package outputprops

import (
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// The luciexe/build API will handle the actual definition of these functions.
// We just need to define a function pointer to pass in.

// UpdateItems is used capture the items that we need to update for ATP.
type UpdateItems struct {
	EncodedTestJobMsg      string
	EncodedTestJobEventMsg string
	// Test job event encapsulates test job msg
	TestJobEventMsgJSON *common.TestJobEventMessage
}

type SummaryMap map[string]*UpdateItems

var CTPv2AtpUpdate = build.RegisterOutputProperty[SummaryMap]("$ctpv2/atp_update")
