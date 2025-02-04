// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"go.chromium.org/luci/common/errors"

	"infra/libs/skylab/common/heuristics"
)

// SwarmingBotProvider is the host that runs a swarming bot, e.g. GCE or Drone.
type SwarmingBotProvider string

const (
	// swarmingBotIDEnvName is the env variable name for bot ID
	swarmingBotIDEnvName = "SWARMING_BOT_ID"

	// swarmingBotPrefixGce is the prefix used in all Gce bots for VMLab.
	// The vmlab PROD pool has a more specific prefix: "chromeos-test-vmlab-"
	// See https://crrev.com/i/5266273. Note that the staging pool is shared.
	swarmingBotPrefixGce = "chromeos-"

	// dockerConfigEnvName is the env variable name for docker config directory
	// it is used to determine when the tests are running in PVS
	dockerConfigEnvName = "DOCKER_CONFIG"

	// dockerConfigMatchPVS is a substring that if found in dockerConfigEnvName
	// indicates the tests are running in PVS
	dockerConfigMatchPVS = ".pvs"

	// List of supported SwarmingBotProvider types.
	// TODO(mingkong): consider moving these values to a proto enum
	BotProviderGce     SwarmingBotProvider = "GCE"
	BotProviderDrone   SwarmingBotProvider = "Drone"
	BotProviderPVS     SwarmingBotProvider = "PVS"
	BotProviderUnknown SwarmingBotProvider = "Unknown"
)

// GetHostIP returns the IP address that is accessible from outside the host
func GetHostIP() (string, error) {
	cmd := exec.Command("hostname", "-I")
	stdout, stderr, err := RunCommand(context.Background(), cmd, "hostname", nil, true)
	if err != nil {
		return "", errors.Annotate(err, "%s: %s", "unable to find localhost IP: ", stderr).Err()
	}
	if strings.TrimSpace(stdout) == "" {
		return "", errors.New("unable to find localhost IP: hostname -I returns no results")
	}
	return strings.Fields(stdout)[0], nil
}

// GetBotProvider detects the SwarmingBotProvider by examining env variable.
func GetBotProvider() SwarmingBotProvider {
	if lookup, found := os.LookupEnv(dockerConfigEnvName); found {
		if strings.Contains(lookup, dockerConfigMatchPVS) {
			return BotProviderPVS
		}
	}
	if lookup, found := os.LookupEnv(swarmingBotIDEnvName); found {
		for _, p := range heuristics.HwSwarmingBotIDPrefixes {
			if strings.HasPrefix(lookup, p) {
				return BotProviderDrone
			}
		}
		if strings.HasPrefix(lookup, swarmingBotPrefixGce) {
			return BotProviderGce
		}
	}
	return BotProviderUnknown
}

// WaitDutVMBoot uses a blocking SSH call to wait for a DUT VM to become ready.
// It doesn't care about the output. If the connection is successful, it
// executes `true` that returns nothing. If permission denied, it means SSH is
// ready. If timeout, we leave it to the following step to detect the error.
func WaitDutVMBoot(ctx context.Context, ip string) {
	cmd := exec.Command("/usr/bin/ssh",
		"-o", "ConnectTimeout=120",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"root@"+ip,
		"true")
	_, _, _ = RunCommand(ctx, cmd, "ssh", nil, true)
}
