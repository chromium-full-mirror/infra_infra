// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spec

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/system/filesystem"
	"go.chromium.org/luci/common/testing/assertions"

	"infra/tools/pkgbuild/pkg/stdenv"
)

func TestCreateParser(t *testing.T) {
	Convey("singe create", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &Spec_Create_Build{},
			},
		})
		So(err, ShouldBeNil)
		So(p.create, assertions.ShouldResembleProto, &Spec_Create{
			Source: &Spec_Create_Source{
				Method: &Spec_Create_Source_Url{
					Url: &UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			},
			Build: &Spec_Create_Build{},
		})
	})

	Convey("multiple create", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						},
					},
					UnpackArchive: true,
				},
				Build: &Spec_Create_Build{},
			},
			{
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							Version: "1.2.12",
						},
					},
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
			},
		})
		So(err, ShouldBeNil)
		So(p.create, assertions.ShouldResembleProto, &Spec_Create{
			Source: &Spec_Create_Source{
				Method: &Spec_Create_Source_Url{
					Url: &UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			},
			Build: &Spec_Create_Build{},
		})
	})

	Convey("match platform", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				PlatformRe: "linux-.*",
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &Spec_Create_Build{},
			},
			{
				PlatformRe:  "unknown-.*",
				Unsupported: true,
			},
		})
		So(err, ShouldBeNil)
		So(p.create, assertions.ShouldResembleProto, &Spec_Create{
			Source: &Spec_Create_Source{
				Method: &Spec_Create_Source_Url{
					Url: &UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			},
			Build: &Spec_Create_Build{},
		})
	})

	Convey("unsupported platform explicit", t, func() {
		_, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Unsupported: true,
			},
			{
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &Spec_Create_Build{},
			},
		})
		So(err, ShouldEqual, ErrPackageNotAvailable)
	})

	Convey("unsupported platform implicit", t, func() {
		_, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				PlatformRe: "unknown-.*",
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &Spec_Create_Build{},
			},
		})
		So(err, ShouldEqual, ErrPackageNotAvailable)
	})

	Convey("merge values", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Source: &Spec_Create_Source{
					Method: &Spec_Create_Source_Url{
						Url: &UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					PatchDir:       []string{"patches1", "patches2"},
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &Spec_Create_Build{},
			},
			{
				Source: &Spec_Create_Source{
					PatchDir:       []string{"patches1"},
					CpeBaseAddress: "cpe:/a:zlib:zlib1",
				},
			},
		})
		So(err, ShouldBeNil)
		So(p.create, assertions.ShouldResembleProto, &Spec_Create{
			Source: &Spec_Create_Source{
				Method: &Spec_Create_Source_Url{
					Url: &UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				PatchDir:       []string{"patches1"},
				CpeBaseAddress: "cpe:/a:zlib:zlib1",
			},
			Build: &Spec_Create_Build{},
		})
	})
}

