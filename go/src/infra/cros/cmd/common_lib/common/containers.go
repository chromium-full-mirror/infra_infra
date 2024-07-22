// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"regexp"
	"strconv"

	buildapi "go.chromium.org/chromiumos/config/go/build/api"
)

const (
	DefaultCrosFwProvisionSha = "36c32627ae54429d861d0df0fbc4d883170f01d83976ef5be9a0b4e719111aa0"
	DefaultPostProcessSha     = "e790463305397ba157cdfd836a23fc385d37cf9a72709f511a7e1973f64291c2"
)

// Create container with provided name and digest, setting repository
// to the hostname `us-docker.pkg.dev` and project `cros-registry/test-services`.
func CreateTestServicesContainer(name, digest string) *buildapi.ContainerImageInfo {
	return &buildapi.ContainerImageInfo{
		Repository: &buildapi.GcrRepository{
			Hostname: "us-docker.pkg.dev",
			Project:  "cros-registry/test-services",
		},
		Name:   name,
		Digest: fmt.Sprintf("sha256:%s", digest),
		Tags:   []string{"prod"},
	}
}

// Set name within images to be a test service container with given name and digest.
func AddTestServiceContainerToImages(images map[string]*buildapi.ContainerImageInfo, name, digest string) {
	images[name] = CreateTestServicesContainer(name, digest)
}

func PatchContainerMetadata(images map[string]*buildapi.ContainerImageInfo, buildStr string) {
	buildNumber := ExtractBuildRNumber(buildStr)
	if _, ok := images[PostProcess]; !ok || buildNumber < 128 {
		AddTestServiceContainerToImages(images, PostProcess, DefaultPostProcessSha)
	}
}

// ExtractBuildRNumber takes any build string and extracts
// the major digits found within the R#.
// If no R number match found, return -1.
func ExtractBuildRNumber(buildStr string) int {
	rNumberRegex := regexp.MustCompile(`R(\d+)`)
	matches := rNumberRegex.FindStringSubmatch(buildStr)
	if len(matches) == 0 {
		return -1
	}
	// If there is a match, then there will also be a captured R#.
	rNum, _ := strconv.Atoi(matches[1])
	return rNum
}
