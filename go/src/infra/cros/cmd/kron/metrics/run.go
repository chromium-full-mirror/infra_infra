// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package metrics holds all the schemas and utilities to handle metrics for
// Kron.
package metrics

import (
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"

	"go.chromium.org/infra/cros/cmd/kron/common"
)

// Pseudo-immutable package variables.
var (
	runID     string
	startTime *timestamppb.Timestamp
	endTime   *timestamppb.Timestamp
)

// SetSuiteSchedulerRunID sets the package variable for the runID. Since we cannot set a
// compile-time constant for a uuid, this setter incorporates logic to make it pseudo-immutable.
func SetSuiteSchedulerRunID(id string) error {
	if runID != "" {
		return fmt.Errorf("suite scheduler runId already set to %s", runID)
	}

	if id == common.DefaultString {
		runID = uuid.NewString()
	} else {
		runID = id
	}

	return nil
}

// GetRunID returns the package level runID
func GetRunID() string {
	return runID
}

// SetStartTime sets the package variable for the startTime. Since we cannot set a
// compile-time constant for startTime, this setter incorporates logic to make it pseudo-immutable.
func SetStartTime() error {
	if startTime != nil {
		return fmt.Errorf("suite scheduler startTime already set to %s", startTime.String())
	}

	startTime = common.TimestamppbNowWithoutNanos()

	return nil
}

// GetStartTime returns the package level startTime
func GetStartTime() *timestamppb.Timestamp {
	return startTime
}

// SetEndTime sets the package variable for the endTime. Since we cannot set a
// compile-time constant for endTime, this setter incorporates logic to make it pseudo-immutable.
func SetEndTime() error {
	if endTime != nil {
		return fmt.Errorf("suite scheduler endTime already set to %s", endTime.String())
	}

	endTime = common.TimestamppbNowWithoutNanos()

	return nil
}

// GetEndTime returns the package level endTime
func GetEndTime() *timestamppb.Timestamp {
	return endTime
}

// GenerateRunMessage returns a SchedulingMetric for the current SuiteScheduler
// run.
func GenerateRunMessage() *kronpb.Run {
	return &kronpb.Run{
		RunUuid:   runID,
		StartTime: startTime,
		EndTime:   endTime,
	}
}
