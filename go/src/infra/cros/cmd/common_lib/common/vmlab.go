// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"slices"
	"strings"
)

var vmSupportedBoards = []string{"betty", "reven-vmtest", "amd64-generic"}

const (
	testRunnerBuilderName    = "test_runner"
	testRunnerGceBuilderName = "test_runner_gce"
)

// ConvertBuilderNameToVM converts the original test runner name to corresponding
// name of the VMLab version. See configs at http://shortn/_86vxOQ0XC6
// test_runner[-env] -> test_runner_gce[-env]
func ConvertBuilderNameToVM(originalBuilderName string) string {
	if !strings.HasPrefix(originalBuilderName, testRunnerBuilderName) {
		return originalBuilderName
	}
	if strings.HasPrefix(originalBuilderName, testRunnerGceBuilderName) {
		return originalBuilderName
	}
	return strings.Replace(originalBuilderName, testRunnerBuilderName, testRunnerGceBuilderName, 1)
}

func IsSupportedVMBoard(board string) bool {
	return slices.Contains(vmSupportedBoards, board)
}
