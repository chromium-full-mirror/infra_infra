// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// We only test the builder on a subset of platforms we support.
// Other platforms should be cross-compiled.
//go:build amd64 || (arm64 && darwin)

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"go.chromium.org/luci/cipd/client/cipd/platform"
	"go.chromium.org/luci/cipkg/base/actions"
	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/base/workflow"
	"go.chromium.org/luci/cipkg/core"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/system/environ"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/tools/pkgbuild/pkg/spec"
	"go.chromium.org/infra/tools/pkgbuild/pkg/spec/loader"
	"go.chromium.org/infra/tools/pkgbuild/pkg/spec/source"
	"go.chromium.org/infra/tools/pkgbuild/pkg/stdenv"
)

func initStdenv(t testing.TB, build generators.Platform) {
	t.Helper()
	assert.Loosely(t, stdenv.Init(&stdenv.Config{
		XcodeDeveloper: &generators.ImportTargets{Name: "xcode_import"},
		WinSDK:         &generators.ImportTargets{Name: "winsdk_files"},
		FindBinary: func(bin string) (string, error) {
			// We don't need to import binaries from host.
			if runtime.GOOS == "windows" {
				return fmt.Sprintf("C:/bin/%s", bin), nil
			}
			return fmt.Sprintf("/bin/%s", bin), nil
		},
		BuildPlatform: build,
	}), should.BeNil, truth.LineContext())
}

