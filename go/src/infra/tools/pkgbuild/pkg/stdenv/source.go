// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stdenv

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/base/workflow"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/system/environ"
)

//go:embed git_archive.py
var gitSourceEmbed embed.FS
var gitSourceGen = generators.InitEmbeddedFS(
	"git_source_script", gitSourceEmbed,
)

func (g *Generator) fetchSource(plats generators.Platforms) (generators.Generator, string, error) {
	// The name of the source derivation. It's also used in environment variable
	// srcs to pointing to the location of source file(s), which will be expanded
	// to absolute path by utilities.BaseGenerator.
	name := fmt.Sprintf("%s_source", g.Name)
	srcPath := fmt.Sprintf("{{.%s}}", name)
	switch s := g.Source.(type) {
	case *SourceGit:
		env := environ.New(nil)
		env.Set("PATH", filepath.Join("{{.stdenv_git}}", "bin"))
		return &workflow.Generator{
			Name: name,
			Metadata: &core.Action_Metadata{
				Cipd: &core.Action_Metadata_CIPD{
					Name:    s.CIPDName,
					Version: s.Version,
				},
				ContextInfo: g.Name + ":" + plats.Host.String(),
			},
			Args: []string{execPath(plats.Build, "{{.stdenv_python3}}", "bin", "python3"), "-I", "-B", "-u", "-X", "utf8", filepath.Join("{{.git_source_script}}", "git_archive.py"), s.URL, s.Ref},
			Dependencies: []generators.Dependency{
				{Type: generators.DepsBuildHost, Generator: git},
				{Type: generators.DepsBuildHost, Generator: cpython},
				{Type: generators.DepsBuildHost, Generator: gitSourceGen},
			},
			Env: env,
		}, "srcs=" + filepath.Join(srcPath, "src.tar"), nil
	case *SourceURLs:
		urls := generators.FetchURLs{
			Name: name,
			Metadata: &core.Action_Metadata{
				Cipd: &core.Action_Metadata_CIPD{
					Name:    s.CIPDName,
					Version: s.Version,
				},
				ContextInfo: g.Name + ":" + plats.Host.String(),
			},
			URLs: map[string]generators.FetchURL{},
		}
		var srcs []string
		for _, u := range s.URLs {
			urls.URLs[u.Filename] = generators.FetchURL{
				URL:           u.URL,
				HashAlgorithm: u.HashAlgorithm,
				HashValue:     u.HashValue,
			}
			srcs = append(srcs, filepath.Join(srcPath, u.Filename))
		}
		return &urls, fmt.Sprintf("srcs=%s", strings.Join(srcs, string(filepath.ListSeparator))), nil
	default:
		return nil, "", fmt.Errorf("unknown source type %#v:", s)
	}
}

type Source interface {
	isSourceMethod()
}

type SourceGit struct {
	// The url to the git repository. Support any protocol used by the git
	// command line interfaces.
	URL string

	// The reference for the git repo. Can be anything supported by git checkout.
	// Typical use cases:
	// - refs/tags/xxx
	// - 8e8722e14772727b0e1cd5bd925a0f089611a60b
	Ref string

	CIPDName string
	Version  string
}

func (s *SourceGit) isSourceMethod() {}

type SourceURL struct {
	URL           string
	Filename      string
	HashAlgorithm core.HashAlgorithm
	HashValue     string
}

type SourceURLs struct {
	URLs []SourceURL

	CIPDName string
	Version  string
}

func (s *SourceURLs) isSourceMethod() {}
