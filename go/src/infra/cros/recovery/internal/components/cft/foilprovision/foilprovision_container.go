// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package foilprovision contains methods to work with an foil-provision container.
package foilprovision

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	lab_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/recovery/ctr"
	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// Setup sets the service for future provision calls.
func Setup(ctx context.Context, client api.GenericProvisionServiceClient, dut *tlw.Dut, labDut *labapi.Dut, servoNexusAddr *labapi.IpEndpoint) error {
	if client == nil {
		return errors.Reason("foil-provision setup: client is not provided").Err()
	}
	if dut == nil {
		return errors.Reason("foil-provision setup: dut is not provided").Err()
	}
	if labDut == nil {
		return errors.Reason("foil-provision setup: lab-dut is not provided").Err()
	}
	req := &api.ProvisionStartupRequest{
		Dut:            labDut,
		ServoNexusAddr: servoNexusAddr,
	}
	log.Debugf(ctx, "Foil-provision setup: call with request %v.", req)
	res, err := client.StartUp(ctx, req)
	if err != nil {
		return errors.Annotate(err, "foil-provision setup: call failure").Err()
	}
	switch res.GetStatus() {
	case api.ProvisionStartupResponse_STATUS_SUCCESS:
		log.Debugf(ctx, "foil-rpovision setup: successed!")
		return nil
	default:
		return errors.Reason("foils-provision setup: status %s", res.GetStatus()).Err()
	}
}

// Install performs provision OS on the DUT.
func Install(ctx context.Context, client api.GenericProvisionServiceClient, imagePath *lab_go.StoragePath, preventReboot bool) error {
	if client == nil {
		return errors.Reason("foil-provision install: client is not provided").Err()
	}
	metadata, err := anypb.New(&api.CrOSProvisionMetadata{})
	if err != nil {
		return errors.Annotate(err, "foil-provision install: fail to create metadata").Err()
	}
	req := &api.InstallRequest{
		ImagePath:     imagePath,
		PreventReboot: preventReboot,
		Metadata:      metadata,
	}
	log.Debugf(ctx, "Foil-provision install: call with request %v.", req)
	op, err := client.Install(ctx, req)
	if err != nil {
		return errors.Annotate(err, "foil-provision install").Err()
	}
	opRes, err := common.ProcessDoneLro(ctx, op)
	if err != nil {
		return errors.Annotate(err, "foil-provision install: lro failure").Err()
	}
	res := &api.InstallResponse{}
	if err := opRes.UnmarshalTo(res); err != nil {
		return errors.Annotate(err, "foil-provision install: lro response unmarshalling failed").Err()
	}
	if res.GetStatus() != api.InstallResponse_STATUS_SUCCESS {
		log.Debugf(ctx, "foil-provision install: metadata %v", res.Metadata)
		return errors.Reason("foil-provision install: status %s", res.GetStatus()).Err()
	}
	log.Infof(ctx, "Foil-rpovision status %s!", res.GetStatus())
	return nil
}

// ServiceClient creates service client to the service running on CFT container.
func ServiceClient(ctx context.Context, ctrInfo ctr.ServiceInfo, dut *tlw.Dut) (api.GenericProvisionServiceClient, error) {
	if dut == nil {
		return nil, errors.Reason("foil-provision service client: dut is not provided").Err()
	}
	if ctrInfo == nil {
		return nil, errors.Reason("foil-provision service client: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, cft.FoilProvisionName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "foil-provision service client").Err()
	}
	conn, err := container.GetClient(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "foil-provision service client").Err()
	}
	client := api.NewGenericProvisionServiceClient(conn)
	if client == nil {
		return nil, errors.Reason("foil-provision service client: fail to create client").Err()
	}
	return client, nil
}
