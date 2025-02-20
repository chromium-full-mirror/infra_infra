// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gerrit contains functions for interacting with gerrit/gitiles.
package gerrit

import (
	"context"
	gerrs "errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"go.chromium.org/luci/common/api/gerrit"

	"go.chromium.org/infra/cros/internal/shared"
)

type ExpectedFetch struct {
	Host    string
	Project string
	Ref     string
}

type ExpectedPathParams struct {
	Host    string
	Project string
	Ref     string
	Path    string
}

type MockClient struct {
	T               *testing.T
	ExpectedFetches map[ExpectedFetch]map[string]string
	// If the string pointer is nil, will raise a "file does not exist" err.
	ExpectedDownloads map[ExpectedPathParams]*string
	// Indexed by host and then by project.
	ExpectedBranches map[string]map[string]map[string]string
	ExpectedProjects map[string][]string
	ExpectedLists    map[ExpectedPathParams][]string
	ExpectedFileLogs map[ExpectedPathParams][]Commit
	// Maps query string to query results.
	// If an entry is set for "*", that result will be returned for all queries.
	ExpectedQuery map[string][]*gerrit.Change
	// Indexed by change ID.
	ExpectedReview map[string]*gerrit.ReviewInput
	// Indexed by change ID.
	ExpectedSubmit map[string]bool
	// Maps host and changeNumber to list of changes
	ExpectedRelatedChanges map[string]map[int][]Change
}

func contains(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}

// FetchFilesFromGitiles fetches file contents from gitiles.
func (c *MockClient) FetchFilesFromGitiles(ctx context.Context, host, project, ref string, paths []string, timeoutOpts shared.Options) (*map[string]string, error) {
	expectedFetch := ExpectedFetch{
		Host:    host,
		Project: project,
		Ref:     ref,
	}
	contents, ok := c.ExpectedFetches[expectedFetch]
	if !ok {
		c.T.Fatalf("unexpected FetchFilesFromGitiles for %+v", expectedFetch)
	}
	// Only return the contents for the requested paths.
	for path := range contents {
		if !contains(paths, path) {
			delete(contents, path)
		}
	}
	return &contents, nil
}

func (c *MockClient) downloadFileFromGitiles(ctx context.Context, host, project, ref, path, fnName string, timeoutOpts shared.Options) (string, error) {
	expectedDownload := ExpectedPathParams{
		Host:    host,
		Project: project,
		Ref:     ref,
		Path:    path,
	}
	contents, ok := c.ExpectedDownloads[expectedDownload]
	if !ok {
		c.T.Fatalf("unexpected %s for %+v", fnName, expectedDownload)
	}
	if contents == nil {
		return "", fmt.Errorf("file does not exist")
	}
	return *contents, nil
}

// DownloadFileFromGitiles downloads a file from Gitiles.
func (c *MockClient) DownloadFileFromGitiles(ctx context.Context, host, project, ref, path string, timeoutOpts shared.Options) (string, error) {
	return c.downloadFileFromGitiles(ctx, host, project, ref, path, "DownloadFileFromGitiles", timeoutOpts)
}

// DownloadFileFromGitilesToPath downloads a file from Gitiles to a specified path.
func (c *MockClient) DownloadFileFromGitilesToPath(ctx context.Context, host, project, ref, path, saveToPath string, timeoutOpts shared.Options) error {
	contents, err := c.downloadFileFromGitiles(ctx, host, project, ref, path, "DownloadFileFromGitilesToPath", timeoutOpts)
	if err != nil {
		return nil
	}

	// Use existing file mode if the file already exists.
	fileMode := os.FileMode(int(0644))
	if fileData, err := os.Stat(saveToPath); err != nil && !gerrs.Is(err, os.ErrNotExist) {
		return err
	} else if fileData != nil {
		fileMode = fileData.Mode()
	}

	return os.WriteFile(saveToPath, []byte(contents), fileMode)
}

