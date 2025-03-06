// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/libs/git"
	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
)

// TODO(jordanmbeleg): refactor to reduce code duplication along with bot_config_reader.go

// ImportBotMaintenanceConfigs gets the MaintenanceConfig and git client and passes them to functions for importing maintenance configs
func ImportBotMaintenanceConfigs(ctx context.Context) error {
	maintenanceConfigs, gitClient, err := GetMaintenanceConfigAndGitClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize connection to Gitiles while importing maintenance configs")
	}

	err = ImportMaintenanceConfig(ctx, maintenanceConfigs, gitClient)
	return err
}

// getConfigAndGitTilesClient reads the MaintenanceConfig and creates a corresponding git client
func GetMaintenanceConfigAndGitClient(ctx context.Context) (*config.MaintenanceConfigs, git.ClientInterface, error) {
	es, err := external.GetServerInterface(ctx)
	if err != nil {
		return nil, nil, err
	}
	maintenanceConfigs := config.Get(ctx).GetMaintenanceConfigs()
	if maintenanceConfigs == nil {
		logging.Errorf(ctx, "No config found to read maintenance config")
		return nil, nil, fmt.Errorf("no config found to read maintenance config")
	}

	gitClient, err := es.NewGitInterface(ctx, maintenanceConfigs.GetGitilesHost(), maintenanceConfigs.GetProject(), maintenanceConfigs.GetBranch())
	if err != nil {
		logging.Errorf(ctx, "Got Error for git client : %s", err.Error())
		return nil, nil, fmt.Errorf("failed to initialize connection to Gitiles while importing enc bot configs")
	}
	return maintenanceConfigs, gitClient, nil
}

// ImportMaintenanceConfig imports Maintenance Config files and stores data for each bot in the DataStore.
func ImportMaintenanceConfig(ctx context.Context, maintenanceConfigs *config.MaintenanceConfigs, gitClient git.ClientInterface) error {
	logging.Infof(ctx, "Parsing Maintenance config for %d files", len(maintenanceConfigs.GetMaintenanceConfig()))
	for _, cfg := range maintenanceConfigs.GetMaintenanceConfig() {
		start := time.Now()
		logging.Debugf(ctx, "########### Parse %s ###########", cfg.GetName())
		conf, err := gitClient.GetFile(ctx, cfg.GetRemotePath())
		if err != nil {
			return err
		}
		content := &ufspb.MaintenanceConfigs{}
		err = prototext.Unmarshal([]byte(conf), content)
		if err != nil {
			return err
		}
		// TODO: Save/Update MaintenanceConfig by name in datastore
		ParseMaintenanceConfigs(ctx, content)
		duration := time.Since(start)
		logging.Debugf(ctx, "########### Done Parsing %s; Time taken %s ###########", cfg.GetName(), duration.String())
	}
	return nil
}

// ParseMaintenanceConfigs parses the Maintenance Config files and stores the maintenance
// data in the DataStore for every bot in the config.
func ParseMaintenanceConfigs(ctx context.Context, configs *ufspb.MaintenanceConfigs) {
	botsMap, _ := mapConfigsToBots(ctx, configs)

	// Updating the BotMaintenanceConfig for the botIds (ie. Hosts) collected so far.
	if err := updateMaintenanceConfigForBotIds(ctx, botsMap); err != nil {
		logging.Debugf(ctx, "Got errors while parsing bot id config %v", err)
	}

	// TODO[b/380310442] :
	// 1) Updating the maintenance configs for the botIdPrefixes
	// 2) Save/Update MaintenanceConfig by name in datastore (MaintenanceConfigKind)
	// 3) Delete stale configs
}

// mapConfigsToBots parses the maintenance configs into bot config maps
func mapConfigsToBots(ctx context.Context, maintenanceConfigs *ufspb.MaintenanceConfigs) (botsMap map[string]*ufspb.BotMaintenanceConfig, botPrefixesMap map[string]*ufspb.BotMaintenanceConfig) {
	botsMap = map[string]*ufspb.BotMaintenanceConfig{}
	botPrefixesMap = map[string]*ufspb.BotMaintenanceConfig{}
	for _, cfg := range maintenanceConfigs.GetMaintenanceConfig() {
		if len(cfg.BotId) == 0 && len(cfg.BotIdPrefix) == 0 {
			continue
		}

		hosts := []string{}
		for _, host := range cfg.BotId {
			if strings.Contains(host, "{") {
				// Parse the Host Range
				hosts = append(hosts, parseBotIds(host)...)
			} else {
				hosts = append(hosts, host)
			}
		}

		pb := &ufspb.BotMaintenanceConfig{
			SwarmingInstance:      cfg.SwarmingInstance,
			Owners:                cfg.Owners,
			Dimensions:            cfg.Dimensions,
			MaintenanceConfigName: cfg.Name,
		}
		for _, host := range hosts {
			botsMap[host] = pb
		}
		for _, prefix := range cfg.BotIdPrefix {
			botPrefixesMap[prefix] = pb
		}
	}
	return botsMap, botPrefixesMap
}

