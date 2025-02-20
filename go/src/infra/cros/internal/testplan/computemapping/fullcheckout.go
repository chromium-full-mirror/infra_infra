// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package computemapping

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/sync/parallel"

	"go.chromium.org/infra/cros/internal/git"
	"go.chromium.org/infra/cros/internal/repo"
	"go.chromium.org/infra/tools/dirmd"
	"go.chromium.org/infra/tools/dirmd/cli/updater"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

// ToDirBQRows computes a DirBQRow for every DIR_METADATA in manifest. All
// default projects in the manifest must be synced. The DirBQRow is computed
// based off the checked out state of each project. Mappings are computed in
// parallel.
//
// src/chromium projects are skipped because the imports between DIR_METADATAs
// (e.g. mixins) generally don't work with the layout ChromeOS manifests use.
func ToDirBQRows(ctx context.Context, chromiumosCheckout string, manifest *repo.Manifest) ([]*dirmdpb.DirBQRow, error) {
	rows := make([]*dirmdpb.DirBQRow, 0)
	mu := sync.Mutex{}

	// Use the current time as the partition time for all rows.
	partitionTime := timestamppb.New(clock.Now(ctx))

	// Create one task in the pool for each project in the manifest.
	if err := parallel.WorkPool(0, func(c chan<- func() error) {
		for _, project := range manifest.Projects {
			project := project
			if strings.Contains(project.Groups, "notdefault") || strings.HasPrefix(project.Path, "src/chromium") {
				logging.Warningf(ctx, "skipping project %q", project.Name)
				continue
			}

			fullpath := filepath.Join(chromiumosCheckout, project.Path)

			c <- func() error {
				mapping, err := dirmd.ReadMapping(ctx, dirmdpb.MappingForm_COMPUTED, true, fullpath)
				if err != nil {
					return fmt.Errorf("failed reading project %q: %w", project.Name, err)
				}

				// Use git to get the current checked out sha, this isn't
				// available in the manifest.
				revision, err := git.GetGitRepoRevision(fullpath, "HEAD")
				if err != nil {
					return fmt.Errorf("failed to parse HEAD for %q: %w", fullpath, err)
				}

				commit := &updater.GitCommit{
					Host:    manifest.GetRemoteByName(project.RemoteName).Fetch,
					Project: project.Name,
					// Ref should be the name of the ref, e.g.
					// "refs/heads/main". Note that sometimes a project pins
					// a specific sha in the manifest, we still report this
					// as the ref.
					Ref: project.Revision,
					// Revision is the specific checked-out sha.
					Revision: revision,
				}

				mu.Lock()
				defer mu.Unlock()

				for dir, metadata := range mapping.Dirs {
					row := updater.CommonDirBQRow(commit, metadata, partitionTime)
					row.Dir = dir

					rows = append(rows, row)
				}

				return nil
			}
		}
	}); err != nil {
		return nil, err
	}

	return rows, nil
}
