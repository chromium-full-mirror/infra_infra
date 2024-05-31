// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package env

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultCloudBotSSHDir = "/usr/local/etc/cloudbots/.ssh/"
)

var DefaultSSHConfigPathOnCloudBot = filepath.Join(defaultCloudBotSSHDir, "config")

// RunningOnBot checks whether or not it is running on a bot, by way of checking
// the USER env var.
func RunningOnBot() bool {
	return os.Getenv("USER") == "chrome-bot"
}

// GetSwarmingTaskID retrieves the swarming task ID.
func GetSwarmingTaskID() string {
	return os.Getenv("SWARMING_TASK_ID")
}

// GetSwarmingBotID retrieves the swarming bot ID.
func GetSwarmingBotID() string {
	return os.Getenv("SWARMING_BOT_ID")
}

// IsCloudBot returns whether the process is running on cloud bot VM.
func IsCloudBot() bool {
	if swarmingBotID := GetSwarmingBotID(); strings.HasPrefix(swarmingBotID, "cloudbots-") {
		return true
	}
	return false
}

// GetBuildBucketID retrieves the build bucket ID.
func GetBuildBucketID() string {
	bbidArr := strings.Split(os.Getenv("LOGDOG_STREAM_PREFIX"), "/")
	bbidArrLen := len(bbidArr)
	if bbidArrLen > 0 {
		return bbidArr[bbidArrLen-1]
	}
	return os.Getenv("BUILD_BUCKET_ID")
}
