// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package stdenv

import (
	"fmt"

	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/base/workflow"
)

// TODO(fancl): build the container using pkgbuild.
// Dict of the dockcross images for all supported platforms.
// Mapping host.Arch() to the image name.
var (
	containerRegistry = "gcr.io/chromium-container-registry/infra-dockerbuild/"
	containerVersion  = ":v1.4.21"
	containers        = map[string]string{
		"amd64":   containerRegistry + "manylinux-x64-py3" + containerVersion,
		"arm64":   containerRegistry + "linux-arm64-py3" + containerVersion,
		"arm":     containerRegistry + "linux-armv6-py3" + containerVersion,
		"riscv64": containerRegistry + "linux-riscv64-py3" + containerVersion,
	}
)

func importLinux(cfg *Config, bins ...string) (gs []generators.Generator, err error) {
	// Import posix utilities
	g, err := generators.FromPathBatch("posix_import", cfg.FindBinary, bins...)
	if err != nil {
		return nil, err
	}
	gs = append(gs, g)

	// Import docker
	g, err = generators.FromPathBatch("docker_import", cfg.FindBinary, "docker")
	if err != nil {
		return nil, err
	}
	gs = append(gs, g)

	return
}

func (g *Generator) generateLinux(plats generators.Platforms, tmpl *workflow.Generator) error {
	containers := containers[plats.Host.Arch()]
	if containers == "" {
		return fmt.Errorf("containers not available for %s", plats.Host)
	}

	tmpl.Env.Set("dockerImage", containers)
	return nil
}
