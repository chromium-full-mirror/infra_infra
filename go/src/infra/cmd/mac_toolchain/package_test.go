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

	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/yaml.v2"

	cipd "go.chromium.org/luci/cipd/client/cipd/builder"
)

func TestMakePackages(t *testing.T) {
	t.Parallel()

	path := func(p string) string {
		return filepath.Join(strings.Split(p, "/")...)
	}

	Convey("makePackage works", t, func() {
		baseMakePackageArgs := MakePackageArgs{
			cipdPackageName:   "cipdPackage",
			cipdPackagePrefix: "cipd/package/prefix",
			rootPath:          "testdata/WalkDir",
			includePrefixes:   nil,
			excludePrefixes:   nil,
		}
		Convey("makePackage works without include / exclude args", func() {
			makePackageArgs := baseMakePackageArgs
			pkg, err := makePackage(makePackageArgs)
			So(err, ShouldBeNil)
			So(pkg.Package, ShouldEqual, "cipd/package/prefix/cipdPackage")
			So(pkg.Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/B/b")},
				{File: path("A/B/b2")},
				{File: path("A/a")},
				{File: path("C/c")},
				{File: path("C/c2")},
				{File: path("symlink")},
			})
		})
		Convey("makePackage works with include prefixes", func() {
			includes := []string{"A/B", "C/c"}
			makePackageArgs := baseMakePackageArgs
			makePackageArgs.includePrefixes = includes
			pkg, err := makePackage(makePackageArgs)
			So(err, ShouldBeNil)
			So(pkg.Package, ShouldEqual, "cipd/package/prefix/cipdPackage")
			So(pkg.Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/B/b")},
				{File: path("A/B/b2")},
				{File: path("C/c")},
				{File: path("C/c2")},
			})
		})
		Convey("makePackage works with exclude prefixes", func() {
			excludes := []string{"A/B", "C/c"}
			makePackageArgs := baseMakePackageArgs
			makePackageArgs.excludePrefixes = excludes
			pkg, err := makePackage(makePackageArgs)
			So(err, ShouldBeNil)
			So(pkg.Package, ShouldEqual, "cipd/package/prefix/cipdPackage")
			So(pkg.Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/a")},
				{File: path("symlink")},
			})
		})
		Convey("makePackage works with include & exclude prefixes", func() {
			includes := []string{"A", "B", "C/c"}
			excludes := []string{"A/B", "C/c2"}
			makePackageArgs := baseMakePackageArgs
			makePackageArgs.includePrefixes = includes
			makePackageArgs.excludePrefixes = excludes
			pkg, err := makePackage(makePackageArgs)
			So(err, ShouldBeNil)
			So(pkg.Package, ShouldEqual, "cipd/package/prefix/cipdPackage")
			So(pkg.Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/cipdPackage.cipd_version"},
				{File: path("A/a")},
				{File: path("C/c")},
			})
		})
	})

	Convey("makeXcodePackages works", t, func() {
		Convey("for a valid directory", func() {
			packages, err := makeXcodePackages("testdata/WalkDir", "test/prefix", false)
			So(err, ShouldBeNil)
			So(packages["mac"].Package, ShouldEqual, "test/prefix/mac")
			So(packages["ios"].Package, ShouldEqual, "test/prefix/ios")
			So(packages["mac"].Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/mac.cipd_version"},
				{File: path("A/B/b")},
				{File: path("A/B/b2")},
				{File: path("A/a")},
				{File: path("C/c")},
				{File: path("C/c2")},
				{File: path("symlink")},
			})
			So(packages["mac"].Package, ShouldEqual, "test/prefix/mac")
			So(packages["ios"].Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/ios.cipd_version"},
			})
		})

		Convey("for a valid real Xcode directory", func() {
			packages, err := makeXcodePackages("testdata/Xcode-new.app", "test/prefix", false)
			So(err, ShouldBeNil)
			So(packages["mac"].Package, ShouldEqual, "test/prefix/mac")
			So(packages["ios"].Package, ShouldEqual, "test/prefix/ios")
			So(packages["mac"].Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/mac.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS.simruntime/Contents/Info.plist")},
				{File: path("Contents/Developer/usr/bin/xyz.txt")},
				{File: path("Contents/Resources/LicenseInfo.plist")},
				{File: path("Contents/version.plist")},
			})
			So(packages["ios"].Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/ios.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
			})
		})

		Convey("for a valid real directory legacy iOS package", func() {
			packages, err := makeXcodePackages("testdata/Xcode-new.app", "test/prefix", true)
			So(err, ShouldBeNil)
			So(packages["mac"].Package, ShouldEqual, "test/prefix/mac")
			So(packages["ios"].Package, ShouldEqual, "test/prefix/ios")
			So(packages["mac"].Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/mac.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS.simruntime/Contents/Info.plist")},
				{File: path("Contents/Developer/usr/bin/xyz.txt")},
				{File: path("Contents/Resources/LicenseInfo.plist")},
				{File: path("Contents/version.plist")},
			})
			So(packages["ios"].Data, ShouldResemble, []cipd.PackageChunkDef{
				{VersionFile: ".xcode_versions/ios.cipd_version"},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/DeviceTypes/iPad.simdevicetype")},
				{File: path("Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes/iOS.simruntime/Contents/Info.plist")},
			})
		})

		Convey("for a nonexistent directory", func() {
			_, err := makeXcodePackages("testdata/nonexistent", "", false)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestBuildCipdPackages(t *testing.T) {
	t.Parallel()

	Convey("buildCipdPackages works", t, func() {
		packages := Packages{
			"a": {Package: "path/a", Data: []cipd.PackageChunkDef{}},
			"b": {Package: "path/b", Data: []cipd.PackageChunkDef{}},
		}
		buildFn := func(p PackageSpec) error {
			name := filepath.Base(p.YamlPath)
			So(strings.HasSuffix(name, ".yaml"), ShouldBeTrue)
			name = name[:len(name)-len(".yaml")]
			data, err := ioutil.ReadFile(p.YamlPath)
			So(err, ShouldBeNil)
			var pd cipd.PackageDef
			err = yaml.Unmarshal(data, &pd)
			So(err, ShouldBeNil)
			So(pd, ShouldResemble, packages[name])
			So(pd.Package, ShouldEqual, p.Name)
			return nil
		}

		Convey("for valid package definitions", func() {
			err := buildCipdPackages(packages, buildFn)
			So(err, ShouldBeNil)
		})
	})
}

func TestPackageXcode(t *testing.T) {
	t.Parallel()

	Convey("packageXcode works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("for remote upload using default credentials", func() {
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 2)

			for i := 0; i < 2; i++ {
				So(s.Calls[i].Executable, ShouldEqual, "cipd")
				So(s.Calls[i].Args, ShouldContain, "create")
				So(s.Calls[i].Args, ShouldContain, "-verification-timeout")
				So(s.Calls[i].Args, ShouldContain, "60m")
				So(s.Calls[i].Args, ShouldContain, "cf_bundle_version:12345")
				So(s.Calls[i].Args, ShouldContain, "xcode_version:TESTXCODEVERSION")
				So(s.Calls[i].Args, ShouldContain, "build_version:TESTBUILDVERSION")
				So(s.Calls[i].Args, ShouldContain, "testbuildversion")

				So(s.Calls[i].Args, ShouldNotContain, "-service-account-json")
			}
		})
		Convey("for remote upload using default credentials without ref/tag", func() {
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 2)

			for i := 0; i < 2; i++ {
				So(s.Calls[i].Executable, ShouldEqual, "cipd")
				So(s.Calls[i].Args, ShouldContain, "create")
				So(s.Calls[i].Args, ShouldContain, "-verification-timeout")
				So(s.Calls[i].Args, ShouldContain, "60m")
				So(s.Calls[i].Args, ShouldNotContain, "-ref")
				So(s.Calls[i].Args, ShouldNotContain, "-tag")

				So(s.Calls[i].Args, ShouldNotContain, "-service-account-json")
			}
		})

		Convey("for remote upload using a service account", func() {
			packageXcodeArgs := PackageXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "test-sa",
				outputDir:          "",
				skipRefTag:         false,
				legacyIOSPackage:   false,
			}
			err := packageXcode(ctx, packageXcodeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 2)

			for i := 0; i < 2; i++ {
				So(s.Calls[i].Executable, ShouldEqual, "cipd")
				So(s.Calls[i].Args, ShouldContain, "create")
				So(s.Calls[i].Args, ShouldContain, "-verification-timeout")
				So(s.Calls[i].Args, ShouldContain, "60m")
				So(s.Calls[i].Args, ShouldContain, "cf_bundle_version:12345")
				So(s.Calls[i].Args, ShouldContain, "xcode_version:TESTXCODEVERSION")
				So(s.Calls[i].Args, ShouldContain, "build_version:TESTBUILDVERSION")
				So(s.Calls[i].Args, ShouldContain, "testbuildversion")

				So(s.Calls[i].Args, ShouldContain, "-service-account-json")
			}
		})

		Convey("for local package creating", func() {
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
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 2)

			So(s.Calls[0].Args, ShouldContain, filepath.Join("testdata/outdir", "ios.cipd"))
			So(s.Calls[1].Args, ShouldContain, filepath.Join("testdata/outdir", "mac.cipd"))

			for i := 0; i < 2; i++ {
				So(s.Calls[i].Executable, ShouldEqual, "cipd")
				So(s.Calls[i].Args, ShouldContain, "pkg-build")

				So(s.Calls[i].Args, ShouldNotContain, "-service-account-json")
				So(s.Calls[i].Args, ShouldNotContain, "-verification-timeout")
				So(s.Calls[i].Args, ShouldNotContain, "60m")
				So(s.Calls[i].Args, ShouldNotContain, "-tag")
				So(s.Calls[i].Args, ShouldNotContain, "-ref")
			}
		})
	})
}

