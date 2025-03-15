// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/realms"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
)

// OldBrowserLabAdminRealm is the Old realm for browser lab
//
// If a client sends this realm, replace it with BrowserLabAdminRealm
const OldBrowserLabAdminRealm = "chromium:ufs/browser-admin"

// BrowserLabAdminRealm is the admin realm for browser lab.
const BrowserLabAdminRealm = "@internal:ufs/browser"

// AtlLabAdminRealm is the admin realm for atl lab.
const AtlLabAdminRealm = "@internal:ufs/os-atl"

// AtlLabChromiumAdminRealm is the admin realm for chromium DUTs in atl lab.
const AtlLabChromiumAdminRealm = "@internal:ufs/os-atl-chromium"

// AcsLabAdminRealm is the admin realm for acs lab.
const AcsLabAdminRealm = "@internal:ufs/os-acs"

// SatLabInternalUserRealm is realm for satlab internal users.
const SatLabInternalUserRealm = "@internal:ufs/satlab-internal-users"

// SkipRealmsCheck flag to skip realms check
var SkipRealmsCheck = false

// Browser-related consts
const (
	ChromiumPool              = "chromium"
	ChromePool                = "chrome"
	ChromiumNamePrefix        = "chromium-"
	ChromeNamePrefix          = "chrome-"
	ChromeClankPrefix         = "chrome-clank-"
	ChromePerfNamePrefix      = "chrome-perf-"
	ChromePerfWaterfallPrefix = "chrome-perf-waterfall-"
	ChromePerfPinpointPrefix  = "chrome-perf-pinpoint-"
)

var (
	ChromePerfPools = []string{"chrome.tests.pinpoint", "chrome.tests.perf"}
)

// UFS registered permissions in process registry
var (
	// ConfigurationsGet allows to get configuration resources.
	ConfigurationsGet = realms.RegisterPermission("ufs.configurations.get")
	// ConfigurationsList allows to list configuration resources.
	ConfigurationsList = realms.RegisterPermission("ufs.configurations.list")
	// ConfigurationsCreate allows to create configuration resources.
	ConfigurationsCreate = realms.RegisterPermission("ufs.configurations.create")
	// ConfigurationUpdate allows to update configuration resources.
	ConfigurationsUpdate = realms.RegisterPermission("ufs.configurations.update")
	// ConfigurationsDelete allows to delete configuration resources.
	ConfigurationsDelete = realms.RegisterPermission("ufs.configurations.delete")

	// RegistrationsGet allows to get registration resources.
	RegistrationsGet = realms.RegisterPermission("ufs.registrations.get")
	// RegistrationsList allows to list registration resources.
	RegistrationsList = realms.RegisterPermission("ufs.registrations.list")
	// RegistrationsCreate allows to create registration resources.
	RegistrationsCreate = realms.RegisterPermission("ufs.registrations.create")
	// RegistrationsUpdate allows to update registration resources.
	RegistrationsUpdate = realms.RegisterPermission("ufs.registrations.update")
	// RegistrationsDelete allows to delete registration resources.
	RegistrationsDelete = realms.RegisterPermission("ufs.registrations.delete")

	// InventoriesGet allows to get inventory resources.
	InventoriesGet = realms.RegisterPermission("ufs.inventories.get")
	// InventoriesList allows to list inventory resources.
	InventoriesList = realms.RegisterPermission("ufs.inventories.list")
	// InventoriesCreate allows to create inventory resources.
	InventoriesCreate = realms.RegisterPermission("ufs.inventories.create")
	// InventoriesUpdate allows to update inventory resources.
	InventoriesUpdate = realms.RegisterPermission("ufs.inventories.update")
	// InventoriesDelete allows to delete inventory resources.
	InventoriesDelete = realms.RegisterPermission("ufs.inventories.delete")

	// NetworksGet allows to get network resources.
	NetworksGet = realms.RegisterPermission("ufs.networks.get")
	// NetworksList allows to list network resources.
	NetworksList = realms.RegisterPermission("ufs.networks.list")
	// NetworksCreate allows to create network resources.
	NetworksCreate = realms.RegisterPermission("ufs.networks.create")
	// NetworksUpdate allows to update network resources.
	NetworksUpdate = realms.RegisterPermission("ufs.networks.update")
	// NetworksDelete allows to delete network resources.
	NetworksDelete = realms.RegisterPermission("ufs.networks.delete")

	// ResourcesImport allows to import resource resources.
	ResourcesImport = realms.RegisterPermission("ufs.resources.import")
)

func init() {
	RegistrationsList.AddFlags(realms.UsedInQueryRealms)
	InventoriesList.AddFlags(realms.UsedInQueryRealms)
	ConfigurationsList.AddFlags(realms.UsedInQueryRealms)
}

// ChromeOSLUCIProject where ChromeOS LUCI infra lives
const ChromeOSLUCIProject = "chromeos"

// CurrentUser returns the current user
func CurrentUser(ctx context.Context) string {
	return auth.CurrentUser(ctx).Email
}

