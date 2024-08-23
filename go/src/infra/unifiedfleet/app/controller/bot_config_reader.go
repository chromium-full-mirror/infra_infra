// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/gae/service/datastore"

	"infra/libs/git"
	ufspb "infra/unifiedfleet/api/v1/models"
	api "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/model/inventory"
	"infra/unifiedfleet/app/model/registration"
)

const (
	POOL_PREFIX = "pool:"
)

type ConfigSha struct {
	mu   sync.Mutex
	sha1 string
}

var prevConfig = ConfigSha{sha1: ""}

var OwnershipConfigsCounter = metric.NewInt(
	"ufs/fleet/ownership",
	"ownership data updated at sync",
	nil,
	field.String("host"),
	field.String("security_level"),
	field.String("swarming_instance"),
	field.String("customer"),
	field.Bool("prefix"),
)

// ImportBotConfigs gets the OwnershipConfig and git client and passes them to functions for importing bot configs
func ImportBotConfigs(ctx context.Context) error {
	ownershipConfig, gitClient, err := GetConfigAndGitClient(ctx)
	if err != nil {
		return err
	}
	if ownershipConfig == nil {
		return nil
	}

	err = ImportSecurityConfig(ctx, ownershipConfig, gitClient)
	return err
}

// processSecurityConfig processes a single security config.
func processSecurityConfig(ctx context.Context, commitsh string, cfg *config.OwnershipConfig_ConfigFile, gitTilesClient external.GitTilesClient, ownershipConfig *config.OwnershipConfig) ([]*api.OwnershipByHost, error) {
	start := time.Now()
	logging.Debugf(ctx, "########### Parse %s at commit %s ###########", cfg.GetName(), commitsh)

	conf, err := fetchConfigProtoFromGitiles(ctx, gitTilesClient, ownershipConfig.GetProject(), commitsh, cfg.GetRemotePath())
	if err != nil {
		return nil, err
	}
	content := &ufspb.SecurityInfos{}
	err = prototext.Unmarshal([]byte(conf), content)
	if err != nil {
		return nil, err
	}
	botsMap, botPrefixesMap := parseConfigs(ctx, content)
	var entities []*api.OwnershipByHost
	for k, v := range botsMap {
		ownership := &api.OwnershipByHost{
			Hostname:  k,
			Ownership: v,
		}
		entities = append(entities, ownership)
		// botConfigs = append(botConfigs, v)
	}
	for k, v := range botPrefixesMap {
		ownership := &api.OwnershipByHost{
			Hostname:  k,
			Ownership: v,
		}
		entities = append(entities, ownership)
	}
	duration := time.Since(start)
	logging.Debugf(ctx, "########### Done Parsing %s at commit %s; Time taken %s ###########", cfg.GetName(), commitsh, duration.String())
	return entities, nil
}

// GetBotConfigsForCommitSh gets the OwnershipConfigs at a particular commitsh
func GetBotConfigsForCommitSh(ctx context.Context, commitsh string) ([]*api.OwnershipByHost, error) {
	ownershipConfig, gitTilesClient, err := getConfigAndGitTilesClient(ctx)
	if err != nil {
		logging.Errorf(ctx, "Got error for gititles client: %s", err.Error())
		return nil, err
	}

	var out []*api.OwnershipByHost
	var errs []error

	// TODO(gregorynisbet): Is this correct? Do we want to stop reading after getting an answer from a single data source?
	for _, cfg := range ownershipConfig.GetSecurityConfig() {
		entities, err := processSecurityConfig(ctx, commitsh, cfg, gitTilesClient, ownershipConfig)
		errs = append(errs, err)
		out = append(out, entities...)
	}

	return out, errors.Append(errs...)
}

