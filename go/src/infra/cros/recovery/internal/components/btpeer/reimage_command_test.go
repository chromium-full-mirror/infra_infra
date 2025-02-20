// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package btpeer

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

// mockResult is a runner that always returns the same value to test result parsing.
func mockResult(output []string) components.Runner {
	return func(context.Context, time.Duration, string, ...string) (string, error) {
		return strings.Join(output, "\n"), nil
	}
}

func TestGetHwInfo(t *testing.T) {
	tests := []struct {
		name             string
		output           []string
		wantErr          bool
		expectedModel    Model
		expectedRevision int
	}{
		{
			name: "pass_modelb_rev1.1",
			output: []string{
				"c03111",
			},
			expectedModel:    17,
			expectedRevision: 1,
		},
		{
			name: "pass_modelb_rev1.1",
			output: []string{
				"c03114",
			},
			expectedModel:    17,
			expectedRevision: 4,
		},
		{
			name: "pass_unknown_model_rev1.0",
			output: []string{
				"a03140",
			},
			expectedModel:    20,
			expectedRevision: 0,
		},
		{
			name: "fail_hex_overflow",
			output: []string{
				"0x2540BE4000",
			},
			expectedModel:    0,
			expectedRevision: 0,
			wantErr:          true,
		},
		{
			name: "fail_invalid_hex",
			output: []string{
				"",
			},
			expectedModel:    0,
			expectedRevision: 0,
			wantErr:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := mockResult(test.output)
			model, rev, err := GetHWInfo(context.Background(), runner)
			if model != test.expectedModel {
				t.Errorf("TestGetHWInfo: got model: %v, want: %v", model, test.expectedModel)
			}
			if rev != test.expectedRevision {
				t.Errorf("TestGetHWInfo: got model: %v, want: %v", rev, test.expectedRevision)
			}
			if (err != nil) != test.wantErr {
				t.Errorf("TestGetHWInfo: error = %v, wantErr %v", err, test.wantErr)
				return
			}
		})
	}
}

func TestGetDeviceInfo(t *testing.T) {
	tests := []struct {
		name           string
		output         []string
		wantErr        bool
		expectedResult *deviceInfo
	}{
		{
			name: "pass",
			output: []string{
				"BYT",
				"/dev/mmcblk0:31914983424B:sd/mmc:512:512:msdos:SD SE32G:",
				"1:4194304B:272629759B:268435456B:fat32::lba",
				"2:272629760B:16656630271B:16384000512B:ext4::",
				"3:16657678336B:17660116991B:1002438656B:fat32::lba",
				"4:17660116992B:31914983423B:14254866432B:ext4::",
			},
			wantErr: false,
			expectedResult: &deviceInfo{
				Name: "/dev/mmcblk0",
				Size: 31914983424,
			},
		},
		{
			name: "fail_no_device",
			output: []string{
				"BYT",
				"1:4194304B:272629759B:268435456B:fat32::lba",
				"2:272629760B:16656630271B:16384000512B:ext4::",
				"3:16657678336B:17660116991B:1002438656B:fat32::lba",
				"4:17660116992B:31914983423B:14254866432B:ext4::",
			},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			name: "fail_malformed_size",
			output: []string{
				"BYT",
				"/dev/mmcblk0:319149834..24B:sd/mmc:512:512:msdos:SD SE32G:",
			},
			wantErr:        true,
			expectedResult: nil,
		},
		{
			name: "fail_negative_size",
			output: []string{
				"BYT",
				"/dev/mmcblk0:-31914983424B:sd/mmc:512:512:msdos:SD SE32G:",
			},
			wantErr:        true,
			expectedResult: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := &partitionHelper{
				device: "/dev/mmcblk0",
				runner: mockResult(test.output),
			}
			info, err := p.getDeviceInfo(context.Background())
			if info != nil && test.expectedResult != nil && *info != *test.expectedResult {
				t.Errorf("TestDeviceInfo: invalid result, got %v expected %v", info, test.expectedResult)
			}
			if (err != nil) != test.wantErr {
				t.Errorf("TestDeviceInfo: error = %v, wantErr %v", err, test.wantErr)
				return
			}
		})
	}
}

