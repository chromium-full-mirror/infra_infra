// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gitiles

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/proto/git"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/common/testing/testfs"

	"go.chromium.org/infra/chromium/bootstrapper/clients/gitiles"
	"go.chromium.org/infra/chromium/util"
)

type PathObject struct {
	contents          string
	submoduleRevision string
}

func File(contents string) *PathObject {
	return &PathObject{contents: contents}
}

func Submodule(revision string) *PathObject {
	return &PathObject{submoduleRevision: revision}
}

type Revision struct {
	// Parent is the commit ID of the parent revision.
	Parent string

	// Files maps file paths to their new contents at the revision.
	//
	// Missing keys will have the same contents as in the parent revision,
	// if there is one, otherwise the file does not exist at the revision. A
	// nil value indicates the file does not exist at the revision.
	Files map[string]*PathObject
}

// Project is the fake data for a gitiles project.
type Project struct {
	// Refs maps refs to their revision.
	//
	// Missing keys will have a default revision computed. An empty string
	// value indicates that the ref does not exist.
	Refs map[string]string

	// Revisions maps commit IDs to the revision of the repo.
	//
	// Missing keys will have a default fake revision. A nil value indicates
	// no revision with the commit ID exists.
	Revisions map[string]*Revision
}

// Host is the fake data for a gitiles host.
type Host struct {
	// Projects maps project names to their details.
	//
	// Missing keys will have a default fake project. A nil value indicates
	// the the project does not exist.
	Projects map[string]*Project
}

// Client is the client that will serve fake data for a given host.
type Client struct {
	hostname string
	gitiles  *Host
}

// Factory creates a factory that returns RPC clients that use fake data to
// respond to requests.
//
// The fake data is taken from the fakes argument, which is a map from host
// names to the Host instances containing the fake data for the host. Missing
// keys will have a default Host. A nil value indicates that the given host is
// not a gitiles instance.
func Factory(fakes map[string]*Host) gitiles.GitilesClientFactory {
	return func(ctx context.Context, host string) (gitiles.GitilesClient, error) {
		fake, ok := fakes[host]
		if !ok {
			fake = &Host{}
		} else if fake == nil {
			return nil, errors.Reason("%s is not a gitiles host", host).Err()
		}
		return &Client{host, fake}, nil
	}
}

func (c *Client) getProject(projectName string) (*Project, error) {
	project, ok := c.gitiles.Projects[projectName]
	if !ok {
		return &Project{}, nil
	} else if project == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("unknown project %#v on host %#v", projectName, c.hostname))
	}
	return project, nil
}

func (c *Client) Log(ctx context.Context, request *gitilespb.LogRequest, options ...grpc.CallOption) (*gitilespb.LogResponse, error) {
	project, err := c.getProject(request.Project)
	if err != nil {
		return nil, err
	}
	commitId, ok := project.Refs[request.Committish]
	if !ok {
		if _, ok := project.Revisions[request.Committish]; ok {
			commitId = request.Committish
		} else {
			commitId = fmt.Sprintf("fake-revision|%s|%s|%s", c.hostname, request.Project, request.Committish)
		}
	} else if commitId == "" {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("unknown ref %#v for project %#v on host %#v", request.Committish, request.Project, c.hostname))
	}

	history, err := c.getRevisionHistory(request.Project, commitId)
	if err != nil {
		return nil, err
	}

	var matchPath func(*Revision) bool
	if request.Path == "" {
		matchPath = func(r *Revision) bool { return true }
	} else {
		regex := regexp.MustCompile(fmt.Sprintf("%s(/.+)?$", regexp.QuoteMeta(request.Path)))
		matchPath = func(r *Revision) bool {
			for path := range r.Files {
				if regex.MatchString(path) {
					return true
				}
			}
			return false
		}
	}

	log := make([]*git.Commit, 0, request.PageSize)
	for _, commit := range history {
		if matchPath(commit.revision) {
			log = append(log, &git.Commit{Id: commit.id})
			if len(log) == cap(log) {
				break
			}
		}
	}

	if request.Path == "" {
		// The fake repo is arbitrarily large, but a commit will only touch specific files
		// if so configured, so fill up the requested page size if not looking at a specific
		// path
		remaining := int(request.PageSize) - len(log)
		for i := 1; i <= remaining; i++ {
			log = append(log, &git.Commit{Id: fmt.Sprintf("%s~%d", commitId, i)})
		}

	} else if len(log) == 0 {
		// If a specific path is requested and the path doesn't exist at the revision, then
		// it is a 404
		return nil, status.Error(codes.NotFound, fmt.Sprintf("path %#v does not exist at %#v of project %#v on host %#v", request.Path, request.Committish, request.Project, c.hostname))
	}

	return &gitilespb.LogResponse{
		Log: log,
	}, nil
}

type commit struct {
	id       string
	revision *Revision
}

