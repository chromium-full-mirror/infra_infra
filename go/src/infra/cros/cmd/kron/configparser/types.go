// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package configparser implements logic to handle SuiteScheduler configuration files.
package configparser

import (
	"fmt"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"
)

type (
	/*
	 * SuiteSchedulerConfigs based types
	 */

	TestPlanName string

	HourMap map[int]ConfigList

	ConfigList []*suschpb.SchedulerConfig

	/*
	 * LabConfig based types
	 */

	// These fields ensure that we aren't using magic strings elsewhere in the
	// code.

	Board   string
	Variant string
	Model   string
	// BuildTarget is in the form board(-<variant>) with the variant being optional.
	BuildTarget string

	// TargetOptions is a map of board->TargetOption to easily retrieve information.
	TargetOptions map[Board]*TargetOption
)

// TargetOption is a struct which contains all information for a targeted piece
// of hardware to be tested.
type TargetOption struct {
	Board        string
	Models       []string
	Variants     []string
	VariantsOnly bool
}

// MultiDUTTarget contains the target definitions required by MULTI_DUT targets.
type MultiDUTTarget struct {
	BuildTarget string
	Board       string
	Model       string

	// NOTE: The following fields are exclusive to Android devices.
	IsAndroid           bool
	AndroidImageVersion string
	GMSCorePackage      string
}

// MultiDutTargetOptions defines a 1-to-N pairing set defined in a MULTI_DUT
// config.
type MultiDutTargetOptions struct {
	Primary *MultiDUTTarget

	Secondaries []*MultiDUTTarget
}

// LabConfigs is a wrapper to provide quick access to boards and models in the lab.
type LabConfigs struct {
	Models        map[Model]*BoardEntry
	Boards        map[Board]*BoardEntry
	AndroidBoards map[Board]*BoardEntry
	AndroidModels map[Model]*BoardEntry
}

// BoardEntry is a wrapper on the infrapb Board type.
type BoardEntry struct {
	board *suschpb.Board
}

func (b *BoardEntry) GetBoard() *suschpb.Board {
	return b.board
}

func (b *BoardEntry) GetName() string {
	return b.board.GetName()
}

// SuiteSchedulerConfigs represents the ADS which will be used for accessing
// SuiteScheduler configurations.
type SuiteSchedulerConfigs struct {
	// Array of all configs. Allows quick access to all configurations.
	configList ConfigList

	// Array of all NEW_BUILD configs. Allows quick access to all new build configurations.
	newBuildList ConfigList

	// Array of multiDUT configs. Allows quick access to all multiDUT configurations.
	multiDUTList ConfigList

	// Array of all NEW_BUILD 3d configs. Allows quick access to all NEW_BUILD 3d configurations.
	newBuild3dList ConfigList

	// newBuild3dMap stores a mapping of all build targets for a given 3d config.
	newBuild3dMap map[*suschpb.SchedulerConfig]map[BuildTarget]bool

	// newBuildMap stores a mapping of build target to relevant NEW_BUILD
	// configs. Allows for retrieval of configs when searching by build target.
	newBuildMap map[BuildTarget]ConfigList

	// multiDUTMap stores a mapping of build target to relevant MULTI_DUT
	// configs. Allows for retrieval of configs when searching by build target.
	multiDUTMap map[BuildTarget]ConfigList

	// configTargets will provided a cached version of the, computationally
	// expensive to build, target options per config.
	configTargets map[string]TargetOptions

	// multiDUTConfigTargets will provided a cached version of the, computationally
	// expensive to build, target options per config. This map is specific to
	// MULTI_DUT configs as they calculate target options differently than
	// "normal" CrOS configs.
	multiDUTConfigTargets map[string]map[string][]*MultiDutTargetOptions

	// This map provides a quick direct access option for fetching configs by name.
	configMap map[TestPlanName]*suschpb.SchedulerConfig

	// The following maps correspond to the specific set of TimedEvents the
	// configuration is of.
	dailyMap       HourMap
	weeklyMap      map[int]HourMap
	fortnightlyMap map[int]HourMap
}

// addConfigToNewBuildMap takes a newBuild configuration and inserts it into the
// appropriate tracking lists.
func (s *SuiteSchedulerConfigs) addConfigToNewBuildMap(config *suschpb.SchedulerConfig, targetOptions TargetOptions) {
	// Fetch all build buildTargets which can trigger this configuration.
	buildTargets := GetBuildTargetsForAllTargets(targetOptions)

	for _, target := range buildTargets {
		// Add entry if no config with this build target has been
		// ingested yet.
		if _, ok := s.newBuildMap[target]; !ok {
			s.newBuildMap[target] = ConfigList{}
		}

		// Add the pointer to the config into the tracking set.
		s.newBuildMap[target] = append(s.newBuildMap[target], config)
	}

	// Add to the array tracking all SuSch configs.
	s.configList = append(s.configList, config)
	s.newBuildList = append(s.newBuildList, config)

	// Add to the direct access map.
	s.configMap[TestPlanName(config.Name)] = config
}

