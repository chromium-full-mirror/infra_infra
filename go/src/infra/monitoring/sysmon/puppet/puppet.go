// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package puppet

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/types"
)

var (
	configVersion = metric.NewInt("puppet/version/config",
		"The version of the puppet configuration.  By default this is the time that the configuration was parsed",
		nil)
	exitStatus = metric.NewInt("puppet/exit_status",
		"Exit status of the previous puppet agent run.",
		nil)
	puppetVersion = metric.NewString("puppet/version/puppet",
		"Version of puppet client installed.",
		nil)
	resources = metric.NewInt("puppet/resources",
		"Number of resources known by the puppet client in its last run",
		nil,
		field.String("action"))
	times = metric.NewFloat("puppet/times",
		"Time taken to perform various parts of the last puppet run",
		nil,
		field.String("step"))
	events = metric.NewInt("puppet/events",
		"Number of changes the puppet client made to the system in its last run, by success or failure",
		nil,
		field.String("result"))
	failure = metric.NewBool("puppet/failure",
		"Puppet client's last run, by success or failure",
		nil)
	age = metric.NewFloat("puppet/age",
		"Time since last run",
		nil)
	isCanary = metric.NewBool("puppet/is_canary",
		"Whether Puppet installs canary versions of CIPD packages on this machine",
		nil)
	certExpiry = metric.NewInt("puppet/cert_expiry",
		"Time until the agent cert expires",
		&types.MetricMetadata{Units: types.Seconds})
)

type lastRunData struct {
	Version struct {
		Config int64
		Puppet string
	}
	Resources map[string]int64
	Time      map[string]float64
	Changes   map[string]int64
	Events    map[string]int64
}

// Register adds tsmon callbacks to set puppet metrics.
func Register() {
	tsmon.RegisterCallback(func(c context.Context) {
		if path, err := lastRunFile(); err != nil {
			logging.Warningf(c, "Failed to get puppet last_run_summary.yaml path: %v", err)
		} else if err = updateLastRunStats(c, path); err != nil {
			logging.Warningf(c, "Failed to update puppet metrics: %v", err)
		}

		if path, err := puppetCertPath(); err != nil {
			logging.Warningf(c, "Failed to get puppet cert file: %v", err)
			certExpiry.Set(c, 0)
		} else if err = updateCertExpiry(c, path); err != nil {
			logging.Warningf(c, "Failed to update puppet cert expiry: %v", err)
		}

		if path, err := puppetConfFile(); err != nil {
			logging.Warningf(c, "Failed to get puppet.conf path: %v", err)
		} else if err = updateIsCanary(c, path); err != nil {
			logging.Warningf(c, "Failed to update puppet canary metric: %v", err)
		}

		if err := updateExitStatus(c, exitStatusFiles()); err != nil {
			logging.Warningf(c, "Failed to update puppet exit status metric: %v", err)
		}
	})
}

func updateLastRunStats(c context.Context, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data lastRunData
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return err
	}

	configVersion.Set(c, data.Version.Config)
	puppetVersion.Set(c, data.Version.Puppet)

	for k, v := range data.Resources {
		resources.Set(c, v, k)
	}

	for k, v := range data.Events {
		if k != "total" {
			events.Set(c, v, k)
		}
	}

	count, exist := data.Events["failure"]
	count += data.Resources["failed"]
	count += data.Resources["failed_to_restart"]
	failure.Set(c, !exist || count > 0)

	for k, v := range data.Time {
		if k == "last_run" {
			age.Set(c, float64(clock.Now(c).Sub(time.Unix(int64(v), 0)))/float64(time.Second))
		} else if k != "total" {
			times.Set(c, v, k)
		}
	}

	return nil
}

func updateCertExpiry(c context.Context, path string) error {
	var expiryTime int64

	defer func() {
		certExpiry.Set(c, expiryTime)
	}()

	hostName, err := os.Hostname()
	if err != nil {
		return err
	}

	matches, _ := filepath.Glob(filepath.Join(path, hostName+"*.pem"))

	if len(matches) == 0 {
		return fmt.Errorf("cert not found for %s at %s", hostName, path)
	}
	certFilePath := matches[0]

	data, err := os.ReadFile(certFilePath)
	if err != nil {
		return fmt.Errorf("error reading puppet cert at %s: %w", certFilePath, err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("error parsing certificate PEM at %s", certFilePath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing certificate at %s: %w", certFilePath, err)
	}

	expiryTime = cert.NotAfter.Unix() - clock.Now(c).Unix()
	return nil
}

func updateIsCanary(c context.Context, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		isCanary.Set(c, err == nil)
		return fmt.Errorf("error reading puppet conf at %s: %w", path, err)
	}

	isCanary.Set(c, strings.Contains(string(raw), "environment=canary"))

	return nil
}

func updateExitStatus(c context.Context, paths []string) error {
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue // Try other paths in the list
		}

		status, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
		if err != nil {
			return fmt.Errorf("file %s does not contain a number: %w", path, err)
		}

		exitStatus.Set(c, status)
		return nil
	}

	return fmt.Errorf("no files found: %s", paths)
}