// Updates the BotMaintenanceConfig for the bot ids collected from the config.
func updateMaintenanceConfigForBotIds(ctx context.Context, botsMap map[string]*ufspb.BotMaintenanceConfig) error {
	var errs errors.MultiError
	for botId, cfg := range botsMap {
		updated, assetType, err := isBotMaintenanceUpdated(ctx, botId, cfg, false)
		if err != nil && status.Code(err) != codes.NotFound {
			logging.Debugf(ctx, "Failed to check if maintenance config is updated %s - %v", botId, err)
			errs = append(errs, err)
		}
		if updated {
			logging.Infof(ctx, "Updating maintenance config for bot id %s", botId)
			err = updateBotMaintenanceConfig(ctx, botId, cfg, assetType)
			if err != nil {
				logging.Debugf(ctx, "Failed to maintenance config for bot id %s - %v", botId, err)
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Checks if the bot maintenance config is updated from the last time we read the configs.
func isBotMaintenanceUpdated(ctx context.Context, botId string, newConfig *ufspb.BotMaintenanceConfig, isPrefix bool) (bool, string, error) {
	entity, err := inventory.GetBotMaintenanceConfig(ctx, botId)
	if err != nil {
		return true, "", err
	}

	assetType := strings.TrimSpace(entity.AssetType)
	if assetType == "" && !isPrefix {
		logging.Infof(ctx, "Found a botId %s with maintenance config but no asset type", botId)
		return true, assetType, nil
	}
	// If we have assetType, check if the asset table has the maintenance_config_name .
	emptyAssetData, err := isMaintenanceConfigEmptyInBotTable(ctx, botId, assetType)
	if emptyAssetData || err != nil {
		if emptyAssetData {
			logging.Infof(ctx, "Found a botId %s with empty maintenance config data and assetType %s", botId, assetType)
		}
		return true, assetType, err
	}

	p, err := entity.GetProto()
	if err != nil {
		return true, assetType, err
	}
	pm := p.(*ufspb.BotMaintenanceConfig)
	if isStringFieldUpdated(pm.GetMaintenanceConfigName(), newConfig.GetMaintenanceConfigName()) ||
		isStringFieldUpdated(pm.GetSwarmingInstance(), newConfig.GetSwarmingInstance()) ||
		isArrayFieldUpdated(pm.GetOwners(), newConfig.GetOwners()) ||
		isArrayFieldUpdated(pm.GetDimensions(), newConfig.GetDimensions()) {
		diff := cmp.Diff(pm, newConfig, protocmp.Transform())
		logging.Debugf(ctx, "Found maintenance config diff for bot  %s - %s", botId, diff)
		return true, assetType, nil
	}
	return false, "", nil
}

// Checks if the asset table entry has maintenance config name
func isMaintenanceConfigEmptyInBotTable(ctx context.Context, botId string, assetType string) (bool, error) {
	switch assetType {
	case inventory.AssetTypeMachineLSE:
		host, err := inventory.GetMachineLSE(ctx, botId)
		if err != nil {
			return false, err
		}
		return host.GetMaintenanceConfigName() == "", nil
	// we should not hit this code path as assettype is expected to be set, but setting a default that maintenance config is non-empty.
	default:
		return false, nil
	}
}

// Updates the maintenance config for the given assetType and name
func updateBotMaintenanceConfig(ctx context.Context, botId string, botMaintenanceConfig *ufspb.BotMaintenanceConfig, assetType string) (err error) {
	return datastore.RunInTransaction(ctx, func(c context.Context) error {
		// First Update the maintenance_config_name for the Asset
		switch assetType {
		case inventory.AssetTypeMachineLSE:
			_, err = inventory.UpdateMachineLSEMaintenanceConfig(ctx, botId, botMaintenanceConfig.GetMaintenanceConfigName())
		default:
			assetType, err = findAndUpdateMaintenanceConfigForAsset(ctx, botId, botMaintenanceConfig)
		}
		if err != nil {
			return err
		}

		// Then update the BotMaintenanceConfig table
		_, err = inventory.PutBotMaintenanceConfig(ctx, botMaintenanceConfig, botId, assetType)
		return err
	}, &datastore.TransactionOptions{})
}

// Updates the maintenance config name for the given id, searches through machineLSE first (and later on Machine and VM entities) as asset type is unknown
func findAndUpdateMaintenanceConfigForAsset(ctx context.Context, botId string, maintenanceConfig *ufspb.BotMaintenanceConfig) (string, error) {
	errs := make(errors.MultiError, 0)
	_, err := inventory.UpdateMachineLSEMaintenanceConfig(ctx, botId, maintenanceConfig.GetMaintenanceConfigName())
	if err == nil {
		return inventory.AssetTypeMachineLSE, nil
	}
	if status.Code(err) != codes.NotFound {
		errs = append(errs, err)
	}
	if errs.First() != nil {
		return "", errs
	}
	return "", nil
}

// GetBotMaintenanceConfig gets the maintenance config data in the Data store for the requested bot in the config.
func GetBotMaintenanceConfig(ctx context.Context, hostName string) (*ufspb.BotMaintenanceConfig, error) {
	host, err := inventory.GetBotMaintenanceConfig(ctx, hostName)
	if err != nil {
		return nil, err
	}
	proto, err := host.GetProto()
	if err != nil {
		return nil, err
	}
	return proto.(*ufspb.BotMaintenanceConfig), err
}
