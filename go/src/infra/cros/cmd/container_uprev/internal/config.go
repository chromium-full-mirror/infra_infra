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
)

var (
	//go:embed dockerfiles/*
	Dockerfiles embed.FS
)

// WriteDockerfile writes the embedded dockerfile to the temporary directory.
func WriteDockerfile(dir string, name string) error {
	dockerfile, err := Dockerfiles.ReadFile(fmt.Sprintf("dockerfiles/Dockerfile_%s", name))
	if err != nil {
		return errors.Annotate(err, "failed to read Dockerfile_%s", name).Err()
	}
	return os.WriteFile(path.Join(dir, name), dockerfile, 0644)
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
	// Binaries used during docker image setup.
	CIPDPackages []*CIPDPackage
	// PopulateDirectory is a function signature representing
	// any custom work needed by the Dockerfile.
	PopulateDirectory func(ctx context.Context, dir string) error
}

// GetConfigs returns the uprev configs.
func GetConfigs() []*UprevConfig {
	return []*UprevConfig{}
}
