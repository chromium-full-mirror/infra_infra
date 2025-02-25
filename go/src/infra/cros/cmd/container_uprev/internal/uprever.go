// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"os"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

// UprevContainer performs the logic for upreving a container.
// This involves creating a temporary directory for the container to
// modify by writing its Dockerfile and ensuring its CIPD packages.
func UprevContainer(ctx context.Context, imageCache map[string]any, config *UprevConfig, cipdLabel, imageTag string) (containerInfoItem *common.ContainerInfoItem, err error) {
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

	// Multiple filters may upload to the same container name.
	// Differentiate between prod and staging in this situation
	// by adding the unique config name as a suffix.
	if imageTag == common.LabelProd || imageTag == common.LabelStaging {
		imageTag = fmt.Sprintf("%s_%s", imageTag, config.Name)
	}
	// Will have exactly one repository after upstream mapping.
	repo := config.Repositories[0]
	if containerInfoItem, err = buildAndPush(ctx, imageCache, repo, dir, config.ContainerName, config.Entrypoint, imageTag); err != nil {
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

	cipdPackages := []ensure.PackageDef{}
	for _, cipdPackage := range config.CIPDPackages {
		label := cipdLabel
		if cipdPackage.Ref != "" {
			label = cipdPackage.Ref
		}
		cipdPackages = append(cipdPackages, common.CIPDPackageDef(cipdPackage.Name, label))
	}

	_, err = common.EnsureCIPDPackages(ctx, cipdClient, authOpts, cipdHost, "", cipdPackages...)
	if err != nil {
		err = errors.Annotate(err, "Failed to ensure cipd packages").Err()
		return
	}

	return
}