// Branches returns a map of branches (to revisions) for a given repo.
func (c *MockClient) Branches(ctx context.Context, host, project string) (map[string]string, error) {
	if hostinfo, ok := c.ExpectedBranches[host]; !ok {
		c.T.Fatalf("unexpected Branches for host %s", host)
	} else if branches, ok := hostinfo[project]; !ok {
		c.T.Fatalf("unexpected Branches for host %s, project %s", host, project)
	} else {
		return branches, nil
	}
	return nil, nil
}

// Projects returns a list of projects for a given host.
func (c *MockClient) Projects(ctx context.Context, host string) ([]string, error) {
	projects, ok := c.ExpectedProjects[host]
	if !ok {
		c.T.Fatalf("unexpected Projects for host %s", host)
	}
	return projects, nil
}

// ListFiles returns a list of files/directories for a given host/project/ref/path.
func (c *MockClient) ListFiles(ctx context.Context, host, project, ref, path string) ([]string, error) {
	expectedList := ExpectedPathParams{
		Host:    host,
		Project: project,
		Ref:     ref,
		Path:    path,
	}
	files, ok := c.ExpectedLists[expectedList]
	if !ok {
		c.T.Fatalf("unexpected ListFiles for %+v", expectedList)
	}
	return files, nil
}

// GetFileLog returns a list of commits that touch the specified file.
func (c *MockClient) GetFileLog(ctx context.Context, host, project, ref, filepath string) ([]Commit, error) {
	expectedLog := ExpectedPathParams{
		Host:    host,
		Project: project,
		Ref:     ref,
		Path:    filepath,
	}
	commits, ok := c.ExpectedFileLogs[expectedLog]
	if !ok {
		c.T.Fatalf("unexpected GetFileLog for %+v", expectedLog)
	}
	return commits, nil
}

// QueryChanges queries a gerrit host for changes matching the supplied query.
func (c *MockClient) QueryChanges(ctx context.Context, host string, query gerrit.ChangeQueryParams) ([]*gerrit.Change, error) {
	anyQuery, ok := c.ExpectedQuery["*"]
	if ok {
		return anyQuery, nil
	}
	changes, ok := c.ExpectedQuery[query.Query]
	if !ok {
		c.T.Fatalf("unexpected QueryChanges for %+v", query.Query)
	}
	return changes, nil
}

// GetRelatedChanges queries a gerrit host for changes related to the supplied change number
func (c *MockClient) GetRelatedChanges(ctx context.Context, host string, changeNumber int) ([]Change, error) {
	expectedChanges, ok := c.ExpectedRelatedChanges[host]
	if !ok {
		return []Change{}, fmt.Errorf("unexpected GetRelatedChange for host %s", host)
	}
	relatedChanges, ok := expectedChanges[changeNumber]
	if !ok {
		return []Change{}, fmt.Errorf("unexpected GetRelatedChange for change # %d and host %s", changeNumber, host)
	}
	return relatedChanges, nil
}

// SetReview applies labels/performs other review operations on the specified CL.
func (c *MockClient) SetReview(ctx context.Context, host, changeID string, review *gerrit.ReviewInput) (*gerrit.ReviewResult, error) {
	expectedReview, ok := c.ExpectedReview[changeID]
	if !ok {
		c.T.Fatalf("unexpected SetReview for %s", changeID)
	}
	if !reflect.DeepEqual(*expectedReview, *review) {
		c.T.Fatalf("mismatch on SetReview for change %s: expected\n%+v\ngot\n%+v", changeID, *expectedReview, *review)
	}
	return nil, nil
}

// SubmitChange submits the specified CL.
func (c *MockClient) SubmitChange(ctx context.Context, host, changeID string) error {
	submit, ok := c.ExpectedSubmit[changeID]
	if !ok || !submit {
		c.T.Fatalf("unexpected SubmitChange for %s", changeID)
	}
	return nil
}