func TestGetPartitionInfo(t *testing.T) {
	tests := []struct {
		name           string
		output         []string
		wantErr        bool
		skipFree       bool
		expectedResult []*partitionInfo
	}{
		{
			name: "pass_skip_free",
			output: []string{
				"BYT",
				"/dev/mmcblk0:100%:sd/mmc:512:512:msdos:SD",
				"1:0.01%:0.85%:0.84%:fat32::lba",
				"2:0.85%:11.9%:11.0%:ext4::",
				"1:11.9%:100%:88.1%:free",
			},
			wantErr:  false,
			skipFree: true,
			expectedResult: []*partitionInfo{
				{
					Number: 1,
					Start:  0.01,
					End:    0.85,
					Size:   0.85 - 0.01,
					Type:   "fat32",
				},
				{
					Number: 2,
					Start:  0.85,
					End:    11.9,
					Size:   11.9 - 0.85,
					Type:   "ext4",
				},
			},
		},
		{
			name: "pass_with_free",
			output: []string{
				"BYT",
				"/dev/mmcblk0:100%:sd/mmc:512:512:msdos:SD",
				"1:0.01%:0.85%:0.84%:fat32::lba",
				"2:0.85%:11.9%:11.0%:ext4::",
				"1:11.9%:100%:88.1%:free",
			},
			wantErr:  false,
			skipFree: false,
			expectedResult: []*partitionInfo{
				{
					Number: 1,
					Start:  0.01,
					End:    0.85,
					Size:   0.85 - 0.01,
					Type:   "fat32",
				},
				{
					Number: 2,
					Start:  0.85,
					End:    11.9,
					Size:   11.9 - 0.85,
					Type:   "ext4",
				},
				{
					Number: 1,
					Start:  11.9,
					End:    100,
					Size:   100 - 11.9,
					Type:   "free",
				},
			},
		},
		{
			name: "pass_empty",
			output: []string{
				"BYT",
				"/dev/mmcblk0:100%:sd/mmc:512:512:msdos:SD",
				"1:0.1%:100%:88.1%:free",
			},
			wantErr:  false,
			skipFree: false,
			expectedResult: []*partitionInfo{
				{
					Number: 1,
					Start:  0.1,
					End:    100,
					Size:   100 - 0.1,
					Type:   "free",
				},
			},
		},
		{
			name: "fail_malformed_start",
			output: []string{
				"1:0..1%:100%:88.1%:free",
			},
			wantErr:        true,
			skipFree:       false,
			expectedResult: nil,
		},
		{
			name: "fail_malformed_start",
			output: []string{
				"1:0..1%:100%:88.1%:free",
			},
			wantErr:        true,
			skipFree:       false,
			expectedResult: nil,
		},
		{
			name: "fail_malformed_end",
			output: []string{
				"1:0.1%:10..0%:88.1%:free",
			},
			wantErr:        true,
			skipFree:       false,
			expectedResult: nil,
		},
		{
			name: "fail_negative_size",
			output: []string{
				"1:10%:1%:88.1%:free",
			},
			wantErr:        true,
			skipFree:       false,
			expectedResult: nil,
		},
		{
			name: "fail_negative_start",
			output: []string{
				"-1.0:10%:100%:88.1%:free",
			},
			wantErr:        true,
			skipFree:       false,
			expectedResult: nil,
		},
		{
			name: "fail_negative_end",
			output: []string{
				"1:1%:-10%:88.1%:free",
			},
			wantErr:        true,
			skipFree:       false,
			expectedResult: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := &partitionHelper{
				device: "/dev/mmcblk0",
				runner: mockResult(test.output),
			}
			info, err := p.GetPartitionInfo(context.Background(), "%", test.skipFree)
			if len(info) != len(test.expectedResult) {
				t.Errorf("TestPartitionInfo: invalid result, got %d results expected %d", len(info), len(test.expectedResult))
				return
			}
			for i := range test.expectedResult {
				if *info[i] != *test.expectedResult[i] {
					t.Errorf("TestPartitionInfo: invalid result, got %v expected %v", info[i], test.expectedResult[i])
					return
				}
			}
			if (err != nil) != test.wantErr {
				t.Errorf("TestPartitionInfo: error = %v, wantErr %v", err, test.wantErr)
				return
			}
		})
	}
}
