// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package topology

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	ufsModels "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
)

func TestGetTopology_validateArgs(t *testing.T) {
	type fields struct {
		SatlabID string
		Hostname string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"all fields passed", fields{"123", "satlab-123-host"}, false},
		{"empty satlab-id", fields{"", "satlab-123-host"}, false},
		{"empty hostname", fields{"123", ""}, true},
		{"empty hostname and satlab id", fields{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &GetTopology{
				SatlabID: tt.fields.SatlabID,
				Hostname: tt.fields.Hostname,
			}
			if err := c.validateArgs(); (err != nil) != tt.wantErr {
				t.Errorf("GetTopology.validateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

const testHostname = "satlab-123-host1"

var (
	ph = &api.PasitHost{
		Hostname: testHostname,
	}
	fe = &executor.FakeCommander{FakeFn: func(c *exec.Cmd) ([]byte, error) {

		gotMachineLSE := []*ufsModels.MachineLSE{machineLSE(testHostname, ph)}
		marshalled := marshallMachineLSESlice(gotMachineLSE)

		return []byte(fmt.Sprintf("[%v]", strings.Join(marshalled, ","))), nil
	}}
)

func TestGetTopology_TriggerRun(t *testing.T) {
	type fields struct {
		SatlabID string
		Hostname string
	}
	type args struct {
		ctx      context.Context
		executor executor.IExecCommander
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *api.PasitHost
		wantErr bool
	}{
		{"should work", fields{"", testHostname}, args{context.Background(), fe}, ph, false},
		{"should fail, invalid args", fields{"", ""}, args{context.Background(), FakeCommanderWithCommandError("")}, &api.PasitHost{}, true},
		{"should fail, get host id", fields{"", testHostname}, args{context.Background(), FakeCommanderWithCommandError(paths.GetHostIdentifierScript)}, &api.PasitHost{}, true},
		{"should fail, get topology via shivas", fields{"", testHostname}, args{context.Background(), FakeCommanderWithCommandError(paths.ShivasCLI)}, &api.PasitHost{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &GetTopology{
				SatlabID: tt.fields.SatlabID,
				Hostname: tt.fields.Hostname,
			}
			got, err := c.TriggerRun(tt.args.ctx, tt.args.executor)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTopology.TriggerRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Ignore unexported Protobuf fields
			ignorePBFieldOpts := cmpopts.IgnoreUnexported(api.PasitHost{})
			if diff := cmp.Diff(got, tt.want, ignorePBFieldOpts); diff != "" {
				fmt.Println("Diff= ", diff)
				t.Errorf("GetTopology.TriggerRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTopology_getDUTTopology(t *testing.T) {
	fe2 := &executor.FakeCommander{FakeFn: func(c *exec.Cmd) ([]byte, error) {

		gotMachineLSE := []*ufsModels.MachineLSE{machineLSE(testHostname, ph), machineLSE(testHostname, ph)}
		marshalled := marshallMachineLSESlice(gotMachineLSE)

		return []byte(fmt.Sprintf("[%v]", strings.Join(marshalled, ","))), nil
	}}
	type fields struct {
		SatlabID string
		Hostname string
	}
	type args struct {
		ctx      context.Context
		executor executor.IExecCommander
		dutName  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *api.PasitHost
		wantErr bool
	}{
		{"should work", fields{"", testHostname}, args{context.Background(), fe, testHostname}, ph, false},
		{"should fail, get dut info via shivas", fields{"", testHostname}, args{context.Background(), FakeCommanderWithCommandError(paths.ShivasCLI), testHostname}, &api.PasitHost{}, true},
		{"should fail, too many duts returned", fields{"", testHostname}, args{context.Background(), fe2, testHostname}, &api.PasitHost{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &GetTopology{
				SatlabID: tt.fields.SatlabID,
				Hostname: tt.fields.Hostname,
			}
			got, err := c.getDUTTopology(tt.args.ctx, tt.args.executor, tt.args.dutName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTopology.getDUTTopology() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTopology.getDUTTopology() = %v, want %v", got, tt.want)
			}
		})
	}
}

func machineLSE(name string, ph *api.PasitHost) *ufsModels.MachineLSE {
	return &ufsModels.MachineLSE{
		Name:     name,
		Hostname: name,
		Machines: []string{"m1"},
		Lse: &ufsModels.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufsModels.ChromeOSMachineLSE{
				ChromeosLse: &ufsModels.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufsModels.ChromeOSDeviceLSE{
						Device: &ufsModels.ChromeOSDeviceLSE_Dut{
							Dut: &ufspb.DeviceUnderTest{
								Hostname: name,
								Pools:    []string{"p1"},
								Peripherals: &ufspb.Peripherals{
									PasitHost2: ph,
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
