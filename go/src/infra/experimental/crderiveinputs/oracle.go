// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/client/cipd/reader"
	"go.chromium.org/luci/cipd/client/cipd/template"
	"go.chromium.org/luci/cipd/common"
	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/git/footer"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

type Oracle struct {
	GClientVars

	CipdOS   string
	CipdArch string

	manifestMu sync.Mutex
	manifest   *inputpb.Manifest

	// NOTE: This is bad style in general. It's still bad style here, but this
	// object is confined to this 'main' module. If this ever gets refactored to
	// be used in a broader context, `ctx` should be pulled out as part of that
	// refactoring effort and passed instead to the leaf functions hung off of
	// Oracle and it's derived structs.
	ctx context.Context

	// path to local git oracle cache repo
	gitpath string

	repoMapMu sync.Mutex
	repoMap   map[string]*gitRepo

	cipdClient   cipd.Client
	cipdExpander template.Expander

	gcsClient    *storage.Client
	gcsCachePath string
}

func (o *Oracle) withSource(path string, cb func(s *inputpb.Source)) {
	o.manifestMu.Lock()
	defer o.manifestMu.Unlock()

	if o.manifest.Sources == nil {
		o.manifest.Sources = map[string]*inputpb.Source{}
	}
	ret := o.manifest.Sources[path]
	if ret == nil {
		ret = &inputpb.Source{Path: path}
		o.manifest.Sources[path] = ret
	}
	cb(ret)
}

func (o *Oracle) nearestSource(originalPath string) (src *inputpb.Source, remainingPath string) {
	o.manifestMu.Lock()
	defer o.manifestMu.Unlock()

	target := originalPath
	for target != "" && target != "." {
		if src := o.manifest.Sources[target]; src != nil {
			// if src.Path != originalPath we want to make sure it's a logical prefix,
			// not just a lexical one.
			prefix := src.Path
			if prefix != originalPath {
				prefix = prefix + "/"
				if !strings.HasPrefix(originalPath, prefix) {
					panic(fmt.Sprintf("impossible: originalPath %q missing prefix %q", originalPath, prefix))
				}
			}
			return proto.Clone(src).(*inputpb.Source), originalPath[len(prefix):]
		}
		target = path.Dir(target)
	}
	return nil, ""
}

func (o *Oracle) AllGitSources() []*inputpb.Source {
	o.manifestMu.Lock()
	defer o.manifestMu.Unlock()

	ret := make([]*inputpb.Source, 0, len(o.manifest.Sources))
	for _, source := range o.manifest.Sources {
		if source.GetGit() != nil {
			ret = append(ret, source)
		}
	}

	return ret
}

func (o *Oracle) ReadFullString(target string) (string, error) {
	src, innerPath := o.nearestSource(target)
	if src == nil {
		return "", errors.Reason("cannot find Manifest Source for %q", target).Err()
	}

	for {
		switch x := src.Content.(type) {
		case *inputpb.Source_Git:
			gits := x.Git
			ret, err := o.gitRepo(gits.Repo).catblob(gits.Version.Resolved, innerPath)
			if err != nil {
				return "", errors.Annotate(err, "cat-file blob %q %q %q", gits.Repo, gits.Version.Resolved, target).Err()
			}
			return ret, nil

		case *inputpb.Source_Cipd:
			for _, pkg := range x.Cipd.Packges {
				pkgSrc, err := o.cipdClient.FetchInstance(o.ctx, common.Pin{
					PackageName: pkg.Pkg.Resolved,
					InstanceID:  pkg.Version.Resolved,
				})
				if err != nil {
					return "", err
				}

				inst, err := reader.OpenInstance(o.ctx, pkgSrc, reader.OpenInstanceOpts{
					VerificationMode: reader.VerifyHash,
					InstanceID:       pkg.Version.Resolved,
				})
				if err != nil {
					return "", err
				}

				for _, file := range inst.Files() {
					if file.Name() == innerPath {
						of, err := file.Open()
						if err != nil {
							return "", err
						}
						data, err := io.ReadAll(of)
						if err != nil {
							return "", err
						}
						return string(data), nil
					}
				}
			}
			Logger.Debugf("Fallthrough - to find %q in any CIPD package at %q", innerPath, src.Path)
		}

		parent, droppedDir := path.Split(src.Path)
		var middlePath string
		src, middlePath = o.nearestSource(parent)
		if src == nil {
			break
		}

		innerPath = path.Join(middlePath, droppedDir, innerPath)
	}

	return "", errors.Reason("do not know how to ReadFullString(%q) for %q", target, src).Err()
}

