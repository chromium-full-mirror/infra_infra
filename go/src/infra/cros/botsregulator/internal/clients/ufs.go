// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clients

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	shivasUtil "go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/libs/skylab/common/heuristics"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	chromeosLab "go.chromium.org/infra/unifiedfleet/api/v1/models/chromeos/lab"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

type contextKey string

// MockUFSClientKey is used for testing.
var MockUFSClientKey contextKey = "used in tests only for setting the mock UFSClient"

// UFSClient is UFS API	wrapper for BotsRegulator specific usage.
// It is used for mocking and testing.
type UFSClient interface {
	BatchListMachineLSEs(ctx context.Context, filters []string, pageSize int, keysOnly, full bool) ([]protoadapt.MessageV1, error)
	BatchListMachines(ctx context.Context, filters []string, pageSize int, keysOnly, full bool) ([]protoadapt.MessageV1, error)
	BatchListSchedulingUnits(ctx context.Context, filters []string, pageSize int, keysOnly, full bool) ([]protoadapt.MessageV1, error)
	UpdateMachineLSE(ctx context.Context, in *ufsAPI.UpdateMachineLSERequest, opts ...grpc.CallOption) (*ufspb.MachineLSE, error)
}

func NewUFSClient(ctx context.Context, host string) (UFSClient, error) {
	if mockClient, ok := ctx.Value(MockUFSClientKey).(UFSClient); ok {
		return mockClient, nil
	}
	pc, err := rawPRPCClient(ctx, host)
	if err != nil {
		return nil, err
	}
	ic := ufsAPI.NewFleetPRPCClient(pc)
	return &ufsService{
		client: ic,
	}, nil
}

// ufsService is used in non-test environments.
type ufsService struct {
	client ufsAPI.FleetClient
}

func (u *ufsService) BatchListMachineLSEs(ctx context.Context, filters []string, pageSize int, keysOnly, full bool) ([]protoadapt.MessageV1, error) {
	return shivasUtil.BatchList(ctx, u.client, listMachineLSEs, filters, pageSize, keysOnly, full, nil)
}

func (u *ufsService) BatchListMachines(ctx context.Context, filters []string, pageSize int, keysOnly, full bool) ([]protoadapt.MessageV1, error) {
	return shivasUtil.BatchList(ctx, u.client, listMachines, filters, pageSize, keysOnly, full, nil)
}

func (u *ufsService) BatchListSchedulingUnits(ctx context.Context, filters []string, pageSize int, keysOnly, full bool) ([]protoadapt.MessageV1, error) {
	return shivasUtil.BatchList(ctx, u.client, listSchedulingUnits, filters, pageSize, keysOnly, full, nil)
}

func (u *ufsService) UpdateMachineLSE(ctx context.Context, in *ufsAPI.UpdateMachineLSERequest, opts ...grpc.CallOption) (*ufspb.MachineLSE, error) {
	return u.client.UpdateMachineLSE(ctx, in, opts...)
}

// SetUFSNamespace is a helper function to set UFS namespace in context.
func SetUFSNamespace(ctx context.Context, namespace string) context.Context {
	md := metadata.Pairs("namespace", namespace)
	return metadata.NewOutgoingContext(ctx, md)
}

// InitializeUpdateLSERequest return a new initialized UpdateMachineLSERequest.
func InitializeUpdateLSERequest(hostname, hive string) *ufsAPI.UpdateMachineLSERequest {
	if heuristics.LooksLikeLabstation(hostname) {
		return initializeUpdateLabstationRequest(hostname, hive)
	}
	return initializeUpdateDUTRequest(hostname, hive)
}

