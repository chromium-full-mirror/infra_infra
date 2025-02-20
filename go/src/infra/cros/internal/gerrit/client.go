// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gerrit contains functions for interacting with gerrit/gitiles.
package gerrit

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	gerrs "errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/api/gitiles"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"

	"go.chromium.org/infra/cros/internal/shared"
)

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Time  string `json:"time"`
}

type Commit struct {
	Commit    string   `json:"commit"`
	Tree      string   `json:"tree"`
	Parents   []string `json:"parents"`
	Author    User     `json:"author"`
	Committer User     `json:"committer"`
	Message   string   `json:"message"`
}

type commitLog struct {
	Commits    []Commit `json:"log"`
	NextCommit string   `json:"next"`
}

type Change struct {
	// This does not include all fields
	Project        string `json:"project"`
	ChangeNumber   int    `json:"_change_number"`
	RevisionNumber int    `json:"_revision_number"`
}

type changeLog struct {
	Changes []Change `json:"changes"`
}

type Client interface {
	FetchFilesFromGitiles(ctx context.Context, host, project, ref string, paths []string, timeoutOpts shared.Options) (*map[string]string, error)
	DownloadFileFromGitiles(ctx context.Context, host, project, ref, path string, timeoutOpts shared.Options) (string, error)
	DownloadFileFromGitilesToPath(ctx context.Context, host, project, ref, path, saveToPath string, timeoutOpts shared.Options) error
	Branches(ctx context.Context, host, project string) (map[string]string, error)
	Projects(ctx context.Context, host string) ([]string, error)
	ListFiles(ctx context.Context, host, project, ref, path string) ([]string, error)
	GetFileLog(ctx context.Context, host, project, ref, filepath string) ([]Commit, error)
	QueryChanges(ctx context.Context, host string, query gerrit.ChangeQueryParams) ([]*gerrit.Change, error)
	GetRelatedChanges(ctx context.Context, host string, changeNumber int) ([]Change, error)
	SetReview(ctx context.Context, host, changeID string, review *gerrit.ReviewInput) (*gerrit.ReviewResult, error)
	SubmitChange(ctx context.Context, host, changeID string) error
}

// Client is a client for interacting with gerrit.
type ProdClient struct {
	isTestClient bool
	authedClient *http.Client
	// gitilesClient maps individual gerrit host to gitiles client.
	gitilesClient map[string]gitilespb.GitilesClient
	// gerritClient maps individual gerrit host to gerrit client.
	gerritClient map[string]*gerrit.Client
}

// NewClient returns a new Client object.
func NewClient(authedClient *http.Client) (*ProdClient, error) {
	return &ProdClient{
		isTestClient:  false,
		authedClient:  authedClient,
		gitilesClient: map[string]gitilespb.GitilesClient{},
		gerritClient:  map[string]*gerrit.Client{},
	}, nil
}

// getGitilesClientForHost retrieves the inner gitilespb.GitilesClient for the specific
// host if it exists and creates a new one if it does not.
func (c *ProdClient) getGitilesClientForHost(host string) (gitilespb.GitilesClient, error) {
	if client, ok := c.gitilesClient[host]; ok {
		return client, nil
	}
	if c.isTestClient {
		return nil, fmt.Errorf("test clients must have all inner clients set at initialization.")
	}
	var err error
	c.gitilesClient[host], err = gitiles.NewRESTClient(c.authedClient, host, true)
	if err != nil {
		return nil, err
	}
	return c.gitilesClient[host], err
}

// getGerritClientForHost retrieves the inner *gerrit.Client for the specific
// host if it exists and creates a new one if it does not.
func (c *ProdClient) getGerritClientForHost(host string) (*gerrit.Client, error) {
	if client, ok := c.gerritClient[host]; ok {
		return client, nil
	}
	if c.isTestClient {
		return nil, fmt.Errorf("test clients must have all inner clients set at initialization.")
	}
	var err error
	c.gerritClient[host], err = gerrit.NewClient(c.authedClient, host)
	if err != nil {
		return nil, err
	}
	return c.gerritClient[host], err
}

// NewTestClient returns a new Client that uses the provided client objects.
func NewTestClient(gitilesClients map[string]gitilespb.GitilesClient) *ProdClient {
	return &ProdClient{
		isTestClient:  true,
		gitilesClient: gitilesClients,
	}
}

// FetchFilesFromGitiles fetches file contents from gitiles.
//
// project is the git project to fetch from.
// ref is the git-ref to fetch from.
// paths lists the paths inside the git project to fetch contents for.
//
// fetchFilesFromGitiles returns a map from path in the git project to the
// contents of the file at that path for each requested path.
//
// If one of paths is not found, an error is returned.
func (c *ProdClient) FetchFilesFromGitiles(ctx context.Context, host, project, ref string, paths []string, timeoutOpts shared.Options) (*map[string]string, error) {
	gc, err := c.getGitilesClientForHost(host)
	if err != nil {
		return nil, err
	}
	contents, err := obtainGitilesBytes(ctx, gc, project, ref, timeoutOpts)
	if err != nil {
		return nil, err
	}
	return extractGitilesArchive(ctx, contents, paths)
}

