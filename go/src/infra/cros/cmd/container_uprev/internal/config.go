// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package internal contains the internals of the uprev service.
package internal

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/common_lib/common"
)

var (
	//go:embed dockerfiles/*
	Dockerfiles embed.FS
	//go:embed resources/*
	Resources embed.FS
)

// WriteDockerfile writes the embedded dockerfile to the temporary directory.
func WriteDockerfile(dir string, name string) error {
	dockerfile, err := Dockerfiles.ReadFile(fmt.Sprintf("dockerfiles/Dockerfile_%s", name))
	if err != nil {
		return errors.Annotate(err, "failed to read Dockerfile_%s", name).Err()
	}
	return os.WriteFile(path.Join(dir, "Dockerfile"), dockerfile, common.FilePermission)
}

// WriteResource writes the embedded resource to the temporary directory.
func WriteResource(dir string, name string) error {
	resource, err := Resources.ReadFile(fmt.Sprintf("resources/%s", name))
	if err != nil {
		return errors.Annotate(err, "failed to read %s", name).Err()
	}
	return os.WriteFile(path.Join(dir, name), resource, common.FilePermission)
}

// CIPDPackage contains relevant information about a CIPDPackage.
type CIPDPackage struct {
	// Name of CIPD package.
	Name string
	// Reference label of CIPD package.
	// Mainly for local development.
	Ref string
}

// NewCIPDPackageWithRef creates a CIPDPackage with a reference label.
func NewCIPDPackageWithRef(name, ref string) *CIPDPackage {
	return &CIPDPackage{
		Name: name,
		Ref:  ref,
	}
}

// NewCIPDPackage creates a CIPDPackage without a reference label.
func NewCIPDPackage(name string) *CIPDPackage {
	return NewCIPDPackageWithRef(name, "")
}

// UprevConfig describes a container's uprev information.
type UprevConfig struct {
	// Dockerfile found by: Dockerfile_<Name>
	Name string
	// Optional repository information.
	// Defaults to
	// 	host: us-docker.pkg.dev
	// 	project: cros-registry/test-services
	RepositoryHostname string
	RepositoryProject  string
	// Defaults to Name, but can be separately set if
	// container name is different than the uprev name.
	ContainerName string
	// Binaries used during docker image setup.
	CIPDPackages []*CIPDPackage
	// Prepper is a function signature representing
	// any custom work needed by the Dockerfile.
	Prepper   func(ctx context.Context, dir string) error
	Resources []string
}

// GetConfigs returns the uprev configs.
func GetConfigs() []*UprevConfig {
	configs := []*UprevConfig{
		{
			Name: "provision-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/provision-filter/${platform}"),
			},
			Resources: []string{
				"provision-filter-q.txt",
			},
		},
		{
			Name: "firmware-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/firmware-filter/${platform}"),
			},
		},
		{
			Name: "foil-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/foil-filter/${platform}"),
			},
		},
		{
			Name: "cros-legacy-hw-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/hardware_solvers/legacy_hw_filter/${platform}"),
			},
		},
		{
			Name: "use_flag_filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/use_flag_filter/${platform}"),
			},
		},
		{
			Name: "al-provision-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/al-provision-filter/${platform}"),
			},
		},
		{
			Name: "adb-base",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/base-adb/${platform}"),
			},
		},
	}

	return CleanConfigs(configs)
}

func CleanConfigs(configs []*UprevConfig) []*UprevConfig {
	for _, config := range configs {
		if config.ContainerName == "" {
			config.ContainerName = config.Name
		}
	}

	return configs
}
