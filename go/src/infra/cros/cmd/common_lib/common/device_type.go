// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func IsAndroid(board string) bool {
	return strings.Contains(
		strings.ToLower(board),
		strings.ToLower("pixel"),
	)
}

func IsCros(board string) bool {
	return (!IsDevBoard(board) && !IsAndroid(board))
}

func IsDevBoard(board string) bool {
	return strings.Contains(
		strings.ToLower(board),
		strings.ToLower("-devboard"),
	)
}

// GetBoardModelDims gets board, model dims from scheduling unit.
func GetBoardModelDims(unit *api.SchedulingUnit) []string {
	dims := []string{}
	boardsMap := map[string]int{}
	modelsMap := map[string]int{}

	// process primary
	primaryBoard := DutModelFromDut(unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo()).GetBuildTarget()
	if primaryBoard != "" {
		dims = append(dims, fmt.Sprintf("label-board:%s", primaryBoard))
		boardsMap[primaryBoard] = 1
	}

	primaryModel := DutModelFromDut(unit.GetPrimaryTarget().GetSwarmingDef().GetDutInfo()).GetModelName()
	if primaryModel != "" {
		dims = append(dims, fmt.Sprintf("label-model:%s", primaryModel))
		modelsMap[primaryModel] = 1
	}

	// process secondary
	for _, secondary := range unit.GetCompanionTargets() {
		board := DutModelFromDut(secondary.GetSwarmingDef().GetDutInfo()).GetBuildTarget()
		if board != "" {
			// When equal, the secondary needs the _n.
			if count, ok := boardsMap[board]; ok {
				count++
				boardsMap[board] = count
				board = fmt.Sprintf("%s_%d", board, count)
			} else {
				boardsMap[board] = 1
			}

			dims = append(dims, fmt.Sprintf("label-board:%s", board))
		}

		model := DutModelFromDut(secondary.GetSwarmingDef().GetDutInfo()).GetModelName()
		if model != "" {
			// When equal, the secondary needs the _2.
			if count, ok := modelsMap[model]; ok {
				count++
				modelsMap[model] = count
				model = fmt.Sprintf("%s_%d", model, count)
			} else {
				modelsMap[model] = 1
			}

			dims = append(dims, fmt.Sprintf("label-model:%s", model))
		}
	}

	return dims
}

// DutModelFromDut gets dutModel from provided dut.
func DutModelFromDut(dut *labapi.Dut) *labapi.DutModel {
	if dut == nil {
		return nil
	}

	switch hw := dut.GetDutType().(type) {
	case *labapi.Dut_Chromeos:
		return hw.Chromeos.GetDutModel()
	case *labapi.Dut_Android_:
		return hw.Android.GetDutModel()
	case *labapi.Dut_Devboard_:
		return hw.Devboard.GetDutModel()
	}
	return nil
}

func ConstructKey(board string, model string, variant string) string {
	key := board
	if model != "" {
		key = fmt.Sprintf("%s-%s", key, model)
	}
	if variant != "" {
		key = fmt.Sprintf("%s-%s", key, variant)
	}

	return fmt.Sprintf("[%s]", key)
}

// ExtractPrefixUntilDelimiter: Extracts a prefix until a delimiter is found.
func ExtractPrefixUntilDelimiter(str string, delimiter string) string {
	index := strings.Index(str, delimiter)
	if index == -1 {
		return str // Delimiter not found, return the whole string
	}
	return str[:index]
}
