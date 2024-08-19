// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dumper

import (
	"context"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"

	ufsdevice "infra/unifiedfleet/api/v1/models/chromeos/device"
	"infra/unifiedfleet/app/config"
	"infra/unifiedfleet/app/external"
	"infra/unifiedfleet/app/model/configuration"
	"infra/unifiedfleet/app/util"
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
	var allCfgs ufsdevice.AllConfigs
	err = fetchConfigProtoFromGitiles(ctx, gc, cronCfg.Project, cronCfg.Committish, cronCfg.ConfigsPath, &allCfgs)
	if err != nil {
		return errors.Annotate(err, "failed fetch from %s:%s:%s", cronCfg.Project, cronCfg.Committish, cronCfg.ConfigsPath).Err()
	}
	logging.Debugf(ctx, "Fetched %d DeviceConfigs from gitiles", len(allCfgs.GetConfigs()))

	cfgs := make([]*ufsdevice.Config, len(allCfgs.GetConfigs()))
	for i, c := range allCfgs.GetConfigs() {
		cfgs[i] = c
	}

	failed_ns := []string{}
	for ns, realmFunc := range namespaceToRealmAssignerMap {
		if err := insertConfigInNamespace(ctx, cfgs, ns, realmFunc); err != nil {
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
func fetchConfigProtoFromGitiles(ctx context.Context, client external.GitTilesClient, project, committish, path string, cfgs *ufsdevice.AllConfigs) error {
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

// insertConfigInNamespace sets the context to the appropriate namespace
// and inserts configs
func insertConfigInNamespace(ctx context.Context, cfgs []*ufsdevice.Config, ns string, realmFunc configuration.RealmAssignerFunc) error {
	ctx, err := util.SetupDatastoreNamespace(ctx, ns)
	if err != nil {
		return errors.Annotate(err, "failed to set namespace").Err()
	}
	_, err = configuration.BatchUpdateDeviceConfigs(ctx, cfgs, realmFunc)
	if err != nil {
		return errors.Annotate(err, "failed to insert configs to datastore in namespace %s", ns).Err()
	}
	logging.Debugf(ctx, "Successfully inserted DeviceConfigs to UFS datastore in namespace %s", ns)
	return nil
}
