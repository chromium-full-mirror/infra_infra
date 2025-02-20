// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/api/gitiles"
	lucierrors "go.chromium.org/luci/common/errors"
	gerritpb "go.chromium.org/luci/common/proto/gerrit"
	gitilespb "go.chromium.org/luci/common/proto/gitiles"
	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/experimental/golangbuild/golangbuildpb"
)

const (
	// N.B. Unfortunately Go still calls the main branch "master" due to technical issues.
	mainBranch   = "master" // nocheck
	publicGoHost = "go.googlesource.com"
)

// sourceSpec indicates a repository to fetch and what state to fetch it at.
//
// One of commit and change must be non-nil.
type sourceSpec struct {
	// project is a project in host. Must not be empty.
	project string

	// branch is the branch of project that change and/or commit are on. May only
	// be empty if commit != nil, tag is non-empty, and cherryPick is false.
	//
	// branch is derived from and lines up with commit.Ref if commit != nil and
	// commit.Ref starts with "refs/heads/".
	branch string

	// tag is the tag referred to by commit. commit.Ref must start with "refs/tags/".
	// commit must be non-nil and cherryPick must be false. Must be non-empty if
	// branch is empty.
	tag string

	// change is a Gerrit CL to fetch. If this is non-nil, commit must be nil.
	change *bbpb.GerritChange

	// commit is a Gitiles commit to fetch. If this is non-nil, change must be nil.
	commit *bbpb.GitilesCommit

	// cherryPick controls whether to cherry-pick change onto branch.
	//
	// This field may only be true when change is non-nil.
	//
	// Note that this will cherry-pick change without any of its parent
	// changes (if any), thus testing the change in isolation.
	cherryPick bool
}

// asURL returns a URL string for the sourceSpec.
func (s *sourceSpec) asURL() string {
	switch {
	case s.commit != nil && s.change != nil:
		panic("sourceSpec has both a change and a commit")
	case s.commit != nil && s.change == nil:
		return fmt.Sprintf("https://%s/%s/+/%s", s.commit.Host, s.commit.Project, s.commit.Id)
	case s.commit == nil && s.change != nil:
		return fmt.Sprintf("https://%s/c/%s/+/%d/%d", s.change.Host, s.change.Project, s.change.Change, s.change.Patchset)
	}
	panic("no commit or change in sourceSpec")
}

// asMarkdown returns a markdown-formatted representation of the source.
func (s *sourceSpec) asMarkdown() string {
	var linkText string
	switch {
	case s.commit != nil && s.change == nil:
		if s.tag != "" {
			linkText = fmt.Sprintf("tag %s", s.tag)
		} else {
			linkText = fmt.Sprintf("commit %s", s.commit.Id[:7])
		}
	case s.commit == nil && s.change != nil:
		linkText = fmt.Sprintf("change %d", s.change.Change)
	}
	branchText := ""
	if s.branch != "" {
		branchText = " on " + s.branch
	}
	return fmt.Sprintf("%s%s ([%s](%s))", s.project, branchText, linkText, s.asURL())
}

// asSource returns a golangbuildpb.Source for the sourceSpec.
func (s *sourceSpec) asSource() *golangbuildpb.Source {
	var src golangbuildpb.Source
	switch {
	case s.commit != nil && s.change != nil:
		panic("sourceSpec has both a change and a commit")
	case s.commit != nil && s.change == nil:
		src.GitilesCommit = s.commit
	case s.commit == nil && s.change != nil:
		src.GerritChange = s.change
	}
	return &src
}

