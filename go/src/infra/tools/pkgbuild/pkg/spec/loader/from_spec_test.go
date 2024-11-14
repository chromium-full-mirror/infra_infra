// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package loader

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipd/client/cipd/template"
	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/system/filesystem"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tools/pkgbuild/pkg/spec"
	"infra/tools/pkgbuild/pkg/spec/source"
	"infra/tools/pkgbuild/pkg/stdenv"
)

func TestCreateParser(t *testing.T) {
	ftt.Run("singe create", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &spec.Spec_Create_Build{},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.create, should.Match(&spec.Spec_Create{
			Source: &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			},
			Build: &spec.Spec_Create_Build{},
		}))
	})

	ftt.Run("multiple create", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						},
					},
					UnpackArchive: true,
				},
				Build: &spec.Spec_Create_Build{},
			},
			{
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							Version: "1.2.12",
						},
					},
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.create, should.Match(&spec.Spec_Create{
			Source: &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			},
			Build: &spec.Spec_Create_Build{},
		}))
	})

	ftt.Run("match platform", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				PlatformRe: "linux-.*",
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &spec.Spec_Create_Build{},
			},
			{
				PlatformRe:  "unknown-.*",
				Unsupported: true,
			},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.create, should.Match(&spec.Spec_Create{
			Source: &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				CpeBaseAddress: "cpe:/a:zlib:zlib",
			},
			Build: &spec.Spec_Create_Build{},
		}))
	})

	ftt.Run("unsupported platform explicit", t, func(t *ftt.Test) {
		_, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Unsupported: true,
			},
			{
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &spec.Spec_Create_Build{},
			},
		})
		assert.Loosely(t, err, should.Equal(spec.ErrPackageNotAvailable))
	})

	ftt.Run("unsupported platform implicit", t, func(t *ftt.Test) {
		_, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				PlatformRe: "unknown-.*",
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &spec.Spec_Create_Build{},
			},
		})
		assert.Loosely(t, err, should.Equal(spec.ErrPackageNotAvailable))
	})

	ftt.Run("merge values", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Source: &spec.Spec_Create_Source{
					Method: &spec.Spec_Create_Source_Url{
						Url: &spec.UrlSource{
							DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
							Version:     "1.2.12",
						},
					},
					UnpackArchive:  true,
					PatchDir:       []string{"patches1", "patches2"},
					CpeBaseAddress: "cpe:/a:zlib:zlib",
				},
				Build: &spec.Spec_Create_Build{},
			},
			{
				Source: &spec.Spec_Create_Source{
					PatchDir:       []string{"patches1"},
					CpeBaseAddress: "cpe:/a:zlib:zlib1",
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.create, should.Match(&spec.Spec_Create{
			Source: &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
				UnpackArchive:  true,
				PatchDir:       []string{"patches1"},
				CpeBaseAddress: "cpe:/a:zlib:zlib1",
			},
			Build: &spec.Spec_Create_Build{},
		}))
	})
}

