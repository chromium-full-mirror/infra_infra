// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package regulator

import (
	"flag"

	"infra/cros/botsregulator/internal/clients"
	ufsUtil "infra/unifiedfleet/app/util"
)

// RegulatorOptions refers to the flag options needed
// to create a new regulator struct.
type RegulatorOptions struct {
	BPI        string
	CfID       string
	Hive       string
	Namespace  string
	UFS        string
	Swarming   string
	Zone       string
	BotConfigs string
	CfIDHives  string
}

// RegisterFlags exposes the command line flags required to run the application.
// We never check for flag emptiness so all options must have defaults.
func (r *RegulatorOptions) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&r.BPI, "bpi", clients.GcepDev, "URI endpoint of the service used to scale bots.")
	fs.StringVar(&r.CfID, "config", clients.ConfigID, "CloudBots config prefix.")
	fs.StringVar(&r.Hive, "hive", "cloudbots", "hive used for UFS filtering.")
	fs.StringVar(&r.Namespace, "ufs-namespace", ufsUtil.OSNamespace, "UFS namespace.")
	fs.StringVar(&r.UFS, "ufs", clients.UfsDev, "UFS endpoint.")
	fs.StringVar(&r.Swarming, "swarming", clients.SwarmingDev, "Swarming server.")
	fs.StringVar(&r.BotConfigs, "botconfigs", "skylab.py", "a comma-separated list of bots configs. e.g skylab.py,cloudbots_config.py")
	fs.StringVar(&r.Zone, "zone", "ZONE_SFO36_OS", "UFS zone. e.g ZONE_SFO36_OS")
	fs.StringVar(&r.CfIDHives, "config-hive", "", "a comma-separated list of config prefix hive names. e.g cloudbots-e2-small:cloudbots,cloudbots-e2-custom-2-6144:cloudbots-large")
}