func TestPackageRuntimeAndXcode(t *testing.T) {
	t.Parallel()

	Convey("packageRuntimeAndXcode works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("package an Xcode and runtime within it", func() {
			packageRuntimeAndXcodeArgs := PackageRuntimeAndXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				legacyIOSPackage:   false,
			}
			err := packageRuntimeAndXcode(ctx, packageRuntimeAndXcodeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 3)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldContain, "ios_runtime_version:iOS 14.4")
			So(s.Calls[0].Args, ShouldContain, "xcode_build_version:testbuildversion")
			So(s.Calls[0].Args, ShouldContain, "type:xcode_default")
			So(s.Calls[0].Args, ShouldContain, "testbuildversion")
			So(s.Calls[0].Args, ShouldContain, "ios-14-4_testbuildversion")
			So(s.Calls[0].Args, ShouldContain, "ios-14-4_latest")
			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")

			for i := 1; i < 3; i++ {
				So(s.Calls[i].Executable, ShouldEqual, "cipd")
				So(s.Calls[i].Args, ShouldContain, "create")
				So(s.Calls[i].Args, ShouldContain, "-verification-timeout")
				So(s.Calls[i].Args, ShouldContain, "60m")
				So(s.Calls[i].Args, ShouldContain, "cf_bundle_version:12345")
				So(s.Calls[i].Args, ShouldContain, "xcode_version:TESTXCODEVERSION")
				So(s.Calls[i].Args, ShouldContain, "build_version:TESTBUILDVERSION")
				So(s.Calls[i].Args, ShouldContain, "testbuildversion")

				So(s.Calls[i].Args, ShouldNotContain, "-service-account-json")
			}
		})

		Convey("package an Xcode and runtime within it legacy", func() {
			packageRuntimeAndXcodeArgs := PackageRuntimeAndXcodeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				legacyIOSPackage:   true,
			}
			err := packageRuntimeAndXcode(ctx, packageRuntimeAndXcodeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 2)

			for i := 0; i < 2; i++ {
				So(s.Calls[i].Executable, ShouldEqual, "cipd")
				So(s.Calls[i].Args, ShouldContain, "create")
				So(s.Calls[i].Args, ShouldContain, "-verification-timeout")
				So(s.Calls[i].Args, ShouldContain, "60m")
				So(s.Calls[i].Args, ShouldContain, "cf_bundle_version:12345")
				So(s.Calls[i].Args, ShouldContain, "xcode_version:TESTXCODEVERSION")
				So(s.Calls[i].Args, ShouldContain, "build_version:TESTBUILDVERSION")
				So(s.Calls[i].Args, ShouldContain, "testbuildversion")
				So(s.Calls[i].Args, ShouldNotContain, "-service-account-json")
			}
		})
	})
}