// GetConfigAndGitClient reads the OwnershipConfig and creates a corresponding git client
func GetConfigAndGitClient(ctx context.Context) (*config.OwnershipConfig, git.ClientInterface, error) {
	es, err := external.GetServerInterface(ctx)
	if err != nil {
		return nil, nil, err
	}
	ownershipConfig, gitTilesClient, err := getConfigAndGitTilesClient(ctx)
	if err != nil {
		logging.Errorf(ctx, "Got error for gititles client: %s", err.Error())
		return nil, nil, err
	}
	currentSha1, err := fetchLatestSHA1(ctx, gitTilesClient, ownershipConfig.GetProject(), ownershipConfig.GetBranch())
	if err != nil {
		logging.Errorf(ctx, "Got error when getting the current SHA: %s", err.Error())
		return nil, nil, err
	}
	isSameSha1 := prevConfig.compareAndSetSha(currentSha1)
	if isSameSha1 {
		logging.Infof(ctx, "Nothing changed for enc/security config files - lastest SHA1 : %s", currentSha1)
		return nil, nil, nil
	}

	gitClient, err := es.NewGitInterface(ctx, ownershipConfig.GetGitilesHost(), ownershipConfig.GetProject(), ownershipConfig.GetBranch())
	if err != nil {
		logging.Errorf(ctx, "Got Error for git client : %s", err.Error())
		return nil, nil, fmt.Errorf("failed to initialize connection to Gitiles while importing enc bot configs")
	}
	logging.Infof(ctx, "Importing new enc/security config files - lastest SHA1 : %s", currentSha1)
	return ownershipConfig, gitClient, nil
}

// ImportSecurityConfig imports Security Config files and stores security data for each bot in the DataStore.
func ImportSecurityConfig(ctx context.Context, ownershipConfig *config.OwnershipConfig, gitClient git.ClientInterface) error {
	logging.Infof(ctx, "Parsing Security config for %d files", len(ownershipConfig.GetSecurityConfig()))
	for _, cfg := range ownershipConfig.GetSecurityConfig() {
		start := time.Now()
		logging.Debugf(ctx, "########### Parse %s ###########", cfg.GetName())
		conf, err := gitClient.GetFile(ctx, cfg.GetRemotePath())
		if err != nil {
			return err
		}
		content := &ufspb.SecurityInfos{}
		err = prototext.Unmarshal([]byte(conf), content)
		if err != nil {
			return err
		}
		ParseSecurityConfig(ctx, content)
		duration := time.Since(start)
		logging.Debugf(ctx, "########### Done Parsing %s; Time taken %s ###########", cfg.GetName(), duration.String())
	}
	return nil
}

// ParseSecurityConfig parses the Security Config files and stores the security
// data in the DataStore for every bot in the config.
func ParseSecurityConfig(ctx context.Context, config *ufspb.SecurityInfos) {
	botsMap, botPrefixesMap := parseConfigs(ctx, config)
	defer func() {
		// Update the counters for metrics
		for botID, od := range botsMap {
			OwnershipConfigsCounter.Set(ctx, 1, botID, od.GetSecurityLevel(), od.GetSwarmingInstance(), od.GetCustomer(), false)
		}
		for botPrefix, od := range botPrefixesMap {
			OwnershipConfigsCounter.Set(ctx, 1, botPrefix, od.GetSecurityLevel(), od.GetSwarmingInstance(), od.GetCustomer(), true)
		}
	}()
	// Updating the ownership for the botIdPrefixes (ie. HostPrefixes).
	if err := updateBotConfigForBotIdPrefix(ctx, botPrefixesMap); err != nil {
		logging.Debugf(ctx, "Got errors while parsing bot id prefixes config %v", err)
	}

	// Updating the ownership data for the botIds (ie. Hosts) collected so far.
	if err := updateBotConfigForBotIds(ctx, botsMap); err != nil {
		logging.Debugf(ctx, "Got errors while parsing bot id config %v", err)
	}

	// Delete stale configs
	deleteStaleConfigs(ctx, botPrefixesMap, botsMap)
}

