// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

// A config represents the config for drone-agent.
//
// Note that the YAML library unmarshals using the field name
// lowercased as the default key.
type config struct {
	// QueenService is the address of the drone queen service.
	QueenService string
	// SwarmingURL is the URL of the Swarming instance to use.
	// Should be a full URL without the path, e.g.,
	// https://host.example.com
	SwarmingURL           string
	DUTCapacity           int `yaml:"dutCapacity"`
	ReportingIntervalMins int
	// Hive value of the drone agent.  This is used for DUT/drone affinity.
	// A drone is assigned DUTs with same hive value.
	Hive string

	// TSMonEndpoint is the URL for posting monitoring metrics.
	// If empty, we will try to load configuration from LUCI TSMon default
	// configuration file /etc/chrome-infra/ts-mon.json.
	TSMonEndpoint       string `yaml:"tsMonEndpoint"`
	TSMonCredentialPath string `yaml:"tsMonCredentialPath"`

	// BotPrefix is used as the prefix for the bot ID.
	// Default value is 'crossk-'
	BotPrefix string
	// PythonVersion is the python version to invoke swarming bot.
	// Default value is 'python3.8'
	PythonVersion string `yaml:"pythonVersion"`
	// WorkingDirPath is the path where tests are run on.
	// Default value is $HOME/skylab_bots
	WorkingDirPath string `yaml:"workingDirPath"`

	// Block IO throttle settings. 0 means no throttling. Only /dev/sda (device
	// number 8:0) is supported.
	BotBlockIOReadBPS  int
	BotBlockIOWriteBPS int

	// OTLPExporterAddr is the destination for traces.
	OTLPExporterAddr string

	// Megadrone config settings

	// EnableMegadrone enables megadrone mode if true.
	// Most config settings like QueenService are ignored in megadrone mode.
	EnableMegadrone bool
	// NumBots sets the number of bots to run in megadrone mode.
	NumBots int
}

func (c *config) ReportingInterval() time.Duration {
	return time.Duration(c.ReportingIntervalMins) * time.Minute
}

// parseConfigFile parses the config file for drone-agent.
// This function always returns a valid config object.
// Errors are logged.
//
// This function also parses the environment and global flag vars to
// implement backward compatibility.
func parseConfigFile(path string) *config {
	// Default values
	cfg := config{
		DUTCapacity:           10,
		ReportingIntervalMins: 1,
		BotPrefix:             "crossk-",
		NumBots:               1,
		PythonVersion:         "python3.8",
		WorkingDirPath:        filepath.Join(os.Getenv("HOME"), "skylab_bots"),
	}
	addBackwardCompatConfig(&cfg)
	if path == "" {
		return &cfg
	}
	b, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Error: parse config file: %s", err)
		return &cfg
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		log.Printf("Error: parse config file: %s", err)
		return &cfg
	}
	return &cfg
}

// addBackwardCompatConfig parses the environment and global flag vars
// and adds settings to the config.  This is for backward
// compatibility.
func addBackwardCompatConfig(cfg *config) {
	// Environment variables.
	cfg.QueenService = queenService
	cfg.SwarmingURL = swarmingURL
	cfg.DUTCapacity = dutCapacity
	cfg.ReportingIntervalMins = int(reportingInterval / time.Minute)
	cfg.Hive = hive
	cfg.TSMonEndpoint = tsmonEndpoint
	cfg.TSMonCredentialPath = tsmonCredentialPath
	cfg.BotPrefix = botPrefix
	cfg.BotBlockIOReadBPS = botBlkIOReadBPS
	cfg.BotBlockIOWriteBPS = botBlkIOWriteBPS

	// Flags.
	cfg.OTLPExporterAddr = *traceTarget
}
