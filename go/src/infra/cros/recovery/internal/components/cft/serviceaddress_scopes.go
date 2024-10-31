// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cft

import (
	"context"
	"strconv"
	"strings"

	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/ctr"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/scopes"
	"infra/cros/recovery/tlw"
)

// AddressToScope puts servic address of cft container to context scope.
func AddressToScope(ctx context.Context, ctrInfo ctr.ServiceInfo, containerName string) error {
	if ctrInfo == nil {
		return errors.Reason("cft address to scopes: ctr client is not provided").Err()
	}
	container, err := ctrInfo.GetContainer(ctx, containerName)
	if err != nil {
		return errors.Annotate(err, "cft address to scopes").Err()
	}
	address, err := container.ServiceAddress(ctx)
	if address == "" {
		return errors.Annotate(err, "cft address to scopes: address is empty").Err()
	}
	scopes.PutConfigParam(ctx, addressScopeKey(containerName), address)
	if _, err := AddressFromScope(ctx, containerName); err != nil {
		return errors.Annotate(err, "cft address to scopes").Err()
	}
	log.Debugf(ctx, "Address of %q saved to the scope context!", containerName)
	return nil
}

// AddressFromScope reads service address of cft container from context scope.
func AddressFromScope(ctx context.Context, containerName string) (string, error) {
	if containerName == "" {
		return "", errors.Reason("cft address from scopes: container name is not provided").Err()
	}
	if v, ok := scopes.ReadConfigParam(ctx, addressScopeKey(containerName)); ok {
		if str, ok := v.(string); ok && str != "" {
			return str, nil
		}
	}
	return "", errors.Reason("cft address from scopes: not found").Err()
}

// ADBServiceAddressFromScope read adb service client from scope.
func ADBServiceAddressFromScope(ctx context.Context, dut *tlw.Dut) (*lab_api.IpEndpoint, error) {
	addr, err := AddressFromScope(ctx, ADBName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "adb service address from scope").Err()
	}
	ip, err := addressToIPEndpoint(addr)
	if err != nil {
		return nil, errors.Annotate(err, "adb service address from scope").Err()
	}
	return ip, nil
}

// ServoServiceAddressFromScope read servo-nexus service client from scope.
func ServoServiceAddressFromScope(ctx context.Context, dut *tlw.Dut) (*lab_api.IpEndpoint, error) {
	addr, err := AddressFromScope(ctx, ServoNexusName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "servo-nexus service address from scope").Err()
	}
	ip, err := addressToIPEndpoint(addr)
	if err != nil {
		return nil, errors.Annotate(err, "servo-nexus service address from scope").Err()
	}
	return ip, nil
}

// FoilProvisionServiceAddressFromScope read foil-provision service client from scope.
func FoilProvisionServiceAddressFromScope(ctx context.Context, dut *tlw.Dut) (*lab_api.IpEndpoint, error) {
	addr, err := AddressFromScope(ctx, FoilProvisionName(dut))
	if err != nil {
		return nil, errors.Annotate(err, "foil-provision service address from scope").Err()
	}
	ip, err := addressToIPEndpoint(addr)
	if err != nil {
		return nil, errors.Annotate(err, "foil-provision service address from scope").Err()
	}
	return ip, nil
}

func addressToIPEndpoint(addr string) (*lab_api.IpEndpoint, error) {
	addr = strings.Trim(strings.TrimSpace(addr), ":")
	if addr == "" {
		return nil, errors.Reason("address to ip endpoint: address is empty").Err()
	}
	parts := strings.Split(addr, ":")
	if len(parts) == 1 {
		return &lab_api.IpEndpoint{
			Address: parts[0],
		}, nil
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, errors.Annotate(err, "address to ip endpoint: bad port").Err()
	}
	return &lab_api.IpEndpoint{
		Address: parts[0],
		Port:    int32(port),
	}, nil
}
