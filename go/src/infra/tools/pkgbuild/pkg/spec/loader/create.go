// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipd/client/cipd/template"
	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/system/environ"

	"infra/tools/pkgbuild/pkg/spec"
	"infra/tools/pkgbuild/pkg/spec/source"
	"infra/tools/pkgbuild/pkg/stdenv"
)

// A parser for Spec_Create spec. It converts the merged create section in the
// spec to information we need for constructing a stdenv generator.
type createParser struct {
	Source        stdenv.Source
	SourceVersion string
	PatchVersion  string
	Patches       []string
	Installer     string
	Tester        string
	Dependencies  []generators.Dependency
	Enviroments   environ.Env
	CIPD          *core.Action_Metadata_CIPD

	host   string
	create *spec.Spec_Create
}

// Merge create specs for the host platform. Return a parser with the merged
// spec.
func newCreateParser(host string, creates []*spec.Spec_Create) (*createParser, error) {
	p := &createParser{
		host:        host,
		Enviroments: environ.New(nil),
	}

	c, err := spec.MergeCreates(host, creates)
	if err != nil {
		return nil, err
	}
	p.create = c

	return p, nil
}

// Extract the cache path from URL
// TODO(fancl): support sso
func gitCachePath(url string) string {
	url = strings.TrimPrefix(url, "https://chromium.googlesource.com/external/")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.ToLower(url)
	return path.Clean(url)
}

// Convert source section in create to source definition in stdenv.
// Source may be cached based on its CIPDName.
func (p *createParser) ParseSource(def *PackageDef, info *source.SourceInfo, cachePrefix string) error {
	src := p.create.GetSource()

	// Subdir is only used by go packages before go module and can be easily
	// replaced by a simple move after unpack stage.
	if src.GetSubdir() != "" {
		return fmt.Errorf("source.subdir not supported")
	}

	if src.GetUnpackArchive() {
		p.Enviroments.Set("_3PP_UNPACK_ARCHIVE", "1")
	}

	if src.GetNoArchivePrune() {
		p.Enviroments.Set("_3PP_NO_ARCHIVE_PRUNE", "1")
	}

	s, err := func() (stdenv.Source, error) {
		switch info.Source.(type) {
		case *source.SourceInfo_Git_:
			git := info.GetGit()

			// We always want to unpack the source if it's a git method.
			p.Enviroments.Set("_3PP_UNPACK_ARCHIVE", "1")

			return &stdenv.SourceGit{
				URL: git.Url,
				Ref: git.Commit,

				CIPDName: path.Join(cachePrefix, "git", gitCachePath(git.Url)),
				Version:  cipdVersionEpoch + info.Version,
			}, nil
		case *source.SourceInfo_Http:
			http := info.GetHttp()

			// info.Name is optional.
			names := http.Name
			if len(names) == 0 {
				for i := range http.Url {
					names = append(names, fmt.Sprintf("raw_source_%d%s", i, http.Ext))
				}
			}

			// Number of names must equal to urls.
			if len(names) != len(http.Url) {
				return nil, fmt.Errorf("failed to get download urls: number of urls should be equal to artifacts")
			}

			var urls []stdenv.SourceURL
			for i, url := range http.Url {
				urls = append(urls, stdenv.SourceURL{
					URL:      url,
					Filename: names[i],
				})
			}

			return &stdenv.SourceURLs{
				URLs: urls,

				CIPDName: path.Join(cachePrefix, "http", def.FullNameWithOverride()),
				Version:  cipdVersionEpoch + info.Version,
			}, nil
		case nil:
			// Fetch checkout workflow
			s := src.GetScript()
			if s.GetUseFetchCheckoutWorkflow() {
				p.Enviroments.Set("_3PP_FETCH_CHECKOUT_WORKFLOW", "1")
				fetch, err := json.Marshal(s.GetName())
				if err != nil {
					return nil, err
				}
				p.Enviroments.Set("fromSpecFetch", string(fetch))
			}
			return nil, nil
		default:
			return nil, fmt.Errorf("unknown source type from spec")
		}
	}()
	if err != nil {
		return err
	}

	p.SourceVersion = info.Version
	p.PatchVersion = src.PatchVersion

	p.Source = s
	return nil
}

