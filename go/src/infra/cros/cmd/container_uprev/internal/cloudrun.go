// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"strings"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/iterator"

	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/cloudrun"
	"go.chromium.org/infra/cros/cmd/common_lib/common"
)

const (
	NameTemplate   = "projects/%s/locations/%s/services/%s"
	CloudRunRegion = "us-central1"
	ProjectPrefix  = "chromeos-test-platform-"
)

var (
	// Allow tagged revisions in staging to live for 5 days.
	StagingActiveExpirationThreshold = time.Hour * 24 * 5
	// Retired revisions in prod should be somewhat long living in case of need for revert.
	StaleRevisionExpirationProd = time.Hour * 24 * 14
	// Retired revisions in staging should be quickly removed to prevent too much build up.
	StaleRevisionExpirationStaging = time.Hour * 12
)

func DeployFilterToCloudRun(ctx context.Context, digest, tag string, config *UprevConfig) (err error) {
	step, ctx := build.StartStep(ctx, "Deploy to Cloud Run")
	defer func() { step.End(err) }()

	if len(config.Repositories) == 0 {
		return fmt.Errorf("repositories must have a value")
	}
	// Upstream should have set only a single repository.
	repo := config.Repositories[0]
	projectTarget := ""

	staleRevisionExpiration := StaleRevisionExpirationStaging
	if tag == common.LabelProd {
		projectTarget = common.LabelProd
		staleRevisionExpiration = StaleRevisionExpirationProd
	} else {
		projectTarget = common.LabelStaging
	}
	if repo.Project == common.PartnerDockerProject {
		projectTarget = common.LabelPartner
		if tag == common.LabelStaging {
			return nil
		}
	}

	target := &cloudrun.Target{
		Project: ProjectPrefix + projectTarget,
		Region:  CloudRunRegion,
		// Gcloud names cannot contain underscores.
		Name: strings.ReplaceAll(config.FirestoreName, "_", "-"),
	}

	container := &cloudrun.Container{
		// Gcloud names cannot contain underscores.
		Name:    strings.ReplaceAll(config.ContainerName, "_", "-"),
		Image:   fmt.Sprintf("%s/%s/%s@%s", repo.Hostname, repo.Project, config.ContainerName, digest),
		Command: config.Entrypoint,
		Args:    common.FilterArgs,
		Port:    common.FilterCloudRunPort,
		Config:  config.CloudRunConfig,
	}

	switch tag {
	case common.LabelProd, common.LabelStaging:
		// Do nothing.
	default:
		container.Tag = tag
	}

	err = cloudrun.DeployToCloudRun(ctx, target, container)
	if err != nil {
		return
	}

	err = RemoveStaleCloudRunRevisions(ctx, tag, target, config, staleRevisionExpiration)
	return
}

func RemoveStaleCloudRunRevisions(ctx context.Context, tag string, target *cloudrun.Target, config *UprevConfig, expirationThreshold time.Duration) (err error) {
	step, ctx := build.StartStep(ctx, "Remove stale revisions")
	defer func() { step.End(err) }()

	c, err := run.NewRevisionsClient(ctx)
	if err != nil {
		logging.Infof(ctx, "failed to start revisions client: %s", err)
		return err
	}
	defer c.Close()

	revisionsIter := c.ListRevisions(ctx, &runpb.ListRevisionsRequest{
		Parent: fmt.Sprintf(NameTemplate, target.Project, target.Region, target.Name),
	})
	revisionsToRemove := []string{}
	now := time.Now()
	for {
		resp, err := revisionsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		for _, condition := range resp.Conditions {
			switch condition.Type {
			case "Ready":
				switch condition.State {
				case runpb.Condition_CONDITION_FAILED:
					revisionsToRemove = append(revisionsToRemove, resp.Name)
				}
			case "Active":
				switch condition.State {
				case runpb.Condition_CONDITION_FAILED:
					if now.Sub(resp.CreateTime.AsTime()) >= expirationThreshold {
						revisionsToRemove = append(revisionsToRemove, resp.Name)
					}
				case runpb.Condition_CONDITION_SUCCEEDED:
					if tag != common.LabelStaging {
						continue
					}
					// Remove "Active" revisions from staging whose age would
					// imply that it is a tagged revision. Only allow tagged revisions
					// to live for a certain amount of time.
					if now.Sub(resp.CreateTime.AsTime()) >= StagingActiveExpirationThreshold {
						revisionsToRemove = append(revisionsToRemove, resp.Name)
					}
				}
			}
		}
	}
	for _, revision := range revisionsToRemove {
		logging.Infof(ctx, "Deleting %s", revision)
		deleteOp, err := c.DeleteRevision(ctx, &runpb.DeleteRevisionRequest{
			Name: revision,
		})
		if err != nil {
			logging.Infof(ctx, "failed to delete %s: %s", revision, err)
			continue
		}
		_, err = deleteOp.Wait(ctx)
		if err != nil {
			logging.Infof(ctx, "failed while waiting for deletion of %s: %s", revision, err)
			continue
		}
	}
	return nil
}
