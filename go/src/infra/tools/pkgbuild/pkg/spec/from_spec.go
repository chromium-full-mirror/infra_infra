// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package spec

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/system/environ"

	"infra/tools/pkgbuild/pkg/stdenv"
)

//go:embed all:from_spec/build-support
var fromSpecEmbed embed.FS
var fromSpecGen = generators.InitEmbeddedFS(
	"from_spec_support", fromSpecEmbed,
).SubDir("from_spec")

// Load 3pp Spec and convert it into a stdenv generator.
type SpecLoader struct {
	cipdPackagePrefix     string
	cipdSourceCachePrefix string
	cipdTargetPlatform    string
	sourceResolver        SourceResolver

	supportFiles generators.Generator

	// Mapping packages's full name to definition.
	specs map[string]*PackageDef
	pkgs  map[string]*stdenv.Generator
}

type SpecLoaderConfig struct {
	CIPDPackagePrefix     string
	CIPDSourceCachePrefix string
	CIPDTargetPlatform    string
	SourceResolver        SourceResolver
}

func DefaultSpecLoaderConfig(vpythonSpecPath, target string) *SpecLoaderConfig {
	return &SpecLoaderConfig{
		CIPDPackagePrefix:     "",
		CIPDSourceCachePrefix: "sources",
		CIPDTargetPlatform:    target,
		SourceResolver: &DefaultSourceResolver{
			VPythonSpecPath: vpythonSpecPath,
		},
	}
}

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
		pkgs:  make(map[string]*stdenv.Generator),
	}, nil
}