func TestBuildPackagesFromSpec(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	ctx := gologger.StdConfig.Use(context.Background())

	ftt.Run("native platform", t, func(t *ftt.Test) {
		tempBase := t.TempDir()
		buildTemp := filepath.Join(tempBase, "build")
		storeTemp := filepath.Join(tempBase, "store")
		specs := filepath.Join(cwd, "testdata")

		buildPlatform := generators.CurrentPlatform()
		cipdPlatform := platform.CurrentPlatform()

		loader, err := loader.NewSpecLoader(specs, MockSpecLoaderConfig(cipdPlatform))
		assert.Loosely(t, err, should.BeNil)
		err = loader.LoadSourceInfos(ctx, []string{cipdPlatform}, true, false)
		assert.Loosely(t, err, should.BeNil)

		initStdenv(t, buildPlatform)

		pm, err := workflow.NewLocalPackageManager(storeTemp)
		assert.Loosely(t, err, should.BeNil)

		plats := generators.Platforms{
			Build:  buildPlatform,
			Host:   buildPlatform,
			Target: buildPlatform,
		}
		b := &PackageBuilder{
			Packages:     pm,
			Platforms:    plats,
			CIPDHost:     cipdPlatform,
			CIPDTarget:   cipdPlatform,
			SpecLoader:   loader,
			BuildTempDir: buildTemp,
			packageExecutor: workflow.NewPackageExecutor(buildTemp, nil, nil, nil,
				func(context.Context, *workflow.ExecutionConfig, *core.Derivation) error { return nil },
			),
		}

		t.Run("build ninja", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/ninja")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			pkg := pkgs[len(pkgs)-1]
			env := environ.New(pkg.Derivation.Env)
			assert.Loosely(t, pkg.Derivation.Name, should.Equal("ninja"))
			assert.Loosely(t, pkg.Derivation.Platform, should.Equal(buildPlatform.String()))
			assert.Loosely(t, env.Get("_3PP_PLATFORM"), should.Equal(cipdPlatform))
		})

		t.Run("Build go", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/go")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			pkg := pkgs[len(pkgs)-1]
			env := environ.New(pkg.Derivation.Env)
			assert.Loosely(t, pkg.Derivation.Name, should.Equal("go"))
			assert.Loosely(t, pkg.Derivation.Platform, should.Equal(buildPlatform.String()))
			assert.Loosely(t, env.Get("_3PP_PLATFORM"), should.Equal(cipdPlatform))
		})

		t.Run("Build virtualenv (universal)", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/virtualenv")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			pkg := pkgs[len(pkgs)-1]
			env := environ.New(pkg.Derivation.Env)
			assert.Loosely(t, pkg.Derivation.Name, should.Equal("virtualenv"))
			assert.Loosely(t, pkg.Derivation.Platform, should.Equal(buildPlatform.String()))
			assert.Loosely(t, pkg.Action.Metadata.Cipd.Name, should.Equal("mock/tools/virtualenv"))
			ver := "3@git-tag.chromium.8"
			if cipdPlatform != "linux-amd64" {
				ver += "-" + cipdPlatform
			}
			assert.Loosely(t, pkg.Action.Metadata.Cipd.Version, should.Equal(ver))
			assert.Loosely(t, env.Get("_3PP_PLATFORM"), should.Equal(cipdPlatform))
		})
	})

	ftt.Run("cross-compile platform", t, func(t *ftt.Test) {
		tempBase := t.TempDir()
		buildTemp := filepath.Join(tempBase, "build")
		storeTemp := filepath.Join(tempBase, "store")
		specs := filepath.Join(cwd, "testdata")

		buildPlatform := generators.NewPlatform("linux", "amd64")
		hostPlatform := generators.NewPlatform("linux", "arm64")
		cipdHost := "linux-amd64"
		cipdTarget := "linux-arm64"

		loader, err := loader.NewSpecLoader(specs, MockSpecLoaderConfig(cipdTarget))
		assert.Loosely(t, err, should.BeNil)
		err = loader.LoadSourceInfos(ctx, []string{cipdHost, cipdTarget}, true, false)
		assert.Loosely(t, err, should.BeNil)

		initStdenv(t, buildPlatform)

		pm, err := workflow.NewLocalPackageManager(storeTemp)
		assert.Loosely(t, err, should.BeNil)

		plats := generators.Platforms{
			Build:  buildPlatform,
			Host:   hostPlatform,
			Target: hostPlatform,
		}
		b := &PackageBuilder{
			Packages:     pm,
			Platforms:    plats,
			CIPDHost:     cipdHost,
			CIPDTarget:   cipdTarget,
			SpecLoader:   loader,
			BuildTempDir: buildTemp,
			packageExecutor: workflow.NewPackageExecutor(buildTemp, nil, nil, nil,
				func(context.Context, *workflow.ExecutionConfig, *core.Derivation) error { return nil },
			),
		}

		t.Run("build packages", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/ninja")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			pkg := pkgs[len(pkgs)-1]
			env := environ.New(pkg.Derivation.Env)
			assert.Loosely(t, pkg.Derivation.Name, should.Equal("ninja"))
			assert.Loosely(t, pkg.Derivation.Platform, should.Equal(buildPlatform.String()))
			assert.Loosely(t, env.Get("_3PP_PLATFORM"), should.Equal(cipdTarget))
		})

		t.Run("Build virtualenv (universal)", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/virtualenv")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			pkg := pkgs[len(pkgs)-1]
			env := environ.New(pkg.Derivation.Env)
			assert.Loosely(t, pkg.Derivation.Name, should.Equal("virtualenv"))
			assert.Loosely(t, pkg.Derivation.Platform, should.Equal(buildPlatform.String()))
			assert.Loosely(t, pkg.Action.Metadata.Cipd.Name, should.Equal("mock/tools/virtualenv"))
			assert.Loosely(t, pkg.Action.Metadata.Cipd.Version, should.Equal("3@git-tag.chromium.8-linux-arm64"))
			assert.Loosely(t, env.Get("_3PP_PLATFORM"), should.Equal(cipdTarget))
		})

		// If a dependency is not available, ErrPackageNotAvailable should be the
		// inner error.
		t.Run("unavailable dependency", func(t *ftt.Test) {
			err := b.Load(ctx, "tests/unavailable_depends")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.NotEqual(spec.ErrPackageNotAvailable))
			assert.Loosely(t, errors.Is(err, spec.ErrPackageNotAvailable), should.BeTrue)
		})

		// If a package itself is not available, ErrPackageNotAvailable should be
		// the direct error.
		t.Run("unavailable", func(t *ftt.Test) {
			err := b.Load(ctx, "tests/unavailable_arm64")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err, should.Equal(spec.ErrPackageNotAvailable))
		})
	})
}

