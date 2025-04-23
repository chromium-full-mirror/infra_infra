// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/dutinfo"
	"go.chromium.org/infra/cros/recovery/internal/localtlw/localinfo"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
)

// ListResourcesForUnit provides list of resources names related to target unit.
func (c *tlwClient) ListResourcesForUnit(ctx context.Context, name string) ([]string, error) {
	if name == "" {
		return nil, errors.Reason("list resources: unit name is expected").Err()
	}
	resourceNames, err := c.readInventory(ctx, name)
	return resourceNames, errors.Annotate(err, "list resources %q", name).Err()
}

// GetDut provides DUT info per requested resource name from inventory.
func (c *tlwClient) GetDut(ctx context.Context, name string) (*tlw.Dut, error) {
	dut, err := c.getDevice(ctx, name)
	if err != nil {
		return nil, errors.Annotate(err, "get DUT %q", name).Err()
	}
	dut.ProvisionedInfo, err = localinfo.ReadProvisionInfo(ctx, dut.Name)
	return dut, errors.Annotate(err, "get dut").Err()
}

// getDevice receives device from inventory.
func (c *tlwClient) getDevice(ctx context.Context, name string) (*tlw.Dut, error) {
	// TODO: remove logic to find device and get it from context.
	if dutName, ok := c.hostToParents[name]; ok {
		// the device was previously
		name = dutName
	}
	// First check if device is already in the cache.
	if d, ok := c.devices[name]; ok {
		return d, nil
	}
	// Ask to read inventory and then get device from the cache.
	// If it is still not in the cache then device is unit, not a DUT
	if _, err := c.readInventory(ctx, name); err != nil {
		return nil, errors.Annotate(err, "get device").Err()
	}
	if d, ok := c.devices[name]; ok {
		log.Debugf(ctx, "Get device info %q: from inventory.", name)
		return d, nil
	}
	return nil, errors.Reason("get device: unexpected error").Err()
}

// Read inventory and return resource names.
// As additional received devices will be cached.
// Please try to check cache before call the method.
func (c *tlwClient) readInventory(ctx context.Context, name string) (resourceNames []string, rErr error) {
	ddrsp, err := c.ufsClient.GetDeviceData(ctx, &ufsAPI.GetDeviceDataRequest{Hostname: name})
	if err != nil {
		return resourceNames, errors.Annotate(err, "read inventory %q", name).Err()
	}
	var dut *tlw.Dut
	switch ddrsp.GetResourceType() {
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_ATTACHED_DEVICE:
		attachedDevice := ddrsp.GetAttachedDeviceData()
		dut, err = dutinfo.ConvertAttachedDeviceToTlw(attachedDevice)
		if err != nil {
			return resourceNames, errors.Annotate(err, "read inventory %q: attached device", name).Err()
		}
		c.cacheDevice(dut)
		resourceNames = []string{dut.Name}
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_CHROMEOS_DEVICE:
		dd := ddrsp.GetChromeOsDeviceData()
		dut, err = dutinfo.ConvertDut(dd)
		if err != nil {
			return resourceNames, errors.Annotate(err, "get device %q: chromeos device", name).Err()
		}
		c.cacheDevice(dut)
		resourceNames = []string{dut.Name}
	case ufsAPI.GetDeviceDataResponse_RESOURCE_TYPE_SCHEDULING_UNIT:
		su := ddrsp.GetSchedulingUnit()
		resourceNames = su.GetMachineLSEs()
	default:
		return resourceNames, errors.Reason("get device %q: unsupported type %q", name, ddrsp.GetResourceType()).Err()
	}
	return resourceNames, nil
}