// List all loaded specs' full names by alphabetical order.
func (l *SpecLoader) ListAllByFullName() (names []string) {
	for name := range l.specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return
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
	if err := create.ParseSource(def, l.cipdPackagePrefix, l.cipdSourceCachePrefix, hostCipdPlatform, l.sourceResolver); err != nil {
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
	if err := create.ParseVerifier(); err != nil {
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
			Name:    def.CIPDPath(l.cipdPackagePrefix, hostCipdPlatform),
			Version: create.Version,

			// Avoid uploading package for platform not matching the target.
			DisableUpload: (hostCipdPlatform != l.cipdTargetPlatform),
		},
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

// A parser for Spec_Create spec. It converts the merged create section in the
// spec to information we need for constructing a stdenv generator.
type createParser struct {
	Source       stdenv.Source
	Version      string
	Patches      []string
	Installer    string
	Tester       string
	Dependencies []generators.Dependency
	Enviroments  environ.Env

	host   string
	create *Spec_Create
}

var (
	ErrPackageNotAvailable = errors.New("package not available on the target platform")
)

// Merge create specs for the host platform. Return a parser with the merged
// spec.
func newCreateParser(host string, creates []*Spec_Create) (*createParser, error) {
	p := &createParser{
		host:        host,
		Enviroments: environ.New(nil),
	}

	for _, c := range creates {
		if c.GetPlatformRe() != "" {
			matched, err := regexp.MatchString(c.GetPlatformRe(), host)
			if err != nil {
				return nil, err
			}
			if !matched {
				continue
			}
		}

		if c.GetUnsupported() == true {
			return nil, ErrPackageNotAvailable
		}

		if p.create == nil {
			p.create = &Spec_Create{}
		}
		protoMerge(p.create, c)
	}

	if p.create == nil {
		return nil, ErrPackageNotAvailable
	}

	// To make this create rule self-consistent instead of just having the last
	// platform_re to be applied.
	p.create.PlatformRe = ""

	return p, nil
}

// Extract the cache path from URL
func gitCachePath(url string) string {
	url = strings.TrimPrefix(url, "https://chromium.googlesource.com/external/")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.ToLower(url)
	return path.Clean(url)
}

// Fetch the latest version and convert source section in create to source
// definition in stdenv. Versions are fetched during the parsing so the source
// definition can be deterministic.
// Source may be cached based on its CIPDName.
func (p *createParser) ParseSource(def *PackageDef, packagePrefix, sourceCachePrefix, hostCipdPlatform string, resolver SourceResolver) error {
	source := p.create.GetSource()

	// Subdir is only used by go packages before go module and can be easily
	// replaced by a simple move after unpack stage.
	if source.GetSubdir() != "" {
		return fmt.Errorf("source.subdir not supported.")
	}

	// Git used to be unpacked - which means we always want to unpack the source
	// if it's a git method.
	if source.GetUnpackArchive() || source.GetGit() != nil {
		p.Enviroments.Set("_3PP_UNPACK_ARCHIVE", "1")
	}

	if source.GetNoArchivePrune() {
		p.Enviroments.Set("_3PP_NO_ARCHIVE_PRUNE", "1")
	}

	s, v, err := func() (stdenv.Source, string, error) {
		switch source.GetMethod().(type) {
		case *Spec_Create_Source_Git:
			s := source.GetGit()
			info, err := resolver.ResolveGitSource(s)
			if err != nil {
				return nil, "", fmt.Errorf("failed to resolve git ref: %w", err)
			}
			return &stdenv.SourceGit{
				URL: s.GetRepo(),
				Ref: info.Commit,

				CIPDName: path.Join(packagePrefix, sourceCachePrefix, "git", gitCachePath(s.Repo)),
				Version:  fmt.Sprintf("3@%s", info.Tag),
			}, info.Tag, nil
		case *Spec_Create_Source_Url:
			s := source.GetUrl()
			ext := s.GetExtension()
			if ext == "" {
				ext = ".tar.gz"
			}
			return &stdenv.SourceURLs{
				URLs: []stdenv.SourceURL{
					{URL: s.GetDownloadUrl(), Filename: fmt.Sprintf("raw_source_0%s", ext)},
				},

				CIPDName: path.Join(packagePrefix, sourceCachePrefix, "url", def.FullNameWithOverride(), p.host),
				Version:  fmt.Sprintf("3@%s", s.Version),
			}, s.Version, nil
		case *Spec_Create_Source_Cipd:
			// source.GetCipd()
			panic("unimplemented")
		case *Spec_Create_Source_Script:
			s := source.GetScript()
			info, err := resolver.ResolveScriptSource(hostCipdPlatform, def.Dir, s)
			if err != nil {
				return nil, "", fmt.Errorf("failed to resolve latest: %w", err)
			}

			if s.UseFetchCheckoutWorkflow {
				p.Enviroments.Set("_3PP_FETCH_CHECKOUT_WORKFLOW", "1")
				fetch, err := json.Marshal(s.GetName())
				if err != nil {
					return nil, "", err
				}
				p.Enviroments.Set("fromSpecFetch", string(fetch))
				return nil, info.Version, nil
			}

			// info.Name is optional.
			names := info.Name
			if len(names) == 0 {
				for i := range info.URL {
					names = append(names, fmt.Sprintf("raw_source_%d%s", i, info.Ext))
				}
			}

			// Number of names must equal to urls.
			if len(names) != len(info.URL) {
				return nil, "", fmt.Errorf("failed to get download urls: number of urls should be equal to artifacts: %w", err)
			}

			var urls []stdenv.SourceURL
			for i, url := range info.URL {
				urls = append(urls, stdenv.SourceURL{
					URL:      url,
					Filename: names[i],
				})
			}

			return &stdenv.SourceURLs{
				URLs: urls,

				CIPDName: path.Join(packagePrefix, sourceCachePrefix, "script", def.FullNameWithOverride(), p.host),
				Version:  fmt.Sprintf("3@%s", info.Version),
			}, info.Version, nil
		}
		return nil, "", fmt.Errorf("unknown source type from spec")
	}()
	if err != nil {
		return err
	}

	p.Enviroments.Set("_3PP_VERSION", v)
	if pv := p.create.GetSource().GetPatchVersion(); pv != "" {
		p.Enviroments.Set("_3PP_PATCH_VERSION", pv)
		v = v + "." + pv
	}
	p.Version = v
	p.Source = s

	return nil
}

func (p *createParser) FindPatches(name, dir string) error {
	source := p.create.GetSource()

	prefix := fmt.Sprintf("{{.%s}}", name)
	for _, pdir := range source.GetPatchDir() {
		dir, err := os.ReadDir(filepath.Join(dir, pdir))
		if err != nil {
			return err
		}

		for _, d := range dir {
			p.Patches = append(p.Patches, filepath.Join(prefix, pdir, d.Name()))
		}
	}

	return nil
}

func (p *createParser) ParseBuilder() error {
	build := p.create.GetBuild()
	if build == nil {
		p.Enviroments.Set("_3PP_NO_INSTALL", "1")
		return nil
	}

	installArgs := build.GetInstall()
	if len(installArgs) == 0 {
		installArgs = []string{"install.sh"}
	}

	installer, err := json.Marshal(installArgs)
	if err != nil {
		return err
	}
	p.Installer = string(installer)

	return nil
}

func (p *createParser) ParseVerifier() error {
	verify := p.create.GetVerify()

	testArgs := verify.GetTest()
	if len(testArgs) == 0 {
		return nil
	}

	tester, err := json.Marshal(testArgs)
	if err != nil {
		return err
	}
	p.Tester = string(tester)

	return nil
}

func (p *createParser) LoadDependencies(buildCipdPlatform string, l *SpecLoader) error {
	build := p.create.GetBuild()
	if build == nil {
		return nil
	}

	fromSpecByURI := func(dep, hostCipdPlatform string) (generators.Generator, error) {
		// tools/go117@1.17.10
		var name, ver string
		ss := strings.SplitN(dep, "@", 2)
		name = ss[0]
		if len(ss) == 2 {
			ver = ss[1]
		}

		g, err := l.FromSpec(name, buildCipdPlatform, hostCipdPlatform)
		if err != nil {
			return nil, fmt.Errorf("failed to load dependency %s on %s: %w", name, hostCipdPlatform, err)
		}
		if ver != "" && ver != g.CIPD.Version {
			return &generators.CIPDExport{
				Name: g.Name,
				Metadata: &core.Action_Metadata{
					Luciexe: &core.Action_Metadata_LUCIExe{
						StepName: fmt.Sprintf("%s@%s:%s from cipd", g.Name, ver, hostCipdPlatform),
					},
				},
				Ensure: ensure.File{
					PackagesBySubdir: map[string]ensure.PackageSlice{
						"": {
							{PackageTemplate: g.CIPD.Name, UnresolvedVersion: fmt.Sprintf("version:%s", ver)},
						},
					},
				},
			}, nil
		}

		return g, nil
	}

	for _, dep := range build.GetTool() {
		g, err := fromSpecByURI(dep, buildCipdPlatform)
		if err != nil {
			return err
		}
		p.Dependencies = append(p.Dependencies, generators.Dependency{
			Type:      generators.DepsBuildHost,
			Generator: g,
		})
	}
	for _, dep := range build.GetDep() {
		g, err := fromSpecByURI(dep, p.host)
		if err != nil {
			return err
		}
		p.Dependencies = append(p.Dependencies, generators.Dependency{
			Type:      generators.DepsHostTarget,
			Generator: g,
		})
	}

	return nil
}