// fetchRepo fetches a repository according to src and places it at dst.
func fetchRepo(ctx context.Context, src *sourceSpec, dst string, inputs *golangbuildpb.Inputs) (err error) {
	step, ctx := build.StartStep(ctx, "fetch "+src.project)
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.

	// Check invariants of src.
	if src.cherryPick && src.branch == "" {
		return fmt.Errorf("requested cherry pick, but found no branch for %s", src.asURL())
	}
	if src.tag != "" && src.branch != "" {
		return fmt.Errorf("both tag (%q) and branch (%q) set for %s", src.tag, src.branch, src.asURL())
	}
	if src.tag != "" && src.change != nil {
		return fmt.Errorf("tag (%s) unexpectedly set for gerrit change %s", src.tag, src.asURL())
	}
	if src.tag == "" && src.branch == "" {
		return fmt.Errorf("missing required branch for %s", src.asURL())
	}

	switch {
	case src.change != nil && !src.cherryPick:
		return fetchRepoChangeAsIs(ctx, inputs, src.change, dst)
	case src.change != nil && src.cherryPick:
		return fetchRepoChangeWithCherryPick(ctx, inputs, src.branch, src.change, dst)
	case src.commit != nil:
		if src.cherryPick {
			return fmt.Errorf("cherryPick is unexpectedly set in the commit case")
		}
		return fetchRepoAtCommit(ctx, inputs, src.commit, dst)
	}
	return fmt.Errorf("one of change or commit must be non-nil")
}

// cachedRepoPath returns the path to the cached bare repo for this gerrit
// change, performing initial clone if it is missing from cache.
func cachedRepoPath(ctx context.Context, inputs *golangbuildpb.Inputs, host, project string) (string, error) {
	cache := inputs.GitCache
	if cache == "" {
		return "", fmt.Errorf("inputs missing GitCache: %+v", inputs)
	}
	if !filepath.IsLocal(cache) {
		return "", fmt.Errorf("GitCache %q must be relative", cache)
	}

	// Double check that the host and project don't form malformed paths.
	if !filepath.IsLocal(host) {
		return "", fmt.Errorf("host %q must make a relative path", host)
	}
	if !filepath.IsLocal(project) {
		return "", fmt.Errorf("project %q must make a relative path", project)
	}

	luciExe := lucictx.GetLUCIExe(ctx)
	if luciExe == nil {
		return "", fmt.Errorf("missing LUCI_CONTEXT")
	}

	// N.B. host is included in the path so that we don't mix contents from
	// potentially conflicting repos with the same project name. e.g.,
	// go/go and go-internal/go.
	//
	// Once annoyance with this approach is that GitilesChange uses host
	// "go.googlesource.com" while GerritChange uses
	// "go-review.googlesource.com", so those repos will be duplicated.
	repo := filepath.Join(luciExe.GetCacheDir(), cache, host, project)

	_, err := os.Stat(repo)
	if err == nil {
		// Repo already exists.
		return repo, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		// Some other error.
		return "", fmt.Errorf("error checking for repo existence: %w", err)
	}

	// Repo does not exist; perform initial clone.

	if err := runGit(ctx, "git clone upstream to cache", "clone", "--bare", urlFromHostProject(host, project), repo); err != nil {
		return "", err
	}

	return repo, nil
}

