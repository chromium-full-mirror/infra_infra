// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"net/http"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipd/client/cipd/template"
)

// CreateAuthClient creates new auth client.
func CreateAuthClient(
	ctx context.Context,
	authOpts auth.Options) (*http.Client, error) {

	return auth.NewAuthenticator(ctx, auth.OptionalLogin, authOpts).Client()
}

// CreateCIPDClient creates new cipd client.
func CreateCIPDClient(
	ctx context.Context,
	authOpts auth.Options,
	host string,
	root string) (cipd.Client, error) {

	authClient, err := CreateAuthClient(ctx, authOpts)
	if err != nil {
		return nil, err
	}
	cipdOpts := cipd.ClientOptions{
		ServiceURL:          host,
		Root:                root,
		AuthenticatedClient: authClient,
	}
	return cipd.NewClient(cipdOpts)
}

// EnsureCIPDPackage ensures the provided cipd package.
func EnsureCIPDPackage(
	ctx context.Context,
	client cipd.Client,
	authOpts auth.Options,
	host string,
	packageTemplate string,
	version string,
	subdir string) (*cipd.Actions, error) {

	actions := &cipd.Actions{}
	packageDef := ensure.PackageDef{
		PackageTemplate:   packageTemplate,
		UnresolvedVersion: version,
	}
	packageSlice := []ensure.PackageDef{packageDef}
	ensureFile := ensure.File{
		ServiceURL:       host,
		ParanoidMode:     cipd.CheckPresence,
		PackagesBySubdir: map[string]ensure.PackageSlice{subdir: packageSlice},
	}
	resolver := cipd.Resolver{Client: client}
	resolved, err := resolver.Resolve(
		ctx,
		&ensureFile,
		template.DefaultExpander())
	if err != nil {
		return actions, err
	}
	actionMap, err := client.EnsurePackages(
		ctx,
		resolved.PackagesBySubdir,
		&cipd.EnsureOptions{
			Paranoia: resolved.ParanoidMode,
			DryRun:   false,
		})
	return actionMap[subdir], err
}

func CIPDPackageDef(packageTemplate, version string) ensure.PackageDef {
	return ensure.PackageDef{
		PackageTemplate:   packageTemplate,
		UnresolvedVersion: version,
	}
}

func EnsureCIPDPackages(
	ctx context.Context,
	client cipd.Client,
	authOpts auth.Options,
	host string,
	subdir string,
	cipdPackages ...ensure.PackageDef) (*cipd.Actions, error) {

	actions := &cipd.Actions{}
	packageSlice := cipdPackages
	ensureFile := ensure.File{
		ServiceURL:       host,
		ParanoidMode:     cipd.CheckPresence,
		PackagesBySubdir: map[string]ensure.PackageSlice{subdir: packageSlice},
	}
	resolver := cipd.Resolver{Client: client}
	resolved, err := resolver.Resolve(
		ctx,
		&ensureFile,
		template.DefaultExpander())
	if err != nil {
		return actions, err
	}
	actionMap, err := client.EnsurePackages(
		ctx,
		resolved.PackagesBySubdir,
		&cipd.EnsureOptions{
			Paranoia: resolved.ParanoidMode,
			DryRun:   false,
		})
	return actionMap[subdir], err
}