// DownloadFileFromGitiles downloads a file from Gitiles.
func (c *ProdClient) DownloadFileFromGitiles(ctx context.Context, host, project, ref, path string, timeoutOpts shared.Options) (string, error) {
	gc, err := c.getGitilesClientForHost(host)
	if err != nil {
		return "", err
	}

	ch := make(chan string, 1)
	err = shared.DoWithRetry(ctx, timeoutOpts, func() error {
		// This sets the deadline for the individual API call, while the outer context sets
		// an overall timeout for all attempts.
		innerCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		req := &gitilespb.DownloadFileRequest{
			Project:    project,
			Path:       path,
			Committish: ref,
		}
		contents, err := gc.DownloadFile(innerCtx, req)
		if err != nil {
			return errors.Annotate(err, "obtain gitiles download").Err()
		}
		ch <- contents.Contents
		return nil
	})
	if err != nil {
		return "", err
	}
	a := <-ch
	return a, nil
}

// DownloadFileFromGitilesToPath downloads a file from Gitiles to a specified path.
func (c *ProdClient) DownloadFileFromGitilesToPath(ctx context.Context, host, project, ref, path, saveToPath string, timeoutOpts shared.Options) error {
	contents, err := c.DownloadFileFromGitiles(ctx, host, project, ref, path, timeoutOpts)
	if err != nil {
		return err
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

func obtainGitilesBytes(ctx context.Context, gc gitilespb.GitilesClient, project string, ref string, timeoutOpts shared.Options) ([]byte, error) {
	ch := make(chan *gitilespb.ArchiveResponse, 1)
	err := shared.DoWithRetry(ctx, timeoutOpts, func() error {
		// This sets the deadline for the individual API call, while the outer context sets
		// an overall timeout for all attempts.
		innerCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		req := &gitilespb.ArchiveRequest{
			Project: project,
			Ref:     ref,
			Format:  gitilespb.ArchiveRequest_GZIP,
		}
		a, err := gc.Archive(innerCtx, req)
		if err != nil {
			return errors.Annotate(err, "obtain gitiles archive").Err()
		}
		ch <- a
		return nil
	})
	if err != nil {
		return nil, err
	}
	a := <-ch
	return a.Contents, nil
}

// extractGitilesArchive extracts file at each path in paths from the given
// gunzipped tarfile.
//
// extractGitilesArchive returns a map from path to the content of the file at
// that path in the archives for each requested path found in the archive.
//
// If one of paths is not found, an error is returned.
//
// This function takes ownership of data. Caller should not use the byte array
// concurrent to / after this call. See io.Reader interface for more details.
func extractGitilesArchive(ctx context.Context, data []byte, paths []string) (*map[string]string, error) {
	// pmap maps files to the requested filename.
	// e.g. if "foo" is a symlink to "bar", then the entry "bar":"foo" exists.
	// if a file is not a symlink, it will be mapped to itself.
	pmap := make(map[string]string)
	for _, p := range paths {
		pmap[p] = p
	}

	res := make(map[string]string)
	foundPaths := make(map[string]bool)
	// Do two passes to resolve links.
	for i := 0; i < 2; i++ {
		abuf := bytes.NewBuffer(data)
		gr, err := gzip.NewReader(abuf)
		if err != nil {
			return nil, errors.Annotate(err, "extract gitiles archive").Err()
		}
		defer gr.Close()

		tr := tar.NewReader(gr)
		for {
			h, err := tr.Next()
			eof := false
			switch {
			case err == io.EOF:
				// Scanned all files.
				eof = true
			case err != nil:
				return nil, errors.Annotate(err, "extract gitiles archive").Err()
			default:
				// good case.
			}
			if eof {
				break
			}
			requestedFile, found := pmap[h.Name]
			if !found {
				// not a requested file.
				continue
			}
			if _, ok := res[requestedFile]; ok {
				// already read this file.
				continue
			}
			if h.Typeflag == tar.TypeSymlink {
				if i == 0 {
					// if symlink, mark link in pmap so it gets picked up on the second pass.
					linkPath := path.Join(path.Dir(h.Name), h.Linkname)
					pmap[linkPath] = h.Name
				}
				continue
			}

			logging.Debugf(ctx, "Inventory data file %s size %d", h.Name, h.Size)
			data := make([]byte, h.Size)
			if _, err := io.ReadFull(tr, data); err != nil {
				return nil, errors.Annotate(err, "extract gitiles archive").Err()
			}
			res[requestedFile] = string(data)
			foundPaths[requestedFile] = true
		}
	}

	for _, path := range paths {
		if _, found := foundPaths[path]; !found {
			return nil, fmt.Errorf("path %q not found", path)
		}
	}
	return &res, nil
}

// Branches returns a map of branches (to revisions) for a given repo.
func (c *ProdClient) Branches(ctx context.Context, host, project string) (map[string]string, error) {
	gc, err := c.getGitilesClientForHost(host)
	if err != nil {
		return nil, err
	}
	req := &gitilespb.RefsRequest{
		Project:  project,
		RefsPath: "refs/heads",
	}
	refs, err := gc.Refs(ctx, req)
	if err != nil {
		return nil, err
	}
	return refs.Revisions, err
}

// Projects returns a list of projects for a given host.
func (c *ProdClient) Projects(ctx context.Context, host string) ([]string, error) {
	gc, err := c.getGitilesClientForHost(host)
	if err != nil {
		return nil, err
	}
	req := &gitilespb.ProjectsRequest{}
	resp, err := gc.Projects(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetProjects(), err
}

// ListFiles returns a list of files/directories for a given host/project/ref/path.
func (c *ProdClient) ListFiles(ctx context.Context, host, project, ref, path string) ([]string, error) {
	gc, err := c.getGitilesClientForHost(host)
	if err != nil {
		return nil, err
	}
	req := &gitilespb.ListFilesRequest{
		Project:    project,
		Committish: ref,
		Path:       path,
	}
	resp, err := gc.ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}
	files := resp.GetFiles()
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.GetPath()
	}
	return names, err
}

// GetFileLog returns a list of commits that touch the specified file.
// Times are in UTC.
func (c *ProdClient) GetFileLog(ctx context.Context, host, project, ref, filepath string) ([]Commit, error) {
	url := fmt.Sprintf("%s/+log/%s/%s?format=JSON", path.Join(host, project), ref, filepath)
	if c.isTestClient {
		url = "http://" + url
	} else {
		url = "https://" + url
	}
	res, err := c.authedClient.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	// The REST API sometimes prepends )]}' to the response body.
	// Trim this.
	body = []byte(strings.TrimPrefix(string(body), ")]}'"))
	var log commitLog
	if err := json.Unmarshal(body, &log); err != nil {
		return nil, err
	}
	return log.Commits, nil
}

// QueryChanges queries a gerrit host for changes matching the supplied query.
func (c *ProdClient) QueryChanges(ctx context.Context, host string, query gerrit.ChangeQueryParams) ([]*gerrit.Change, error) {
	if c.isTestClient {
		// Test data for TestGetRelatedChanges in client_test.go.
		if query.Query == "change:4279218" {
			return []*gerrit.Change{{
				ID:              "chromium%2fsrc~main~Ia201b3605faefcc65cfbded4cf933f5d8f00d661",
				CurrentRevision: "1c0bdaf5d67a03046f78c8ffef6b697ff3458c6e",
			}}, nil
		}
	}
	gc, err := c.getGerritClientForHost(host)
	if err != nil {
		return nil, err
	}
	changes, _, err := gc.ChangeQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	return changes, nil
}

// GetRelatedChanges queries a gerrit host for related changes.
// It returns a list of Changes describing the related changes.
// Sorted by git commit order, newest to oldest. Empty if there are no related
// changes. Requires OAuthScope. See https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-related-changes.
func (c *ProdClient) GetRelatedChanges(ctx context.Context, host string, changeNumber int) ([]Change, error) {
	if !strings.HasPrefix(host, "http") {
		if c.isTestClient {
			host = "http://" + host
		} else {
			host = "https://" + host
		}
	}
	opt := gerrit.ChangeQueryParams{}
	opt.Query = fmt.Sprintf("change:%d", changeNumber)
	opt.Options = []string{"CURRENT_REVISION"}
	changes, err := c.QueryChanges(ctx, host, opt)
	if err != nil {
		return nil, err
	}
	var log changeLog
	for _, change := range changes {
		changeID := change.ID
		revision := change.CurrentRevision
		url := fmt.Sprintf("%s/a/changes/%s/revisions/%s/related", host, changeID, revision)
		res, err := c.authedClient.Get(url)
		if err != nil {
			return nil, err
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		body = []byte(strings.TrimPrefix(string(body), ")]}'"))
		if err := json.Unmarshal(body, &log); err != nil {
			return nil, err
		}
		// There should only be one change per changeNumber passed to c.QueryChanges.
		if err == nil {
			break
		}
	}
	return log.Changes, nil
}

// SetReview applies labels/performs other review operations on the specified CL.
func (c *ProdClient) SetReview(ctx context.Context, host, changeID string, review *gerrit.ReviewInput) (*gerrit.ReviewResult, error) {
	gc, err := c.getGerritClientForHost(host)
	if err != nil {
		return nil, err
	}

	// "current" selects the most recent patchset.
	return gc.SetReview(ctx, changeID, "current", review)
}

// SubmitChange submits the specified CL.
func (c *ProdClient) SubmitChange(ctx context.Context, host, changeID string) error {
	gc, err := c.getGerritClientForHost(host)
	if err != nil {
		return nil
	}

	_, err = gc.Submit(ctx, changeID, &gerrit.SubmitInput{})
	return err
}