// Fetch a remote object (a ref or explicit commit SHA) from host/project into
// the cache repo. Returns the explicit commit SHA that obj resolves to
// (unnecessary if obj is already an explicit commit SHA).
//
// N.B. We don't need to worry about git gc pruning these unreferenced objects
// before we get a chance to clone to repo because git gc never prunes objects
// less than 2 weeks old.
func fetchObjIntoCache(ctx context.Context, inputs *golangbuildpb.Inputs, host, project, obj string) (string, error) {
	cacheRepo, err := cachedRepoPath(ctx, inputs, host, project)
	if err != nil {
		return "", err
	}

	// Fetch object into cached repo (may be a no-op).
	if err := runGit(ctx, "git fetch into cache", "--git-dir", cacheRepo, "fetch", urlFromHostProject(host, project), obj); err != nil {
		return "", err
	}

	// Resolve fetched commit for subsequent fetch. We need this because
	// the fetched refs aren't named in the cache repo, so they need to be
	// fetched by explicit sha.
	//
	// This is useless if obj is already a sha, but it doesn't hurt.
	out, err := runGitOutput(ctx, "git rev-parse", "--git-dir", cacheRepo, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(string(out))

	return sha, nil
}

// Clone the cache repo to dst and checkout sha.
func cloneFromCache(ctx context.Context, inputs *golangbuildpb.Inputs, host, project, dst, sha string) error {
	cacheRepo, err := cachedRepoPath(ctx, inputs, host, project)
	if err != nil {
		return err
	}

	// It is very tempting to be clever and try to "optimize" this. e.g.,
	// by creating a new repo and fetching only the objects we need from
	// the cache. However, that ends up being less efficient because fetch
	// is fairly expensive, while a local clone is optimized to a direct
	// copy of the .git directory (see --local in git help clone), which is
	// much faster.
	if err := runGit(ctx, "git clone from cache", "clone", cacheRepo, dst); err != nil {
		return err
	}

	if err := runGit(ctx, "git checkout", "-C", dst, "checkout", sha); err != nil {
		return err
	}

	return nil
}

// fetchRepoChangeAsIs checks out a change to be tested as is, without
// rebasing.
func fetchRepoChangeAsIs(ctx context.Context, inputs *golangbuildpb.Inputs, change *bbpb.GerritChange, dst string) error {
	ref := refFromChange(change)
	sha, err := fetchObjIntoCache(ctx, inputs, change.Host, change.Project, ref)
	if err != nil {
		return err
	}

	return cloneFromCache(ctx, inputs, change.Host, change.Project, dst, sha)
}

// fetchRepoChangeWithCherryPick checks out a change by cherry-picking it on
// top of branch.
func fetchRepoChangeWithCherryPick(ctx context.Context, inputs *golangbuildpb.Inputs, branch string, change *bbpb.GerritChange, dst string) error {
	// Fetch branch and change into cache so they will be both be available
	// in the clone.
	branchRef := "refs/heads/" + branch
	branchSha, err := fetchObjIntoCache(ctx, inputs, change.Host, change.Project, branchRef)
	if err != nil {
		return err
	}
	changeRef := refFromChange(change)
	changeSha, err := fetchObjIntoCache(ctx, inputs, change.Host, change.Project, changeRef)
	if err != nil {
		return err
	}

	if err := cloneFromCache(ctx, inputs, change.Host, change.Project, dst, branchSha); err != nil {
		return err
	}

	if err := runGit(ctx, "git cherry-pick", "-C", dst, "cherry-pick", "--allow-empty", changeSha); err != nil {
		return err
	}

	return nil
}

// fetchRepoAtCommit checks out a commit to be tested as is.
func fetchRepoAtCommit(ctx context.Context, inputs *golangbuildpb.Inputs, commit *bbpb.GitilesCommit, dst string) error {
	_, err := fetchObjIntoCache(ctx, inputs, commit.Host, commit.Project, commit.Id)
	if err != nil {
		return err
	}

	return cloneFromCache(ctx, inputs, commit.Host, commit.Project, dst, commit.Id)
}

// fetchDependencies uses 'go mod download' to fetch
// dependencies for the given modules.
func fetchDependencies(ctx context.Context, spec *buildSpec, modules []module) (err error) {
	step, ctx := build.StartStep(ctx, "fetch dependencies")
	defer endStep(step, &err)

	var errs []error
	for _, m := range modules {
		ctx := setupModuleEnv(ctx, m)
		err := goModDownload(ctx, spec, fmt.Sprintf("fetch %q dependencies", m.Path), m.RootDir)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// goModDownload runs 'go mod download' in dir in a build step.
//
// Its output (in JSON format) is parsed to determine whether a
// failure in the build step is a test or infrastructure failure.
func goModDownload(ctx context.Context, spec *buildSpec, stepName, dir string) (err error) {
	cmd := spec.goCmd(ctx, dir, "mod", "download", "-json")
	// Invest reasonable effort to estimate whether the failure was likely due to
	// an infrastructure problem rather than a problem in the input being tested.
	//
	// It's viable to default to non-infra unless a known-infra error is seen, or vice versa,
	// so use whichever strikes a better balance of low false positives and maintenance costs.
	var infra bool
	step, ctx, startStepErr := cmdStartStep(ctx, stepName, cmd)
	defer func() {
		if infra {
			err = infraWrap(err) // Failure is deemed to be an infrastructure failure.
		}
		step.End(err)
	}()
	if startStepErr != nil {
		return startStepErr
	}
	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(step.Log("stdout"), &stdout)
	cmd.Stderr = step.Log("stderr")
	err = cmd.Run()
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) && ee.ExitCode() == 1 {
		for dec := json.NewDecoder(&stdout); ; {
			var m struct{ Error string }
			err := dec.Decode(&m)
			if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("error decoding JSON object from go mod download -json: %w\n", err)
			}
			if strings.Contains(m.Error, "dial tcp") && strings.HasSuffix(m.Error, ": i/o timeout") {
				// An I/O timeout error to a Go module proxy is deemed to be an infrastructure failure.
				// See https://ci.chromium.org/b/8772399708036918561 for an example.
				infra = true
				break
			} else if strings.HasSuffix(m.Error, ": EOF") {
				// An EOF error from a Go module proxy is deemed to be an infrastructure failure.
				// See go.dev/issue/63684 and https://ci.chromium.org/b/8766452547270027457 for an example.
				infra = true
				break
			}
		}
	}
	if err != nil {
		return lucierrors.Annotate(err, "Failed to run %q", stepName).Err()
	}
	return nil
}

func readFile(ctx context.Context, path string) (data string, exists bool, err error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("read %s", filepath.Base(path)))
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.
	contentsLog := step.Log("contents")

	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	data = string(b)
	_, err = io.WriteString(contentsLog, data)
	return data, true, err
}