func TestRootPackges(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	ctx := gologger.StdConfig.Use(context.Background())

	ftt.Run("native platform", t, func(t *ftt.Test) {
		tempBase := t.TempDir()
		storeTemp := filepath.Join(tempBase, "store")
		specs := filepath.Join(cwd, "testdata")

		buildPlatform := generators.NewPlatform("linux", "amd64")
		cipdPlatform := "linux-amd64"

		loader, err := loader.NewSpecLoader(specs, MockSpecLoaderConfig(cipdPlatform))
		assert.Loosely(t, err, should.BeNil)
		err = loader.LoadSourceInfos(ctx, []string{cipdPlatform}, true, false)
		assert.Loosely(t, err, should.BeNil)

		initStdenv(t, buildPlatform)

		pm, err := workflow.NewLocalPackageManager(storeTemp)
		assert.Loosely(t, err, should.BeNil)

		plats := generators.Platforms{
			Build:  buildPlatform,
			Host:   buildPlatform,
			Target: buildPlatform,
		}

		initStdenv(t, buildPlatform)

		var loaded []generators.Generator
		load := func(name string) {
			g, err := loader.FromSpec(name, cipdPlatform, cipdPlatform)
			assert.Loosely(t, err, should.BeNil)
			loaded = append(loaded, g)
		}

		t.Run("ok", func(t *ftt.Test) {
			load("tests/step_1")
			load("tests/step_2")

			builder := workflow.NewBuilder(plats, pm, actions.NewActionProcessor())
			pkgs, err := builder.GeneratePackages(ctx, loaded)
			assert.Loosely(t, err, should.BeNil)

			rootSteps := NewRootSteps()
			for _, pkg := range pkgs {
				_, err = rootSteps.UpdateRoot(ctx, pkg)
				assert.Loosely(t, err, should.BeNil)
			}
			var roots []string
			for id, s := range rootSteps {
				if s.ID() == id {
					roots = append(roots, id[:strings.IndexAny(id, "+-")])
				}
				slices.Sort(roots)
			}
			expected := []string{
				"stdenv", "setup", "stdenv_git", "stdenv_python3", "stdenv_vpython3", "posix_import", // stdenv
				"from_spec_support", "docker_import", // from_spec

				"step_1", "step_2", // loaded packages

				"step_dep", "step_tool", "step_tool_pinned",
			}
			slices.Sort(expected)

			assert.Loosely(t, roots, should.Match(expected))
		})
	})

	ftt.Run("cross-compile platform", t, func(t *ftt.Test) {
		tempBase := t.TempDir()
		storeTemp := filepath.Join(tempBase, "store")
		specs := filepath.Join(cwd, "testdata")

		buildPlatform := generators.NewPlatform("linux", "amd64")
		hostPlatform := generators.NewPlatform("linux", "arm64")
		cipdHost := "linux-amd64"
		cipdTarget := "linux-arm64"

		loader, err := loader.NewSpecLoader(specs, MockSpecLoaderConfig(cipdTarget))
		assert.Loosely(t, err, should.BeNil)
		err = loader.LoadSourceInfos(ctx, []string{cipdHost, cipdTarget}, true, false)
		assert.Loosely(t, err, should.BeNil)

		initStdenv(t, buildPlatform)

		pm, err := workflow.NewLocalPackageManager(storeTemp)
		assert.Loosely(t, err, should.BeNil)

		plats := generators.Platforms{
			Build:  buildPlatform,
			Host:   hostPlatform,
			Target: hostPlatform,
		}

		initStdenv(t, buildPlatform)

		var loaded []generators.Generator
		load := func(name string) {
			g, err := loader.FromSpec(name, cipdHost, cipdTarget)
			assert.Loosely(t, err, should.BeNil)
			loaded = append(loaded, g)
		}

		t.Run("self depends", func(t *ftt.Test) {
			load("tests/step_cross")

			builder := workflow.NewBuilder(plats, pm, actions.NewActionProcessor())
			pkgs, err := builder.GeneratePackages(ctx, loaded)
			assert.Loosely(t, err, should.BeNil)

			rootSteps := NewRootSteps()
			for _, pkg := range pkgs {
				_, err = rootSteps.UpdateRoot(ctx, pkg)
				assert.Loosely(t, err, should.BeNil)
			}
			var roots []string
			for id, s := range rootSteps {
				if s.ID() == id {
					roots = append(roots, id[:strings.IndexAny(id, "+-")])
				}
				slices.Sort(roots)
			}
			expected := []string{
				"stdenv", "setup", "stdenv_git", "stdenv_python3", "stdenv_vpython3", "posix_import", // stdenv
				"from_spec_support", "docker_import", // from_spec

				"step_cross", "step_cross",
			}
			slices.Sort(expected)

			assert.Loosely(t, roots, should.Match(expected))
		})
	})
}

