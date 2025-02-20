// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configparser

import (
	"fmt"

	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"go.chromium.org/infra/cros/cmd/kron/common"
)

// FetchAllConfigs returns all configs.
func (s *SuiteSchedulerConfigs) FetchAllConfigs() ConfigList {
	return s.configList
}

// FetchConfigTargetOptionsForBoard returns the target options for the specified
// board for the specified config.
func (s *SuiteSchedulerConfigs) FetchConfigTargetOptionsForBoard(configName string, board Board) (*TargetOption, error) {
	targetOptions, ok := s.configTargets[configName]
	if !ok {
		return nil, fmt.Errorf("target options for config %s not found", configName)
	}

	if targetOption, ok := targetOptions[board]; !ok {
		return nil, fmt.Errorf("board %s not found in %s's target options", board, configName)
	} else {
		return targetOption, nil
	}
}

// FetchMultiDUTConfigTargetOptionsForBoard returns the target options for the specified
// board for the specified MULTI_DUT config.
func (s *SuiteSchedulerConfigs) FetchMultiDUTConfigTargetOptionsForBoard(configName string, primaryBoard string) ([]*MultiDutTargetOptions, error) {
	targetOptions, ok := s.multiDUTConfigTargets[configName]
	if !ok {
		return nil, fmt.Errorf("target options for config %s not found", configName)
	}

	if targetOption, ok := targetOptions[primaryBoard]; !ok {
		return nil, fmt.Errorf("board %s not found in %s's target options", primaryBoard, configName)
	} else {
		return targetOption, nil
	}
}

// FetchConfigTargetOptions returns all target options.
func (s *SuiteSchedulerConfigs) FetchConfigTargetOptions(configName string) (TargetOptions, error) {
	targetOptions, ok := s.configTargets[configName]
	if !ok {
		return nil, fmt.Errorf("target options for config %s not found", configName)
	}
	return targetOptions, nil
}

// FetchMultiDUTConfigTargetOptions returns all target options for the given
// MULTI_DUT config.
func (s *SuiteSchedulerConfigs) FetchMultiDUTConfigTargetOptions(configName string) (map[string][]*MultiDutTargetOptions, error) {
	targetOptions, ok := s.multiDUTConfigTargets[configName]
	if !ok {
		return nil, fmt.Errorf("target options for config %s not found", configName)
	}
	return targetOptions, nil
}

// FetchAllNewBuildConfigs returns all NEW_BUILD type configs.
func (s *SuiteSchedulerConfigs) FetchAllNewBuildConfigs() ConfigList {
	return s.newBuildList
}

// FetchAllMultiDUTConfigs returns all MULTI_DUT type configs.
func (s *SuiteSchedulerConfigs) FetchAllMultiDUTConfigs() ConfigList {
	return s.multiDUTList
}

// FetchAllNewBuild3dConfigs returns all NEW_BUILD_3D type configs.
func (s *SuiteSchedulerConfigs) FetchAllNewBuild3dConfigs() ConfigList {
	return s.newBuild3dList
}

// FetchNewBuild3dMap returns newBuild3dMap.
func (s *SuiteSchedulerConfigs) FetchNewBuild3dMap() map[*suschpb.SchedulerConfig]map[BuildTarget]bool {
	return s.newBuild3dMap
}

// FetchNewBuildConfigsByBuildTarget returns all NEW_BUILD configs that are
// to be triggered by a new image of the given build target.
func (s *SuiteSchedulerConfigs) FetchNewBuildConfigsByBuildTarget(target BuildTarget) ConfigList {
	if obj, ok := s.newBuildMap[target]; ok {
		return obj
	}
	return nil
}

// FetchMultiDUTConfigsByBuildTarget returns all MULTI_DUT configs that are
// to be triggered by a new image of the given build target.
func (s *SuiteSchedulerConfigs) FetchMultiDUTConfigsByBuildTarget(target BuildTarget) ConfigList {
	if obj, ok := s.multiDUTMap[target]; ok {
		return obj
	}
	return nil
}

// FetchAllDailyConfigs returns all DAILY type configs.
func (s *SuiteSchedulerConfigs) FetchAllDailyConfigs() ConfigList {
	tempList := ConfigList{}

	for _, list := range s.dailyMap {
		tempList = append(tempList, list...)
	}

	return tempList
}

// FetchDailyByHour returns all DAILY configs that are to be scheduled at the
// specified hour.
func (s *SuiteSchedulerConfigs) FetchDailyByHour(hour int) (ConfigList, error) {
	err := isHourCompliant(hour)
	if err != nil {
		return nil, err
	}

	if obj, ok := s.dailyMap[hour]; ok {
		return obj, nil
	}
	return nil, nil
}

// FetchAllWeeklyConfigs returns all WEEKLY type configs.
func (s *SuiteSchedulerConfigs) FetchAllWeeklyConfigs() ConfigList {
	tempList := ConfigList{}

	for _, mapobj := range s.weeklyMap {
		for _, list := range mapobj {
			tempList = append(tempList, list...)
		}
	}

	return tempList
}

