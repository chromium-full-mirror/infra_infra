// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package loader provides SpecLoader for loading 3pp specs and converting
// them into stdenv.Generator.
package loader

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/core"

	"infra/tools/pkgbuild/pkg/spec"
	"infra/tools/pkgbuild/pkg/spec/source"
	"infra/tools/pkgbuild/pkg/stdenv"
)

const cipdVersionEpoch = "3@"

//go:embed all:build-support
var fromSpecEmbed embed.FS
var fromSpecGen = generators.InitEmbeddedFS(
	"from_spec_support", fromSpecEmbed,
)

// SpecLoader loads 3pp Spec and convert it into a stdenv generator.
type SpecLoader struct {
	cipdPackagePrefix     string
	cipdSourceCachePrefix string
	cipdTargetPlatform    string
	sourceResolver        source.Resolver

	supportFiles generators.Generator

	// Mapping packages's full name to definition.
	specs map[string]*PackageDef
	srcs  map[string]*source.Source
	pkgs  map[string]*stdenv.Generator
}

type SpecLoaderConfig struct {
	CIPDPackagePrefix     string
	CIPDSourceCachePrefix string
	CIPDTargetPlatform    string
	SourceResolver        source.Resolver
}

// DefaultSpecLoaderConfig is the config for default spec loader with
// default source.Resolver.
func DefaultSpecLoaderConfig(target string) *SpecLoaderConfig {
	return &SpecLoaderConfig{
		CIPDPackagePrefix:     "",
		CIPDSourceCachePrefix: "sources",
		CIPDTargetPlatform:    target,
		SourceResolver:        source.NewResolver(),
	}
}

// NewSpecLoader creates SpecLoader, loading specs from root directory.
func NewSpecLoader(root string, cfg *SpecLoaderConfig) (*SpecLoader, error) {
	defs, err := FindPackageDefs(root)
	if err != nil {
		return nil, err
	}

	specs := make(map[string]*PackageDef)
	for _, def := range defs {
		specs[def.FullName()] = def
	}

	return &SpecLoader{
		cipdPackagePrefix:     cfg.CIPDPackagePrefix,
		cipdSourceCachePrefix: cfg.CIPDSourceCachePrefix,
		cipdTargetPlatform:    cfg.CIPDTargetPlatform,
		sourceResolver:        cfg.SourceResolver,

		supportFiles: fromSpecGen,

		specs: specs,
		srcs:  make(map[string]*source.Source),
		pkgs:  make(map[string]*stdenv.Generator),
	}, nil
}

// ListAllByFullName lists all loaded specs' full names by alphabetical order.
func (l *SpecLoader) ListAllByFullName() (names []string) {
	for name := range l.specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return
}

// LoadSourceInfos load the lock file in the spec directory. If update is true,
// LoadSourceInfos also checks the latest version. If persistent is true, it will
// update the lock file in the spec directory.
func (l *SpecLoader) LoadSourceInfos(ctx context.Context, names, cipdHostPlatforms []string, update, persistent bool) error {
	var errs []error
	for _, name := range names {
		spec := l.specs[name]
		if spec == nil {
			return fmt.Errorf("package spec not available: %s", name)
		}
		s := source.LoadSource(spec.Dir, spec.Spec.Create)

		if update {
			if err := s.UpdateInfo(ctx, cipdHostPlatforms, l.sourceResolver); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				continue
			}
		}

		l.srcs[name] = s

		if persistent {
			if err := s.PersistInfo(); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				continue
			}
		}
	}

	return errors.Join(errs...)
}

