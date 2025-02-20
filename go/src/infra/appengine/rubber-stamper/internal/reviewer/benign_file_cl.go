// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reviewer

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"go.chromium.org/luci/common/logging"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"

	"go.chromium.org/infra/appengine/rubber-stamper/config"
	"go.chromium.org/infra/appengine/rubber-stamper/internal/gerrit"
	"go.chromium.org/infra/appengine/rubber-stamper/tasks/taskspb"
)

// reviewBegignFileChange checks whether a CL follows the BenignFilePattern.
// It returns an array of strings and error, where the array provides the paths
// of those files which breaks the pattern. Iff the array is empty and error is
// nil, the CL is a benign CL.
func reviewBenignFileChange(ctx context.Context, hostCfg *config.HostConfig, gc gerrit.Client, t *taskspb.ChangeReviewTask) ([]string, error) {
	listReq := &gerritpb.ListFilesRequest{
		Number:     t.Number,
		RevisionId: t.Revision,
	}
	resp, err := gc.ListFiles(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("gerrit ListFiles rpc call failed with error: request %+v, error %v", listReq, err)
	}

	bfp := retrieveBenignFilePattern(ctx, hostCfg, t.Repo)
	if bfp == nil {
		logging.Debugf(ctx, "there's no BenignFilePattern config for host %s, cl %d, revision %s: %v", t.Host, t.Number, t.Revision)
		invalidFiles := make([]string, 0, len(resp.Files))
		for file := range resp.Files {
			if file == "/COMMIT_MSG" {
				continue
			}

			invalidFiles = append(invalidFiles, file)
		}
		return invalidFiles, nil
	}

	var patterns []gitignore.Pattern
	for _, path := range bfp.Paths {
		patterns = append(patterns, gitignore.ParsePattern(path, nil))
	}
	matcher := gitignore.NewMatcher(patterns)

	var invalidFiles []string
	for file := range resp.Files {
		if file == "/COMMIT_MSG" {
			continue
		}

		if !matcher.Match(splitPath(file), false) {
			invalidFiles = append(invalidFiles, file)
		}
	}

	sort.Strings(invalidFiles)
	return invalidFiles, nil
}

// splitPath splits a path into components, as weird go-git.v4 API wants it.
func splitPath(p string) []string {
	return strings.Split(filepath.Clean(p), string(filepath.Separator))
}

// retrieveBenignFilePattern retrieves the corresponding BenignFilePattern
// config for the given repository.
//
// Return the BenignFilePattern when there is one. Return nil when it doesn't
// exist.
func retrieveBenignFilePattern(ctx context.Context, hostCfg *config.HostConfig, repo string) *config.BenignFilePattern {
	if hostCfg == nil {
		return nil
	}
	if hostCfg.GetRepoConfigs()[repo] != nil {
		return hostCfg.RepoConfigs[repo].BenignFilePattern
	}
	if hostCfg.GetRepoRegexpConfigs() != nil {
		rrcfg := config.RetrieveRepoRegexpConfig(ctx, repo, hostCfg.GetRepoRegexpConfigs())
		if rrcfg != nil {
			return rrcfg.BenignFilePattern
		}
	}
	return nil
}