func TestPackageSources(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	ctx := gologger.StdConfig.Use(context.Background())
	ctx = logging.SetLevel(ctx, logging.Error)

	ftt.Run("native platform", t, func(t *ftt.Test) {
		tempBase := t.TempDir()
		buildTemp := filepath.Join(tempBase, "build")
		storeTemp := filepath.Join(tempBase, "store")
		specs := filepath.Join(cwd, "testdata")

		buildPlatform := generators.NewPlatform("linux", "amd64")
		cipdPlatform := "linux-amd64"

		loader, err := loader.NewSpecLoader(specs, MockSpecLoaderConfig(cipdPlatform))
		assert.Loosely(t, err, should.BeNil)
		err = loader.LoadSourceInfos(ctx, []string{cipdPlatform}, true, false)
		assert.Loosely(t, err, should.BeNil)

		initStdenv(t, buildPlatform)

		pm, err := workflow.NewLocalPackageManager(storeTemp)
		assert.Loosely(t, err, should.BeNil)

		plats := generators.Platforms{
			Build:  buildPlatform,
			Host:   buildPlatform,
			Target: buildPlatform,
		}
		b := &PackageBuilder{
			Packages:     pm,
			Platforms:    plats,
			CIPDHost:     cipdPlatform,
			CIPDTarget:   cipdPlatform,
			SpecLoader:   loader,
			BuildTempDir: buildTemp,
			packageExecutor: workflow.NewPackageExecutor(buildTemp, nil, nil, nil,
				func(context.Context, *workflow.ExecutionConfig, *core.Derivation) error { return nil },
			),
		}

		t.Run("git source", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/ninja")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			verifySource(t, pkgs, &core.Action_Metadata{
				Cipd: &core.Action_Metadata_CIPD{
					Name:    "mock/sources/git/github.com/ninja-build/ninja",
					Version: "3@git-tag",
				},
				ContextInfo: "ninja:arch=amd64,os=linux",
			})
		})

		t.Run("url source", func(t *ftt.Test) {
			err := b.Load(ctx, "static_libs/curl")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			verifySource(t, pkgs, &core.Action_Metadata{
				Cipd: &core.Action_Metadata_CIPD{
					Name:    "mock/sources/http/static_libs/curl",
					Version: "3@7.59.0_0",
				},
				ContextInfo: "curl:arch=amd64,os=linux",
			})
		})

		t.Run("script source", func(t *ftt.Test) {
			err := b.Load(ctx, "tools/go")
			assert.Loosely(t, err, should.BeNil)
			pkgs, err := b.BuildAll(ctx, false)
			assert.Loosely(t, err, should.BeNil)

			verifySource(t, pkgs, &core.Action_Metadata{
				Cipd: &core.Action_Metadata_CIPD{
					Name:    "mock/sources/http/tools/go",
					Version: "3@script-version_0",
				},
				ContextInfo: "go:arch=amd64,os=linux",
			})
		})
	})
}

func verifySource(t testing.TB, pkgs []actions.Package, metadata *core.Action_Metadata) {
	t.Helper()
	pkg := pkgs[len(pkgs)-1]
	prefix := fmt.Sprintf("%s_source", pkg.Derivation.Name)
	for _, p := range pkg.BuildDependencies {
		if strings.HasPrefix(p.Derivation.Name, prefix) {
			assert.Loosely(t, p.Action.Metadata, should.Match(metadata), truth.LineContext())
			return
		}
	}
	t.Fatalf("source not found: %s", prefix)
}

type MockSourceResolver struct{}

func (*MockSourceResolver) Resolve(ctx context.Context, plat, dir string, src *spec.Spec_Create_Source) (*source.SourceInfo, error) {
	switch src.Method.(type) {
	case *spec.Spec_Create_Source_Git:
		git := src.GetGit()
		return &source.SourceInfo{
			Version: "git-tag",
			Source: &source.SourceInfo_Git_{
				Git: &source.SourceInfo_Git{
					Url:    git.Repo,
					Commit: "git-commit",
				},
			},
		}, nil
	case *spec.Spec_Create_Source_Script:
		return &source.SourceInfo{
			Version: "script-version",
			Source: &source.SourceInfo_Http{
				Http: &source.SourceInfo_HTTP{
					Url:  []string{"url"},
					Name: []string{"name"},
				},
			},
		}, nil
	case *spec.Spec_Create_Source_Url:
		u := src.GetUrl()
		return &source.SourceInfo{
			Version: u.Version,
			Source: &source.SourceInfo_Http{
				Http: &source.SourceInfo_HTTP{
					Url: []string{u.DownloadUrl},
					Ext: u.Extension,
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("unkown source type")
}

func MockSpecLoaderConfig(targetPlatform string) *loader.SpecLoaderConfig {
	return &loader.SpecLoaderConfig{
		CIPDPackagePrefix:     "mock",
		CIPDSourceCachePrefix: "sources",
		CIPDTargetPlatform:    targetPlatform,
		SourceResolver:        &MockSourceResolver{},
	}
}
