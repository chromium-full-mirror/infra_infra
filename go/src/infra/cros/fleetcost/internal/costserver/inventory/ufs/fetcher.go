// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"

	"google.golang.org/protobuf/protoadapt"

	"go.chromium.org/luci/common/errors"

	shivasUtil "go.chromium.org/infra/cmd/shivas/utils"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// GetAllMachineLSEs gets all MachineLSEs.
func GetAllMachineLSEs(ctx context.Context, ic ufsAPI.FleetClient, keysOnly bool, sink chan<- protoadapt.MessageV1) ([]*ufspb.MachineLSE, error) {
	res, err := shivasUtil.BatchList(ctx, ic, listMachineLSEs, []string{}, 0, keysOnly, false, sink)
	if err != nil {
		return nil, errors.Annotate(err, "get all chromeos machinelses").Err()
	}
	lses := make([]*ufspb.MachineLSE, len(res))
	for i, r := range res {
		lses[i] = r.(*ufspb.MachineLSE)
	}
	return lses, nil
}

// GetChromeosDeviceData gets a single ChromeosDeviceData based on hostname.
func GetChromeosDeviceData(ctx context.Context, ic ufsAPI.FleetClient, name string) (*ufspb.ChromeOSDeviceData, error) {
	res, err := ic.GetChromeOSDeviceData(ctx, &ufsAPI.GetChromeOSDeviceDataRequest{Hostname: name})
	if err != nil {
		return nil, errors.Annotate(err, "get chromeos device data").Err()
	}
	return res, nil
}

// GetLabstationDutMapping gets a map of labstation to dut hostnames based on provided labstation hostnames.
func GetLabstationDutMapping(ctx context.Context, ic ufsAPI.FleetClient, labs []string) (map[string][]string, error) {
	res, err := ic.GetDUTsForLabstation(ctx, &ufsAPI.GetDUTsForLabstationRequest{Hostname: labs})
	if err != nil {
		return nil, errors.Annotate(err, "get labstation dut mapping").Err()
	}
	labMap := make(map[string][]string)
	for _, item := range res.GetItems() {
		labMap[item.GetHostname()] = item.GetDutName()
	}
	return labMap, nil
}

// listMachineLSEs calls the list MachineLSE in UFS to get a list of MachineLSEs
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
		return nil, "", errors.Annotate(err, "list machine lses").Err()
	}
	protos := make([]protoadapt.MessageV1, len(res.GetMachineLSEs()))
	for i, kvm := range res.GetMachineLSEs() {
		protos[i] = kvm
	}
	return protos, res.GetNextPageToken(), nil
}