// initializeUpdateDUTRequest return a new initialized UpdateMachineLSERequest for DUT.
func initializeUpdateDUTRequest(hostname, hive string) *ufsAPI.UpdateMachineLSERequest {
	// An empty machineLSE is enough. UFS will fetch the correct lse from the machinelse.name.
	// 3679c23a3c07de90bc8d4241ea77416cf3dcda45:infra/go/src/infra/unifiedfleet/app/controller/dut.go;l=160
	// Shivas for ref: 132b2fe1a670c91e9eaad45b3cb0d04601ad0ce3:go/src/infra/cmd/shivas/internal/ufs/subcmds/dut/update_dut_batch.go;l=255
	lse := &ufspb.MachineLSE{
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Dut{
							Dut: &chromeosLab.DeviceUnderTest{
								Peripherals: &chromeosLab.Peripherals{
									Chameleon:     &chromeosLab.Chameleon{},
									Servo:         &chromeosLab.Servo{},
									Rpm:           &chromeosLab.OSRPM{},
									Audio:         &chromeosLab.Audio{},
									Wifi:          &chromeosLab.Wifi{},
									Touch:         &chromeosLab.Touch{},
									CameraboxInfo: &chromeosLab.Camerabox{},
									Dolos:         &chromeosLab.Dolos{},
								},
							},
						},
					},
				},
			},
		},
	}
	lse.Name = ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, hostname)
	lse.Hostname = hostname
	lse.GetChromeosMachineLse().GetDeviceLse().GetDut().Hostname = hostname
	lse.GetChromeosMachineLse().GetDeviceLse().GetDut().Hive = hive
	req := &ufsAPI.UpdateMachineLSERequest{
		MachineLSE: lse,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"dut.hive"},
		},
	}
	return req
}

// initializeUpdateLabstationRequest return a new initialized UpdateMachineLSERequest for labstation.
func initializeUpdateLabstationRequest(hostname, hive string) *ufsAPI.UpdateMachineLSERequest {
	// Generate lse and mask
	lse := &ufspb.MachineLSE{
		Lse: &ufspb.MachineLSE_ChromeosMachineLse{
			ChromeosMachineLse: &ufspb.ChromeOSMachineLSE{
				ChromeosLse: &ufspb.ChromeOSMachineLSE_DeviceLse{
					DeviceLse: &ufspb.ChromeOSDeviceLSE{
						Device: &ufspb.ChromeOSDeviceLSE_Labstation{
							Labstation: &chromeosLab.Labstation{
								Rpm: &chromeosLab.OSRPM{},
							},
						},
					},
				},
			},
		},
	}

	lse.Name = ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, hostname)
	lse.Hostname = hostname
	lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Hostname = hostname
	lse.GetChromeosMachineLse().GetDeviceLse().GetLabstation().Hive = hive
	req := &ufsAPI.UpdateMachineLSERequest{
		MachineLSE: lse,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"labstation.hive"},
		},
	}
	return req
}

// listMachineLSEs is a helper function to list MachineLSEs from UFS.
func listMachineLSEs(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]protoadapt.MessageV1, string, error) {
	req := &ufsAPI.ListMachineLSEsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
		Full:      full,
	}
	res, err := ic.ListMachineLSEs(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]protoadapt.MessageV1, len(res.GetMachineLSEs()))
	for i, lse := range res.GetMachineLSEs() {
		protos[i] = lse
	}
	return protos, res.GetNextPageToken(), nil
}

// listMachines is a helper function to list Machines from UFS.
func listMachines(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]protoadapt.MessageV1, string, error) {
	req := &ufsAPI.ListMachinesRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
		Full:      full,
	}
	res, err := ic.ListMachines(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]protoadapt.MessageV1, len(res.GetMachines()))
	for i, mc := range res.GetMachines() {
		protos[i] = mc
	}
	return protos, res.GetNextPageToken(), nil
}

// listSchedulingUnits is a helper function to list SchedulingUnits from UFS.
func listSchedulingUnits(ctx context.Context, ic ufsAPI.FleetClient, pageSize int32, pageToken, filter string, keysOnly, full bool) ([]protoadapt.MessageV1, string, error) {
	req := &ufsAPI.ListSchedulingUnitsRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		KeysOnly:  keysOnly,
	}
	res, err := ic.ListSchedulingUnits(ctx, req)
	if err != nil {
		return nil, "", err
	}
	protos := make([]protoadapt.MessageV1, len(res.GetSchedulingUnits()))
	for i, su := range res.GetSchedulingUnits() {
		protos[i] = su
	}
	return protos, res.GetNextPageToken(), nil
}