func TestParseSource(t *testing.T) {
	parseSource := func(def *PackageDef) *createParser {
		ctx := context.Background()

		r := &MockSourceResolver{}
		sinfo, err := r.Resolve(ctx, "linux-amd64", "", def.Spec.Create[0].Source)
		assert.Loosely(t, err, should.BeNil)
		p, err := newCreateParser("linux-amd64", def.Spec.Create)
		assert.Loosely(t, err, should.BeNil)
		err = p.ParseSource(def, sinfo, "pkg_prefix/src_prefix")
		assert.Loosely(t, err, should.BeNil)
		return p
	}

	ftt.Run("url", t, func(t *ftt.Test) {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &spec.Spec{
				Create: []*spec.Spec_Create{
					{
						Source: &spec.Spec_Create_Source{
							Method: &spec.Spec_Create_Source_Url{
								Url: &spec.UrlSource{
									DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
									Version:     "1.2.12",
								},
							},
							UnpackArchive:  true,
							CpeBaseAddress: "cpe:/a:zlib:zlib",
						},
						Build: &spec.Spec_Create_Build{},
					},
				},
			},
		}
		p := parseSource(def)
		assert.Loosely(t, p.Source, should.Match(&stdenv.SourceURLs{
			URLs: []stdenv.SourceURL{
				{URL: "https://zlib.net/fossils/zlib-1.2.12.tar.gz", Filename: "raw_source_0.tar.gz"},
			},
			CIPDName: "pkg_prefix/src_prefix/http/pkg_name",
			Version:  "3@1.2.12",
		}))
		assert.Loosely(t, p.Enviroments.Get("_3PP_UNPACK_ARCHIVE"), should.Equal("1"))
	})
	ftt.Run("git", t, func(t *ftt.Test) {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &spec.Spec{
				Create: []*spec.Spec_Create{
					{
						Source: &spec.Spec_Create_Source{
							Method: &spec.Spec_Create_Source_Git{
								Git: &spec.GitSource{
									Repo:       "https://chromium.googlesource.com/external/github.com/ninja-build/Ninja",
									TagPattern: "v%s",
								},
							},
						},
						Build: &spec.Spec_Create_Build{},
					},
				},
			},
		}
		p := parseSource(def)
		assert.Loosely(t, p.Source, should.Match(&stdenv.SourceGit{
			URL: "https://chromium.googlesource.com/external/github.com/ninja-build/Ninja",
			Ref: "commit",

			CIPDName: "pkg_prefix/src_prefix/git/github.com/ninja-build/ninja",
			Version:  "3@git-tag",
		}))
	})
	ftt.Run("script", t, func(t *ftt.Test) {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &spec.Spec{
				Create: []*spec.Spec_Create{
					{
						Source: &spec.Spec_Create_Source{
							Method: &spec.Spec_Create_Source_Script{
								Script: &spec.ScriptSource{
									Name: []string{"fetch.py"},
								},
							},
						},
						Build: &spec.Spec_Create_Build{},
					},
				},
			},
		}
		p := parseSource(def)
		assert.Loosely(t, p.Source, should.Match(&stdenv.SourceURLs{
			URLs: []stdenv.SourceURL{
				{URL: "url1", Filename: "name1"},
				{URL: "url2", Filename: "name2"},
			},
			CIPDName: "pkg_prefix/src_prefix/http/pkg_name",
			Version:  "3@script-version",
		}))
	})
	ftt.Run("version", t, func(t *ftt.Test) {
		def := &PackageDef{
			packageName: "pkg_name",
			Spec: &spec.Spec{
				Create: []*spec.Spec_Create{
					{
						Source: &spec.Spec_Create_Source{
							Method: &spec.Spec_Create_Source_Url{
								Url: &spec.UrlSource{
									DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
									Version:     "1.2.12",
								},
							},
							PatchVersion: "chromium.1",
						},
						Build: &spec.Spec_Create_Build{},
					},
				},
			},
		}
		p := parseSource(def)
		assert.Loosely(t, p.SourceVersion, should.Equal("1.2.12"))
		assert.Loosely(t, p.PatchVersion, should.Equal("chromium.1"))
		assert.Loosely(t, p.CIPDVersion(), should.Equal("3@1.2.12.chromium.1"))
	})
}

type MockSourceResolver struct{}

func (*MockSourceResolver) Resolve(_ context.Context, plat, dir string, src *spec.Spec_Create_Source) (*source.SourceInfo, error) {
	switch src.Method.(type) {
	case *spec.Spec_Create_Source_Git:
		git := src.GetGit()
		return &source.SourceInfo{
			Version: "git-tag",
			Source: &source.SourceInfo_Git_{
				Git: &source.SourceInfo_Git{
					Url:    git.Repo,
					Commit: "commit",
				},
			},
		}, nil
	case *spec.Spec_Create_Source_Script:
		return &source.SourceInfo{
			Version: "script-version",
			Source: &source.SourceInfo_Http{
				Http: &source.SourceInfo_HTTP{
					Url:  []string{"url1", "url2"},
					Name: []string{"name1", "name2"},
				},
			},
		}, nil
	case *spec.Spec_Create_Source_Url:
		u := src.GetUrl()
		ext := u.Extension
		if ext == "" {
			ext = ".tar.gz"
		}
		return &source.SourceInfo{
			Version: u.Version,
			Source: &source.SourceInfo_Http{
				Http: &source.SourceInfo_HTTP{
					Url: []string{u.DownloadUrl},
					Ext: ext,
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("unkown source type")
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

	ftt.Run("single dir", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Source: &spec.Spec_Create_Source{
					PatchDir: []string{"patches1"},
				},
				Build: &spec.Spec_Create_Build{},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		err = p.FindPatches("something", dir)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.Patches, should.Match([]string{
			filepath.Join("{{.something}}", "patches1", "01-file2"),
			filepath.Join("{{.something}}", "patches1", "02-file1"),
		}))
	})

	ftt.Run("multiple dir", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Source: &spec.Spec_Create_Source{
					PatchDir: []string{"patches1", "patches2"},
				},
				Build: &spec.Spec_Create_Build{},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		err = p.FindPatches("something", dir)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.Patches, should.Match([]string{
			filepath.Join("{{.something}}", "patches1", "01-file2"),
			filepath.Join("{{.something}}", "patches1", "02-file1"),
			filepath.Join("{{.something}}", "patches2", "01-file2"),
			filepath.Join("{{.something}}", "patches2", "02-file1"),
		}))
	})
}

