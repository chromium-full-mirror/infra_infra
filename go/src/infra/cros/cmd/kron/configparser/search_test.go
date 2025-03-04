// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configparser

import (
	"sort"
	"testing"
)

func TestFetchConfigTargetOptionsForBoard(t *testing.T) {
	t.Parallel()

	mockConfigs := SuiteSchedulerConfigs{
		configTargets: map[string]TargetOptions{
			"config1": {
				Board("board1"): {
					Board: "board1",
					Models: []string{
						"a",
						"b",
						"c",
					},
					Variants: []string{
						"-var1",
						"-var2",
						"-var3",
					},
				},
				Board("board2"): {
					Board: "board2",
					Models: []string{
						"d",
						"e",
						"f",
						"g",
					},
					Variants: []string{
						"-var3",
						"-var4",
						"-var5",
					},
				},
			},
		},
	}

	targetOption, err := mockConfigs.FetchConfigTargetOptionsForBoard("config1", "board2")
	if err != nil {
		t.Error(err)
		return
	}

	expectedBoard := "board2"
	expectedModels := sort.StringSlice([]string{
		"d",
		"e",
		"f",
		"g",
	})
	expectedModels.Sort()

	if targetOption.Board != expectedBoard {
		t.Errorf("Given board %s does not match expected board %s\n", targetOption.Board, expectedBoard)
		return
	}

	if targetOption.Board != expectedBoard {
		t.Errorf("Given board %s does not match expected board %s\n", targetOption.Board, expectedBoard)
		return
	}
	sortedGivenModels := sort.StringSlice(targetOption.Models)
	sortedGivenModels.Sort()

	if len(targetOption.Models) != len(expectedModels) {
		t.Error("Given models lists shorter than the expected list")
		return
	}

	for i := range expectedModels.Len() {
		if sortedGivenModels[i] != expectedModels[i] {
			t.Errorf("Model %s expected at position %d, %s given", expectedModels[i], i, sortedGivenModels[i])
			return
		}
	}
}
