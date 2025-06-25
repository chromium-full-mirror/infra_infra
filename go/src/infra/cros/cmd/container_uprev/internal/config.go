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

	"go.chromium.org/infra/cros/cmd/common_lib/cloudrun"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/container_uprev/internal/preppers"
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
	resourcePath := fmt.Sprintf("resources/%s", name)

	isDir, err := isResourceDir(resourcePath)
	if err != nil {
		return err
	}
	if isDir {
		return WriteDir(path.Join(dir, name), resourcePath)
	} else {
		return WriteFile(path.Join(dir, name), resourcePath)
	}
}

func WriteDir(dst, resourcePath string) error {
	err := os.Mkdir(dst, common.DirPermission)
	if err != nil {
		return fmt.Errorf("failed to make directory: %s", err)
	}
	entries, err := Resources.ReadDir(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to read directory from embedded filesystem: %s", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			// Call recursively. Need to still be in the correct format of resourcePath.
			err = WriteDir(path.Join(dst, entry.Name()), path.Join(resourcePath, entry.Name()))
			if err != nil {
				return fmt.Errorf("failed to write directory: %s", err)
			}
		} else {
			// Write the file
			err = WriteFile(path.Join(dst, entry.Name()), path.Join(resourcePath, entry.Name()))
			if err != nil {
				return fmt.Errorf("failed to write file: %s", err)
			}
		}
	}
	return nil
}

func WriteFile(dst, resourcePath string) error {
	resourceBytes, err := Resources.ReadFile(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to read file from embedded filesystem: %s", err)
	}
	return os.WriteFile(dst, resourceBytes, common.FilePermission)
}

func isResourceDir(resourcePath string) (isDir bool, err error) {
	resource, err := Resources.Open(resourcePath)
	defer resource.Close()
	if err != nil {
		err = fmt.Errorf("failed to open embedded resource: %s", err)
		return
	}
	resourceInfo, err := resource.Stat()
	if err != nil {
		err = fmt.Errorf("failed to stat embedded resource: %s", err)
		return
	}

	isDir = resourceInfo.IsDir()
	return
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
	// Name of the firestore document this config will upload to.
	// Defaults to Name if not set.
	FirestoreName string
	// Repository information.
	// If empty, defaults to
	// 	host: us-docker.pkg.dev
	// 	project: cros-registry/test-services
	//  firestoreHost: test-platform-store
	Repositories []*Repository
	// Defaults to Name, but can be separately set if
	// container name is different than the uprev name.
	ContainerName string
	// Name of the binary entrypoint.
	Entrypoint string
	// Binaries used during docker image setup.
	CIPDPackages []*CIPDPackage
	// Prepper is a function signature representing
	// any custom work needed by the Dockerfile.
	Prepper   func(ctx context.Context, dir string) error
	Resources []string
	// CloudRunConfig contains arguments for setting up a cloud
	// run instance. If not nil, will use default or set values
	// to push this config into cloud run.
	CloudRunConfig *cloudrun.Config
}

type Repository struct {
	Hostname      string
	Project       string
	FirestoreHost string
}

// GetConfigs returns the uprev configs.
func GetConfigs() []*UprevConfig {
	configs := []*UprevConfig{
		// {
		// 	Name: "example-filter",
		// 	CIPDPackages: []*CIPDPackage{
		// 		NewCIPDPackage("chromiumos/infra/ctpv2-filters/example-filter/${platform}"),
		// 	},
		// 	CloudRunConfig: &cloudrun.Config{},
		// },
		{
			Name: "partner-staging",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/partner-staging/${platform}"),
			},
			Repositories: []*Repository{
				PartnerRepository,
			},
			CloudRunConfig: &cloudrun.Config{},
		},
		{
			Name: "ash-chrome-provision-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/ash-chrome-provision-filter/${platform}"),
			},
			CloudRunConfig: &cloudrun.Config{},
		},
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
			CloudRunConfig: &cloudrun.Config{},
		},
		{
			Name: "firmware-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/firmware-filter/${platform}"),
			},
			CloudRunConfig: &cloudrun.Config{},
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
			CloudRunConfig: &cloudrun.Config{},
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
			CloudRunConfig: &cloudrun.Config{},
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
			CloudRunConfig: &cloudrun.Config{},
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
			CloudRunConfig: &cloudrun.Config{},
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
			CloudRunConfig: &cloudrun.Config{},
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
			CloudRunConfig: &cloudrun.Config{},
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
			Name: "cpcon-publish",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/publish/cpcon-publish/${platform}"),
			},
			Resources: []string{
				"cpcon_requirements_py3.txt",
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
			Name: "android-provision",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/provision/android-provision/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "ash-chrome-provision",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/provision/ash-chrome-provision/${platform}"),
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
				// CLEAN(b/408454320): Remove once adb-logcat is containerized.
				NewCIPDPackage("chromiumos/infra/cft/provision/adb-logcat/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "vm-provision",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/provision/vm-provision/${platform}"),
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
			CloudRunConfig: &cloudrun.Config{},
		},
		{
			Name: "pretest-container-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/pretest-container-filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
			CloudRunConfig: &cloudrun.Config{},
		},
		{
			Name:          "test-finder",
			ContainerName: "cros-test-finder",
			Entrypoint:    "test-finder",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/test-finder/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
			CloudRunConfig: &cloudrun.Config{
				CPU:    "8",
				Memory: "32Gi",
			},
		},
		{
			Name: "foil-test",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/execution/cros-test/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
			},
			Resources: []string{
				"tradefed_runner.sh",
				"wellknown_ssh_key",
			},
			Prepper: preppers.PrepFoilTestInternal,
		},
		{
			Name:          "foil-test-aosp",
			FirestoreName: "foil-test",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/execution/cros-test/${platform}"),
			},
			Repositories: []*Repository{
				PartnerRepository,
			},
			Prepper: preppers.PrepFoilTestAosp,
		},
		{
			Name:       "cros-ddd-filter",
			Entrypoint: "cros-ddd-filter",
			Resources: []string{
				"cros-ddd",
			},
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/cros-ddd-filter/${platform}"),
			},
			Prepper: preppers.PrepareCrosDDD,
			CloudRunConfig: &cloudrun.Config{
				CPU:    "8",
				Memory: "32Gi",
			},
		},
		{
			Name: "bols_satlab",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/bols_satlab/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "lsnexus",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/lsnexus/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "lsnexus-filter",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/ctpv2-filters/lsnexus-filter/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
			CloudRunConfig: &cloudrun.Config{},
		},
		{
			Name: "cros-passport",
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
			},
		},
		{
			Name: "post-process",
			CIPDPackages: []*CIPDPackage{
				NewCIPDPackage("chromiumos/infra/cft/post-test/post-process/${platform}"),
			},
			Repositories: []*Repository{
				DefaultRepository,
				PartnerRepository,
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

		if config.FirestoreName == "" {
			config.FirestoreName = config.Name
		}

		if len(config.Repositories) == 0 {
			config.Repositories = []*Repository{DefaultRepository}
		}

		if config.Entrypoint == "" {
			config.Entrypoint = config.ContainerName
		}
	}

	return configs
}