// cacheDevice puts device to local cache and set list host name knows for DUT.
func (c *tlwClient) cacheDevice(dut *tlw.Dut) {
	if dut == nil {
		// Skip as DUT not found.
		return
	}
	c.devices[dut.Name] = dut
	c.hostToParents[dut.Name] = dut.Name
	if dut.GetAndroid() != nil {
		c.hostTypes[dut.Name] = hostTypeAndroid
		return
	}
	c.hostTypes[dut.Name] = hostTypeChromeOs
	chromeos := dut.GetChromeos()
	if s := chromeos.GetServo(); s.GetName() != "" {
		c.hostTypes[s.GetName()] = hostTypeServo
		c.hostToParents[s.GetName()] = dut.Name
	}
	for _, bt := range chromeos.GetBluetoothPeers() {
		c.hostTypes[bt.GetName()] = hostTypeBtPeer
		c.hostToParents[bt.GetName()] = dut.Name
	}
	for _, router := range chromeos.GetWifiRouters() {
		c.hostTypes[router.GetName()] = hostTypeRouter
		c.hostToParents[router.GetName()] = dut.Name
	}
	if chameleon := chromeos.GetChameleon(); chameleon.GetName() != "" {
		c.hostTypes[chameleon.GetName()] = hostTypeChameleon
		c.hostToParents[chameleon.GetName()] = dut.Name
	}
	hmr := chromeos.GetHumanMotionRobot()
	if hmr.GetName() != "" {
		c.hostTypes[hmr.GetName()] = hostTypeHmrPi
		c.hostToParents[hmr.GetName()] = dut.Name
	}
	if hmr.GetTouchhost() != "" {
		c.hostTypes[hmr.GetTouchhost()] = hostTypeHmrGateway
		c.hostToParents[hmr.GetTouchhost()] = dut.Name
	}
}

// unCacheDevice removes device from the local cache.
func (c *tlwClient) unCacheDevice(dut *tlw.Dut) {
	if dut == nil {
		// Skip as DUT not provided.
		return
	}
	name := dut.Name
	delete(c.hostToParents, name)
	delete(c.hostTypes, name)
	if chromeos := dut.GetChromeos(); chromeos != nil {
		if sh := chromeos.GetServo(); sh.GetName() != "" {
			delete(c.hostTypes, sh.GetName())
			delete(c.hostToParents, sh.GetName())
		}
		for _, bt := range chromeos.GetBluetoothPeers() {
			delete(c.hostTypes, bt.GetName())
			delete(c.hostToParents, bt.GetName())
		}
		if chameleon := chromeos.GetChameleon(); chameleon.GetName() != "" {
			delete(c.hostTypes, chameleon.GetName())
			delete(c.hostToParents, chameleon.GetName())
		}
		hmr := chromeos.GetHumanMotionRobot()
		if hmr.GetName() != "" {
			delete(c.hostTypes, hmr.GetName())
			delete(c.hostToParents, hmr.GetName())
		}
		if hmr.GetTouchhost() != "" {
			delete(c.hostTypes, hmr.GetTouchhost())
			delete(c.hostToParents, hmr.GetTouchhost())
		}
	}
	delete(c.devices, name)
}

// UpdateDut updates DUT info into inventory.
func (c *tlwClient) UpdateDut(ctx context.Context, dut *tlw.Dut) error {
	if dut == nil {
		return errors.Reason("update DUT: DUT is not provided").Err()
	}
	dut, err := c.getDevice(ctx, dut.Name)
	if err != nil {
		return errors.Annotate(err, "update DUT %q", dut.Name).Err()
	}
	log.Debugf(ctx, "Creating update DUT request ....")
	req, err := dutinfo.CreateUpdateDutRequest(dut.Id, dut)
	if err != nil {
		return errors.Annotate(err, "update DUT %q", dut.Name).Err()
	}
	log.Debugf(ctx, "Update DUT: update request: %s", req)
	rsp, err := c.ufsClient.UpdateDeviceRecoveryData(ctx, req)
	if err != nil {
		log.Debugf(ctx, "Fail to update inventory for %q: %v", dut.Name, err)
		return errors.Annotate(err, "update DUT %q", dut.Name).Err()
	}
	log.Debugf(ctx, "Update DUT: update response: %s", rsp)
	c.unCacheDevice(dut)
	// Update provisioning data on the execution env.
	err = localinfo.UpdateProvisionInfo(ctx, dut)
	return errors.Annotate(err, "udpate dut").Err()
}
