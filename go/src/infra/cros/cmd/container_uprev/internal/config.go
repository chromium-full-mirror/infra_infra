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
	"infra/cros/cmd/container_uprev/internal/preppers"
)

var (
	//go:embed dockerfiles/*
	Dockerfiles embed.FS
	//go:embed resources/*
	Resources embed.FS

	DefaultRepository = &Repository{
		Hostname:      common.DefaultDockerHost,
		Project:       common.DefaultDockerProject,
		FirestoreHost: common.TestPlatformFireStore,
	}
	PartnerRepository = &Repository{
		Hostname:      common.DefaultDockerHost,
		Project:       common.PartnerDockerProject,
		FirestoreHost: common.PartnerTestPlatformFireStore,
	}
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
	// Repository information.
	// If empty, defaults to
	// 	host: us-docker.pkg.dev
	// 	project: cros-registry/test-services
	//  firestoreHost: test-platform-store
	Repositories []*Repository
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

type Repository struct {
	Hostname      string
	Project       string
	FirestoreHost string
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
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
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
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "cros-legacy-hw-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/hardware_solvers/legacy_hw_filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "use_flag_filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/use_flag_filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "pre_process_filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/pre_process_filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "al-provision-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/al-provision-filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "adb-base",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/base-adb/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
			Prepper: preppers.AdbBase,
		},
		{
			Name: "ants-publish",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/publish/ants-publish/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "ants-publish-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/ants-publish-filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "rdb-publish",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/publish/rdb-publish/${platform}"),
				NewCIPDPackageWithRef("infra/tools/result_adapter/linux-amd64", "prod"),
				NewCIPDPackageWithRef("infra/tools/rdb/linux-amd64", "latest"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "gcs-publish",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/publish/gcs-publish/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "cros-dut",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/dut/cros-dut/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "servo-nexus",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/dut/cros-servod/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "cros-provision",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/provision/cros-provision/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "cros-fw-provision",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/provision/cros-fw-provision/${platform}"),
			},
		},
		{
			Name: "foil-provision",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/provision/foil-provision/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "autovm_test_shifter_filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/autovm_test_shifter_filter/${platform}"),
			},
		},
		// {
		// 	Name: "tradefed",
		// 	CIPDPackages: []*CIPDPackage{
		// 		NewCIPDPackage("chromiumos/infra/cft/execution/cros-test/${platform}"),
		// 	},
		// 	Repositories: []*Repository{
		// 		DefaultRepository,
		// 	},
		// 	Prepper: preppers.InternalTF,
		// },
	}

	return CleanConfigs(configs)
}

func CleanConfigs(configs []*UprevConfig) []*UprevConfig {
	for _, config := range configs {
		if config.ContainerName == "" {
			config.ContainerName = config.Name
		}

		if config.Repositories == nil || len(config.Repositories) == 0 {
			config.Repositories = []*Repository{DefaultRepository}
		}
	}

	return configs
}
