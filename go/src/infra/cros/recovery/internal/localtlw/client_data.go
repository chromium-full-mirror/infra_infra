// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localtlw provides local implementation of TLW Access.
package localtlw

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"

	fleet "infra/appengine/crosskylabadmin/api/fleet/v1"
	"infra/cros/recovery/internal/execs/wifirouter/controller"
	"infra/cros/recovery/internal/localtlw/dutinfo"
	"infra/cros/recovery/internal/localtlw/localinfo"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/models"
	"infra/cros/recovery/tlw"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
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

// Version provides versions for requested device and type of versions.
func (c *tlwClient) Version(ctx context.Context, req *tlw.VersionRequest) (*tlw.VersionResponse, error) {
	if req == nil {
		return nil, errors.Reason("version: request is not provided").Err()
	}
	var versionKey string
	var dut *tlw.Dut
	// Creating cache key for versions based on hostname which is targeted.
	if req.GetResource() != "" {
		versionKey = fmt.Sprintf("%s|%s", req.GetType(), req.GetResource())
		var err error
		dut, err = c.getDevice(ctx, req.GetResource())
		if err != nil {
			return nil, errors.Annotate(err, "version").Err()
		}
	}
	// TODO(otabek): That fix is till we move to CFT components.
	// The change will be extracted to execs and always provide board/model.
	if dut != nil && req.GetType() == tlw.VersionRequest_CROS && strings.HasSuffix(req.GetResource(), "-tablet") {
		req.Board = dut.GetChromeos().GetBoard()
		req.Model = fmt.Sprintf("%s_tablet", dut.GetChromeos().GetModel())
		// Table has special version.
		log.Debugf(ctx, "Version: detected tablet device, update request to %v", req)
	}
	if req.GetBoard() != "" || req.GetModel() != "" {
		versionKey = fmt.Sprintf("%s|%s|%s", req.GetType(), req.GetBoard(), req.GetModel())
	}
	if versionKey == "" {
		return nil, errors.Reason("version: request is empty").Err()
	}
	if v, ok := c.versionMap[versionKey]; ok {
		log.Debugf(ctx, "Received version %q (cache): %#v", req.GetType(), v)
		return v, nil
	}
	switch req.GetType() {
	case tlw.VersionRequest_CROS:
		sv, err := c.getCrosStableVersion(ctx, dut, req)
		if err != nil && dut != nil {
			log.Debugf(ctx, "version: %s", err)
			log.Debugf(ctx, "Failed to fetch stable version from Cros Skylab Admin. Checking for local stable version...")
			sv, err = c.getLocalStableVersion(ctx, dut)
			if err != nil {
				log.Debugf(ctx, "local version: %s", err)
				log.Debugf(ctx, "Failed to fetch stable version from both Cros Skylab Admin and local directory.")
				return nil, errors.Annotate(err, "local version").Err()
			}
		}
		log.Debugf(ctx, "Received Cros version: %#v", sv)
		// Cache received version for future usage.
		c.versionMap[versionKey] = sv
		return sv, nil

	case tlw.VersionRequest_WIFI_ROUTER:
		// TODO(otabek): Re-point for external source as soon we have data for that.
		// TODO(otabek): Need apply cache.
		// On this step we only support gale/gale devices other will fail to receive version.
		var routerHost *tlw.WifiRouterHost
		for _, router := range dut.GetChromeos().GetWifiRouters() {
			if router.GetName() == req.Resource {
				routerHost = router
				break
			}
		}
		if routerHost == nil {
			return nil, errors.Reason("version: target device not  found").Err()
		} else if routerHost.GetModel() == controller.RouterModelGale {
			return &tlw.VersionResponse{
				Value: map[string]string{
					"os_image": "gale-test-ap-tryjob/R92-13982.81.0-b4959409",
				},
			}, nil
		}
	}
	return nil, errors.Reason("version: version not found").Err()
}