func writeFile(ctx context.Context, path, data string) (err error) {
	step, ctx := build.StartStep(ctx, fmt.Sprintf("write %s", filepath.Base(path)))
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.
	contentsLog := step.Log("contents")

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		r := f.Close()
		if err == nil {
			err = r
		} else {
			if _, r2 := io.WriteString(step.Log("close error"), r.Error()); r2 != nil {
				log.Printf("%v", r2)
			}
		}
	}()
	_, err = io.WriteString(io.MultiWriter(contentsLog, f), data)
	return err
}

func urlFromHostProject(host, project string) string {
	return "https://" + host + "/" + project
}

func refFromChange(change *bbpb.GerritChange) string {
	return fmt.Sprintf("refs/changes/%02d/%d/%d", change.Change%100, change.Change, change.Patchset)
}

func runGit(ctx context.Context, stepName string, args ...string) (err error) {
	return cmdStepRun(ctx, stepName, exec.CommandContext(ctx, "git", args...), true)
}

func runGitOutput(ctx context.Context, stepName string, args ...string) (output []byte, err error) {
	return cmdStepOutput(ctx, stepName, exec.CommandContext(ctx, "git", args...), true)
}

// sourceForBranch produces a sourceSpec representing the tip of a branch for a project.
func sourceForBranch(ctx context.Context, auth *auth.Authenticator, host, project, branch string) (*sourceSpec, error) {
	return sourceForRef(ctx, auth, host, project, fmt.Sprintf("refs/heads/%s", branch))
}

func sourceForTag(ctx context.Context, auth *auth.Authenticator, host, project, tag string) (*sourceSpec, error) {
	return sourceForRef(ctx, auth, host, project, fmt.Sprintf("refs/tags/%s", tag))
}

