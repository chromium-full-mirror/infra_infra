// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package linux

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/common/errors"
)

type runnerResponse struct {
	output string
	err    error
}

var IsPathExistTests = []struct {
	testName string
	runnerResponse
	expectedErr error
}{
	{
		"Path Exist, no error",
		runnerResponse{"", nil},
		nil,
	},
	{
		"Path Not Exist, path exist error",
		runnerResponse{"", errors.Reason("runner: path not exist").Err()},
		errors.Reason("path exist: runner: path not exist").Err(),
	},
}

func TestIsPathExist(t *testing.T) {
	t.Parallel()
	for _, tt := range IsPathExistTests {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			run := func(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
				if strings.HasPrefix(cmd, "test -e") {
					return tt.runnerResponse.output, tt.runnerResponse.err
				}
				return "", errors.Reason("runner: cmd not recognized").Err()
			}
			actualErr := IsPathExist(ctx, run, "test_path")
			if actualErr != nil && tt.expectedErr != nil {
				if !strings.Contains(actualErr.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
				}
			}
			if (actualErr == nil && tt.expectedErr != nil) || (actualErr != nil && tt.expectedErr == nil) {
				t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
			}
		})
	}
}

var PathHasEnoughValueTests = []struct {
	testName       string
	mapOfRunner    map[string]runnerResponse
	typeOfSpace    SpaceType
	minSpaceNeeded float64
	expectedErr    error
}{
	{
		"Path Exist, Enough Disk Storage, no error",
		map[string]runnerResponse{
			"test -e": {"", nil},
			"df -P":   {"/xxx/yyy/root         828753   164335      622245      21% /", nil},
		},
		SpaceTypeDisk,
		0.6,
		nil,
	},
	{
		"Path Exist, Enough Inode Storage, no error",
		map[string]runnerResponse{
			"test -e": {"", nil},
			"df -Pi":  {"/xxx/yyy/root         828753   164335      622245      21% /", nil},
		},
		SpaceTypeInode,
		100,
		nil,
	},
	{
		"Path Not Exist, Enough Disk Storage, path exist error",
		map[string]runnerResponse{
			"test -e": {"", errors.Reason("runner: path not exist").Err()},
			"df -P":   {"/xxx/yyy/root         828753   164335      622245      21% /", nil},
		},
		SpaceTypeDisk,
		0.6,
		errors.Reason("path exist").Err(),
	},
	{
		"Path Exist, Not Enough Disk Storage, no enough disk space error",
		map[string]runnerResponse{
			"test -e": {"", nil},
			"df -P":   {"/xxx/yyy/root         828753   164335      622245      21% /", nil},
		},
		SpaceTypeDisk,
		9999,
		errors.Reason("Not enough free disk").Err(),
	},
}

func TestPathHasEnoughValue(t *testing.T) {
	t.Parallel()
	for _, tt := range PathHasEnoughValueTests {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			run := func(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
				for testCmd, res := range tt.mapOfRunner {
					if strings.HasPrefix(cmd, testCmd) {
						return res.output, res.err
					}
				}
				return "", errors.Reason("runner: cmd not recognized").Err()
			}
			actualErr := PathHasEnoughValue(ctx, run, "test_dut", "test_path", tt.typeOfSpace, tt.minSpaceNeeded)
			if actualErr != nil && tt.expectedErr != nil {
				if !strings.Contains(actualErr.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
				}
			}
			if (actualErr == nil && tt.expectedErr != nil) || (actualErr != nil && tt.expectedErr == nil) {
				t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
			}
		})
	}
}

var PathOccupiedSpacePercentageTests = []struct {
	testName           string
	mapOfRunner        map[string]runnerResponse
	expectedPercentage float64
	expectedErr        error
}{
	{
		"Path Exist, 21 percent occupied space, no error",
		map[string]runnerResponse{
			"test -e": {"", nil},
			"df":      {"/xxx/yyy/root         828753   164335      622245      21% /", nil},
		},
		21,
		nil,
	},
	{
		"Path Exist, runner error",
		map[string]runnerResponse{
			"test -e": {"", nil},
			"df":      {"/xxx/yyy/root         828753   164335      622245      21% /", errors.Reason("runner: df command not found").Err()},
		},
		-1,
		errors.Reason("path occupied space percentage: runner: df command not found").Err(),
	},
	{
		"Path Exist, convert float error",
		map[string]runnerResponse{
			"test -e": {"", nil},
			"df":      {"/xxx/yyy/root         828753   164335      622245      test% /", nil},
		},
		-1,
		errors.Reason("path occupied space percentage: strconv.ParseFloat").Err(),
	},
}

func TestPathOccupiedSpacePercentage(t *testing.T) {
	t.Parallel()
	for _, tt := range PathOccupiedSpacePercentageTests {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			run := func(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
				for testCmd, res := range tt.mapOfRunner {
					if strings.HasPrefix(cmd, testCmd) {
						return res.output, res.err
					}
				}
				return "", errors.Reason("runner: cmd not recognized").Err()
			}
			actualPercentage, actualErr := PathOccupiedSpacePercentage(ctx, run, "test_path")
			if actualErr != nil && tt.expectedErr != nil {
				if !strings.Contains(actualErr.Error(), tt.expectedErr.Error()) {
					t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
				}
			}
			if (actualErr == nil && tt.expectedErr != nil) || (actualErr != nil && tt.expectedErr == nil) {
				t.Errorf("Expected error %q, but got %q", tt.expectedErr, actualErr)
			}
			if actualPercentage != tt.expectedPercentage {
				t.Errorf("Expected percentage: %v, but got: %v", tt.expectedPercentage, actualPercentage)
			}
		})
	}
}