// FetchWeeklyByDay returns all WEEKLY configs that are to be scheduled on the
// specified DAY.
func (s *SuiteSchedulerConfigs) FetchWeeklyByDay(day int) (ConfigList, error) {
	err := isDayCompliant(day, false, false)
	if err != nil {
		return nil, err
	}

	if obj, ok := s.weeklyMap[day]; ok {
		tempList := ConfigList{}

		for _, hour := range obj {
			tempList = append(tempList, hour...)
		}

		return tempList, nil
	} else {
		return nil, fmt.Errorf("no WEEKLY configs found at Day %d", day)
	}
}

// FetchWeeklyByDayHour returns all WEEKLY configs that are to be scheduled on the
// specified DAY at the given HOUR.
func (s *SuiteSchedulerConfigs) FetchWeeklyByDayHour(day, hour int) (ConfigList, error) {
	err := isDayCompliant(day, false, false)
	if err != nil {
		return nil, err
	}

	if _, ok := s.weeklyMap[day]; !ok {
		return nil, nil
	}

	if list, ok := s.weeklyMap[day][hour]; ok {
		return list, nil
	}

	return nil, nil
}

// FetchAllFortnightlyConfigs returns all FORTNIGHTLY type configs.
func (s *SuiteSchedulerConfigs) FetchAllFortnightlyConfigs() ConfigList {
	tempList := ConfigList{}

	for _, mapObj := range s.fortnightlyMap {
		for _, list := range mapObj {
			tempList = append(tempList, list...)
		}
	}

	return tempList
}

// FetchFortnightlyByDay returns all FORTNIGHTLY configs that are to be scheduled on the
// specified DAY.
func (s *SuiteSchedulerConfigs) FetchFortnightlyByDay(day int) (ConfigList, error) {
	err := isDayCompliant(day, true, false)
	if err != nil {
		return nil, err
	}

	if obj, ok := s.fortnightlyMap[day]; ok {
		tempList := ConfigList{}

		for _, hour := range obj {
			tempList = append(tempList, hour...)
		}

		return tempList, nil
	}
	return nil, nil
}

// FetchFortnightlyByDayHour returns all FORTNIGHTLY configs that are to be scheduled on the
// specified DAY at the given HOUR.
func (s *SuiteSchedulerConfigs) FetchFortnightlyByDayHour(day, hour int) (ConfigList, error) {
	err := isDayCompliant(day, true, false)
	if err != nil {
		return nil, err
	}

	if _, ok := s.fortnightlyMap[day]; !ok {
		return nil, nil
	}

	if list, ok := s.fortnightlyMap[day][hour]; ok {
		return list, nil
	}
	return nil, nil
}

// FetchNDayByDays returns all N_DAYS configs that are to be scheduled on the
// specified interval.
func (s *SuiteSchedulerConfigs) FetchNDayByDays(interval, hour int) (ConfigList, error) {
	err := isDayCompliant(interval, false, true)
	if err != nil {
		return nil, err
	}

	if _, ok := s.nDaysMap[interval]; !ok {
		return nil, nil
	}

	if list, ok := s.nDaysMap[interval]; ok {
		tempList := ConfigList{}

		for _, hour := range list {
			tempList = append(tempList, hour...)
		}

		return tempList, nil
	}
	return nil, nil
}

// FetchNDayByDaysHour returns all N_DAYS configs that are to be scheduled on the
// specified interval at the given HOUR.
func (s *SuiteSchedulerConfigs) FetchNDayByDaysHour(interval, hour int) (ConfigList, error) {
	err := isDayCompliant(interval, false, true)
	if err != nil {
		return nil, err
	}

	if _, ok := s.nDaysMap[interval]; !ok {
		return nil, nil
	}

	if list, ok := s.nDaysMap[interval][hour]; ok {
		return list, nil
	}
	return nil, nil
}

// FetchAllNDayConfigs returns all N_DAYS type configs.
func (s *SuiteSchedulerConfigs) FetchAllNDayConfigs() ConfigList {
	tempList := ConfigList{}

	for _, mapObj := range s.nDaysMap {
		for _, list := range mapObj {
			tempList = append(tempList, list...)
		}
	}

	return tempList
}

// FetchConfigByName returns the config with the name provided. If it does not
// exist then an error is returned.
func (s *SuiteSchedulerConfigs) FetchConfigByName(name string) *suschpb.SchedulerConfig {
	if val, ok := s.configMap[TestPlanName(name)]; ok {
		return val
	}

	return nil
}

// ValidateHoursAheadArgs will check that all of the arguments are within the
// specified bounds that can be worked with.
func ValidateHoursAheadArgs(startTime common.KronTime, hoursAhead int64) error {
	// Validate that all input values fit within the expected bounds.
	if hoursAhead < 0 {
		return fmt.Errorf("hours head must be a positive value, %d was given", hoursAhead)
	}

	if err := isHourCompliant(startTime.Hour); err != nil {
		return err
	}

	// This check allows the same function to be used by for the daily configs
	// function as long as it sends over the default int64 value stored as a
	// constant.
	if startTime.WeeklyDay != int(common.DefaultInt64) {
		if err := isDayCompliant(startTime.WeeklyDay, false, false); err != nil {
			return err
		}
	}
	if startTime.FortnightDay != int(common.DefaultInt64) {
		if err := isDayCompliant(startTime.FortnightDay, true, false); err != nil {
			return err
		}
	}

	return nil
}

func (s *SuiteSchedulerConfigs) FetchAllTargetOptions() map[string]TargetOptions {
	return s.configTargets
}
