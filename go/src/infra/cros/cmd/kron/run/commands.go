// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package run holds all of the internal logic for the execution steps of a
// SuiteScheduler run.
package run

import (
	kronpb "go.chromium.org/chromiumos/infra/proto/go/test_platform/kron"
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"go.chromium.org/infra/cros/cmd/kron/builds"
	"go.chromium.org/infra/cros/cmd/kron/common"
)

// DDDCommand defines the schema that any 3D type command will need
// to follow.
type DDDCommand interface {
	Name() string
	FetchTriggeredConfigs() error
	FetchBuilds() error
	ScheduleRequests() error
}

// NewBuildCommand defines the schema that any NEW_BUILD type command will need
// to follow. The functions were designed such that the returned values feed
// into the next function.
type NewBuildCommand interface {
	Name() string
	FetchBuilds() ([]*kronpb.Build, error)
	FetchTriggeredConfigs([]*kronpb.Build) (map[*kronpb.Build][]*suschpb.SchedulerConfig, error)
	ScheduleRequests(map[*kronpb.Build][]*suschpb.SchedulerConfig) error
}

// TimedEventCommand defines the schema that any TIME_EVENTS type command will
// need to follow. The functions were designed such that the returned values
// feed into the next function.
type TimedEventCommand interface {
	Name() string
	FetchTriggeredConfigs(common.KronTime) (map[builds.RequiredBuild][]*suschpb.SchedulerConfig, error)
	FetchBuilds(map[builds.RequiredBuild][]*suschpb.SchedulerConfig) (map[*kronpb.Build][]*suschpb.SchedulerConfig, error)
	ScheduleRequests(map[*kronpb.Build][]*suschpb.SchedulerConfig) error
}

// RunDDDCommand runs an arbitrary 3D command.
func RunDDDCommand(command DDDCommand) error {
	err := command.FetchTriggeredConfigs()
	if err != nil {
		return err
	}

	err = command.FetchBuilds()
	if err != nil {
		return err
	}

	return command.ScheduleRequests()
}

// RunNewBuildCommand runs any arbitrary NewBuildCommand Interface.
func RunNewBuildCommand(command NewBuildCommand) error {
	kronBuilds, err := command.FetchBuilds()
	if err != nil {
		return err
	}
	if len(kronBuilds) == 0 {
		common.Stdout.Printf("No builds in the Release Pub/Sub queue")
		return nil
	}

	buildToConfigsMap, err := command.FetchTriggeredConfigs(kronBuilds)
	if err != nil {
		return err
	}

	return command.ScheduleRequests(buildToConfigsMap)
}

// RunTimedEventsCommand runs any arbitrary TimedEventCommand Interface.
func RunTimedEventsCommand(command TimedEventCommand, runTime common.KronTime) error {
	requiredBuildMap, err := command.FetchTriggeredConfigs(runTime)
	if err != nil {
		return err
	}

	if len(requiredBuildMap) == 0 {
		common.Stdout.Printf("No configs found triggered at %s", runTime.String())
		return nil
	}

	kronBuildMap, err := command.FetchBuilds(requiredBuildMap)
	if err != nil {
		return err
	}

	return command.ScheduleRequests(kronBuildMap)
}