// addConfigToNewBuildMap takes a newBuild configuration and inserts it into the
// appropriate tracking lists.
func (s *SuiteSchedulerConfigs) addConfigToMultiDUTMap(config *suschpb.SchedulerConfig, targetOptions map[string][]*MultiDutTargetOptions) {
	for buildTargetName := range targetOptions {
		if _, ok := s.multiDUTMap[BuildTarget(buildTargetName)]; !ok {
			s.multiDUTMap[BuildTarget(buildTargetName)] = ConfigList{}
		}

		s.multiDUTMap[BuildTarget(buildTargetName)] = append(s.multiDUTMap[BuildTarget(buildTargetName)], config)
	}

	// Add to the array tracking all SuSch configs.
	s.configList = append(s.configList, config)
	s.multiDUTList = append(s.multiDUTList, config)

	// Add to the direct access map.
	s.configMap[TestPlanName(config.GetName())] = config
}

// addConfigToDailyMap takes a daily configuration and inserts it into the
// appropriate tracking lists.
func (s *SuiteSchedulerConfigs) addConfigToDailyMap(config *suschpb.SchedulerConfig) error {
	configHour := int(config.LaunchCriteria.Hour)
	err := isHourCompliant(configHour)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Ingesting %s encountered %s", config.Name, err))
	}

	if _, ok := s.dailyMap[configHour]; !ok {
		s.dailyMap[configHour] = ConfigList{}
	}

	s.dailyMap[configHour] = append(s.dailyMap[configHour], config)

	// Add to the array tracking all SuSch configs.
	s.configList = append(s.configList, config)

	// Add to the direct access map.
	s.configMap[TestPlanName(config.Name)] = config

	return nil
}

// addConfigToWeeklyMap takes a weekly configuration and inserts it into the
// appropriate tracking lists.
func (s *SuiteSchedulerConfigs) addConfigToWeeklyMap(config *suschpb.SchedulerConfig) error {
	configDay := int(config.LaunchCriteria.Day)
	err := isDayCompliant(configDay, false)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Ingesting %s encountered %s", config.Name, err))
	}
	configHour := int(config.LaunchCriteria.Hour)
	err = isHourCompliant(configHour)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Ingesting %s encountered %s", config.Name, err))
	}

	if _, ok := s.weeklyMap[configDay]; !ok {
		s.weeklyMap[configDay] = make(HourMap)
	}

	dayMap := s.weeklyMap[configDay]

	if _, ok := dayMap[configHour]; !ok {
		dayMap[configHour] = ConfigList{}
	}
	dayMap[configHour] = append(dayMap[configHour], config)

	// Add to the array tracking all SuSch configs.
	s.configList = append(s.configList, config)

	// Add to the direct access map.
	s.configMap[TestPlanName(config.Name)] = config

	return nil
}

// addConfigToFortnightlyMap takes a fortnightly configuration and inserts it into the
// appropriate tracking lists.
func (s *SuiteSchedulerConfigs) addConfigToFortnightlyMap(config *suschpb.SchedulerConfig) error {
	configDay := int(config.LaunchCriteria.Day)
	err := isDayCompliant(configDay, true)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Ingesting %s encountered %s", config.Name, err))
	}
	configHour := int(config.LaunchCriteria.Hour)
	err = isHourCompliant(configHour)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Ingesting %s encountered %s", config.Name, err))
	}

	if _, ok := s.fortnightlyMap[configDay]; !ok {
		s.fortnightlyMap[configDay] = make(HourMap)
	}

	dayMap := s.fortnightlyMap[configDay]

	if _, ok := dayMap[configHour]; !ok {
		dayMap[configHour] = ConfigList{}
	}
	dayMap[configHour] = append(dayMap[configHour], config)

	// Add to the array tracking all SuSch configs.
	s.configList = append(s.configList, config)

	// Add to the direct access map.
	s.configMap[TestPlanName(config.Name)] = config

	return nil
}

// addConfigToNewBuild3dMap takes a 3d configuration and inserts it into the 3d config map.
func (s *SuiteSchedulerConfigs) addConfigToNewBuild3dMap(config *suschpb.SchedulerConfig, targetOptions TargetOptions) {
	s.newBuild3dList = append(s.newBuild3dList, config)
	// Add to the array tracking all SuSch configs.
	s.configList = append(s.configList, config)

	// Add to the direct access map.
	s.configMap[TestPlanName(config.Name)] = config

	// Fetch all build targets for the config
	buildTargets := GetBuildTargetsForAllTargets(targetOptions)

	// Add all buildtargets as a map to the newBuilds3d map
	targetOptionsMap := make(map[BuildTarget]bool)

	for _, target := range buildTargets {
		targetOptionsMap[target] = true
	}

	s.newBuild3dMap[config] = targetOptionsMap
}
