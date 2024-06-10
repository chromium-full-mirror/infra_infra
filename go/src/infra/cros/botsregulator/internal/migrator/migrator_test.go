// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package migrator

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	ufspb "infra/unifiedfleet/api/v1/models"
	chromeosLab "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

func TestComputeBoardModelToState(t *testing.T) {
	t.Parallel()
	m := &migrator{}
	t.Run("Happy path", func(t *testing.T) {
		t.Parallel()
		mcs := []*ufspb.Machine{
			{
				Name: "machines/machine-1",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-1",
						Model:       "model-1",
					},
				},
			},
			{
				Name: "machines/machine-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-1",
						Model:       "model-1",
					},
				},
			},
			{
				Name: "machines/machine-3",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-1",
						Model:       "model-1",
					},
				},
			},
			{
				Name: "machines/machine-4",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-2",
						Model:       "model-1",
					},
				},
			},
			{
				Name: "machines/machine-5",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-2",
						Model:       "model-1",
					},
				},
			},
		}
		lses := []*ufspb.MachineLSE{
			{
				Name: "machineLSEs/dut-1",
				Machines: []string{
					"machine-1",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "cloudbots",
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "machineLSEs/dut-2",
				Machines: []string{
					"machine-2",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "cloudbots",
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "machineLSEs/dut-3",
				Machines: []string{
					"machine-3",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "e",
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "machineLSEs/dut-4",
				Machines: []string{
					"machine-4",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "",
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "machineLSEs/dut-5",
				Machines: []string{
					"machine-5",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive:  "",
										Pools: []string{"wifi"},
									},
								},
							},
						},
					},
				},
			},
		}
		cs := &configSearchable{
			excludeDUTs: []*regexp.Regexp{
				regexp.MustCompile("dut-1"),
			},
			excludePools: map[string]struct{}{
				"wifi": {},
			},
		}
		got, err := m.ComputeBoardModelToState(context.Background(), mcs, lses, cs)
		if err != nil {
			t.Fatalf("should not error: %v", err)
		}
		want := map[string]*migrationState{
			"board-1/model-1": {
				Cloudbots: []string{
					"dut-2",
				},
				Drone: []string{
					"dut-3",
				},
			},
			"board-2/model-1": {
				Drone: []string{
					"dut-4",
				},
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Complex regex", func(t *testing.T) {
		t.Parallel()
		mcs := []*ufspb.Machine{
			{
				Name: "machines/machine-1",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-1",
						Model:       "model-1",
					},
				},
			},
			{
				Name: "machines/machine-2",
				Device: &ufspb.Machine_ChromeosMachine{
					ChromeosMachine: &ufspb.ChromeOSMachine{
						BuildTarget: "board-1",
						Model:       "model-1",
					},
				},
			},
		}
		lses := []*ufspb.MachineLSE{
			{
				Name: "machineLSEs/chromeos6-row1-rack10-host53",
				Machines: []string{
					"machine-1",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "cloudbots",
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "machineLSEs/chromeos8-row1-rack10-phone2a",
				Machines: []string{
					"machine-2",
				},
				Lse: &ufspb.MachineLSE_ChromeosMachineLse{
					ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
						ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
							DeviceLse: &ufspb.ChromeOSDeviceLSE{
								Device: &ufspb.ChromeOSDeviceLSE_Dut{
									Dut: &chromeosLab.DeviceUnderTest{
										Hive: "cloudbots",
									},
								},
							},
						},
					},
				},
			},
		}
		cs := &configSearchable{
			excludeDUTs: []*regexp.Regexp{
				regexp.MustCompile("^chromeos(6|15)"),
				regexp.MustCompile("phone[0-9A-Za-z]*$"),
			},
		}
		got, err := m.ComputeBoardModelToState(context.Background(), mcs, lses, cs)
		if err != nil {
			t.Fatalf("should not error: %v", err)
		}
		want := map[string]*migrationState{}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestComputeModelState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		amount       int32
		currentState *migrationState
		want         *migrationState
	}{
		{
			amount: 1,
			currentState: &migrationState{
				Cloudbots: []string{
					"dut-1",
					"dut-2",
				},
				Drone: []string{
					"dut-3",
					"dut-4",
				},
			},
			want: &migrationState{
				Drone: []string{
					"dut-1",
				},
			},
		},
		{
			amount: 100,
			currentState: &migrationState{
				Drone: []string{
					"dut-1",
					"dut-2",
					"dut-3",
					"dut-4",
				},
			},
			want: &migrationState{
				Cloudbots: []string{
					"dut-1",
					"dut-2",
					"dut-3",
					"dut-4",
				},
			},
		},
		{
			amount: 0,
			currentState: &migrationState{
				Cloudbots: []string{
					"dut-1",
					"dut-2",
				},
				Drone: []string{
					"dut-3",
					"dut-4",
				},
			},
			want: &migrationState{
				Drone: []string{
					"dut-1",
					"dut-2",
				},
			},
		},
		{
			amount: 70,
			currentState: &migrationState{
				Cloudbots: []string{
					"dut-1",
					"dut-2",
					"dut-5",
					"dut-6",
				},
				Drone: []string{
					"dut-3",
					"dut-4",
					"dut-7",
					"dut-8",
					"dut-9",
					"dut-10",
				},
			},
			want: &migrationState{
				Cloudbots: []string{
					"dut-3",
					"dut-4",
					"dut-7",
				},
			},
		},
	}
	for _, c := range cases {
		// Loop closure.
		c := c
		t.Run(fmt.Sprintf("case: %d", c.amount), func(t *testing.T) {
			t.Parallel()
			got := &migrationState{}
			computeNextModelState(context.Background(), "model for log only", c.amount, c.currentState, got)
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestComputeNextMigrationSate(t *testing.T) {
	t.Parallel()
	m := &migrator{}
	// Sort string slices before being compared.
	trans := cmpopts.SortSlices(func(a, b string) bool {
		return a < b
	})

	t.Run("Happy path", func(t *testing.T) {
		t.Parallel()
		bms := map[string]*migrationState{
			"board-1/model-1": {
				Cloudbots: []string{
					"dut-1",
					"dut-2",
				},
				Drone: []string{
					"dut-3",
				},
			},
			"board-1/model-2": {
				Cloudbots: []string{
					"dut-41",
					"dut-42",
					"dut-43",
					"dut-44",
					"dut-45",
					"dut-46",
					"dut-47",
					"dut-48",
					"dut-49",
					"dut-50",
				},
			},
			"board-2/model-4": {
				Cloudbots: []string{
					"dut-61",
					"dut-62",
					"dut-63",
					"dut-64",
				},
				Drone: []string{
					"dut-65",
				},
			},
			"board-2/model-5": {
				Cloudbots: []string{
					"dut-70",
				},
				Drone: []string{
					"dut-71",
					"dut-72",
					"dut-73",
					"dut-74",
					"dut-75",
					"dut-76",
					"dut-77",
					"dut-78",
				},
			},
			"board-3/model-3": {
				Cloudbots: []string{
					"dut-51",
					"dut-52",
					"dut-53",
					"dut-54",
					"dut-55",
					"dut-56",
				},
				Drone: []string{
					"dut-57",
					"dut-58",
					"dut-59",
					"dut-60",
				},
			},
			"board-3/model-6": {
				Cloudbots: []string{
					"dut-81",
					"dut-82",
					"dut-83",
					"dut-84",
					"dut-85",
					"dut-86",
				},
				Drone: []string{
					"dut-87",
					"dut-88",
					"dut-89",
					"dut-90",
				},
			},
		}
		cs := &configSearchable{
			minCloudbotsPercentage:     1,
			minLowRiskModelsPercentage: 50,
			overrideLowRisks: map[string]struct{}{
				"model-1": {},
				"model-2": {},
			},
			// computeNextMigrationSate does not filter out excludeDUTs.
			// The filtering happens earlier.
			excludeDUTs: []*regexp.Regexp{
				regexp.MustCompile("dut-74"),
				regexp.MustCompile("dut-75"),
			},
			// computeNextMigrationSate does not filter out excludePools.
			// The filtering happens earlier.
			excludePools: nil,
			overrideBoardModel: map[string]int32{
				"board-2/*":       90,
				"board-1/model-1": 0,
				"board-3/model-3": 58,
			},
		}
		got := m.ComputeNextMigrationState(context.Background(), bms, cs)
		want := &migrationState{
			Cloudbots: []string{
				"dut-65",
				"dut-71",
				"dut-72",
				"dut-73",
				"dut-74",
				"dut-75",
				"dut-76",
				"dut-77",
				"dut-78",
			},
			Drone: []string{
				"dut-1",
				"dut-2",
				"dut-41",
				"dut-42",
				"dut-43",
				"dut-44",
				"dut-45",
				"dut-81",
				"dut-82",
				"dut-83",
				"dut-84",
				"dut-85",
			},
		}
		if diff := cmp.Diff(want, got, trans); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestGetExcludedDUTs(t *testing.T) {
	t.Parallel()
	m := &migrator{}
	lses := []*ufspb.MachineLSE{
		{
			Name: "dut-1",
		},
		{
			Name: "dut-2",
		},
		{
			Name: "dut-3",
		},
		{
			Name: "dut-4",
			Lse: &ufspb.MachineLSE_ChromeosMachineLse{
				ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
					ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
						DeviceLse: &ufspb.ChromeOSDeviceLSE{
							Device: &ufspb.ChromeOSDeviceLSE_Dut{
								Dut: &chromeosLab.DeviceUnderTest{
									Hive:  "",
									Pools: []string{"pool-1"},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "dut-5",
			Lse: &ufspb.MachineLSE_ChromeosMachineLse{
				ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
					ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
						DeviceLse: &ufspb.ChromeOSDeviceLSE{
							Device: &ufspb.ChromeOSDeviceLSE_Dut{
								Dut: &chromeosLab.DeviceUnderTest{
									Hive:  "",
									Pools: []string{"pool-2"},
								},
							},
						},
					},
				},
			},
		},
	}
	cs := &configSearchable{
		excludeDUTs: []*regexp.Regexp{
			regexp.MustCompile("dut-1"),
			regexp.MustCompile("dut-2"),
			regexp.MustCompile("dut-4"),
		},
		excludePools: map[string]struct{}{
			"pool-1": {},
		},
	}
	want := []string{
		"dut-1",
		"dut-2",
		"dut-4",
	}
	got := m.GetExcludedDUTs(context.Background(), lses, cs)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}

}
