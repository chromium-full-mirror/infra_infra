// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"

	"google.golang.org/protobuf/encoding/protojson"

	deviceconfig "go.chromium.org/chromiumos/infra/proto/go/device"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"

	"go.chromium.org/infra/unifiedfleet/app/config"
	"go.chromium.org/infra/unifiedfleet/app/external"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

// namespaceToRealmAssignerMap controls what namespaces are synced, and how realms are assigned
var namespaceToRealmAssignerMap = map[string]configuration.RealmAssignerFunc{
	util.OSPartnerNamespace: configuration.BoardModelRealmAssigner,
	util.OSNamespace:        configuration.CrOSRealmAssigner,
}

// syncDeviceConfigs fetches devices configs from a file checked into gerrit
// and inserts to UFS datastore
func syncDeviceConfigs(ctx context.Context) (err error) {
	defer func() {
		syncDeviceConfigsTick.Add(ctx, 1, err == nil)
	}()
	// get ufs-level config for this cron job
	cronCfg := config.Get(ctx).GetDeviceConfigsPushConfigs()

	if !cronCfg.Enabled {
		logging.Infof(ctx, "ufs.device_config.sync scheduled but is disabled in this env")
		return
	}

	es, err := external.GetServerInterface(ctx)
	if err != nil {
		return err
	}
	gc, err := es.NewGitTilesInterface(ctx, cronCfg.GetGitilesHost())
	if err != nil {
		return errors.Annotate(err, "problem creating gitiles client:").Err()
	}

	logging.Debugf(ctx, "Downloading the device config file %s:%s:%s from gitiles repo", cronCfg.Project, cronCfg.Committish, cronCfg.ConfigsPath)
	var allCfgs deviceconfig.AllConfigs
	err = fetchConfigProtoFromGitiles(ctx, gc, cronCfg.Project, cronCfg.Committish, cronCfg.ConfigsPath, &allCfgs)
	if err != nil {
		return errors.Annotate(err, "failed fetch from %s:%s:%s", cronCfg.Project, cronCfg.Committish, cronCfg.ConfigsPath).Err()
	}
	logging.Debugf(ctx, "Fetched %d DeviceConfigs from gitiles", len(allCfgs.GetConfigs()))

	cfgs := make([]*deviceconfig.Config, len(allCfgs.GetConfigs()))
	for i, c := range allCfgs.GetConfigs() {
		cfgs[i] = c
	}

	failed_ns := []string{}
	for ns, realmFunc := range namespaceToRealmAssignerMap {
		if err := updateConfigInNamespace(ctx, cfgs, ns, realmFunc); err != nil {
			failed_ns = append(failed_ns, ns)
		}
	}
	if len(failed_ns) != 0 {
		return errors.Reason("failed to sync DeviceConfigs in the following namespaces: %v", failed_ns).Err()
	}
	return nil
}

// fetchConfigProtoFromGitiles fetches device configs and unmarshalls the raw
// string into an array of proto messages
func fetchConfigProtoFromGitiles(ctx context.Context, client external.GitTilesClient, project, committish, path string, cfgs *deviceconfig.AllConfigs) error {
	req := &gitilespb.DownloadFileRequest{
		Project:    project,
		Committish: committish,
		Path:       path,
	}
	content, err := downloadDeviceConfigFromGitiles(ctx, client, req)
	if err != nil {
		return err
	}

	if err := protojson.Unmarshal([]byte(content), cfgs); err != nil {
		return err
	}
	return nil
}

// downloadDeviceConfigFromGitiles fetches content of the file as a string
func downloadDeviceConfigFromGitiles(ctx context.Context, client external.GitTilesClient, req *gitilespb.DownloadFileRequest) (string, error) {
	rsp, err := client.DownloadFile(ctx, req)
	if err != nil {
		return "", err
	}
	if rsp == nil {
		return "", errors.Reason("downloaded device config was empty").Err()
	}
	return rsp.Contents, nil
}

// updateConfigInNamespace sets the context to the appropriate namespace
func updateConfigInNamespace(ctx context.Context, cfgs []*deviceconfig.Config, ns string, realmFunc configuration.RealmAssignerFunc) error {
	ctx, err := util.SetupDatastoreNamespace(ctx, ns)
	if err != nil {
		return errors.Annotate(err, "failed to set namespace").Err()
	}
	_, err = configuration.BatchUpdateDeviceConfigs(ctx, cfgs, realmFunc)
	if err != nil {
		return errors.Annotate(err, "failed to insert configs to datastore in namespace %s", ns).Err()
	}
	logging.Debugf(ctx, "Successfully inserted DeviceConfigs to UFS datastore in namespace %s", ns)

	cfgMap := make(map[string]*deviceconfig.Config)
	for _, cfg := range cfgs {
		cfgMap[configuration.GetDeviceConfigIDStr(cfg.GetId())] = cfg
	}
	toDeleteDCIDs := make([]*deviceconfig.ConfigId, 0)
	for startToken := ""; ; {
		res, nextToken, err := configuration.ListDeviceConfigs(ctx, pageSize, startToken, nil, false)
		if err != nil {
			return errors.Annotate(err, "get all DeviceConfigs").Err()
		}
		for _, r := range res {
			if _, ok := cfgMap[configuration.GetDeviceConfigIDStr(r.GetId())]; !ok {
				toDeleteDCIDs = append(toDeleteDCIDs, r.GetId())
			}
		}
		if nextToken == "" {
			break
		}
		startToken = nextToken
	}
	return configuration.BatchDeleteDeviceConfigsACL(ctx, toDeleteDCIDs)
}
