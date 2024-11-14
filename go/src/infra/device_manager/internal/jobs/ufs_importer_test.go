// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jobs

import (
	"context"
	"strings"
	"testing"

	"infra/device_manager/internal/model"
)

func Test_isDeviceNeedsUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name      string
		oldDevice model.Device
		newDevice model.Device
		wantBool  bool
		panicMsg  string
	}{
		{
			name: "fail; different Device IDs",
			oldDevice: model.Device{
				ID: "d1",
			},
			newDevice: model.Device{
				ID: "d2",
			},
			wantBool: false,
			panicMsg: "comparing two different devices d1 and d2",
		},
		{
			name: "pass; no update on different DeviceAddress",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "2.2.2.2:2",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
		{
			name: "pass; no update on different DeviceType",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_VIRTUAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
		{
			name: "pass; different SchedulableLabels",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-different-1"},
					},
				},
				IsActive: true,
			},
			wantBool: true,
			panicMsg: "",
		},
		{
			name: "pass; different IsActive",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: false,
			},
			wantBool: true,
			panicMsg: "",
		},
		{
			name: "pass; Device fields are the same",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
		{
			name: "pass; Device fields are the same and SchedulableLabels are in different order",
			oldDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
					"label-test-2": model.LabelValues{
						Values: []string{"test-value-2"},
					},
				},
				IsActive: true,
			},
			newDevice: model.Device{
				ID:            "d1",
				DeviceAddress: "1.1.1.1:1",
				DeviceType:    "DEVICE_TYPE_PHYSICAL",
				SchedulableLabels: model.SchedulableLabels{
					"label-test-2": model.LabelValues{
						Values: []string{"test-value-2"},
					},
					"label-test": model.LabelValues{
						Values: []string{"test-value-1"},
					},
				},
				IsActive: true,
			},
			wantBool: false,
			panicMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.panicMsg != "" && r == nil {
					t.Errorf("The code did not panic")
				} else if tt.panicMsg != "" && r != nil {
					if err, ok := r.(error); ok && !strings.Contains(err.Error(), tt.panicMsg) {
						t.Errorf("Wrong panic message: got %q, want %q", err.Error(), tt.panicMsg)
					}
				}
			}()

			t.Parallel()

			gotBool := areLabelsOrActiveStateDifferent(ctx, tt.oldDevice, tt.newDevice)

			if gotBool != tt.wantBool {
				t.Errorf("areLabelsOrActiveStateDifferent() gotBool = %v, wantBool %v", gotBool, tt.wantBool)
			}
		})
	}
}
