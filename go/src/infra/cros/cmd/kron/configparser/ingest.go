// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package configparser implements logic to handle SuiteScheduler configuration files.
package configparser

import (
	"fmt"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
	"go.chromium.org/luci/auth"

	"go.chromium.org/infra/cros/cmd/kron/common"
)

// IsFirmware returns if the given config is a firmware config.
func IsFirmware(config *suschpb.SchedulerConfig) bool {
	// If the config targets firmware suites then skip ingesting the config.
	if config.GetFirmwareEcRo() != nil || config.GetFirmwareEcRw() != nil || config.GetFirmwareRo() != nil || config.GetFirmwareRw() != nil || config.GetFirmwareBoardName() != "" {
		return true
	}

	return false
}

// IsMultiDut returns if the given config is a multi-dut config.
func IsMultiDut(config *suschpb.SchedulerConfig) bool {
	// If the config targets multi-dut suites then skip ingesting the config.
	if config.GetTargetOptions().GetMultiDutsBoardsList() != nil || config.GetTargetOptions().GetMultiDutsModelsList() != nil {
		return true
	}

	return false
}

// IngestSuSchConfigs takes in all of the raw Suite Scheduler and Lab configs and ingests
// them into a more usage structure.
func IngestSuSchConfigs(configs ConfigList, lab *LabConfigs) (*SuiteSchedulerConfigs, error) {
	configDS := &SuiteSchedulerConfigs{
		configList:            ConfigList{},
		newBuildList:          []*suschpb.SchedulerConfig{},
		multiDUTList:          []*suschpb.SchedulerConfig{},
		newBuild3dList:        ConfigList{},
		newBuild3dMap:         map[*suschpb.SchedulerConfig]map[BuildTarget]bool{},
		newBuildMap:           map[BuildTarget]ConfigList{},
		multiDUTMap:           map[BuildTarget]ConfigList{},
		configTargets:         map[string]TargetOptions{},
		multiDUTConfigTargets: map[string]map[string][]*MultiDutTargetOptions{},
		configMap:             map[TestPlanName]*suschpb.SchedulerConfig{},
		dailyMap:              map[int]ConfigList{},
		weeklyMap:             map[int]HourMap{},
		fortnightlyMap:        map[int]HourMap{},
		nDaysMap:              map[int]HourMap{},
	}

	for _, config := range configs {
		var multiDUTTargets map[string][]*MultiDutTargetOptions
		var targetOptions TargetOptions
		var err error

		if IsMultiDut(config) {
			multiDUTTargets, err = GetMultiDutTargets(config, lab)
			if err != nil {
				return nil, err
			}
			configDS.multiDUTConfigTargets[config.Name] = multiDUTTargets
		} else {
			targetOptions, err = GetTargetOptions(config, lab)
			if err != nil {
				return nil, err
			}
			// Cache the calculated target options.
			configDS.configTargets[config.Name] = targetOptions
		}

		// If the config targets firmware suites then skip ingesting the config.
		if IsFirmware(config) {
			continue
		}

		// Add the configuration to the map which holds stores information on
		// its LaunchProfile type
		switch config.LaunchCriteria.LaunchProfile {
		case suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD:
			configDS.addConfigToNewBuildMap(config, targetOptions)
		case suschpb.SchedulerConfig_LaunchCriteria_DAILY:
			err := configDS.addConfigToDailyMap(config)
			if err != nil {
				return nil, err
			}
		case suschpb.SchedulerConfig_LaunchCriteria_WEEKLY:
			err := configDS.addConfigToWeeklyMap(config)
			if err != nil {
				return nil, err
			}
		case suschpb.SchedulerConfig_LaunchCriteria_FORTNIGHTLY:
			err := configDS.addConfigToFortnightlyMap(config)
			if err != nil {
				return nil, err
			}
		case suschpb.SchedulerConfig_LaunchCriteria_N_DAYS:
			err := configDS.addConfigToNDayMap(config)
			if err != nil {
				return nil, err
			}
		case suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD_3D:
			configDS.addConfigToNewBuild3dMap(config, targetOptions)
		case suschpb.SchedulerConfig_LaunchCriteria_MULTI_DUT:
			configDS.addConfigToMultiDUTMap(config, multiDUTTargets)
		default:
			return nil, fmt.Errorf("unsupported or unknown launch profile encountered in config %s", config.Name)
		}
	}

	return configDS, nil
}

