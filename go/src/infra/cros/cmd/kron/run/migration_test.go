// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	"testing"

	"github.com/google/uuid"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
)

func TestIsAllowedNewBuildAllow(t *testing.T) {
	t.Parallel()
	config := suschpb.SchedulerConfig{
		LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
			LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD,
		},
	}

	allowed := isAllowed(&config)

	if !allowed {
		t.Errorf("Config was rejected incorrectly.")
	}
}

func TestIsAllowedOnListAllow(t *testing.T) {
	// No names to test on.
	if len(allowedConfigs) == 0 {
		return
	}

	configName := ""

	for key := range allowedConfigs {
		configName = key
		break
	}

	config := suschpb.SchedulerConfig{
		Name:           configName,
		LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_DAILY},
	}

	allowed := isAllowed(&config)

	if !allowed {
		t.Errorf("Config was rejected incorrectly.")
	}
}

func TestIsAllowedFirmwareSkip(t *testing.T) {
	t.Parallel()

	configs := []*suschpb.SchedulerConfig{
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD},
			FirmwareRo:     &suschpb.SchedulerConfig_FirmwareRoBuildSpec{},
			FirmwareRw:     nil,
			FirmwareEcRo:   nil,
			FirmwareEcRw:   nil,
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD},
			FirmwareRo:     nil,
			FirmwareRw:     &suschpb.SchedulerConfig_FirmwareRwBuildSpec{},
			FirmwareEcRo:   nil,
			FirmwareEcRw:   nil,
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD},
			FirmwareRo:     &suschpb.SchedulerConfig_FirmwareRoBuildSpec{},
			FirmwareRw:     &suschpb.SchedulerConfig_FirmwareRwBuildSpec{},
			FirmwareEcRo:   nil,
			FirmwareEcRw:   nil,
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD},
			FirmwareRo:     &suschpb.SchedulerConfig_FirmwareRoBuildSpec{},
			FirmwareRw:     nil,
			FirmwareEcRo:   nil,
			FirmwareEcRw:   &suschpb.SchedulerConfig_FirmwareEcRwBuildSpec{},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD},
			FirmwareRo:     nil,
			FirmwareRw:     &suschpb.SchedulerConfig_FirmwareRwBuildSpec{},
			FirmwareEcRo:   nil,
			FirmwareEcRw:   &suschpb.SchedulerConfig_FirmwareEcRwBuildSpec{},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD},
			FirmwareRo:     &suschpb.SchedulerConfig_FirmwareRoBuildSpec{},
			FirmwareRw:     &suschpb.SchedulerConfig_FirmwareRwBuildSpec{},
			FirmwareEcRo:   nil,
			FirmwareEcRw:   &suschpb.SchedulerConfig_FirmwareEcRwBuildSpec{},
		},
		{
			FirmwareBoardName: "fwBoard",
		},
	}

	for _, config := range configs {
		allowed := isAllowed(config)

		if allowed {
			t.Errorf("Config was accepted incorrectly.")
		}

	}
}
func TestIsAllowedMultidutSkip(t *testing.T) {
	t.Parallel()

	configs := []*suschpb.SchedulerConfig{
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_DAILY,
			},
			TargetOptions: &suschpb.SchedulerConfig_TargetOptions{
				MultiDutsBoardsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByBoard{},
				MultiDutsModelsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByModel{},
			},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD,
			},
			TargetOptions: &suschpb.SchedulerConfig_TargetOptions{
				MultiDutsBoardsList: nil,
				MultiDutsModelsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByModel{},
			},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_WEEKLY,
			},
			TargetOptions: &suschpb.SchedulerConfig_TargetOptions{
				MultiDutsBoardsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByBoard{},
				MultiDutsModelsList: nil,
			},
		},
		{
			Name: "",
		},
	}

	for _, config := range configs {
		allowed := isAllowed(config)

		if allowed {
			t.Errorf("Config was accepted incorrectly.")
		}

	}
}

func TestIsAllowedMultidutAllowed(t *testing.T) {
	t.Parallel()

	configs := []*suschpb.SchedulerConfig{
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_MULTI_DUT,
			},
			TargetOptions: &suschpb.SchedulerConfig_TargetOptions{
				MultiDutsBoardsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByBoard{},
				MultiDutsModelsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByModel{},
			},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_MULTI_DUT,
			},
			TargetOptions: &suschpb.SchedulerConfig_TargetOptions{
				MultiDutsBoardsList: nil,
				MultiDutsModelsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByModel{},
			},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_MULTI_DUT,
			},
			TargetOptions: &suschpb.SchedulerConfig_TargetOptions{
				MultiDutsBoardsList: []*suschpb.SchedulerConfig_TargetOptions_MultiDutsByBoard{},
				MultiDutsModelsList: nil,
			},
		},
		{
			LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{
				LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_MULTI_DUT,
			},
			Name: "",
		},
	}

	for _, config := range configs {
		allowed := isAllowed(config)

		if !allowed {
			t.Errorf("Config was skipped incorrectly.")
		}

	}
}

func TestIsAllowedNotOnListSkip(t *testing.T) {
	// No names to test on.
	if len(allowedConfigs) == 0 {
		return
	}

	configName := uuid.New().String()

	if _, ok := allowedConfigs[configName]; ok {
		t.Errorf("config name %s is in the allowed configs, it should not be.", configName)
	}

	config := suschpb.SchedulerConfig{
		Name:           configName,
		LaunchCriteria: &suschpb.SchedulerConfig_LaunchCriteria{LaunchProfile: suschpb.SchedulerConfig_LaunchCriteria_DAILY},
	}

	allowed := isAllowed(&config)

	if allowed {
		t.Errorf("Config was accepted incorrectly.")
	}
}