func TestPackageRuntime(t *testing.T) {
	t.Parallel()

	Convey("packageRuntime works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("package an Xcode default runtime", func() {
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				runtimePath:        filepath.Join("testdata", "Xcode-new.app", XcodeIOSSimulatorRuntimeRelPath, "iOS.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldContain, "ios_runtime_version:iOS 14.4")
			So(s.Calls[0].Args, ShouldContain, "xcode_build_version:testbuildversion")
			So(s.Calls[0].Args, ShouldContain, "type:xcode_default")
			So(s.Calls[0].Args, ShouldContain, "testbuildversion")
			So(s.Calls[0].Args, ShouldContain, "ios-14-4_testbuildversion")
			So(s.Calls[0].Args, ShouldContain, "ios-14-4_latest")

			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
		})

		Convey("package an Xcode default runtime without refs & tags", func() {
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "testdata/Xcode-new.app",
				runtimePath:        filepath.Join("testdata", "Xcode-new.app", XcodeIOSSimulatorRuntimeRelPath, "iOS.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldNotContain, "-tag")
			So(s.Calls[0].Args, ShouldNotContain, "-ref")
			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
		})

		Convey("package a runtime in cutomized path", func() {
			packageRuntimeArgs := PackageRuntimeArgs{
				xcodeAppPath:       "",
				runtimePath:        filepath.FromSlash("testdata/runtimes/iOS 12.4.simruntime"),
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageRuntime(ctx, packageRuntimeArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldContain, "ios_runtime_version:iOS 12.4")
			So(s.Calls[0].Args, ShouldContain, "type:manually_uploaded")
			So(s.Calls[0].Args, ShouldContain, "ios-12-4")
			So(s.Calls[0].Args, ShouldContain, "ios-12-4_latest")

		})

		Convey("for local package creating", func() {
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
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Args, ShouldContain, filepath.Join("testdata/outdir", "ios_runtime.cipd"))

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "pkg-build")

			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
			So(s.Calls[0].Args, ShouldNotContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldNotContain, "60m")
			So(s.Calls[0].Args, ShouldNotContain, "-tag")
			So(s.Calls[0].Args, ShouldNotContain, "-ref")
		})
	})
}

