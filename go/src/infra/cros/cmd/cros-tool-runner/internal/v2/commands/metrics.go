// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"log"
	"regexp"
	"time"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/docker"
)

const (
	statusPass       = "pass"
	statusFail       = "fail"
	imageNameUnknown = "UNKNOWN"
)

// monitorTime calls metrics API to track the execution time of a command. Only
// a startTime is required as the API calculates the time elapsed.
func monitorTime(cmd Command, startTime time.Time) {
	switch cmd := cmd.(type) {
	case *DockerPull:
		service := getContainerImageNameFrom(cmd.ContainerImage)
		docker.LogPullTime(context.Background(), startTime, service)
	case *DockerRun:
		service := getContainerImageNameFrom(cmd.GetContainerImage())
		docker.LogRunTime(context.Background(), startTime, service)
	default:
		log.Printf("warning: unsupported command for monitoring time %s", cmd)
	}
}

// monitorStatus calls metrics API to track the success or failure of a
// DockerRun command.
func monitorStatus(cmd Command, status string) {
	switch cmd.(type) {
	case *DockerRun:
		docker.LogStatus(context.Background(), status)
	default:
		log.Printf("warning: unsupported command for monitoring status %s", cmd)
	}
}

// getContainerImageNameFrom extracts image name from a full qualified docker
// image location/uri. e.g. cros-test will be extracted from
// us-docker.pkg.dev/cros-registry/test-services/cros-test:tag-name
func getContainerImageNameFrom(location string) string {
	r := regexp.MustCompile(`.+/([^/:@]+)(?:[:@].+)?`)
	match := r.FindStringSubmatch(location)
	if match == nil || len(match) != 2 {
		log.Printf("warning: unable to extract image name from location %s", location)
		return imageNameUnknown
	}
	return match[1]
}