func TestParseSource(t *testing.T) {
	Convey("url", t, func() {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &Spec{
				Create: []*Spec_Create{
					{
						Source: &Spec_Create_Source{
							Method: &Spec_Create_Source_Url{
								Url: &UrlSource{
									DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
									Version:     "1.2.12",
								},
							},
							UnpackArchive:  true,
							CpeBaseAddress: "cpe:/a:zlib:zlib",
						},
						Build: &Spec_Create_Build{},
					},
				},
			},
		}
		p, err := newCreateParser("linux-amd64", def.Spec.Create)
		So(err, ShouldBeNil)
		err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
		So(err, ShouldBeNil)
		So(p.Source, ShouldEqual, &stdenv.SourceURLs{
			URLs: []stdenv.SourceURL{
				{URL: "https://zlib.net/fossils/zlib-1.2.12.tar.gz", Filename: "raw_source_0.tar.gz"},
			},
			CIPDName: "pkg_prefix/src_prefix/url/pkg_name/linux-amd64",
			Version:  "3@1.2.12",
		})
		So(p.Enviroments.Get("_3PP_UNPACK_ARCHIVE"), ShouldEqual, "1")
	})
	Convey("git", t, func() {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &Spec{
				Create: []*Spec_Create{
					{
						Source: &Spec_Create_Source{
							Method: &Spec_Create_Source_Git{
								Git: &GitSource{
									Repo:       "https://chromium.googlesource.com/external/github.com/ninja-build/Ninja",
									TagPattern: "v%s",
								},
							},
						},
						Build: &Spec_Create_Build{},
					},
				},
			},
		}
		p, err := newCreateParser("linux-amd64", def.Spec.Create)
		So(err, ShouldBeNil)
		err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
		So(err, ShouldBeNil)
		So(p.Source, ShouldEqual, &stdenv.SourceGit{
			URL: "https://chromium.googlesource.com/external/github.com/ninja-build/Ninja",
			Ref: "commit",

			CIPDName: "pkg_prefix/src_prefix/git/github.com/ninja-build/ninja",
			Version:  "3@git-tag",
		})
	})
	Convey("script", t, func() {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &Spec{
				Create: []*Spec_Create{
					{
						Source: &Spec_Create_Source{
							Method: &Spec_Create_Source_Script{
								Script: &ScriptSource{
									Name: []string{"fetch.py"},
								},
							},
						},
						Build: &Spec_Create_Build{},
					},
				},
			},
		}
		p, err := newCreateParser("linux-amd64", def.Spec.Create)
		So(err, ShouldBeNil)
		err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
		So(err, ShouldBeNil)
		So(p.Source, ShouldEqual, &stdenv.SourceURLs{
			URLs: []stdenv.SourceURL{
				{URL: "url1", Filename: "name1"},
				{URL: "url2", Filename: "name2"},
			},
			CIPDName: "pkg_prefix/src_prefix/script/pkg_name/linux-amd64",
			Version:  "3@script-version",
		})
	})
	Convey("version", t, func() {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &Spec{
				Create: []*Spec_Create{
					{
						Source: &Spec_Create_Source{
							Method: &Spec_Create_Source_Url{
								Url: &UrlSource{
									DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
									Version:     "1.2.12",
								},
							},
							PatchVersion: "chromium.1",
						},
						Build: &Spec_Create_Build{},
					},
				},
			},
		}
		p, err := newCreateParser("linux-amd64", def.Spec.Create)
		So(err, ShouldBeNil)
		err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
		So(err, ShouldBeNil)
		So(p.SourceVersion, ShouldEqual, "1.2.12")
		So(p.PatchVersion, ShouldEqual, "chromium.1")
		So(p.CIPDVersion(), ShouldEqual, "3@1.2.12.chromium.1")
	})
}

type MockSourceResolver struct{}

func (*MockSourceResolver) ResolveGitSource(git *GitSource) (GitSourceInfo, error) {
	return GitSourceInfo{
		Tag:    "git-tag",
		Commit: "commit",
	}, nil
}
func (*MockSourceResolver) ResolveScriptSource(cipdHostPlatform, dir string, script *ScriptSource) (ScriptSourceInfo, error) {
	return ScriptSourceInfo{
		Version: "script-version",
		URL:     []string{"url1", "url2"},
		Name:    []string{"name1", "name2"},
	}, nil
}

func MockSpecLoaderConfig() *SpecLoaderConfig {
	return &SpecLoaderConfig{
		CIPDPackagePrefix:     "mock",
		CIPDSourceCachePrefix: "sources",
		SourceResolver:        &MockSourceResolver{},
	}
}

func TestFindPatch(t *testing.T) {
	dir := t.TempDir()
	for _, pdir := range []string{"patches1", "patches2"} {
		if err := os.MkdirAll(filepath.Join(dir, pdir), fs.ModePerm); err != nil {
			t.Fatal(err)
		}
		if err := filesystem.Touch(filepath.Join(dir, pdir, "02-file1"), time.Now(), fs.ModePerm); err != nil {
			t.Fatal(err)
		}
		if err := filesystem.Touch(filepath.Join(dir, pdir, "01-file2"), time.Now(), fs.ModePerm); err != nil {
			t.Fatal(err)
		}
	}

	Convey("single dir", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Source: &Spec_Create_Source{
					PatchDir: []string{"patches1"},
				},
				Build: &Spec_Create_Build{},
			},
		})
		So(err, ShouldBeNil)
		err = p.FindPatches("something", dir)
		So(err, ShouldBeNil)
		So(p.Patches, ShouldEqual, []string{
			filepath.Join("{{.something}}", "patches1", "01-file2"),
			filepath.Join("{{.something}}", "patches1", "02-file1"),
		})
	})

	Convey("multiple dir", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Source: &Spec_Create_Source{
					PatchDir: []string{"patches1", "patches2"},
				},
				Build: &Spec_Create_Build{},
			},
		})
		So(err, ShouldBeNil)
		err = p.FindPatches("something", dir)
		So(err, ShouldBeNil)
		So(p.Patches, ShouldEqual, []string{
			filepath.Join("{{.something}}", "patches1", "01-file2"),
			filepath.Join("{{.something}}", "patches1", "02-file1"),
			filepath.Join("{{.something}}", "patches2", "01-file2"),
			filepath.Join("{{.something}}", "patches2", "02-file1"),
		})
	})
}