// FromSpec converts the 3pp spec to stdenv generator by its full name, which
// builds the package for running on the cipd host platform.
// Ideally we should use the Host Platform in BuildContext during the
// generation. But it's much easier to construct the Spec.Create before
// generate and call SpecLoader.FromSpec recursively for dependencies.
func (l *SpecLoader) FromSpec(fullName, buildCipdPlatform, hostCipdPlatform string) (*stdenv.Generator, error) {
	pkgCacheKey := fmt.Sprintf("%s@%s", fullName, hostCipdPlatform)
	if g, ok := l.pkgs[pkgCacheKey]; ok {
		if g == nil {
			return nil, fmt.Errorf("circular dependency detected: %s", pkgCacheKey)
		}
		return g, nil
	}

	// Mark the package visited to prevent circular dependency.
	// Remove the mark if we end up with not updating the result.
	l.pkgs[pkgCacheKey] = nil
	defer func() {
		if l.pkgs[pkgCacheKey] == nil {
			delete(l.pkgs, pkgCacheKey)
		}
	}()

	def := l.specs[fullName]
	if def == nil {
		return nil, fmt.Errorf("package spec not available: %s", fullName)
	}

	// Copy files for building from spec
	defDerivation := &generators.ImportTargets{
		Name:     fmt.Sprintf("%s_from_spec_def", def.DerivationName()),
		Metadata: &core.Action_Metadata{ContextInfo: pkgCacheKey},
		Targets: map[string]generators.ImportTarget{
			".": {Source: filepath.ToSlash(def.Dir), Mode: fs.ModeDir, FollowSymlinks: true},
		},
	}

	// Parse create spec for host
	create, err := newCreateParser(hostCipdPlatform, def.Spec.GetCreate())
	if err != nil {
		return nil, err
	}

	sinfo := l.srcs[fullName].GetInfo(hostCipdPlatform)
	if sinfo == nil {
		return nil, fmt.Errorf("package source info not available: %s", fullName)
	}

	if err := create.ParseSource(def, sinfo, path.Join(l.cipdPackagePrefix, l.cipdSourceCachePrefix)); err != nil {
		return nil, err
	}
	if err := create.FindPatches(defDerivation.Name, def.Dir); err != nil {
		return nil, err
	}
	if err := create.ParseBuilder(); err != nil {
		return nil, err
	}
	if err := create.LoadDependencies(buildCipdPlatform, l); err != nil {
		return nil, err
	}
	if err := create.ParseExternalDependencies(defDerivation.Name, buildCipdPlatform); err != nil {
		return nil, err
	}
	if err := create.ParseVerifier(); err != nil {
		return nil, err
	}
	if err := create.ParsePackage(); err != nil {
		return nil, err
	}

	plat := generators.PlatformFromCIPD(hostCipdPlatform)

	env := create.Enviroments.Clone()
	env.Set("patches", strings.Join(create.Patches, string(os.PathListSeparator)))
	env.Set("fromSpecInstall", create.Installer)
	env.Set("fromSpecTest", create.Tester)
	env.Set("_3PP_DEF", fmt.Sprintf("{{.%s}}", defDerivation.Name))
	env.Set("_3PP_PLATFORM", hostCipdPlatform)
	env.Set("_3PP_TOOL_PLATFORM", buildCipdPlatform)
	env.Set("_3PP_VERSION", create.SourceVersion)
	env.Set("_3PP_PATCH_VERSION", create.PatchVersion)

	// TODO(fancl): These should be moved to go package
	env.Set("GOOS", plat.OS())
	env.Set("GOARCH", plat.Arch())

	baseDeps := []generators.Dependency{
		{Type: generators.DepsBuildHost, Generator: defDerivation},
		{Type: generators.DepsBuildHost, Generator: l.supportFiles},
	}
	if create.Tester != "" {
		// See https://source.chromium.org/chromium/infra/infra/+/main:recipes/recipe_modules/support_3pp/verify.py
		// Ensure cipd & vpython for verify script.
		baseDeps = append(baseDeps, generators.Dependency{
			Type: generators.DepsBuildHost,
			Generator: &generators.CIPDExport{
				Name: "from_spec_tools",
				Ensure: ensure.File{
					PackagesBySubdir: map[string]ensure.PackageSlice{
						"bin": {
							{PackageTemplate: path.Join("infra/tools/cipd", buildCipdPlatform), UnresolvedVersion: "latest"},
							{PackageTemplate: path.Join("infra/tools/luci/vpython3", buildCipdPlatform), UnresolvedVersion: "latest"},
						},
					},
				},
			},
		})
	}

	g := &stdenv.Generator{
		Name:         def.DerivationName(),
		Source:       create.Source,
		Dependencies: append(baseDeps, create.Dependencies...),
		Env:          env,
		CIPD: &core.Action_Metadata_CIPD{
			Name: def.CIPDPath(l.cipdPackagePrefix, hostCipdPlatform),

			// Avoid uploading package for platform not matching the target.
			DisableUpload: (hostCipdPlatform != l.cipdTargetPlatform),
		},
	}
	spec.ProtoMerge(g.CIPD, create.CIPD)

	if def.Spec.Upload.GetUniversal() && l.cipdTargetPlatform != "linux-amd64" {
		// 3pp recipe claims universal package will always come from linux-amd64
		// for consistency.
		// Append target platform to cipd version if this is a universal package
		// but not targeting linux-amd64. This ensures that the behaviour is same
		// from the user's perspective. Package with "correct" version tag (without
		// platform) is always from linux-amd64.
		// Also clear all extra refs & tags for same reason.
		g.CIPD.Version += "-" + l.cipdTargetPlatform
		g.CIPD.Tags = nil
		g.CIPD.Refs = nil
	}

	switch hostCipdPlatform {
	case "mac-amd64":
		g.Env.Set("MACOSX_DEPLOYMENT_TARGET", "10.10")
	case "mac-arm64":
		g.Env.Set("MACOSX_DEPLOYMENT_TARGET", "11.0")
	}

	l.pkgs[pkgCacheKey] = g
	return g, nil
}