func TestPackageRuntimeDMG(t *testing.T) {
	t.Parallel()

	Convey("packageRuntimeDMG works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("package a test runtime dmg", func() {
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
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldContain, "ios_runtime_version:test-ios-version")
			So(s.Calls[0].Args, ShouldContain, "ios_runtime_build:test-ios-build")
			So(s.Calls[0].Args, ShouldContain, "test-xcode-version")
			So(s.Calls[0].Args, ShouldContain, "test-ios-version")

			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
		})

		Convey("package a test runtime dmg without refs & tags", func() {
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
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldNotContain, "-tag")
			So(s.Calls[0].Args, ShouldNotContain, "-ref")
			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
		})

		Convey("package a test runtime dmg with wrong file path", func() {
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
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "the runtime-path should only contain exactly one runtime DMG file")
		})

		Convey("for local package creating", func() {
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
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Args, ShouldContain, filepath.Join("testdata/outdir", "ios_runtime_dmg.cipd"))

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "pkg-build")

			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
			So(s.Calls[0].Args, ShouldNotContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldNotContain, "60m")
			So(s.Calls[0].Args, ShouldNotContain, "-tag")
			So(s.Calls[0].Args, ShouldNotContain, "-ref")
		})
	})
}

func TestPackageXcodeArchive(t *testing.T) {
	t.Parallel()

	Convey("packageXcodeArchive works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("package a test xcode archive", func() {
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "xcode-archive"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldContain, "xcode_version:test-xcode-version")
			So(s.Calls[0].Args, ShouldContain, "test-xcode-version")

			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
		})

		Convey("package a test xcode archive without refs & tags", func() {
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "xcode-archive"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         true,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "create")
			So(s.Calls[0].Args, ShouldContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldContain, "60m")
			So(s.Calls[0].Args, ShouldNotContain, "-tag")
			So(s.Calls[0].Args, ShouldNotContain, "-ref")
			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
		})

		Convey("package a test xcode archive with wrong file path", func() {
			packageXcodeArchiveArgs := PackageXcodeArchiveArgs{
				xcodePath:          filepath.Join("testdata", "runtimes"),
				xcodeVersion:       "test-xcode-version",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
				outputDir:          "",
				skipRefTag:         false,
			}
			err := packageXcodeArchive(ctx, packageXcodeArchiveArgs)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "the xcode-path should only contain exactly one archive file")
		})

		Convey("for local package creating", func() {
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
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)

			So(s.Calls[0].Args, ShouldContain, filepath.Join("testdata/outdir", "xcode_archive.cipd"))

			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldContain, "pkg-build")

			So(s.Calls[0].Args, ShouldNotContain, "-service-account-json")
			So(s.Calls[0].Args, ShouldNotContain, "-verification-timeout")
			So(s.Calls[0].Args, ShouldNotContain, "60m")
			So(s.Calls[0].Args, ShouldNotContain, "-tag")
			So(s.Calls[0].Args, ShouldNotContain, "-ref")
		})
	})

}