// parseConfigs parses the security configs into bot config maps
func parseConfigs(ctx context.Context, config *ufspb.SecurityInfos) (map[string]*ufspb.OwnershipData, map[string]*ufspb.OwnershipData) {
	var botsMap = map[string]*ufspb.OwnershipData{}
	var botPrefixesMap = map[string]*ufspb.OwnershipData{}
	for _, pool := range config.Pools {
		if len(pool.Hosts) == 0 && len(pool.HostPrefixes) == 0 {
			continue
		}

		hosts := []string{}
		for _, host := range pool.Hosts {
			if strings.Contains(host, "{") {
				// Parse the Host Range
				hosts = append(hosts, parseBotIds(host)...)
			} else {
				hosts = append(hosts, host)
			}
		}

		// Collecting all the pools for each bot.
		for _, host := range hosts {
			val, ok := botsMap[host]
			if ok {
				if !existsInList(val.GetPools(), pool.GetPoolName()) {
					val.Pools = append(val.Pools, pool.GetPoolName())
					botsMap[host] = val
				}
			} else {
				botsMap[host] = &ufspb.OwnershipData{
					SecurityLevel:    pool.SecurityLevel,
					Pools:            []string{pool.PoolName},
					SwarmingInstance: pool.SwarmingServerId,
					Customer:         pool.Customer,
					Builders:         pool.Builders,
				}
			}
		}

		// Collecting all the pools for each bot prefix.
		for _, prefix := range pool.HostPrefixes {
			val, ok := botPrefixesMap[prefix]
			if ok {
				if !existsInList(val.GetPools(), pool.GetPoolName()) {
					val.Pools = append(val.Pools, pool.GetPoolName())
					botPrefixesMap[prefix] = val
				}
			} else {
				botPrefixesMap[prefix] = &ufspb.OwnershipData{
					SecurityLevel:    pool.SecurityLevel,
					Pools:            []string{pool.PoolName},
					SwarmingInstance: pool.SwarmingServerId,
					Customer:         pool.Customer,
					Builders:         pool.Builders,
				}
			}
		}
	}
	return botsMap, botPrefixesMap
}

// Cleanup ownership data that is no longer present in the starlark configs
func deleteStaleConfigs(ctx context.Context, botPrefixesMap map[string]*ufspb.OwnershipData, botsMap map[string]*ufspb.OwnershipData) {
	var pageToken string
	staleEntries := make([]string, 0)
	for {
		entries, token, err := listOwnershipEntities(ctx, 1000, pageToken, "", true, nil)
		if err != nil {
			logging.Warningf(ctx, "List ownership configs failed during cleanup : %s", err)
		}
		for _, entry := range entries {
			// check if this key exists in the bots map or the botIdPrefixes map
			// otherwise mark it as stale
			hostName := entry.Name
			_, ok := botsMap[hostName]
			if !ok {
				_, ok = botPrefixesMap[hostName]
				if !ok && !mapContainsPrefix(botPrefixesMap, hostName) {
					updateOwnership(ctx, hostName, nil, entry.AssetType)
					staleEntries = append(staleEntries, hostName)
					logging.Warningf(ctx, "found stale entry during cleanup : %s", hostName)
				}
			}
		}
		if token == "" {
			break
		}
		pageToken = token
	}

	inventory.BatchDeleteOwnerships(ctx, staleEntries)
}

// ListOwnershipConfigs lists the ownerships based on the specified parameters.
func ListOwnershipConfigs(ctx context.Context, pageSize int32, pageToken, filter string, keysOnly bool) ([]*api.OwnershipByHost, string, error) {
	var filterMap map[string][]interface{}
	var err error
	if filter != "" {
		filterMap, err = getFilterMap(filter, inventory.GetOwnershipIndexedFieldName)
		if err != nil {
			return nil, "", errors.Annotate(err, "failed to read filter for listing Ownerships").Err()
		}
	}
	commitsh := ""
	if vals, ok := filterMap[inventory.CommitSh]; ok {
		if len(vals) != 1 {
			return nil, "", errors.Annotate(err, "Can only specify one commitsh for returning configs").Err()
		}
		commitsh = strings.TrimSpace(vals[0].(string))
		// Delete the commitsh from filterMap as we don't need it anymore
		// and that field doesn't exist in the database
		delete(filterMap, inventory.CommitSh)
	}
	if commitsh == "" {
		logging.Infof(ctx, "Getting ownership for latest commit")
		res, pageToken, err := listOwnershipEntities(ctx, pageSize, pageToken, filter, keysOnly, filterMap)
		if err != nil {
			return nil, "", err
		}
		var entities []*api.OwnershipByHost

		for _, entity := range res {
			p, err := entity.GetProto()
			if err != nil {
				logging.Errorf(ctx, "Error parsing entity for ListOwnershipConfigs : %s", err)
			} else {
				pm := p.(*ufspb.OwnershipData)
				ownership := &api.OwnershipByHost{
					Hostname:  entity.Name,
					Ownership: pm,
				}
				entities = append(entities, ownership)
			}
		}
		return entities, pageToken, nil
	}
	logging.Infof(ctx, "Getting ownership for specified commit %s", commitsh)
	entities, err := GetBotConfigsForCommitSh(ctx, commitsh)
	if err == nil {
		return entities, "", nil
	}
	return nil, "", err
}