func (c *Client) getRevisionHistory(projectName, commitId string) ([]*commit, error) {
	project, err := c.getProject(projectName)
	if err != nil {
		return nil, err
	}
	var history []*commit
	for commitId != "" {
		revision, ok := project.Revisions[commitId]
		if !ok {
			revision = &Revision{}
		} else if revision == nil {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("unknown revision %#v of project %#v on host %#v", commitId, projectName, c.hostname))
		}
		history = append(history, &commit{commitId, revision})
		commitId = revision.Parent
	}
	return history, nil
}

func (c *Client) DownloadFile(ctx context.Context, request *gitilespb.DownloadFileRequest, options ...grpc.CallOption) (*gitilespb.DownloadFileResponse, error) {
	history, err := c.getRevisionHistory(request.Project, request.Committish)
	if err != nil {
		return nil, err
	}
	var pathObject *PathObject
	var ok bool
	for _, commit := range history {
		pathObject, ok = commit.revision.Files[request.Path]
		if ok {
			break
		}
	}
	if pathObject == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("unknown file %#v at revision %#v of project %#v on host %#v", request.Path, request.Committish, request.Project, c.hostname))
	}
	var response *gitilespb.DownloadFileResponse
	switch request.Format {
	case gitilespb.DownloadFileRequest_JSON:
		util.PanicIf(pathObject.submoduleRevision == "", "downloading JSON is only supported for submodules in the fake (path: %#v)", request.Path)
		response = &gitilespb.DownloadFileResponse{
			Contents: fmt.Sprintf(`{"revision": "%s"}`, pathObject.submoduleRevision),
		}
	case gitilespb.DownloadFileRequest_TEXT:
		util.PanicIf(pathObject.submoduleRevision != "", "downloading submodule is not supported in the fake (path: %#v)", request.Path)
		response = &gitilespb.DownloadFileResponse{
			Contents: pathObject.contents,
		}
	default:
		panic("format is not set in DownloadFileRequest")
	}
	return response, nil
}

// DownloadDiff downloads the diff between a revision and its parent.
//
// To ensure that the diffs created are accurate and match the behavior of git
// (which implements its own diffing with rename/copy detection), a local git
// instance is created and commits populated with the fake data. This git
// instance then produces the diff.
func (c *Client) DownloadDiff(ctx context.Context, request *gitilespb.DownloadDiffRequest, options ...grpc.CallOption) (*gitilespb.DownloadDiffResponse, error) {
	getPathObjectsFromHistory := func(history []*commit) map[string]PathObject {
		pathObjects := map[string]PathObject{}
		for i := len(history) - 1; i >= 0; i -= 1 {
			for path, pathObject := range history[i].revision.Files {
				if pathObject == nil {
					delete(pathObjects, path)
				} else {
					pathObjects[path] = *pathObject
				}
			}
		}
		return pathObjects
	}

	history, err := c.getRevisionHistory(request.Project, request.Committish)
	if err != nil {
		return nil, err
	}

	tmp, err := ioutil.TempDir("", "")
	util.PanicOnError(err)

	git := func(args ...string) string {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = tmp
		output, err := cmd.Output()
		util.PanicOnError(err)
		return string(output)
	}
	git("init")

	writeFiles := func(pathObjects map[string]PathObject) {
		files := map[string]string{}
		for path, file := range pathObjects {
			util.PanicIf(file.submoduleRevision != "", "attempting to create working tree with submodule %s", path)
			files[path] = file.contents
		}
		util.PanicOnError(testfs.Build(tmp, files))
	}
	commit := func(message string) {
		git("add", ".")
		git("commit", "--allow-empty", "-m", message)
	}

	// For computing a diff, the relationship between the two commits isn't actually important.
	// We'll just create a commit containing the files for the parent or base as appropriate,
	// then create another commit with the files for committish and get a diff of HEAD vs HEAD^.
	var basePathObjects map[string]PathObject
	if request.Base == "" {
		basePathObjects = getPathObjectsFromHistory(history[1:])
		writeFiles(basePathObjects)
		commit(fmt.Sprintf("parent of committish %s", request.Committish))
	} else {
		baseHistory, err := c.getRevisionHistory(request.Project, request.Base)
		if err != nil {
			return nil, err
		}
		basePathObjects = getPathObjectsFromHistory(baseHistory)
		writeFiles(basePathObjects)
		commit(fmt.Sprintf("base %s", request.Base))
	}

	pathObjects := getPathObjectsFromHistory(history)
	for path := range basePathObjects {
		if _, ok := pathObjects[path]; !ok {
			f := filepath.Join(tmp, filepath.FromSlash(path))
			util.PanicOnError(os.Remove(f))
		}
	}
	writeFiles(pathObjects)
	commit(fmt.Sprintf("committish %s", request.Committish))

	args := []string{"diff", "HEAD^", "HEAD"}
	if request.Path != "" {
		args = append(args, "--", request.Path)
	}
	diff := git(args...)
	return &gitilespb.DownloadDiffResponse{Contents: string(diff)}, nil
}