func TestParseBuilder(t *testing.T) {
	Convey("default", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Build: &Spec_Create_Build{},
			},
		})
		So(err, ShouldBeNil)
		err = p.ParseBuilder()
		So(err, ShouldBeNil)
		So(p.Installer, ShouldEqual, `["install.sh"]`)
	})
	Convey("customize", t, func() {
		p, err := newCreateParser("linux-amd64", []*Spec_Create{
			{
				Build: &Spec_Create_Build{
					Install: []string{"install.py"},
				},
			},
		})
		So(err, ShouldBeNil)
		err = p.ParseBuilder()
		So(err, ShouldBeNil)
		So(p.Installer, ShouldEqual, `["install.py"]`)
	})
}

func TestParsePackage(t *testing.T) {
	Convey("package", t, func() {
		create := &Spec_Create{
			Source: &Spec_Create_Source{
				Method: &Spec_Create_Source_Url{
					Url: &UrlSource{Version: "1.2.12.xxx.1-rc1"},
				},
			},
		}

		def := &PackageDef{packageName: "pkg_name", Spec: &Spec{Create: []*Spec_Create{create}}}

		Convey("ok", func() {
			create.Package = &Spec_Create_Package{}

			p, err := newCreateParser("linux-amd64", def.Spec.Create)
			So(err, ShouldBeNil)
			err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
			So(err, ShouldBeNil)
			err = p.ParsePackage()
			So(err, ShouldBeNil)

			So(p.CIPD, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Version: "3@1.2.12.xxx.1-rc1",
				Refs:    []string{"latest"},
			})
		})

		Convey("install mode", func() {
			create.Package = &Spec_Create_Package{InstallMode: Spec_Create_Package_symlink}

			p, err := newCreateParser("linux-amd64", def.Spec.Create)
			So(err, ShouldBeNil)
			err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
			So(err, ShouldBeNil)
			err = p.ParsePackage()
			So(err, ShouldBeNil)

			So(p.CIPD, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Version:     "3@1.2.12.xxx.1-rc1",
				InstallMode: core.Action_Metadata_CIPD_symlink,
				Refs:        []string{"latest"},
			})
		})

		Convey("version file", func() {
			create.Package = &Spec_Create_Package{VersionFile: "something.version"}

			p, err := newCreateParser("linux-amd64", def.Spec.Create)
			So(err, ShouldBeNil)
			err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
			So(err, ShouldBeNil)
			err = p.ParsePackage()
			So(err, ShouldBeNil)

			So(p.CIPD, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Version:     "3@1.2.12.xxx.1-rc1",
				Refs:        []string{"latest"},
				VersionFile: "something.version",
			})
		})

		Convey("alternative version", func() {
			create.Source.PatchVersion = "chromium.1"
			create.Package = &Spec_Create_Package{AlterVersionRe: "(.*)\\.xxx\\.\\d*(.*)", AlterVersionReplace: "\\1\\2"}

			p, err := newCreateParser("linux-amd64", def.Spec.Create)
			So(err, ShouldBeNil)
			err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
			So(err, ShouldBeNil)
			err = p.ParsePackage()
			So(err, ShouldBeNil)

			So(p.SourceVersion, ShouldEqual, "1.2.12-rc1")
			So(p.PatchVersion, ShouldEqual, "chromium.1")
			So(p.CIPDVersion(), ShouldEqual, "3@1.2.12-rc1.chromium.1")
			So(p.CIPD, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Version: "3@1.2.12-rc1.chromium.1",
				Refs:    []string{"latest"},
				Tags:    []string{"real_version:1.2.12.xxx.1-rc1"},
			})
		})

		Convey("disable latest", func() {
			create.Package = &Spec_Create_Package{DisableLatestRef: true}

			p, err := newCreateParser("linux-amd64", def.Spec.Create)
			So(err, ShouldBeNil)
			err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
			So(err, ShouldBeNil)
			err = p.ParsePackage()
			So(err, ShouldBeNil)

			So(p.CIPD, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Version: "3@1.2.12.xxx.1-rc1",
			})
		})

		Convey("additional ref", func() {
			create.Package = &Spec_Create_Package{AdditionalRef: []string{"ref1"}}

			p, err := newCreateParser("linux-amd64", def.Spec.Create)
			So(err, ShouldBeNil)
			err = p.ParseSource(def, "pkg_prefix", "src_prefix", "linux-amd64", &MockSourceResolver{})
			So(err, ShouldBeNil)
			err = p.ParsePackage()
			So(err, ShouldBeNil)

			So(p.CIPD, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Version: "3@1.2.12.xxx.1-rc1",
				Refs:    []string{"ref1", "latest"},
			})
		})
	})
}