// listOwnershipEntities lists the ownership datastore entities,
// based on the specified parameters.
func listOwnershipEntities(ctx context.Context, pageSize int32, pageToken,
	filter string, keysOnly bool, filterMap map[string][]interface{}) ([]inventory.OwnershipDataEntity, string, error) {

	res, pageToken, err := inventory.ListOwnerships(ctx, pageSize, pageToken, filterMap, keysOnly)
	if err != nil {
		return nil, "", err
	}
	return res, pageToken, nil
}

// getConfigAndGitTilesClient reads the OwnershipConfig and creates a corresponding git client
func getConfigAndGitTilesClient(ctx context.Context) (*config.OwnershipConfig, external.GitTilesClient, error) {
	es, err := external.GetServerInterface(ctx)
	if err != nil {
		return nil, nil, err
	}
	ownershipConfig := config.Get(ctx).GetOwnershipConfig()
	if ownershipConfig == nil {
		logging.Errorf(ctx, "No config found to read ownership data")
		return nil, nil, fmt.Errorf("no config found to read ownership data")
	}
	gitTilesClient, err := es.NewGitTilesInterface(ctx, ownershipConfig.GetGitilesHost())
	if err != nil {
		logging.Errorf(ctx, "Got error for gititles client: %s", err.Error())
		return nil, nil, err
	}
	return ownershipConfig, gitTilesClient, nil
}

