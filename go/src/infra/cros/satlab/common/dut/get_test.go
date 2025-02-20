// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

func TestMakeGetShivasFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		getCmd *GetDUT
		want   flagmap
	}{
		{
			name:   "default",
			getCmd: &GetDUT{},
			want: flagmap{
				"namespace": []string{"os"},
				"json":      []string{},
			},
		},
		{
			name: "all fields",
			getCmd: &GetDUT{
				Zones:         []string{"input_zone"},
				Racks:         []string{"input_racks"},
				Machines:      []string{"input_machines"},
				Prototypes:    []string{"input_prototypes"},
				Servos:        []string{"input_servos"},
				Servotypes:    []string{"input_servotypes"},
				Switches:      []string{"input_switches"},
				Rpms:          []string{"input_rpms"},
				Pools:         []string{"input_pools"},
				HostInfoStore: true,
				Namespace:     "os-partner",
			},
			want: flagmap{
				"zone":            []string{"input_zone"},
				"rack":            []string{"input_racks"},
				"machine":         []string{"input_machines"},
				"prototype":       []string{"input_prototypes"},
				"servo":           []string{"input_servos"},
				"servotype":       []string{"input_servotypes"},
				"switch":          []string{"input_switches"},
				"rpms":            []string{"input_rpms"},
				"pools":           []string{"input_pools"},
				"host-info-store": []string{},
				"namespace":       []string{"os-partner"},
				"json":            []string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeGetDUTShivasFlags(tt.getCmd); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeGetShivasFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newMachineLSE(name string) *ufsModels.MachineLSE {
	return &ufsModels.MachineLSE{
		Name:     name,
		Hostname: name,
		Machines: []string{"machine"},
		Lse: &ufsModels.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufsModels.ChromeOSMachineLSE{
				ChromeosLse: &ufsModels.ChromeOSMachineLSE_Dut{
					Dut: &ufsModels.ChromeOSDeviceLSE{
						Device: &ufsModels.ChromeOSDeviceLSE_Dut{
							Dut: &ufspb.DeviceUnderTest{
								Hostname: name,
								Pools:    []string{"pool"},
								Peripherals: &ufspb.Peripherals{
									Servo: &ufspb.Servo{
										ServoFwChannel: ufspb.ServoFwChannel_SERVO_FW_PREV,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func marshallMachineLSESlice(expected []*ufsModels.MachineLSE) []string {
	m := jsonpb.Marshaler{}
	s := []string{}
	for _, elem := range expected {
		js, _ := m.MarshalToString(elem)
		s = append(s, js)
	}

	return s
}

func TestGetDUTShouldWork(t *testing.T) {
	t.Parallel()

	// Create a request
	p := GetDUT{}
	ctx := context.Background()

	// Create a fake data
	expected := []*ufsModels.MachineLSE{newMachineLSE("name")}

	// Lazy mock command executor which just returns a MachineLSE named after
	// the last work in the shivas get dut
	executor := executor.FakeCommander{FakeFn: func(c *exec.Cmd) ([]byte, error) {
		if c.Args[0] != paths.ShivasCLI {
			return nil, nil
		}

		gotMachineLSE := []*ufsModels.MachineLSE{newMachineLSE(c.Args[len(c.Args)-1])}
		marshalled := marshallMachineLSESlice(gotMachineLSE)

		return []byte(fmt.Sprintf("[%v]", strings.Join(marshalled, ","))), nil
	}}

	// Act
	res, err := p.TriggerRun(ctx, &executor, []string{"name"})

	// Asset
	if err != nil {
		t.Errorf("Should be success, but got an error: %v", err)
		return
	}

	// ignore pb fields in `MachineLSE`
	ignorePBFieldOpts := cmpopts.IgnoreUnexported(
		ufsModels.MachineLSE{},
		ufsModels.MachineLSE_ChromeosMachineLse{},
		ufsModels.ChromeOSMachineLSE{},
		ufsModels.ChromeOSMachineLSE_Dut{},
		ufsModels.ChromeOSDeviceLSE{},
		ufsModels.ChromeOSDeviceLSE_Dut{},
		ufspb.DeviceUnderTest{},
		ufspb.Peripherals{},
		ufspb.Servo{},
	)

	if diff := cmp.Diff(res, expected, ignorePBFieldOpts); diff != "" {
		t.Errorf("Expected: %v\n, got: %v\n, diff: %v\n", expected, res, diff)
		return
	}
}

func TestGetDUTShouldFail(t *testing.T) {
	t.Parallel()

	// Create a request
	p := GetDUT{}
	ctx := context.Background()

	// Create a fake data
	executor := executor.FakeCommander{Err: errors.New("cmd error")}

	// Act
	res, err := p.TriggerRun(ctx, &executor, []string{"name"})

	// Asset
	if err == nil {
		t.Errorf("should be failed")
		return
	}

	if res != nil {
		t.Errorf("result should be empty, but got: %v", res)
		return
	}
}