func sourceForRef(ctx context.Context, auth *auth.Authenticator, host, project, ref string) (*sourceSpec, error) {
	// Determine whether we have a branch or a tag.
	branch, isBranch := strings.CutPrefix(ref, "refs/heads/")
	tag, isTag := strings.CutPrefix(ref, "refs/tags/")
	if !isTag && !isBranch {
		return nil, fmt.Errorf("invalid ref %q, must be a tag or a branch", ref)
	}
	// N.B. CutPrefix returns the original string, if it returns false.
	// Clear the tag or branch, whichever is not present.
	if !isTag {
		tag = ""
	}
	if !isBranch {
		branch = ""
	}

	// Fetch the commit for the branch or tag.
	hc, err := auth.Client()
	if err != nil {
		return nil, fmt.Errorf("auth.Client: %w", err)
	}
	gc, err := gitiles.NewRESTClient(hc, host, true)
	if err != nil {
		return nil, fmt.Errorf("gitiles.NewRESTClient: %w", err)
	}
	log, err := gc.Log(ctx, &gitilespb.LogRequest{
		Project:    project,
		Committish: ref,
		PageSize:   1,
	})
	if err != nil {
		return nil, fmt.Errorf("gc.Log: %w", err)
	}
	if len(log.Log) == 0 {
		return nil, fmt.Errorf("no commits found for project %s at ref %s", project, ref)
	}
	return &sourceSpec{
		project: project,
		branch:  branch,
		tag:     tag,
		commit: &bbpb.GitilesCommit{
			Host:    host,
			Project: project,
			Id:      log.Log[0].Id,
			Ref:     ref,
		},
	}, nil
}

// sourceForGoBranchAndCommit produces a sourceSpec representing
// the specified branch and commit in the main Go repo.
func sourceForGoBranchAndCommit(host, branch, commit string) (*sourceSpec, error) {
	if branch == "" {
		return nil, fmt.Errorf("empty branch")
	}
	if len(commit) != len("4368e1cdfd37cbcdbc7a4fbcc78ad61139f7ba90") {
		return nil, fmt.Errorf("unsupported commit ID %q: length is %d, want 40", commit, len(commit))
	}
	for _, c := range commit {
		if ok := '0' <= c && c <= '9' || 'a' <= c && c <= 'f'; !ok {
			return nil, fmt.Errorf("unsupported commit ID %q: contains %q, want 0-9a-f only", commit, c)
		}
	}
	return &sourceSpec{
		project: "go",
		branch:  branch,
		commit: &bbpb.GitilesCommit{
			Host:    host,
			Project: "go",
			Id:      commit,
			Ref:     "refs/heads/" + branch,
		},
	}, nil
}