func TestStorageUtilizationReportOfFilesInDir(t *testing.T) {
	const testDirPath = "/tmp/stateful_partition"
	const sampleDfOutput1 = `Filesystem      1B-blocks      Used  Available Use% Mounted on
/dev/mmcblk0p1 2053300224 617697280 1312817152  32% /mnt/stateful_partition`
	const sampleDuOutput1 = `4096	/mnt/stateful_partition/var/lock
4096	/mnt/stateful_partition/var/db/pkg
8192	/mnt/stateful_partition/var/db
4096	/mnt/stateful_partition/var/tmp/portage
307200	/mnt/stateful_partition/var/tmp
4096	/mnt/stateful_partition/var/run
12288	/mnt/stateful_partition/var/lib/dhcpcd
81920	/mnt/stateful_partition/var/lib/update_engine
12288	/mnt/stateful_partition/var/lib/trim
4096	/mnt/stateful_partition/var/lib/chaps
4096	/mnt/stateful_partition/var/lib/tpm
`
	expectedHeaderRow := []string{
		"filepath",
		"usage % of partition",
		"bytes",
		"KB",
		"MB",
	}
	type mockRunnerOutput struct {
		dfCmdOutput string
		duCmdOutput string
	}
	tests := []struct {
		name             string
		mockRunnerOutput mockRunnerOutput
		wantCsvDataRows  [][]string
		wantErr          bool
	}{
		{
			"no output",
			mockRunnerOutput{
				"",
				"",
			},
			nil,
			true,
		},
		{
			"valid available space and no files found",
			mockRunnerOutput{
				sampleDfOutput1,
				"",
			},
			[][]string{
				{"[Mounted Drive \"/mnt/stateful_partition\"]", "31.996512", "617697280 of 1930514432", "603220.000000 of 1885268.000000", "589.082031 of 1841.082031"},
			},
			false,
		},
		{
			"valid available space and multiple files found",
			mockRunnerOutput{
				sampleDfOutput1,
				sampleDuOutput1,
			},
			[][]string{
				{"[Mounted Drive \"/mnt/stateful_partition\"]", "31.996512", "617697280 of 1930514432", "603220.000000 of 1885268.000000", "589.082031 of 1841.082031"},
				{"/mnt/stateful_partition/var/tmp", "0.015913", "307200", "300.000000", "0.292969"},
				{"/mnt/stateful_partition/var/lib/update_engine", "0.004243", "81920", "80.000000", "0.078125"},
				{"/mnt/stateful_partition/var/lib/dhcpcd", "0.000637", "12288", "12.000000", "0.011719"},
				{"/mnt/stateful_partition/var/lib/trim", "0.000637", "12288", "12.000000", "0.011719"},
				{"/mnt/stateful_partition/var/db", "0.000424", "8192", "8.000000", "0.007812"},
				{"/mnt/stateful_partition/var/db/pkg", "0.000212", "4096", "4.000000", "0.003906"},
				{"/mnt/stateful_partition/var/lib/chaps", "0.000212", "4096", "4.000000", "0.003906"},
				{"/mnt/stateful_partition/var/lib/tpm", "0.000212", "4096", "4.000000", "0.003906"},
				{"/mnt/stateful_partition/var/lock", "0.000212", "4096", "4.000000", "0.003906"},
				{"/mnt/stateful_partition/var/run", "0.000212", "4096", "4.000000", "0.003906"},
				{"/mnt/stateful_partition/var/tmp/portage", "0.000212", "4096", "4.000000", "0.003906"},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := func(ctx context.Context, timeout time.Duration, cmd string, args ...string) (string, error) {
				if strings.HasPrefix(cmd, "test") {
					return "", nil
				}
				if cmd == "df" {
					return tt.mockRunnerOutput.dfCmdOutput, nil
				}
				if cmd == "du" {
					return tt.mockRunnerOutput.duCmdOutput, nil
				}
				return "", errors.Reason("unexpected command run of %q", cmd).Err()
			}
			got, err := StorageUtilizationReportOfFilesInDir(context.Background(), mockRunner, testDirPath)
			if err != nil || tt.wantErr {
				if tt.wantErr != (err != nil) {
					t.Errorf("StorageUtilizationReportOfFilesInDir() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			allWantedCsvRows := append([][]string{expectedHeaderRow}, tt.wantCsvDataRows...)
			wantCsv, err := marshallCsvRows(allWantedCsvRows)
			if err != nil {
				t.Errorf("test config error: invalid wantCsvDataRows: %v", err)
			}
			if got != wantCsv {
				t.Errorf("StorageUtilizationReportOfFilesInDir()\ngot CSV: = \n%vwant CSV:\n%v", got, wantCsv)
			}
		})
	}
}
