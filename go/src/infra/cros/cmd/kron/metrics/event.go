// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package metrics holds all the schemas and utilities to handle metrics for
// Kron.
package metrics

import (
	"fmt"

	"github.com/google/uuid"

	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"infra/cros/cmd/kron/common"
)

// GenerateEventMessage builds a metric event.
func GenerateEventMessage(config *suschpb.SchedulerConfig, schedulingDecision *kronpb.SchedulingDecision, bbid int64, buildUUID, board, model, buildTarget string) (*kronpb.Event, error) {
	if runID == "" {
		return nil, fmt.Errorf("runID cannot be empty")
	}

	return &kronpb.Event{
		RunUuid:     runID,
		EventUuid:   uuid.NewString(),
		ConfigName:  config.GetName(),
		SuiteName:   config.GetSuite(),
		EventTime:   common.TimestamppbNowWithoutNanos(),
		Decision:    schedulingDecision,
		Bbid:        bbid,
		BuildUuid:   buildUUID,
		Board:       board,
		Model:       model,
		BuildTarget: buildTarget,
	}, nil
}