func TestLoadDependencies(t *testing.T) {
	Convey("loader", t, func() {
		cfg := DefaultSpecLoaderConfig("", "linux-amd64")
		cfg.SourceResolver = &MockSourceResolver{}
		root, err := filepath.Abs("testdata")
		So(err, ShouldBeNil)
		l, err := NewSpecLoader(root, cfg)
		So(err, ShouldBeNil)

		So(l.ListAllByFullName(), ShouldEqual, []string{
			"tests/unavailable_arm64",
			"tests/unavailable_depends",
			"tools/ninja",
			"tools/re2c",
		})

		plats := generators.Platforms{
			Build:  generators.NewPlatform("linux", "amd64"),
			Host:   generators.NewPlatform("linux", "amd64"),
			Target: generators.NewPlatform("linux", "amd64"),
		}

		Convey("no install", func() {
			p, err := newCreateParser("linux-amd64", []*Spec_Create{{}})
			So(err, ShouldBeNil)
			err = p.ParseBuilder()
			So(err, ShouldBeNil)
			So(p.Enviroments.Get("_3PP_NO_INSTALL"), ShouldEqual, "1")
		})
		Convey("tool", func() {
			p, err := newCreateParser("linux-arm64", []*Spec_Create{
				{
					Build: &Spec_Create_Build{
						Tool: []string{"tools/ninja"},
					},
				},
			})
			So(err, ShouldBeNil)
			err = p.LoadDependencies("linux-amd64", l)
			So(err, ShouldBeNil)

			a, err := p.Dependencies[0].Generate(context.Background(), plats)
			So(err, ShouldBeNil)
			So(a.Name, ShouldEqual, "ninja")
			So(a.Metadata.Cipd, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Name:    "tools/ninja/linux-amd64",
				Version: "3@git-tag.chromium.4",
				Refs:    []string{"latest"},
			})
		})
		Convey("dep", func() {
			p, err := newCreateParser("linux-arm64", []*Spec_Create{
				{
					Build: &Spec_Create_Build{
						Dep: []string{"tools/ninja"},
					},
				},
			})
			So(err, ShouldBeNil)
			err = p.LoadDependencies("linux-amd64", l)
			So(err, ShouldBeNil)

			a, err := p.Dependencies[0].Generate(context.Background(), plats)
			So(err, ShouldBeNil)
			So(a.Name, ShouldEqual, "ninja")
			So(a.Metadata.Cipd, assertions.ShouldResembleProto, &core.Action_Metadata_CIPD{
				Name:    "tools/ninja/linux-arm64",
				Version: "3@git-tag.chromium.4",
				Refs:    []string{"latest"},

				// Avoid uploading linux-arm64 package from linux-amd64 builder
				DisableUpload: true,
			})
		})
		Convey("pin", func() {
			p, err := newCreateParser("linux-arm64", []*Spec_Create{
				{
					Build: &Spec_Create_Build{
						Tool: []string{"tools/ninja@version1"},
						Dep:  []string{"tools/ninja@version2"},
					},
				},
			})
			So(err, ShouldBeNil)
			err = p.LoadDependencies("linux-amd64", l)
			So(err, ShouldBeNil)

			a, err := p.Dependencies[0].Generate(context.Background(), plats)
			So(err, ShouldBeNil)
			So(a.Name, ShouldEqual, "ninja")
			So(a.Spec, assertions.ShouldResembleProto, &core.Action_Cipd{
				Cipd: &core.ActionCIPDExport{
					EnsureFile: "tools/ninja/linux-amd64  version:version1\n",
				},
			})
			a, err = p.Dependencies[1].Generate(context.Background(), plats)
			So(err, ShouldBeNil)
			So(a.Name, ShouldEqual, "ninja")
			So(a.Spec, assertions.ShouldResembleProto, &core.Action_Cipd{
				Cipd: &core.ActionCIPDExport{
					EnsureFile: "tools/ninja/linux-arm64  version:version2\n",
				},
			})
		})
		Convey("unavailable", func() {
			p, err := newCreateParser("linux-arm64", []*Spec_Create{
				{
					Build: &Spec_Create_Build{
						Dep: []string{"tests/unavailable_arm64"},
					},
				},
			})
			So(err, ShouldBeNil)
			err = p.LoadDependencies("linux-amd64", l)
			So(errors.Is(err, ErrPackageNotAvailable), ShouldBeTrue)
		})
	})
}