func (o *Oracle) WalkDirectory(target string, patterns ...string) (fullpaths []string, err error) {
	// TODO - maybe support walking a directory with a mix of sources under it?

	src, remainingPath := o.nearestSource(target)
	if src == nil {
		return nil, errors.Reason("unable to find source for target %q", target).Err()
	}

	switch x := src.Content.(type) {
	case *inputpb.Source_Git:
		expandedPatterns := make([]string, 0, len(patterns)+1)
		if len(patterns) == 0 {
			if remainingPath != "" {
				expandedPatterns = append(expandedPatterns, remainingPath)
			}
		} else {
			for _, pattern := range patterns {
				expandedPatterns = append(expandedPatterns, path.Join(remainingPath, pattern))
			}
		}

		repo := o.gitRepo(x.Git.Repo)
		if err := repo.ensureFetched(x.Git.Version.Resolved); err != nil {
			return nil, err
		}

		args := append([]string{"ls-files", "--with-tree", x.Git.Version.Resolved, "--"}, expandedPatterns...)
		listing, err := repo.output(args...)
		if err != nil {
			return nil, err
		}
		rawFiles := strings.Split(listing, "\n")
		ret := make([]string, 0, len(rawFiles))
		for _, repopath := range rawFiles {
			if repopath == "" {
				continue
			}
			ret = append(ret, path.Join(src.Path, repopath))
		}
		return ret, nil
	}

	return nil, errors.Reason("do not know how to WalkDirectory(%q) for %q", target, src).Err()
}

func (o *Oracle) GetCommitObject(target, grepPattern string) (commit *object.Commit, footers strpair.Map, err error) {
	src, _ := o.nearestSource(target)
	srcGit := src.GetGit()
	if srcGit == nil {
		err = errors.Reason("GetCommitObject only supported for git sources, not %q - %v", target, src).Err()
		return
	}

	repo := o.gitRepo(srcGit.Repo)
	if err = repo.ensureFetched(srcGit.Version.Resolved); err != nil {
		return
	}

	args := []string{"log", "-1", "--format=%H"}
	if grepPattern != "" {
		args = append(args, "--grep", grepPattern)
	}
	args = append(args, srcGit.Version.Resolved)
	hash, err := repo.output(args...)
	if err != nil {
		return
	}

	contents, err := repo.output("cat-file", "commit", strings.TrimSpace(hash))
	if err != nil {
		return
	}

	mo := &plumbing.MemoryObject{}
	mo.SetType(plumbing.CommitObject)
	amtWritten, err := io.WriteString(mo, contents)
	if err != nil {
		return
	}
	if amtWritten != len(contents) {
		panic("impossible: MemoryObject.Write did not write all of the commit contents?")
	}

	commit = &object.Commit{}
	if err = commit.Decode(mo); err != nil {
		return
	}

	footers = footer.ParseMessage(commit.Message)
	return
}

func NewOracle(ctx context.Context, manifest *inputpb.Manifest, args *Args, authenticator *auth.Authenticator) (*Oracle, error) {
	gitpath := filepath.Join(args.CacheDirectory, "oracle", "git")
	if err := os.MkdirAll(gitpath, 0700); err != nil {
		return nil, errors.Annotate(err, "creating cache directory").Err()
	}

	gcsPath := filepath.Join(args.CacheDirectory, "oracle", "gcs")
	if err := os.MkdirAll(gcsPath, 0700); err != nil {
		return nil, errors.Annotate(err, "creating cache directory").Err()
	}

	ret := &Oracle{
		GClientVars: args.GClientVars,
		CipdOS:      args.CIPDOS(),
		CipdArch:    args.CIPDArch(),

		manifest:     manifest,
		ctx:          ctx,
		gitpath:      gitpath,
		repoMap:      map[string]*gitRepo{},
		gcsCachePath: gcsPath,
	}

	var err error
	authClient, err := authenticator.Client()
	if err != nil {
		return nil, err
	}
	ret.cipdClient, err = cipd.NewClient(cipd.ClientOptions{
		ServiceURL:          "https://chrome-infra-packages.appspot.com",
		UserAgent:           cipd.UserAgent + " + crderiveinputs",
		CacheDir:            filepath.Join(args.CacheDirectory, "oracle", "cipd"),
		AuthenticatedClient: authClient,
	})
	if err != nil {
		return nil, err
	}

	ret.cipdExpander = template.Expander{
		"os":       ret.CipdOS,
		"arch":     ret.CipdArch,
		"platform": fmt.Sprintf("%s-%s", ret.CipdOS, ret.CipdArch),
	}

	ts, err := authenticator.TokenSource()
	if err != nil {
		return nil, err
	}
	ret.gcsClient, err = storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}

	return ret, nil
}
