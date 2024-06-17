// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"os"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/common_lib/common"
)

// UprevContainer performs the logic for upreving a container.
// This involves creating a temporary directory for the container to
// modify by writing its Dockerfile and ensuring its CIPD packages.
func UprevContainer(ctx context.Context, config *UprevConfig, cipdLabel, imageTag string) (sha string, err error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("Uprev %s", config.Name))
	defer func() { step.End(err) }()

	dir, err := os.MkdirTemp("", fmt.Sprintf("uprev_%s", config.Name))
	if err != nil {
		err = errors.Annotate(err, "failed to create temporary directory for %s", config.Name).Err()
		return
	}
	defer func() {
		removeErr := os.RemoveAll(dir)
		if removeErr != nil {
			logging.Infof(ctx, "failed to remove all from %s, %w", dir, removeErr)
		}
	}()

	if err = WriteDockerfile(dir, config.Name); err != nil {
		err = errors.Annotate(err, "failed to write Dockerfile").Err()
		return
	}

	for _, resource := range config.Resources {
		if err = WriteResource(dir, resource); err != nil {
			err = errors.Annotate(err, "failed to write %s", resource).Err()
			return
		}
	}

	if err = ensureCipdPackages(ctx, dir, config, cipdLabel); err != nil {
		err = errors.Annotate(err, "failed to ensure CIPD packages").Err()
		return
	}

	if config.Prepper != nil {
		if err = config.Prepper(ctx, dir); err != nil {
			err = errors.Annotate(err, "failed to populate directory").Err()
			return
		}
	}

	host := config.RepositoryHostname
	project := config.RepositoryProject
	if host == "" {
		host = common.DefaultDockerHost
	}
	if project == "" {
		project = common.DefaultDockerProject
	}
	if sha, err = buildAndPush(ctx, dir, host, project, config.Name, imageTag); err != nil {
		err = errors.Annotate(err, "failed to build and push image").Err()
		return
	}

	return
}

// ensureCipdPackages pulls down each cipd package defined by
// the config and stores them within the directory.
func ensureCipdPackages(ctx context.Context, dir string, config *UprevConfig, cipdLabel string) (err error) {
	step, ctx := build.StartStep(ctx, "Ensure CIPD Packages")
	defer func() { step.End(err) }()

	cipdHost := chromeinfra.CIPDServiceURL
	authOpts := chromeinfra.DefaultAuthOptions()
	cipdClient, err := common.CreateCIPDClient(ctx, authOpts, cipdHost, dir)
	if err != nil {
		err = errors.Annotate(err, "failed to create CIPD client").Err()
		return
	}

	for _, cipdPackage := range config.CIPDPackages {
		label := cipdLabel
		if cipdPackage.Ref != "" {
			label = cipdPackage.Ref
		}
		err = ensureCipdPackage(ctx, cipdClient, authOpts, cipdHost, cipdPackage.Name, label)
		if err != nil {
			err = errors.Annotate(err, "Failed to ensure %s", cipdPackage).Err()
			return
		}
	}

	return
}

// ensureCipdPackage calls EnsureCIPDPackage.
func ensureCipdPackage(
	ctx context.Context,
	cipdClient cipd.Client,
	authOpts auth.Options,
	cipdHost string,
	cipdPackage string,
	cipdLabel string) (err error) {

	step, ctx := build.StartStep(ctx, fmt.Sprintf("Ensure %s", cipdPackage))
	defer func() { step.End(err) }()

	_, err = common.EnsureCIPDPackage(
		ctx,
		cipdClient,
		authOpts,
		cipdHost,
		cipdPackage,
		cipdLabel,
		"")

	return
}