func TestParseBuilder(t *testing.T) {
	ftt.Run("default", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Build: &spec.Spec_Create_Build{},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		err = p.ParseBuilder()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.Installer, should.Equal(`["install.sh"]`))
	})
	ftt.Run("customize", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{
			{
				Build: &spec.Spec_Create_Build{
					Install: []string{"install.py"},
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		err = p.ParseBuilder()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.Installer, should.Equal(`["install.py"]`))
	})
}

func TestParsePackage(t *testing.T) {
	parsePackage := func(def *PackageDef) *createParser {
		ctx := context.Background()

		r := &MockSourceResolver{}
		sinfo, err := r.Resolve(ctx, "linux-amd64", "", def.Spec.Create[0].Source)
		assert.Loosely(t, err, should.BeNil)
		p, err := newCreateParser("linux-amd64", def.Spec.Create)
		assert.Loosely(t, err, should.BeNil)
		err = p.ParseSource(def, sinfo, "pkg_prefix/src_prefix")
		assert.Loosely(t, err, should.BeNil)
		err = p.ParsePackage()
		assert.Loosely(t, err, should.BeNil)
		return p
	}

	ftt.Run("package", t, func(t *ftt.Test) {
		create := &spec.Spec_Create{
			Source: &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{Version: "1.2.12.xxx.1-rc1"},
				},
			},
		}

		def := &PackageDef{packageName: "pkg_name", Spec: &spec.Spec{Create: []*spec.Spec_Create{create}}}

		t.Run("ok", func(t *ftt.Test) {
			create.Package = &spec.Spec_Create_Package{}

			p := parsePackage(def)

			assert.Loosely(t, p.CIPD, should.Match(&core.Action_Metadata_CIPD{
				Version: "3@1.2.12.xxx.1-rc1",
				Refs:    []string{"latest"},
			}))
		})

		t.Run("install mode", func(t *ftt.Test) {
			create.Package = &spec.Spec_Create_Package{InstallMode: spec.Spec_Create_Package_symlink}

			p := parsePackage(def)

			assert.Loosely(t, p.CIPD, should.Match(&core.Action_Metadata_CIPD{
				Version:     "3@1.2.12.xxx.1-rc1",
				InstallMode: core.Action_Metadata_CIPD_symlink,
				Refs:        []string{"latest"},
			}))
		})

		t.Run("version file", func(t *ftt.Test) {
			create.Package = &spec.Spec_Create_Package{VersionFile: "something.version"}

			p := parsePackage(def)

			assert.Loosely(t, p.CIPD, should.Match(&core.Action_Metadata_CIPD{
				Version:     "3@1.2.12.xxx.1-rc1",
				Refs:        []string{"latest"},
				VersionFile: "something.version",
			}))
		})

		t.Run("alternative version", func(t *ftt.Test) {
			create.Source.PatchVersion = "chromium.1"
			create.Package = &spec.Spec_Create_Package{AlterVersionRe: "(.*)\\.xxx\\.\\d*(.*)", AlterVersionReplace: "\\1\\2"}

			p := parsePackage(def)

			assert.Loosely(t, p.SourceVersion, should.Equal("1.2.12-rc1"))
			assert.Loosely(t, p.PatchVersion, should.Equal("chromium.1"))
			assert.Loosely(t, p.CIPDVersion(), should.Equal("3@1.2.12-rc1.chromium.1"))
			assert.Loosely(t, p.CIPD, should.Match(&core.Action_Metadata_CIPD{
				Version: "3@1.2.12-rc1.chromium.1",
				Refs:    []string{"latest"},
				Tags:    []string{"real_version:1.2.12.xxx.1-rc1"},
			}))
		})

		t.Run("disable latest", func(t *ftt.Test) {
			create.Package = &spec.Spec_Create_Package{DisableLatestRef: true}

			p := parsePackage(def)

			assert.Loosely(t, p.CIPD, should.Match(&core.Action_Metadata_CIPD{
				Version: "3@1.2.12.xxx.1-rc1",
			}))
		})

		t.Run("additional ref", func(t *ftt.Test) {
			create.Package = &spec.Spec_Create_Package{AdditionalRef: []string{"ref1"}}

			p := parsePackage(def)

			assert.Loosely(t, p.CIPD, should.Match(&core.Action_Metadata_CIPD{
				Version: "3@1.2.12.xxx.1-rc1",
				Refs:    []string{"ref1", "latest"},
			}))
		})
	})
}

