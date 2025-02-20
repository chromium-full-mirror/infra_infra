// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc"

	"go.chromium.org/luci/common/errors"

	ufsModel "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// fakeUFSClient maintains a map of all MachineLSEs. Get/Update commands both
// operate on the machineLSE map.
type fakeUFSClient struct {
	machineLSEs           map[string]*ufsModel.MachineLSE
	getMachineLSECalls    []*ufsApi.GetMachineLSERequest
	updateMachineLSECalls []*ufsApi.UpdateMachineLSERequest
}

func (c *fakeUFSClient) GetMachine(ctx context.Context, req *ufsApi.GetMachineRequest, opts ...grpc.CallOption) (*ufsModel.Machine, error) {
	return nil, nil
}

func (c *fakeUFSClient) GetMachineLSE(ctx context.Context, req *ufsApi.GetMachineLSERequest, opts ...grpc.CallOption) (*ufsModel.MachineLSE, error) {
	c.getMachineLSECalls = append(c.getMachineLSECalls, req)

	lse, ok := c.machineLSEs[req.GetName()]
	if !ok {
		return nil, errors.New("No LSE found")
	}

	return lse, nil
}

func (c *fakeUFSClient) UpdateMachineLSE(ctx context.Context, req *ufsApi.UpdateMachineLSERequest, opts ...grpc.CallOption) (*ufsModel.MachineLSE, error) {
	c.updateMachineLSECalls = append(c.updateMachineLSECalls, req)

	c.machineLSEs[req.GetMachineLSE().GetName()] = req.GetMachineLSE()

	return req.GetMachineLSE(), nil
}

// fakePinger will error or not when pinging depending on `err` status.
type fakePinger struct {
	err bool
}

func (p *fakePinger) Ping() error {
	if p.err {
		return errors.New("couldnt ping")
	}

	return nil
}

// Tests the behavior of calls to UFS.
func TestUpdateDUTStateCallsUFS(t *testing.T) {
	tests := []struct {
		name            string
		hostname        string
		machineLSEs     map[string]*ufsModel.MachineLSE
		wantGetCalls    []*ufsApi.GetMachineLSERequest
		wantUpdateCalls []*ufsApi.UpdateMachineLSERequest
		wantErr         bool
	}{
		{
			name:            "machine does not exist causes error + no update",
			hostname:        "fake",
			machineLSEs:     map[string]*ufsModel.MachineLSE{},
			wantGetCalls:    []*ufsApi.GetMachineLSERequest{{Name: "machineLSEs/fake"}},
			wantUpdateCalls: nil,
			wantErr:         true,
		},
		{
			name:         "machine exists causes update called",
			hostname:     "real",
			machineLSEs:  map[string]*ufsModel.MachineLSE{"machineLSEs/real": {Name: "real"}},
			wantGetCalls: []*ufsApi.GetMachineLSERequest{{Name: "machineLSEs/real"}},
			wantUpdateCalls: []*ufsApi.UpdateMachineLSERequest{
				{
					MachineLSE: &ufsModel.MachineLSE{
						Name:          "real",
						ResourceState: ufsModel.State_STATE_SERVING,
					},
					UpdateMask: &field_mask.FieldMask{
						Paths: []string{"resourceState"},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &updateDUTState{
				updateDUTStateFlags: updateDUTStateFlags{
					hostname: tt.hostname,
					state:    "ready",
				},
			}

			ufs := &fakeUFSClient{
				machineLSEs: tt.machineLSEs,
			}

			if err := c.innerRunWithClients(context.Background(), ufs, tt.hostname, &fakePinger{}); (err != nil) != tt.wantErr {
				t.Errorf("updateDUTState.innerRunWithClients() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(ufs.getMachineLSECalls, tt.wantGetCalls, cmpopts.IgnoreUnexported(ufsApi.GetMachineLSERequest{})); diff != "" {
				t.Errorf("unexpected diff in getMachineLSE calls: %s", diff)
			}

			if diff := cmp.Diff(ufs.updateMachineLSECalls, tt.wantUpdateCalls, cmpopts.IgnoreUnexported(field_mask.FieldMask{}, ufsApi.UpdateMachineLSERequest{}, ufsModel.MachineLSE{})); diff != "" {
				t.Errorf("unexpected diff in getMachineLSE calls: %s", diff)
			}
		})
	}
}

// TestPingerBehavior checks the UFS, return status of command given certain
// ping status and force params.
func TestPingerBehavior(t *testing.T) {
	tests := []struct {
		name         string
		pingerError  bool
		force        bool
		wantGetCalls []*ufsApi.GetMachineLSERequest
		wantErr      bool
	}{
		{
			name:         "pinger fine without force calls UFS and doesnt error",
			pingerError:  false,
			force:        false,
			wantGetCalls: []*ufsApi.GetMachineLSERequest{{Name: "machineLSEs/real"}},
			wantErr:      false,
		},
		{
			name:         "pinger erroring without force doesnt call UFS and doesnt error",
			pingerError:  true,
			force:        false,
			wantGetCalls: nil,
			wantErr:      true,
		},
		{
			name:         "pinger fine with force calls UFS and doesnt error",
			pingerError:  false,
			force:        true,
			wantGetCalls: []*ufsApi.GetMachineLSERequest{{Name: "machineLSEs/real"}},
			wantErr:      false,
		},
		{
			name:         "pinger erroring with force doesnt calls UFS and doesnt error",
			pingerError:  true,
			force:        true,
			wantGetCalls: []*ufsApi.GetMachineLSERequest{{Name: "machineLSEs/real"}},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &updateDUTState{
				updateDUTStateFlags: updateDUTStateFlags{
					hostname: "real",
					state:    "ready",
					force:    tt.force,
				},
			}

			ufs := &fakeUFSClient{
				machineLSEs: map[string]*ufsModel.MachineLSE{"machineLSEs/real": {Name: "real"}},
			}

			// check error
			if err := c.innerRunWithClients(context.Background(), ufs, "real", &fakePinger{err: tt.pingerError}); (err != nil) != tt.wantErr {
				t.Errorf("updateDUTState.innerRunWithClients() error = %v, wantErr %v", err, tt.wantErr)
			}

			// check to see if it hits UFS
			if diff := cmp.Diff(ufs.getMachineLSECalls, tt.wantGetCalls, cmpopts.IgnoreUnexported(ufsApi.GetMachineLSERequest{})); diff != "" {
				t.Errorf("unexpected diff in getMachineLSE calls: %s", diff)
			}
		})
	}
}