func sourceForParent(ctx context.Context, auth *auth.Authenticator, src *sourceSpec) (*sourceSpec, error) {
	hc, err := auth.Client()
	if err != nil {
		return nil, fmt.Errorf("auth.Client: %w", err)
	}
	switch {
	case src.commit != nil && src.change == nil:
		gc, err := gitiles.NewRESTClient(hc, src.commit.Host, true)
		if err != nil {
			return nil, fmt.Errorf("gitiles.NewRESTClient: %w", err)
		}
		log, err := gc.Log(ctx, &gitilespb.LogRequest{
			Project:    src.commit.Project,
			Committish: src.commit.Id,
			PageSize:   2,
		})
		if err != nil {
			return nil, fmt.Errorf("gc.Log: %w", err)
		}
		if len(log.Log) == 0 {
			return nil, fmt.Errorf("commit %s not found in repository %s", src.commit.Id, src.project)
		} else if len(log.Log) == 1 {
			return nil, fmt.Errorf("commit %s has no parent in repository %s", src.commit.Id, src.project)
		}
		return &sourceSpec{
			project: src.project,
			// N.B. Drop tag here. We're getting the parent, and have no idea if that's another tag.
			branch: src.branch,
			commit: &bbpb.GitilesCommit{
				Host:    src.commit.Host,
				Project: src.commit.Project,
				Id:      log.Log[1].Id,
				Ref:     src.commit.Ref,
			},
		}, nil
	case src.commit == nil && src.change != nil:
		gc, err := gerrit.NewRESTClient(hc, src.change.Host, true)
		if err != nil {
			return nil, fmt.Errorf("gerrit.NewRESTClient: %w", err)
		}
		changeInfo, err := gc.GetChange(ctx, &gerritpb.GetChangeRequest{
			Number:  src.change.Change,
			Project: src.change.Project,
			Options: []gerritpb.QueryOption{
				gerritpb.QueryOption_ALL_REVISIONS,
				gerritpb.QueryOption_ALL_COMMITS,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("gc.GetChange: %w", err)
		}
		var commit *gerritpb.CommitInfo
		for _, rev := range changeInfo.Revisions {
			if rev.Number == int32(src.change.Patchset) {
				commit = rev.Commit
				break
			}
		}
		if commit == nil {
			return nil, fmt.Errorf("invalid patchset %d for %s not found", src.change.Patchset, src.asURL())
		} else if len(commit.Parents) > 1 {
			return nil, fmt.Errorf("change %s has multiple parents and is possibly a merge commit: not supported", src.asURL())
		} else if len(commit.Parents) == 0 {
			return nil, fmt.Errorf("change %s has no parent commits", src.asURL())
		}
		return &sourceSpec{
			project: src.project,
			branch:  src.branch,
			commit: &bbpb.GitilesCommit{
				Host:    src.change.Host,
				Project: src.change.Project,
				Ref:     "refs/heads/" + src.branch,
				Id:      commit.Parents[0].Id,
			},
		}, nil
	case src.commit != nil && src.change != nil:
		panic("sourceSpec has both a change and a commit")
	}
	panic("no commit or change in sourceSpec")
}

// sourceForLatestGoRelease produces a sourceSpec corresponding to the latest
// stable release cut from goBranch. If goBranch is the main branch, it means
// to use the latest stable release.
func sourceForLatestGoRelease(ctx context.Context, auth *auth.Authenticator, goBranch string) (*sourceSpec, error) {
	releases, err := fetchStableGoReleases(ctx)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("failed to find a stable Go release at https://go.dev/dl/?mode=json")
	}
	var tag string
	if goBranch == mainBranch {
		// Pick the latest release. This is also the name of the tag.
		tag = releases[0].Version
	} else if branchVersion, ok := strings.CutPrefix(goBranch, "release-branch.go1."); ok {
		// Figure out what major version we're at.
		ver, err := strconv.Atoi(branchVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse go branch name: %s", goBranch)
		}
		// Look for the latest release of that version.
		majorVer := fmt.Sprintf("go1.%d", ver)
		for _, release := range releases {
			if strings.HasPrefix(release.Version, majorVer) {
				tag = release.Version
				break
			}
		}
		if tag == "" {
			return nil, fmt.Errorf("failed to find a latest release for %s at https://go.dev/dl/?mode=json", majorVer)
		}
	} else {
		return nil, fmt.Errorf("unsupported go branch %s (is it a development branch?)", goBranch)
	}
	return sourceForTag(ctx, auth, publicGoHost, "go", tag)
}

// fetchStableGoReleases returns a list of all current stable Go release versions in descending order of Go version.
func fetchStableGoReleases(ctx context.Context) (stableReleases []goRelease, err error) {
	// Get the latest Go releases.
	req, err := http.NewRequestWithContext(ctx, "GET", "https://go.dev/dl/?mode=json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if r := resp.Body.Close(); r != nil && err == nil {
			err = r
		}
	}()

	// Decode the response.
	var releases []goRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	// Filter down to only stable releases.
	for _, release := range releases {
		if !release.Stable {
			continue
		}
		stableReleases = append(stableReleases, release)
	}

	// N.B. Releases are already listed in descending order by Go version.
	return stableReleases, nil
}

type goRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// fetchCommitTime fetches and returns the commit time for the sourceSpec.
func fetchCommitTime(ctx context.Context, auth *auth.Authenticator, commit *bbpb.GitilesCommit) (time.Time, error) {
	hc, err := auth.Client()
	if err != nil {
		return time.Time{}, fmt.Errorf("auth.Client: %w", err)
	}
	gc, err := gitiles.NewRESTClient(hc, commit.Host, true)
	if err != nil {
		return time.Time{}, fmt.Errorf("gitiles.NewRESTClient: %w", err)
	}
	log, err := gc.Log(ctx, &gitilespb.LogRequest{
		Project:    commit.Project,
		Committish: commit.Id,
		PageSize:   1,
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("gc.Log: %w", err)
	}
	if len(log.Log) == 0 {
		return time.Time{}, fmt.Errorf("commit %s not found in repository %s", commit.Id, commit.Project)
	}
	return log.Log[0].Committer.Time.AsTime(), nil
}