func TestLoadDependencies(t *testing.T) {
	ftt.Run("loader", t, func(t *ftt.Test) {
		ctx := context.Background()

		cfg := DefaultSpecLoaderConfig("linux-amd64")
		cfg.SourceResolver = &MockSourceResolver{}
		root, err := filepath.Abs("testdata")
		assert.Loosely(t, err, should.BeNil)
		l, err := NewSpecLoader(root, cfg)
		assert.Loosely(t, err, should.BeNil)
		err = l.LoadSourceInfos(ctx, l.ListAllByFullName(), []string{"linux-amd64", "linux-arm64"}, true, false)
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, l.ListAllByFullName(), should.Match([]string{
			"tests/unavailable_arm64",
			"tests/unavailable_depends",
			"tools/ninja",
			"tools/re2c",
		}))

		plats := generators.Platforms{
			Build:  generators.NewPlatform("linux", "amd64"),
			Host:   generators.NewPlatform("linux", "amd64"),
			Target: generators.NewPlatform("linux", "amd64"),
		}

		t.Run("no install", func(t *ftt.Test) {
			p, err := newCreateParser("linux-amd64", []*spec.Spec_Create{{}})
			assert.Loosely(t, err, should.BeNil)
			err = p.ParseBuilder()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, p.Enviroments.Get("_3PP_NO_INSTALL"), should.Equal("1"))
		})
		t.Run("tool", func(t *ftt.Test) {
			p, err := newCreateParser("linux-arm64", []*spec.Spec_Create{
				{
					Build: &spec.Spec_Create_Build{
						Tool: []string{"tools/ninja"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			err = p.LoadDependencies("linux-amd64", l)
			assert.Loosely(t, err, should.BeNil)

			a, err := p.Dependencies[0].Generate(context.Background(), plats)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.Name, should.Equal("ninja"))
			assert.Loosely(t, a.Metadata.Cipd, should.Match(&core.Action_Metadata_CIPD{
				Name:    "tools/ninja/linux-amd64",
				Version: "3@git-tag.chromium.4",
				Refs:    []string{"latest"},
			}))
		})
		t.Run("dep", func(t *ftt.Test) {
			p, err := newCreateParser("linux-arm64", []*spec.Spec_Create{
				{
					Build: &spec.Spec_Create_Build{
						Dep: []string{"tools/ninja"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			err = p.LoadDependencies("linux-amd64", l)
			assert.Loosely(t, err, should.BeNil)

			a, err := p.Dependencies[0].Generate(context.Background(), plats)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.Name, should.Equal("ninja"))
			assert.Loosely(t, a.Metadata.Cipd, should.Match(&core.Action_Metadata_CIPD{
				Name:    "tools/ninja/linux-arm64",
				Version: "3@git-tag.chromium.4",
				Refs:    []string{"latest"},

				// Avoid uploading linux-arm64 package from linux-amd64 builder
				DisableUpload: true,
			}))
		})
		t.Run("pin", func(t *ftt.Test) {
			p, err := newCreateParser("linux-arm64", []*spec.Spec_Create{
				{
					Build: &spec.Spec_Create_Build{
						Tool: []string{"tools/ninja@version1"},
						Dep:  []string{"tools/ninja@version2"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			err = p.LoadDependencies("linux-amd64", l)
			assert.Loosely(t, err, should.BeNil)

			a, err := p.Dependencies[0].Generate(context.Background(), plats)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.Name, should.Equal("ninja"))
			assert.Loosely(t, a.Spec, should.Match(&core.Action_Cipd{
				Cipd: &core.ActionCIPDExport{
					EnsureFile: "tools/ninja/linux-amd64  version:version1\n",
				},
			}))
			a, err = p.Dependencies[1].Generate(context.Background(), plats)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, a.Name, should.Equal("ninja"))
			assert.Loosely(t, a.Spec, should.Match(&core.Action_Cipd{
				Cipd: &core.ActionCIPDExport{
					EnsureFile: "tools/ninja/linux-arm64  version:version2\n",
				},
			}))
		})
		t.Run("unavailable", func(t *ftt.Test) {
			p, err := newCreateParser("linux-arm64", []*spec.Spec_Create{
				{
					Build: &spec.Spec_Create_Build{
						Dep: []string{"tests/unavailable_arm64"},
					},
				},
			})
			assert.Loosely(t, err, should.BeNil)
			err = p.LoadDependencies("linux-amd64", l)
			assert.Loosely(t, errors.Is(err, spec.ErrPackageNotAvailable), should.BeTrue)
		})
	})
}

func TestParseExternalDependencies(t *testing.T) {
	ftt.Run("tool", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-arm64", []*spec.Spec_Create{
			{
				Build: &spec.Spec_Create_Build{
					ExternalTool: []string{"infra/3pp/static_libs/zlib/${platform}@2@1.2.12.chromium.1"},
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		err = p.ParseExternalDependencies("something", "linux-amd64")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.Dependencies, should.HaveLength(1))
		assert.Loosely(t, p.Dependencies[0], should.Match(generators.Dependency{
			Type: generators.DepsBuildHost,
			Generator: &generators.CIPDExport{
				Name: "something" + "_dep",
				Metadata: &core.Action_Metadata{
					Luciexe: &core.Action_Metadata_LUCIExe{
						StepName: "infra/3pp/static_libs/zlib/${platform}@2@1.2.12.chromium.1:linux-amd64 from cipd",
					},
				},
				Ensure: ensure.File{
					PackagesBySubdir: map[string]ensure.PackageSlice{
						"": {
							{PackageTemplate: "infra/3pp/static_libs/zlib/${platform}", UnresolvedVersion: "version:2@1.2.12.chromium.1"},
						},
					},
				},
				Expander: template.Platform{OS: "linux", Arch: "amd64"}.Expander(),
			},
		}))
	})
	ftt.Run("dep", t, func(t *ftt.Test) {
		p, err := newCreateParser("linux-arm64", []*spec.Spec_Create{
			{
				Build: &spec.Spec_Create_Build{
					ExternalDep: []string{"infra/3pp/static_libs/zlib/${platform}@2@1.2.12.chromium.1"},
				},
			},
		})
		assert.Loosely(t, err, should.BeNil)
		err = p.ParseExternalDependencies("something", "linux-amd64")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, p.Dependencies, should.HaveLength(1))
		assert.Loosely(t, p.Dependencies[0], should.Match(generators.Dependency{
			Type: generators.DepsHostTarget,
			Generator: &generators.CIPDExport{
				Name: "something" + "_dep",
				Metadata: &core.Action_Metadata{
					Luciexe: &core.Action_Metadata_LUCIExe{
						StepName: "infra/3pp/static_libs/zlib/${platform}@2@1.2.12.chromium.1:linux-arm64 from cipd",
					},
				},
				Ensure: ensure.File{
					PackagesBySubdir: map[string]ensure.PackageSlice{
						"": {
							{PackageTemplate: "infra/3pp/static_libs/zlib/${platform}", UnresolvedVersion: "version:2@1.2.12.chromium.1"},
						},
					},
				},
				Expander: template.Platform{OS: "linux", Arch: "arm64"}.Expander(),
			},
		}))
	})
}
