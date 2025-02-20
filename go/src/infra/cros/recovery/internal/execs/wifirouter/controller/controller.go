// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package controller defines a common router controller class and
// implementations of this class per router type.
package controller

import (
	"context"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/scopes"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// IdentifyRouterDeviceType probes the router host over ssh to determine its
// device type.
func IdentifyRouterDeviceType(ctx context.Context, sshAccess ssh.Access, resource string) (labapi.WifiRouterDeviceType, error) {
	if sshAccess == nil {
		return 0, errors.Reason("sshAccess must not be nil").Err()
	}

	// Check if it's a Gale device.
	deviceType := labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_CHROMEOS_GALE
	log.Infof(ctx, "Checking if router host has the device type of %q", deviceType)
	sshRunner := newRouterSSHRunner(sshAccess, resource, deviceType)
	if err := ssh.TryAccess(ctx, sshRunner); err != nil {
		log.Debugf(ctx, "Failed to ssh into router host when treating it as the device type of %q: %v", deviceType, err)
	} else {
		isChromeOSGale, err := hostIsChromeOSGaleRouter(ctx, sshRunner)
		if err != nil {
			return 0, errors.Annotate(err, "failed to check if host has the device type %s", deviceType).Err()
		}
		if isChromeOSGale {
			return deviceType, nil
		}
	}

	// Check if it's an OpenWrt device.
	deviceType = labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_OPENWRT
	log.Infof(ctx, "Checking if router host has the device type of %q", deviceType)
	sshRunner = newRouterSSHRunner(sshAccess, resource, deviceType)
	if err := ssh.TryAccess(ctx, sshRunner); err != nil {
		log.Debugf(ctx, "Failed to ssh into router host when treating it as the device type %q: %v", deviceType, err)
	} else {
		isOpenWrt, err := hostIsOpenWrtRouter(ctx, sshRunner)
		if err != nil {
			return 0, errors.Annotate(err, "failed to check if host has the device type of %s", deviceType).Err()
		}
		if isOpenWrt {
			return deviceType, nil
		}
	}

	// Check if it's an Ubuntu device.
	deviceType = labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_UBUNTU
	log.Infof(ctx, "Checking if router host has the device type of %q", deviceType)
	sshRunner = newRouterSSHRunner(sshAccess, resource, deviceType)
	if err := ssh.TryAccess(ctx, sshRunner); err != nil {
		log.Debugf(ctx, "Failed to ssh into router host when treating it as the device type %q: %v", deviceType, err)
	} else {
		isUbuntu, err := hostIsUbuntuRouter(ctx, sshRunner)
		if err != nil {
			return 0, errors.Annotate(err, "failed to check if host has the device type of %s", deviceType).Err()
		}
		if isUbuntu {
			return deviceType, nil
		}
	}

	// Check if it's an AsusWrt device.
	deviceType = labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_ASUSWRT
	log.Infof(ctx, "Checking if router host has the device type of %q", deviceType)
	sshRunner = newRouterSSHRunner(sshAccess, resource, deviceType)
	if err := ssh.TryAccess(ctx, sshRunner); err != nil {
		log.Debugf(ctx, "Failed to ssh into router host when treating it as the device type %q: %v", deviceType, err)
	} else {
		isAsusWrt, err := hostIsAsusWrtRouter(ctx, sshRunner)
		if err != nil {
			return 0, errors.Annotate(err, "failed to check if host has the device type of %s", deviceType).Err()
		}
		if isAsusWrt {
			return deviceType, nil
		}
	}
	return labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_INVALID, nil
}

// RouterController is the common interface that all router controllers must
// implement so that router execs that are not dependent upon the device type
// may rely just on RouterController for usage and NewRouterDeviceController for
// implementation selection.
type RouterController interface {
	// WifiRouterHost returns the corresponding tlw.WifiRouterHost instance for
	// this router. Changes to this instance are persisted across execs.
	WifiRouterHost() *tlw.WifiRouterHost

	// DeviceType returns the labapi.WifiRouterDeviceType of the router.
	DeviceType() labapi.WifiRouterDeviceType

	// Runner returns a components.Runner for running ssh commands on the router.
	Runner() components.Runner

	// Model returns a unique name for the router model.
	Model() (string, error)

	// Features returns the router features that this router supports.
	Features() ([]labapi.WifiRouterFeature, error)

	// Reboot will reboot the router and wait for it to come back up. A non-nil
	// error indicates that the router was rebooted and is ssh-able again.
	Reboot(ctx context.Context) error
}

// NewRouterDeviceController creates a new router controller instance for the
// specified router host. The controller implementation used is dependent upon
// the wifiRouter.DeviceType, so this must be populated.
func NewRouterDeviceController(ctx context.Context, sshAccess ssh.Access, cacheAccess CacheAccess, hostResource, dutName string, wifiRouter *tlw.WifiRouterHost) (RouterController, error) {
	if sshAccess == nil {
		return nil, errors.Reason("sshAccess must not be nil").Err()
	}
	if wifiRouter == nil {
		return nil, errors.Reason("wifiRouter must not be nil").Err()
	}
	sshRunner := newRouterSSHRunner(sshAccess, hostResource, wifiRouter.DeviceType)
	routerControllerStateKey := "wifirouter_controller_state/" + hostResource
	switch wifiRouter.DeviceType {
	case labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_UNKNOWN:
		return nil, errors.Reason("cannot control unknown router; it must be analyzed first").Err()
	case labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_INVALID:
		return nil, errors.Reason("cannot control invalid router").Err()
	case labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_CHROMEOS_GALE:
		return newChromeOSGaleRouterController(sshRunner, wifiRouter), nil
	case labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_OPENWRT:
		var controllerState *tlw.OpenWrtRouterControllerState
		if state, ok := scopes.ReadConfigParam(ctx, routerControllerStateKey); !ok {
			controllerState = &tlw.OpenWrtRouterControllerState{}
			scopes.PutConfigParam(ctx, routerControllerStateKey, controllerState)
		} else {
			controllerState, ok = state.(*tlw.OpenWrtRouterControllerState)
			if !ok {
				return nil, errors.Reason("stored controller state does not match device type %q: %v", wifiRouter.DeviceType.String(), state).Err()
			}
		}
		return newOpenWrtRouterController(sshRunner, wifiRouter, controllerState, cacheAccess, dutName), nil
	case labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_ASUSWRT:
		var controllerState *tlw.AsusWrtRouterControllerState
		if state, ok := scopes.ReadConfigParam(ctx, routerControllerStateKey); !ok {
			controllerState = &tlw.AsusWrtRouterControllerState{}
			scopes.PutConfigParam(ctx, routerControllerStateKey, controllerState)
		} else {
			controllerState, ok = state.(*tlw.AsusWrtRouterControllerState)
			if !ok {
				return nil, errors.Reason("stored controller state does not match device type %q: %v", wifiRouter.DeviceType.String(), state).Err()
			}
		}
		return newAsusWrtRouterController(sshRunner, wifiRouter, controllerState), nil
	case labapi.WifiRouterDeviceType_WIFI_ROUTER_DEVICE_TYPE_UBUNTU:
		var controllerState *tlw.UbuntuRouterControllerState
		if state, ok := scopes.ReadConfigParam(ctx, routerControllerStateKey); !ok {
			controllerState = &tlw.UbuntuRouterControllerState{}
			scopes.PutConfigParam(ctx, routerControllerStateKey, controllerState)
		} else {
			controllerState, ok = state.(*tlw.UbuntuRouterControllerState)
			if !ok {
				return nil, errors.Reason("stored controller state does not match device type %q: %v", wifiRouter.DeviceType.String(), state).Err()
			}
		}
		return newUbuntuRouterController(sshRunner, wifiRouter, controllerState), nil
	}
	return nil, errors.Reason("unsupported DeviceType %q", wifiRouter.DeviceType).Err()
}