// IngestLabConfigs takes in all of the raw Lab configs and ingests
// them into a more usage structure.
func IngestLabConfigs(labConfig *suschpb.LabConfig) *LabConfigs {
	tempConfig := &LabConfigs{
		Boards:        map[Board]*BoardEntry{},
		Models:        map[Model]*BoardEntry{},
		AndroidBoards: map[Board]*BoardEntry{},
		AndroidModels: map[Model]*BoardEntry{},
	}

	for _, board := range labConfig.Boards {
		entry := &BoardEntry{
			board: board,
		}
		tempConfig.Boards[Board(board.Name)] = entry

		for _, model := range board.Models {
			tempConfig.Models[Model(model)] = entry
		}
	}

	// Android hardware is used in multi-DUT testing only and should be
	// separated so that it does not get indirectly included in normal CrOS
	// configs.
	for _, board := range labConfig.AndroidBoards {
		entry := &BoardEntry{
			board: board,
		}
		tempConfig.AndroidBoards[Board(board.Name)] = entry

		for _, model := range board.Models {
			tempConfig.AndroidModels[Model(model)] = entry
		}
	}

	return tempConfig
}

// BytesToLabProto takes a JSON formatted string and transforms it into an
// infrapb.LabConfig object.
func BytesToLabProto(configsBuffer []byte) (*suschpb.LabConfig, error) {
	configs := &suschpb.LabConfig{}

	err := common.ProtoJSONUnmarshaller.Unmarshal(configsBuffer, configs)
	if err != nil {
		return nil, err
	}

	return configs, nil
}

// BytesToSchedulerProto takes a JSON formatted string and transforms it into an
// infrapb.SchedulerCfg object.
func BytesToSchedulerProto(configsBuffer []byte) (*suschpb.SchedulerCfg, error) {
	configs := &suschpb.SchedulerCfg{}

	err := common.ProtoJSONUnmarshaller.Unmarshal(configsBuffer, configs)
	if err != nil {
		return nil, err
	}

	return configs, nil
}

// FetchLabConfigs fetches and ingests the lab configs. It will
// determine where to read the configs from based on the user provided flags.
func FetchLabConfigs(path string, authOpts *auth.Options) (*LabConfigs, error) {
	var err error
	var labBytes []byte

	// If a file path was passed in for the Lab then parse that file. If not
	// then fetch the LabConfig from the ToT .cfg and ingest it in memory.
	if path != common.DefaultString {
		labBytes, err = common.ReadLocalFile(path)
		if err != nil {
			return nil, err
		}
	} else {
		labBytes, err = common.FetchFileFromInternalURL(common.LabCfgURL, authOpts)
		if err != nil {
			return nil, err
		}

	}

	labProto, err := BytesToLabProto(labBytes)
	if err != nil {
		return nil, err
	}

	labConfigs := IngestLabConfigs(labProto)

	return labConfigs, nil
}

// FetchSchedulerConfigs fetches and ingests the SuiteScheduler configs. It will
// determine where to read the configs from based on the user provided flags.
func FetchSchedulerConfigs(path string, labConfigs *LabConfigs, authOpts *auth.Options) (*SuiteSchedulerConfigs, error) {
	var err error
	var schedulerBytes []byte

	// If a file path was passed in for the ScheduleConfigs then parse that file. If not
	// then fetch the SuiteSchedulerConfigs from the ToT .cfg and ingest it in memory.
	if path != common.DefaultString {
		schedulerBytes, err = common.ReadLocalFile(path)
		if err != nil {
			return nil, err
		}

	} else {
		schedulerBytes, err = common.FetchFileFromInternalURL(common.SuiteSchedulerCfgURL, authOpts)
		if err != nil {
			return nil, err
		}

	}

	// Convert from []byte to a usable object type.
	scheduleProto, err := BytesToSchedulerProto(schedulerBytes)
	if err != nil {
		return nil, err
	}

	// Ingest the configs into a data structure which easier and more efficient
	// to search.
	schedulerConfigs, err := IngestSuSchConfigs(scheduleProto.Configs, labConfigs)
	if err != nil {
		return nil, err
	}

	return schedulerConfigs, nil
}
