// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tsmon

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.chromium.org/luci/common/tsmon"
	"go.chromium.org/luci/common/tsmon/target"
)

const (
	saOnDrone  = "/creds/service_accounts/service_account_prodx_mon.json"
	saOnGceBot = "/creds/service_accounts/service-account-mon-sysmon.json"
)

func Init() error {
	// Application level flags.
	log.Printf("Setting up CTR docker tsmon...")

	credsFile, err := locateFile([]string{saOnDrone, saOnGceBot})
	if err != nil {
		return err
	}

	tsmonFlags := tsmon.NewFlags()
	tsmonFlags.Endpoint = "https://prodxmon-pa.googleapis.com/v1:insert"
	tsmonFlags.Credentials = credsFile
	tsmonFlags.Target.TargetType = target.TaskType
	tsmonFlags.Target.TaskServiceName = "CTR-DockerOps"
	tsmonFlags.Target.TaskJobName = "CTR-DockerOps"
	tsmonFlags.Flush = "auto"

	// Initialize the library once on application start:
	if err := tsmon.InitializeFromFlags(context.Background(), &tsmonFlags); err != nil {
		return fmt.Errorf("metrics: error setup tsmon: %w", err)
	}
	return nil
}

// Shutdown stops the metrics.
func Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Printf("Shutting down metrics...")
	tsmon.Shutdown(ctx)
}

// locateFile locates file from multiple possible locations where the file may
// exist. Return the located file as soon as the first one is found, or an error
// if none of the candidates exists.
func locateFile(candidates []string) (string, error) {
	for _, file := range candidates {
		if _, err := os.Stat(file); err == nil {
			return file, nil
		} else {
			log.Printf("warning: failed to locate candidate file %s error: %v", file, err)
		}
	}
	return "", fmt.Errorf("failed to find locate file from all candidates %v", candidates)
}