// hasPermission checks if the user has permission in the realm
func hasPermission(ctx context.Context, perm realms.Permission, realm string) (bool, error) {
	has, err := auth.HasPermission(ctx, perm, realm, nil)
	if err != nil {
		logging.Errorf(ctx, "failed to check realm %q ACLs", err.Error())
		return false, status.Errorf(codes.PermissionDenied, "failed to check realm %q ACLs", err)
	}
	return has, nil
}

// CheckPermission checks if the user has permission in the realm
//
// return error if user doesnt have permission or unable to check permission in realm
// else returns nil
func CheckPermission(ctx context.Context, perm realms.Permission, realm string) error {
	if SkipRealmsCheck {
		logging.Infof(ctx, "Skipping Realms check")
		return nil
	}
	if realm == "" {
		logging.Infof(ctx, "No permission check for empty realm. Entity permission %s allowed for the user %s", perm, auth.CurrentIdentity(ctx))
		return nil
	}
	allow, err := hasPermission(ctx, perm, realm)
	if err != nil {
		return err
	}
	if !allow {
		logging.Errorf(ctx, "%s does not have permission %s in the realm %s", auth.CurrentIdentity(ctx), perm, realm)
		return status.Errorf(codes.PermissionDenied, "%s does not have permission %s in the realm %s", auth.CurrentIdentity(ctx), perm, realm)
	}
	logging.Infof(ctx, "%s has permission %s in the realm %s", auth.CurrentIdentity(ctx), perm, realm)
	return nil
}

// ToUFSRealm returns the realm name based on zone string.
func ToUFSRealm(zone string) string {
	ufsZone := ToUFSZone(zone)
	if ufsZone == ufspb.Zone_ZONE_UNSPECIFIED {
		return ""
	} else if IsInBrowserZone(ufsZone.String()) {
		return BrowserLabAdminRealm
	} else if ufsZone == ufspb.Zone_ZONE_CHROMEOS3 || ufsZone == ufspb.Zone_ZONE_CHROMEOS5 ||
		ufsZone == ufspb.Zone_ZONE_CHROMEOS7 || ufsZone == ufspb.Zone_ZONE_CHROMEOS15 {
		return AcsLabAdminRealm
	} else if ufsZone == ufspb.Zone_ZONE_SATLAB {
		return SatLabInternalUserRealm
	} else if ufsZone == ufspb.Zone_ZONE_SFO36_OS_CHROMIUM {
		return AtlLabChromiumAdminRealm
	} else if IsSFPZone(ufsZone.String()) {
		return GetSatlabForPartnersRealm(ufsZone.String())
	}
	return AtlLabAdminRealm
}

// GetSatlabForPartnersRealm determines the realm for zones that are formatted
// in a manner consistent with SfP naming conventions
func GetSatlabForPartnersRealm(zone string) string {
	project := ChromeOSLUCIProject
	realm_id := strings.ToLower(strings.TrimPrefix(zone, fmt.Sprintf("%s_", ZonePrefix)))

	return fmt.Sprintf("%s:ufs/%s", project, realm_id)
}

// GetValidRealmName replaces the older Browser realm with newer realm
func GetValidRealmName(realm string) string {
	if realm == OldBrowserLabAdminRealm {
		return BrowserLabAdminRealm
	}
	return realm
}

// IsBrowserLegacyAsset returns if an asset is a legacy asset migrated from browser lab which doesn't have an assetTag
func IsBrowserLegacyAsset(name string) bool {
	if name == fmt.Sprintf("%sasset", ChromeNamePrefix) || name == fmt.Sprintf("%sasset", ChromiumNamePrefix) {
		return true
	}
	return false
}

// IsChromeLegacyHost returns if a host is a legacy browser host used internally
func IsChromeLegacyHost(name string) bool {
	return strings.HasPrefix(name, ChromeNamePrefix)
}

// IsChromiumLegacyHost returns if a host is a legacy browser host used externally
func IsChromiumLegacyHost(name string) bool {
	return strings.HasPrefix(name, ChromiumNamePrefix)
}

// IsChromePerfHost returns if a host is a legacy browser perf host used internally
func IsChromePerfHost(name string) bool {
	return strings.HasPrefix(name, ChromePerfNamePrefix)
}

// IsClankHost returns if a host is a DUT used for clank
func IsClankHost(name string) bool {
	return strings.HasPrefix(name, ChromeClankPrefix)
}

// IsInChromiumPool checks if any chromium pool exist in the given pool labels.
func IsInChromiumPool(pools []string) bool {
	if len(pools) != 1 {
		// Hosts in chromium pool cannot have multiple pools in case they're wrongly scheduled.
		return false
	}
	if pools[0] == ChromiumPool {
		return true
	}
	return false
}

// IsInChromePool checks if any chrome pool exist in the given pool labels.
func IsInChromePool(pools []string) bool {
	for _, p := range pools {
		if p == ChromePool {
			return true
		}
	}
	return false
}

// IsInChromePerfPool checks if any chrome perf pool exist in the given pool labels
func IsInChromePerfPool(pools []string) bool {
	if len(pools) != 1 {
		// Hosts in perf pool cannot have multiple pools in case they're wrongly scheduled.
		return false
	}
	for _, perfPool := range ChromePerfPools {
		if pools[0] == perfPool {
			return true
		}
	}
	return false
}