// Updates the Ownership config for the bot ids collected from the config.
func updateBotConfigForBotIds(ctx context.Context, botsMap map[string]*ufspb.OwnershipData) error {
	var errs errors.MultiError
	for botId, ownershipData := range botsMap {
		updated, assetType, err := isBotOwnershipUpdated(ctx, botId, ownershipData, false)
		if err != nil && status.Code(err) != codes.NotFound {
			logging.Debugf(ctx, "Failed to check if ownership is updated %s - %v", botId, err)
			errs = append(errs, err)
		}
		if updated {
			logging.Infof(ctx, "Updating ownership for for bot id %s", botId)
			err = updateOwnership(ctx, botId, ownershipData, assetType)
			if err != nil {
				logging.Debugf(ctx, "Failed to update ownership for bot id %s - %v", botId, err)
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Checks if the bot ownership is updated from the last time we read the configs.
func isBotOwnershipUpdated(ctx context.Context, botId string, newOwnership *ufspb.OwnershipData, isPrefix bool) (bool, string, error) {
	entity, err := inventory.GetOwnershipData(ctx, botId)
	// Update ownership for bot if it does not exist in the ownership table or if there is error in retrieving the entity
	if err != nil {
		return true, "", err
	}
	// If we don't know the asset type for a bot, try to update the data unless it is a bot prefix
	// This is to handle the case where a bot entry gets added to the starlark files before UFS
	assetType := strings.TrimSpace(entity.AssetType)
	if assetType == "" && !isPrefix {
		logging.Infof(ctx, "Found a botId %s with ownership data but no asset type", botId)
		return true, assetType, nil
	}

	// If we have assetType, check if the asset table has the ownershipdata.
	// It is possible that due to a cornercase scenario ownershipdata can be redacted when updating an asset.
	emptyAssetData, err := isOwnershipEmptyInBotTable(ctx, botId, assetType)
	if emptyAssetData || err != nil {
		if emptyAssetData {
			logging.Infof(ctx, "Found a botId %s with empty ownership data and assetType %s", botId, assetType)
		}
		return true, assetType, err
	}

	p, err := entity.GetProto()
	if err != nil {
		return true, assetType, err
	}
	pm := p.(*ufspb.OwnershipData)
	if isOwnershipFieldUpdated(pm.GetCustomer(), newOwnership.GetCustomer()) ||
		isOwnershipFieldUpdated(pm.GetSecurityLevel(), newOwnership.GetSecurityLevel()) ||
		isOwnershipFieldUpdated(pm.GetPoolName(), newOwnership.GetPoolName()) ||
		isOwnershipFieldUpdated(pm.GetSwarmingInstance(), newOwnership.GetSwarmingInstance()) ||
		isOwnershipArrayFieldUpdated(pm.GetPools(), newOwnership.GetPools()) ||
		isOwnershipArrayFieldUpdated(pm.GetBuilders(), newOwnership.GetBuilders()) {
		diff := cmp.Diff(pm, newOwnership, protocmp.Transform())
		logging.Debugf(ctx, "Found ownership diff for bot  %s - %s", botId, diff)
		return true, assetType, nil
	}
	return false, "", nil
}

// Checks if the Ownership.Builder field was updated, ignoring an empty slice
func isOwnershipArrayFieldUpdated(oldVal []string, newVal []string) bool {
	if len(newVal) != 0 && len(oldVal) != len(newVal) {
		return true
	}
	for i := range newVal {
		if isOwnershipFieldUpdated(oldVal[i], newVal[i]) {
			return true
		}
	}
	return false
}

// Checks if the ownership field was updated, ignoring empty values
func isOwnershipFieldUpdated(oldVal string, newVal string) bool {
	if (oldVal == "" && newVal != "") || (oldVal != "" && newVal != "" && oldVal != newVal) {
		return true
	}
	return false
}

// Checks if the asset table entry has ownership data
func isOwnershipEmptyInBotTable(ctx context.Context, botId string, assetType string) (bool, error) {
	switch assetType {
	case inventory.AssetTypeMachine:
		host, err := registration.GetMachine(ctx, botId)
		if err != nil {
			return false, err
		}
		return host.GetOwnership() == nil, nil
	case inventory.AssetTypeMachineLSE:
		host, err := inventory.GetMachineLSE(ctx, botId)
		if err != nil {
			return false, err
		}
		return host.GetOwnership() == nil, nil
	case inventory.AssetTypeVM:
		host, err := inventory.GetVM(ctx, botId)
		if err != nil {
			return false, err
		}
		return host.GetOwnership() == nil, nil
	// we should not hit this code path as assettype is expected to be set, but setting a default that ownership is non-empty.
	default:
		return false, nil
	}
}

// Updates the ownership for the given assetType and name
func updateOwnership(ctx context.Context, botId string, ownership *ufspb.OwnershipData, assetType string) (err error) {
	return datastore.RunInTransaction(ctx, func(c context.Context) error {
		// First Update the Ownership for the Asset
		switch assetType {
		case inventory.AssetTypeMachine:
			_, err = registration.UpdateMachineOwnership(ctx, botId, ownership)
		case inventory.AssetTypeMachineLSE:
			_, err = inventory.UpdateMachineLSEOwnership(ctx, botId, ownership)
		case inventory.AssetTypeVM:
			_, err = inventory.UpdateVMOwnership(ctx, botId, ownership)
		default:
			assetType, err = findAndUpdateOwnershipForAsset(ctx, botId, ownership)
		}
		if err != nil {
			return err
		}

		// Then update the ownership data table
		_, err = inventory.PutOwnershipData(ctx, ownership, botId, assetType)
		return err
	}, &datastore.TransactionOptions{})
}

// Updates the ownership for the given id, searches through machine, machineLSE	 and VM entities as asset type is unknown
func findAndUpdateOwnershipForAsset(ctx context.Context, botId string, ownershipData *ufspb.OwnershipData) (string, error) {
	errs := make(errors.MultiError, 0)
	_, err := registration.UpdateMachineOwnership(ctx, botId, ownershipData)
	if err == nil {
		return inventory.AssetTypeMachine, nil
	}
	if status.Code(err) != codes.NotFound {
		errs = append(errs, err)
	}
	_, err = inventory.UpdateVMOwnership(ctx, botId, ownershipData)
	if err == nil {
		return inventory.AssetTypeVM, nil
	}
	if status.Code(err) != codes.NotFound {
		errs = append(errs, err)
	}
	_, err = inventory.UpdateMachineLSEOwnership(ctx, botId, ownershipData)
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

// Update the Ownership config for the bot id prefixes collected from the config.
func updateBotConfigForBotIdPrefix(ctx context.Context, botIdPrefixesMap map[string]*ufspb.OwnershipData) error {
	var errs errors.MultiError
	for prefix, ownershipData := range botIdPrefixesMap {
		updated, assetType, err := isBotOwnershipUpdated(ctx, prefix, ownershipData, true)
		if err != nil && status.Code(err) != codes.NotFound {
			logging.Debugf(ctx, "Failed to check if ownership is updated for prefix %s - %v", prefix, err)
			errs = append(errs, err)
		}
		if updated {
			logging.Infof(ctx, "Updating ownership for bot id prefix %s", prefix)
			err = updateOwnershipForAssetByPrefix(ctx, prefix, ownershipData, assetType)
			if err != nil {
				logging.Debugf(ctx, "Failed to update ownership for bot id prefix %s - %v", prefix, err)
				errs = append(errs, err)
			} else {
				logging.Debugf(ctx, "updated ownership for bot id prefix %s", prefix)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Updates Ownership for the given asset type and name prefix
func updateOwnershipForAssetByPrefix(ctx context.Context, prefix string, ownership *ufspb.OwnershipData, assetType string) (err error) {
	return datastore.RunInTransaction(ctx, func(c context.Context) error {
		// First Update the Ownership for the Asset
		switch assetType {
		case inventory.AssetTypeMachine:
			_, err = findAndUpdateMachineOwnershipForPrefix(ctx, prefix, ownership)
		case inventory.AssetTypeMachineLSE:
			_, err = findAndUpdateMachineLSEOwnershipForPrefix(ctx, prefix, ownership)
		case inventory.AssetTypeVM:
			_, err = findAndUpdateVMOwnershipForPrefix(ctx, prefix, ownership)
		default:
			assetType, err = findAndUpdateOwnershipForPrefix(ctx, prefix, ownership)
		}
		if err != nil {
			return err
		}

		// Then update the ownership data table
		_, err = inventory.PutOwnershipData(ctx, ownership, prefix, assetType)
		return err
	}, &datastore.TransactionOptions{})

}

// Searches for entities with names starting with the bot id prefix and
// updates their ownership config
func findAndUpdateOwnershipForPrefix(ctx context.Context, prefix string, ownershipData *ufspb.OwnershipData) (string, error) {
	errs := make(errors.MultiError, 0)
	found, err := findAndUpdateMachineOwnershipForPrefix(ctx, prefix, ownershipData)
	if err != nil {
		errs = append(errs, err)
	}
	if found {
		return inventory.AssetTypeMachine, errs.AsError()
	}
	found, err = findAndUpdateMachineLSEOwnershipForPrefix(ctx, prefix, ownershipData)
	if err != nil {
		errs = append(errs, err)
	}
	if found {
		return inventory.AssetTypeMachineLSE, errs.AsError()
	}
	found, err = findAndUpdateVMOwnershipForPrefix(ctx, prefix, ownershipData)
	if err != nil {
		errs = append(errs, err)
	}
	if found {
		return inventory.AssetTypeVM, errs.AsError()
	}
	return "", errs.AsError()
}

// Searches for machine entities with names starting with the bot id prefix and
// updates their ownership config
func findAndUpdateMachineOwnershipForPrefix(ctx context.Context, prefix string, ownershipData *ufspb.OwnershipData) (bool, error) {
	entities, _, err := registration.ListMachinesByIdPrefixSearch(ctx, -1, "", prefix, true)
	if err != nil || len(entities) == 0 {
		return false, err
	}
	logging.Infof(ctx, "Found %d machines with id prefix %s", len(entities), prefix)
	botsIds := map[string]*ufspb.OwnershipData{}
	for _, machine := range entities {
		botsIds[machine.GetName()] = ownershipData
	}
	err = updateBotConfigForBotIds(ctx, botsIds)
	return true, err
}

// Searches for machineLSE entities with names starting with the bot id prefix and
// updates their ownership config
func findAndUpdateMachineLSEOwnershipForPrefix(ctx context.Context, prefix string, ownershipData *ufspb.OwnershipData) (bool, error) {
	entities, _, err := inventory.ListMachineLSEsByIdPrefixSearch(ctx, -1, "", prefix, true)
	if err != nil || len(entities) == 0 {
		return false, err
	}
	logging.Infof(ctx, "Found %d machineLSEs with id prefix %s", len(entities), prefix)
	botsIds := map[string]*ufspb.OwnershipData{}
	for _, machineLSE := range entities {
		botsIds[machineLSE.GetName()] = ownershipData
	}
	err = updateBotConfigForBotIds(ctx, botsIds)
	return true, err
}

// Searches for machineLSE entities with names starting with the bot id prefix and
// updates their ownership config
func findAndUpdateVMOwnershipForPrefix(ctx context.Context, prefix string, ownershipData *ufspb.OwnershipData) (bool, error) {
	entities, _, err := inventory.ListVMsByIdPrefixSearch(ctx, -1, -1, "", prefix, true, nil)
	if err != nil || len(entities) == 0 {
		return false, err
	}
	logging.Infof(ctx, "Found %d VMs with id prefix %s", len(entities), prefix)
	botsIds := map[string]*ufspb.OwnershipData{}
	for _, vm := range entities {
		botsIds[vm.GetName()] = ownershipData
	}
	err = updateBotConfigForBotIds(ctx, botsIds)
	return true, err
}

// GetOwnershipData gets the ownership data in the Data store for the requested bot in the config.
func GetOwnershipData(ctx context.Context, hostName string) (*ufspb.OwnershipData, error) {
	host, err := inventory.GetOwnershipData(ctx, hostName)
	if err != nil {
		return nil, err
	}
	proto, err := host.GetProto()
	if err != nil {
		return nil, err
	}
	return proto.(*ufspb.OwnershipData), err
}

// parseBotIds parses a range of bot Ids from the input string and returns an array of bot Ids
func parseBotIds(idExpr string) []string {
	prefix := ""
	suffix := ""
	botsIds := []string{}
	if strings.Contains(idExpr, "{") && strings.Contains(idExpr, "}") && strings.Index(idExpr, "{") < strings.Index(idExpr, "}") {
		// Get the prefix and suffix but trimming string after and before '{' and '}'
		prefix = strings.Split(idExpr, "{")[0]
		suffix = strings.Split(idExpr, "}")[1]

		// Get the number range by trimming prefix and suffix
		numRange := strings.TrimSuffix(strings.TrimPrefix(idExpr, prefix+"{"), "}"+suffix)
		nums := strings.Split(numRange, ",")

		for _, num := range nums {
			if !strings.Contains(num, "..") {
				botsIds = append(botsIds, prefix+num+suffix)
			} else {
				// Expand the range of numbers. Ex 1..10 will be expanded to number 1 to 10 inclusive
				rangeIds := strings.Split(num, "..")
				if len(rangeIds) != 2 {
					continue
				}
				start, starterr := strconv.Atoi(rangeIds[0])
				end, enderr := strconv.Atoi(rangeIds[1])

				// Skip this range is it not correctly formed
				if starterr != nil || enderr != nil || start > end {
					continue
				}

				for i := start; i <= end; i++ {
					botsIds = append(botsIds, fmt.Sprintf("%s%d%s", prefix, i, suffix))
				}
			}
		}
	}
	return botsIds
}

// Checks if the hashes are equal, updates s.sha1 to the current hash, and returns the comparison result
func (s *ConfigSha) compareAndSetSha(currentSha1 string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	isSameSha := s.sha1 == currentSha1
	s.sha1 = currentSha1
	return isSameSha
}

// Gets the latest SHA1 for the given project and branch.
func fetchLatestSHA1(ctx context.Context, gitilesC external.GitTilesClient, project, branch string) (string, error) {
	resp, err := gitilesC.Log(ctx, &gitiles.LogRequest{
		Project:    project,
		Committish: fmt.Sprintf("refs/heads/%s", branch),
		PageSize:   1,
	})
	if err != nil {
		return "", errors.Annotate(err, "fetch sha1 for %s branch of %s", branch, project).Err()
	}
	if len(resp.Log) == 0 {
		return "", fmt.Errorf("fetch sha1 for %s branch of %s: empty git-log", branch, project)
	}
	return resp.Log[0].GetId(), nil
}

// fetchConfigProtoFromGitiles fetches security configs at a particular commitsh
func fetchConfigProtoFromGitiles(ctx context.Context, gitilesC external.GitTilesClient, project, committish, path string) (string, error) {
	req := &gitiles.DownloadFileRequest{
		Project:    project,
		Committish: committish,
		Path:       path,
	}
	rsp, err := gitilesC.DownloadFile(ctx, req)
	if err != nil {
		return "", err
	}
	if rsp == nil {
		return "", errors.Reason("downloaded security config was empty").Err()
	}
	return rsp.Contents, nil
}

// Checks if value already exists in the array
func existsInList(list []string, name string) bool {
	for i := range list {
		// Already exists in the pools list
		if name == list[i] {
			return true
		}
	}
	return false
}

// Checks if the hostname prefix exists in the map
func mapContainsPrefix(botPrefixesMap map[string]*ufspb.OwnershipData, hostName string) bool {
	for key := range botPrefixesMap {
		if strings.HasPrefix(hostName, key) {
			return true
		}
	}
	return false
}