// CIPDVersion returns the version tag used in cipd.
func (p *createParser) CIPDVersion() string {
	if p.PatchVersion == "" {
		return cipdVersionEpoch + p.SourceVersion
	}
	return cipdVersionEpoch + p.SourceVersion + "." + p.PatchVersion
}

// FindPatches searched PatchDir for source patches.
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

// ParseBuilder parses build field for install script.
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

// ParseVerifier parses verify field for test script.
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

// ParsePackage parses package field for customizing cipd package.
func (p *createParser) ParsePackage() error {
	pkgSpec := p.create.Package
	if pkgSpec == nil {
		// default package spec
		pkgSpec = &spec.Spec_Create_Package{
			InstallMode:      spec.Spec_Create_Package_copy,
			DisableLatestRef: false,
		}
	}

	var tags []string
	if pkgSpec.AlterVersionRe != "" {
		tags = append(tags, "real_version:"+p.SourceVersion)

		re, err := regexp.Compile(pkgSpec.AlterVersionRe)
		if err != nil {
			return err
		}
		// Note: Python re package uses "\" as group quote while golang uses "$".
		// We don't expect "\" appearing in the altered version anyway so replacing
		// all "\" with "$" should be safe.
		repl := strings.ReplaceAll(pkgSpec.AlterVersionReplace, "\\", "$")
		p.SourceVersion = re.ReplaceAllString(p.SourceVersion, repl)
	}

	p.CIPD = &core.Action_Metadata_CIPD{
		Refs:        pkgSpec.AdditionalRef,
		Tags:        tags,
		Version:     p.CIPDVersion(),
		VersionFile: pkgSpec.VersionFile,
	}

	if !pkgSpec.DisableLatestRef {
		p.CIPD.Refs = append(p.CIPD.Refs, "latest")
	}

	switch pkgSpec.InstallMode {
	case spec.Spec_Create_Package_copy:
		p.CIPD.InstallMode = core.Action_Metadata_CIPD_copy
	case spec.Spec_Create_Package_symlink:
		p.CIPD.InstallMode = core.Action_Metadata_CIPD_symlink
	default:
		return fmt.Errorf("unknown install mode: %v", pkgSpec)
	}

	return nil
}

// LoadDependencies converts all dependencies from specs to generators
// recursively.
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

// ParseExternalDependencies is parsing Spec.Create.Build.External{Tool,Dep}
// and converting them to CIPDExport generators.
func (p *createParser) ParseExternalDependencies(name, buildCipdPlatform string) error {
	build := p.create.GetBuild()
	if build == nil {
		return nil
	}

	cipdDep := func(dep, hostCipdPlatform string) (generators.Generator, error) {
		// infra/tools/foo@1.3.1
		var cipdName, ver string
		ss := strings.SplitN(dep, "@", 2)
		if len(ss) != 2 {
			return nil, fmt.Errorf("invalid external dependency (must be '<CIPD_PATH>@<VERSION>'): %s", dep)
		}
		cipdName, ver = ss[0], ss[1]

		cipdPlat, err := template.ParsePlatform(hostCipdPlatform)
		if err != nil {
			return nil, fmt.Errorf("invalid cipd platform: %w", err)
		}

		return &generators.CIPDExport{
			Name: name + "_dep",
			Metadata: &core.Action_Metadata{
				Luciexe: &core.Action_Metadata_LUCIExe{
					StepName: fmt.Sprintf("%s@%s:%s from cipd", cipdName, ver, hostCipdPlatform),
				},
			},
			Ensure: ensure.File{
				PackagesBySubdir: map[string]ensure.PackageSlice{
					"": {
						{PackageTemplate: cipdName, UnresolvedVersion: fmt.Sprintf("version:%s", ver)},
					},
				},
			},
			Expander: cipdPlat.Expander(),
		}, nil
	}

	for _, dep := range build.GetExternalTool() {
		g, err := cipdDep(dep, buildCipdPlatform)
		if err != nil {
			return err
		}
		p.Dependencies = append(p.Dependencies, generators.Dependency{
			Type:      generators.DepsBuildHost,
			Generator: g,
		})
	}
	for _, dep := range build.GetExternalDep() {
		g, err := cipdDep(dep, p.host)
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