// getDevice receives device from inventory.
func (c *tlwClient) getDevice(ctx context.Context, name string) (*tlw.Dut, error) {
	if dutName, ok := c.hostToParents[name]; ok {
		// the device was previously
		name = dutName
	}
	// First check if device is already in the cache.
	if d, ok := c.devices[name]; ok {
		log.Debugf(ctx, "Get device info %q: received from cache.", name)
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

// getCrosStableVersion receives stable versions for ChromeOS device.
func (c *tlwClient) getCrosStableVersion(ctx context.Context, dut *tlw.Dut, vReq *tlw.VersionRequest) (*tlw.VersionResponse, error) {
	var req *fleet.GetStableVersionRequest
	if vReq.GetBoard() != "" && vReq.GetModel() != "" {
		log.Debugf(ctx, "Use board: %q and model: %q to find stable version!", vReq.GetBoard(), vReq.GetModel())
		// Do not pass hostname to avoid use of it as primary key of the request.
		req = &fleet.GetStableVersionRequest{
			BuildTarget: vReq.GetBoard(),
			Model:       vReq.GetModel(),
		}
	} else {
		log.Debugf(ctx, "Use host: %q to find stable version!", dut.Name)
		req = &fleet.GetStableVersionRequest{Hostname: dut.Name}
	}
	if c.csaClient == nil {
		return nil, errors.Reason("get stable-version %q: service is not specified", dut.Name).Err()
	}
	res, err := c.csaClient.GetStableVersion(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.Reason("get stable-version %q: record not found", dut.Name).Err()
		}
		return nil, errors.Annotate(err, "get stable-version %q", dut.Name).Err()
	}
	if res.GetCrosVersion() == "" {
		return nil, errors.Reason("get stable-version %q: version is empty", dut.Name).Err()
	}
	return &tlw.VersionResponse{
		Value: map[string]string{
			"os_image":   fmt.Sprintf("%s-release/%s", dut.GetChromeos().GetBoard(), res.GetCrosVersion()),
			"fw_image":   res.GetFaftVersion(),
			"fw_version": res.GetFirmwareVersion(),
		},
	}, nil
}

// getLocalStableVersion receives local stable versions for ChromeOS device.
func (c *tlwClient) getLocalStableVersion(ctx context.Context, dut *tlw.Dut) (*tlw.VersionResponse, error) {

	if dut.GetChromeos().GetBoard() == "" {
		return nil, fmt.Errorf("get local stable-version %q: board missing", dut.Name)
	}
	if dut.GetChromeos().GetModel() == "" {
		return nil, fmt.Errorf("get local stable-version %q: model missing", dut.Name)
	}
	stableVersionDirectory := os.Getenv("DRONE_RECOVERY_VERSIONS_DIR")
	if stableVersionDirectory == "" {
		return nil, fmt.Errorf("get local stable-version %q: recovery versions directory not set", dut.Name)
	}
	stableVersionPath := fmt.Sprintf("%s%s-%s.json", stableVersionDirectory, dut.GetChromeos().GetBoard(), dut.GetChromeos().GetModel())

	svFile, err := os.Open(stableVersionPath)
	if err != nil {
		return nil, errors.Annotate(err, "get local stable-version %q: cannot open stable version file", dut.Name).Err()
	}
	defer svFile.Close()

	svByteArr, err := io.ReadAll(svFile)
	if err != nil {
		return nil, errors.Annotate(err, "get local stable-version %q: cannot read stable version file", dut.Name).Err()
	}

	recovery_version := models.RecoveryVersion{}
	err = json.Unmarshal(svByteArr, &recovery_version)
	if err != nil {
		return nil, errors.Annotate(err, "get local stable-version %q: cannot parse stable version file", dut.Name).Err()
	}
	return &tlw.VersionResponse{
		Value: map[string]string{
			"board":      recovery_version.GetBoard(),
			"model":      recovery_version.GetModel(),
			"os_image":   fmt.Sprintf("%s-release/%s", dut.GetChromeos().GetBoard(), recovery_version.GetOsImage()),
			"fw_image":   recovery_version.GetFwImage(),
			"fw_version": recovery_version.GetFwVersion(),
		},
	}, nil
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
