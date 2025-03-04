// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	cipd "go.chromium.org/luci/cipd/client/cipd/builder"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestMakePackages(t *testing.T) {
	t.Parallel()

	path := func(p string) string {
		return filepath.Join(strings.Split(p, "/")...)
	}

	ftt.Run("makePackage works", t, func(t *ftt.Test) {
		baseMakePackageArgs := MakePackageArgs{
			cipdPackageName:   "cipdPackage",
			cipdPackagePrefix: "cipd/package/prefix",
			rootPath:          "testdata/WalkDir",
			includePrefixes:   nil,
			excludePrefixes:   nil,
		}
		t.Run("makePackage works without include / exclude args", func(t *ftt.Test) {
			makePackageArgs := baseMakePackageArgs
			pkg, err := makePackage(makePackageArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, pkg.Package, should.Equal("cipd/package/prefix/cipdPackage"))
			assert.Loosely(t, pkg.Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/B/b")},
				{File: path("A/B/b2")},
				{File: path("A/a")},
				{File: path("C/c")},
				{File: path("C/c2")},
				{File: path("symlink")},
			}))
		})
		t.Run("makePackage works with include prefixes", func(t *ftt.Test) {
			includes := []string{"A/B", "C/c"}
			makePackageArgs := baseMakePackageArgs
			makePackageArgs.includePrefixes = includes
			pkg, err := makePackage(makePackageArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, pkg.Package, should.Equal("cipd/package/prefix/cipdPackage"))
			assert.Loosely(t, pkg.Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/B/b")},
				{File: path("A/B/b2")},
				{File: path("C/c")},
				{File: path("C/c2")},
			}))
		})
		t.Run("makePackage works with exclude prefixes", func(t *ftt.Test) {
			excludes := []string{"A/B", "C/c"}
			makePackageArgs := baseMakePackageArgs
			makePackageArgs.excludePrefixes = excludes
			pkg, err := makePackage(makePackageArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, pkg.Package, should.Equal("cipd/package/prefix/cipdPackage"))
			assert.Loosely(t, pkg.Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/a")},
				{File: path("symlink")},
			}))
		})
		t.Run("makePackage works with include & exclude prefixes", func(t *ftt.Test) {
			includes := []string{"A", "B", "C/c"}
			excludes := []string{"A/B", "C/c2"}
			makePackageArgs := baseMakePackageArgs
			makePackageArgs.includePrefixes = includes
			makePackageArgs.excludePrefixes = excludes
			pkg, err := makePackage(makePackageArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, pkg.Package, should.Equal("cipd/package/prefix/cipdPackage"))
			assert.Loosely(t, pkg.Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/a")},
				{File: path("C/c")},
			}))
		})
	})

	ftt.Run("makeXcodePackages works", t, func(t *ftt.Test) {
		t.Run("for a valid directory", func(t *ftt.Test) {
			packages, err := makeXcodePackages("testdata/WalkDir", "test/prefix", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packages["mac"].Package, should.Equal("test/prefix/mac"))
			assert.Loosely(t, packages["ios"].Package, should.Equal("test/prefix/ios"))
			assert.Loosely(t, packages["mac"].Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/mac.cipd_version"},
				{File: path("A/B/b")},
				{File: path("A/B/b2")},
				{File: path("A/a")},
				{File: path("C/c")},
				{File: path("C/c2")},
				{File: path("symlink")},
			}))
			assert.Loosely(t, packages["mac"].Package, should.Equal("test/prefix/mac"))
			assert.Loosely(t, packages["ios"].Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/ios.cipd_version"},
			}))
		})

		t.Run("for a valid real Xcode directory", func(t *ftt.Test) {
			packages, err := makeXcodePackages("testdata/Xcode-new.app", "test/prefix", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packages["mac"].Package, should.Equal("test/prefix/mac"))
			assert.Loosely(t, packages["ios"].Package, should.Equal("test/prefix/ios"))
			assert.Loosely(t, packages["mac"].Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/mac.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS.simruntime/Contents/Info.plist")},
				{File: path("Contents/Developer/usr/bin/xyz.txt")},
				{File: path("Contents/Resources/LicenseInfo.plist")},
				{File: path("Contents/version.plist")},
			}))
			assert.Loosely(t, packages["ios"].Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/ios.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
			}))
		})

		t.Run("for a valid real directory legacy iOS package", func(t *ftt.Test) {
			packages, err := makeXcodePackages("testdata/Xcode-new.app", "test/prefix", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, packages["mac"].Package, should.Equal("test/prefix/mac"))
			assert.Loosely(t, packages["ios"].Package, should.Equal("test/prefix/ios"))
			assert.Loosely(t, packages["mac"].Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/mac.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS.simruntime/Contents/Info.plist")},
				{File: path("Contents/Developer/usr/bin/xyz.txt")},
				{File: path("Contents/Resources/LicenseInfo.plist")},
				{File: path("Contents/version.plist")},
			}))
			assert.Loosely(t, packages["ios"].Data, should.Resemble([]cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/ios.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS.simruntime/Contents/Info.plist")},
			}))
		})

		t.Run("for a nonexistent directory", func(t *ftt.Test) {
			_, err := makeXcodePackages("testdata/nonexistent", "", false)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}

func TestBuildCipdPackages(t *testing.T) {
	t.Parallel()

	ftt.Run("buildCipdPackages works", t, func(t *ftt.Test) {
		packages := Packages{
			"a": {Package: "path/a", Data: []cipd.PackageChunkDef{}},
			"b": {Package: "path/b", Data: []cipd.PackageChunkDef{}},
		}
		buildFn := func(p PackageSpec) error {
			name := filepath.Base(p.YamlPath)
			assert.Loosely(t, strings.HasSuffix(name, ".yaml"), should.BeTrue)
			name = name[:len(name)-len(".yaml")]
			data, err := ioutil.ReadFile(p.YamlPath)
			assert.Loosely(t, err, should.BeNil)
			var pd cipd.PackageDef
			err = yaml.Unmarshal(data, &pd)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, pd, should.Resemble(packages[name]))
			assert.Loosely(t, pd.Package, should.Equal(p.Name))
			return nil
		}

		t.Run("for valid package definitions", func(t *ftt.Test) {
			err := buildCipdPackages(packages, buildFn)
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestPackageXcode(t *testing.T) {
	t.Parallel()

	ftt.Run("packageXcode works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("for remote upload using default credentials", func(t *ftt.Test) {
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(2))

			for i := range 2 {
				assert.Loosely(t, s.Calls[i].Executable, should.Equal("cipd"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("create"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("-verification-timeout"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("60m"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("cf_bundle_version:12345"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("xcode_version:TESTXCODEVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("build_version:TESTBUILDVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("testbuildversion"))

				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-service-account-json"))
			}
		})
		t.Run("for remote upload using default credentials without ref/tag", func(t *ftt.Test) {
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(2))

			for i := range 2 {
				assert.Loosely(t, s.Calls[i].Executable, should.Equal("cipd"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("create"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("-verification-timeout"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("60m"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-ref"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-tag"))

				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-service-account-json"))
			}
		})

		t.Run("for remote upload using a service account", func(t *ftt.Test) {
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "test-sa",
				outputDir:          "",
				skipRefTag:         false,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(2))

			for i := range 2 {
				assert.Loosely(t, s.Calls[i].Executable, should.Equal("cipd"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("create"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("-verification-timeout"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("60m"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("cf_bundle_version:12345"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("xcode_version:TESTXCODEVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("build_version:TESTBUILDVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("testbuildversion"))

				assert.Loosely(t, s.Calls[i].Args, should.Contain("-service-account-json"))
			}
		})

		t.Run("for local package creating", func(t *ftt.Test) {
			// Make sure `outputDir` actually exists in testdata; otherwise the test
			// will needlessly create a directory and leave it behind.
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "testdata/outdir",
				skipRefTag:         false,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(2))

			assert.Loosely(t, s.Calls[0].Args, should.Contain(filepath.Join("testdata/outdir", "ios.cipd")))
			assert.Loosely(t, s.Calls[1].Args, should.Contain(filepath.Join("testdata/outdir", "mac.cipd")))

			for i := range 2 {
				assert.Loosely(t, s.Calls[i].Executable, should.Equal("cipd"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("pkg-build"))

				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-service-account-json"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-verification-timeout"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("60m"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-tag"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-ref"))
			}
		})
	})
}

func TestPackageRuntimeAndXcode(t *testing.T) {
	t.Parallel()

	ftt.Run("packageRuntimeAndXcode works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("package an Xcode and runtime within it", func(t *ftt.Test) {
			packageRuntimeAndXcodeArgs := PackageRuntimeAndXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				legacyIOSPackage:   false,
			}
			err := packageRuntimeAndXcode(ctx, packageRuntimeAndXcodeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(3))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios_runtime_version:iOS 14.4"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("xcode_build_version:testbuildversion"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("type:xcode_default"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("testbuildversion"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios-14-4_testbuildversion"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios-14-4_latest"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))

			for i := 1; i < 3; i++ {
				assert.Loosely(t, s.Calls[i].Executable, should.Equal("cipd"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("create"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("-verification-timeout"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("60m"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("cf_bundle_version:12345"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("xcode_version:TESTXCODEVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("build_version:TESTBUILDVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("testbuildversion"))

				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-service-account-json"))
			}
		})

		t.Run("package an Xcode and runtime within it legacy", func(t *ftt.Test) {
			packageRuntimeAndXcodeArgs := PackageRuntimeAndXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				legacyIOSPackage:   true,
			}
			err := packageRuntimeAndXcode(ctx, packageRuntimeAndXcodeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(2))

			for i := range 2 {
				assert.Loosely(t, s.Calls[i].Executable, should.Equal("cipd"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("create"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("-verification-timeout"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("60m"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("cf_bundle_version:12345"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("xcode_version:TESTXCODEVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("build_version:TESTBUILDVERSION"))
				assert.Loosely(t, s.Calls[i].Args, should.Contain("testbuildversion"))
				assert.Loosely(t, s.Calls[i].Args, should.NotContain("-service-account-json"))
			}
		})
	})
}

func TestPackageRuntime(t *testing.T) {
	t.Parallel()

	ftt.Run("packageRuntime works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("package an Xcode default runtime", func(t *ftt.Test) {
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				runtimePath:        filepath.Join("testdata", "Xcode-new.app", XcodeIOSSimulatorRuntimeRelPath, "iOS.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios_runtime_version:iOS 14.4"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("xcode_build_version:testbuildversion"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("type:xcode_default"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("testbuildversion"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios-14-4_testbuildversion"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios-14-4_latest"))

			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
		})

		t.Run("package an Xcode default runtime without refs & tags", func(t *ftt.Test) {
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				runtimePath:        filepath.Join("testdata", "Xcode-new.app", XcodeIOSSimulatorRuntimeRelPath, "iOS.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-tag"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-ref"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
		})

		t.Run("package a runtime in cutomized path", func(t *ftt.Test) {
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "",
				runtimePath:        filepath.FromSlash("testdata/runtimes/iOS 12.4.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios_runtime_version:iOS 12.4"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("type:manually_uploaded"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios-12-4"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios-12-4_latest"))

		})

		t.Run("for local package creating", func(t *ftt.Test) {
			// Make sure `outputDir` actually exists in testdata; otherwise the test
			// will needlessly create a directory and leave it behind.
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "",
				runtimePath:        filepath.FromSlash("testdata/runtimes/iOS 12.4.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "testdata/outdir",
				skipRefTag:         false,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Args, should.Contain(filepath.Join("testdata/outdir", "ios_runtime.cipd")))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("pkg-build"))

			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-tag"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-ref"))
		})
	})
}

func TestPackageRuntimeDMG(t *testing.T) {
	t.Parallel()

	ftt.Run("packageRuntimeDMG works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("package a test runtime dmg", func(t *ftt.Test) {
			packageRuntimeDMGArgs := PackageRuntimeDMGArgs{
				runtimePath:        filepath.Join("testdata", "runtime-dmg"),
				runtimeVersion:     "test-ios-version",
				runtimeBuild:       "test-ios-build",
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageRuntimeDMG(ctx, packageRuntimeDMGArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios_runtime_version:test-ios-version"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("ios_runtime_build:test-ios-build"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("test-xcode-version"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("test-ios-version"))

			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
		})

		t.Run("package a test runtime dmg without refs & tags", func(t *ftt.Test) {
			packageRuntimeDMGArgs := PackageRuntimeDMGArgs{
				runtimePath:        filepath.Join("testdata", "runtime-dmg"),
				runtimeVersion:     "test-ios-version",
				runtimeBuild:       "test-ios-build",
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
			}
			err := packageRuntimeDMG(ctx, packageRuntimeDMGArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-tag"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-ref"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
		})

		t.Run("package a test runtime dmg with wrong file path", func(t *ftt.Test) {
			packageRuntimeDMGArgs := PackageRuntimeDMGArgs{
				runtimePath:        filepath.Join("testdata", "runtimes"),
				runtimeVersion:     "test-ios-version",
				runtimeBuild:       "test-ios-build",
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageRuntimeDMG(ctx, packageRuntimeDMGArgs)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("the runtime-path should only contain exactly one runtime DMG file"))
		})

		t.Run("for local package creating", func(t *ftt.Test) {
			// Make sure `outputDir` actually exists in testdata; otherwise the test
			// will needlessly create a directory and leave it behind.
			packageRuntimeDMGArgs := PackageRuntimeDMGArgs{
				runtimePath:        filepath.Join("testdata", "runtime-dmg"),
				runtimeVersion:     "test-ios-version",
				runtimeBuild:       "test-ios-build",
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "testdata/outdir",
				skipRefTag:         false,
			}
			err := packageRuntimeDMG(ctx, packageRuntimeDMGArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Args, should.Contain(filepath.Join("testdata/outdir", "ios_runtime_dmg.cipd")))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("pkg-build"))

			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-tag"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-ref"))
		})
	})
}

func TestPackageXcodeArchive(t *testing.T) {
	t.Parallel()

	ftt.Run("packageXcodeArchive works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("package a test xcode archive", func(t *ftt.Test) {
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "xcode-archive"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("xcode_version:test-xcode-version"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("test-xcode-version"))

			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
		})

		t.Run("package a test xcode archive without refs & tags", func(t *ftt.Test) {
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "xcode-archive"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("create"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-tag"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-ref"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
		})

		t.Run("package a test xcode archive with wrong file path", func(t *ftt.Test) {
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "runtimes"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("the xcode-path should only contain exactly one archive file"))
		})

		t.Run("for local package creating", func(t *ftt.Test) {
			// Make sure `outputDir` actually exists in testdata; otherwise the test
			// will needlessly create a directory and leave it behind.
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "xcode-archive"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "testdata/outdir",
				skipRefTag:         false,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))

			assert.Loosely(t, s.Calls[0].Args, should.Contain(filepath.Join("testdata/outdir", "xcode_archive.cipd")))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Contain("pkg-build"))

			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-service-account-json"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-verification-timeout"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("60m"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-tag"))
			assert.Loosely(t, s.Calls[0].Args, should.NotContain("-ref"))
		})
	})

}
