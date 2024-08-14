// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.chromium.org/luci/common/errors"
)

const ListRuntimeJson = `{
  "1111111" : {
    "build" : "21A5248u",
    "deletable" : true,
    "identifier" : "11111111",
    "kind" : "Disk Image",
    "mountPath" : "\/Library\/Developer\/CoreSimulator\/Volumes\/iOS_21A5248u",
    "path" : "\/Library\/Developer\/CoreSimulator\/Images\/11111111.dmg",
    "platformIdentifier" : "com.apple.platform.iphonesimulator",
    "runtimeBundlePath" : "\/Library\/Developer\/CoreSimulator\/Volumes\/iOS_21A5248u\/Library\/Developer\/CoreSimulator\/Profiles\/Runtimes\/iOS 17.0.simruntime",
    "runtimeIdentifier" : "com.apple.CoreSimulator.SimRuntime.iOS-17-0",
    "signatureState" : "Verified",
    "sizeBytes" : 7534187147,
    "state" : "Ready",
    "version" : "17.0"
  }
}`

const ListRuntimeDiffBuildJson = `{
  "1111111" : {
    "build" : "21A51234",
    "deletable" : true,
    "identifier" : "11111111",
    "kind" : "Disk Image",
    "mountPath" : "\/Library\/Developer\/CoreSimulator\/Volumes\/iOS_21A5248u",
    "path" : "\/Library\/Developer\/CoreSimulator\/Images\/11111111.dmg",
    "platformIdentifier" : "com.apple.platform.iphonesimulator",
    "runtimeBundlePath" : "\/Library\/Developer\/CoreSimulator\/Volumes\/iOS_21A5248u\/Library\/Developer\/CoreSimulator\/Profiles\/Runtimes\/iOS 17.0.simruntime",
    "runtimeIdentifier" : "com.apple.CoreSimulator.SimRuntime.iOS-17-0",
    "signatureState" : "Verified",
    "sizeBytes" : 7534187147,
    "state" : "Ready",
    "version" : "17.0"
  }
}`

const ListRuntimeSDKJson = `{
	"appletvos17.0": {
        "chosenRuntimeBuild": "21J11111",
        "defaultBuild": "21J11111",
        "platform": "com.apple.platform.appletvos",
        "preferredBuild": "21J11111",
        "sdkBuild": "21J11111",
        "sdkVersion": "13.1"
    },
    "iphoneos17.0": {
        "chosenRuntimeBuild": "21A111112",
        "defaultBuild": "21A111112",
        "platform": "com.apple.platform.iphoneos",
        "preferredBuild": "21A111111",
        "sdkBuild": "21A111112",
        "sdkVersion": "17.0"
    }
}`

const TestRuntimeId = "1111111"

func TestInstallXcode(t *testing.T) {
	t.Parallel()

	Convey("installXcode works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		installArgs := InstallArgs{
			xcodeVersion:           "testVersion",
			xcodeAppPath:           "testdata/Xcode-old.app",
			acceptedLicensesFile:   "testdata/acceptedLicenses.plist",
			cipdPackagePrefix:      "test/prefix",
			kind:                   macKind,
			serviceAccountJSON:     "",
			packageInstallerOnBots: "testdata/dummy_installer",
			withRuntime:            false,
		}

		Convey("for accepted license, mac", func() {
			s.ReturnOutput = []string{
				"12.2.1", // MacOS Version
				"cipd dry run",
				"cipd ensures",
				"chomod prints nothing",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"accept license prints nothing",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch installs packages",
				"xcrun simctl list prints a list of all simulators installed",
				"Developer mode is currently enabled.\n",
			}
			err := installXcode(ctx, installArgs)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 12)
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "testdata/Xcode-old.app",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})

		Convey("for already installed package with Developer mode enabled and -runFirstLaunch needs to run", func() {
			s.ReturnError = []error{
				errors.Reason("check OS version error").Err(),
				errors.Reason("CIPD package already installed").Err(),
			}
			s.ReturnOutput = []string{
				"12.2.1", // MacOS Version
				"cipd dry run",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"accept license prints nothing",
				"original/Xcode.app",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch installs packages",
				"xcrun simctl list prints a list of all simulators installed",
				"xcode-select -s prints nothing",
				"Developer mode is currently enabled.\n",
			}
			err := installXcode(ctx, installArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 11)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\n")
			So(s.Calls[callCounter].Env, ShouldResemble, []string(nil))

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "original/Xcode.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})

		})

		Convey("for already installed package with Developer mode disabled", func() {
			s.ReturnError = []error{
				errors.Reason("check OS version error").Err(),
				errors.Reason("already installed").Err(),
			}
			s.ReturnOutput = []string{
				"12.2.1", // MacOS Version
				"",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"accept license prints nothing",
				"original/Xcode.app",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch installs packages",
				"xcrun simctl list prints a list of all simulators installed",
				"xcode-select -s prints nothing",
				"Developer mode is currently disabled.",
			}
			err := installXcode(ctx, installArgs)
			So(err.Error(), ShouldContainSubstring, "Developer mode is currently disabled! Please use `sudo /usr/sbin/DevToolsSecurity -enable` to enable.")
			So(s.Calls, ShouldHaveLength, 11)

			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "original/Xcode.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})

		Convey("with a service account", func() {
			s.ReturnOutput = []string{
				"12.2.1", // MacOS Version
				"cipd dry run",
				"cipd ensures",
				"chomod prints nothing",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch installs packages",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch installs packages",
				"xcrun simctl list prints a list of all simulators installed",
				"Developer mode is currently enabled.\n",
			}
			installArgs.serviceAccountJSON = "test/service-account.json"
			err := installXcode(ctx, installArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 12)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
				"-service-account-json", "test/service-account.json",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
				"-service-account-json", "test/service-account.json",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "testdata/Xcode-old.app",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})

		Convey("for new license, ios", func() {
			s.ReturnError = []error{
				errors.Reason("check OS version error").Err(),
				errors.Reason("check OS version error").Err(),
				errors.Reason("already installed").Err(),
			}
			s.ReturnOutput = []string{
				"12.2.1", // MacOS Version
				"12.2.1", // MacOS Version
				"cipd dry run",
				"old/xcode/path",
				"xcode-select -s prints nothing",
				"license accept",
				"xcode-select -s prints nothing",
				"old/xcode/path",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch",
				"xcrun simctl list prints a list of all simulators installed",
				"xcode-select -s prints nothing",
				"Developer mode is currently enabled.",
			}

			installArgsForIOS := installArgs
			installArgsForIOS.xcodeAppPath = "testdata/Xcode-new.app"
			installArgsForIOS.kind = iosKind
			err := installXcode(ctx, installArgsForIOS)
			So(err, ShouldBeNil)
			So(len(s.Calls), ShouldEqual, 13)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-new.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual,
				"test/prefix/mac testVersion\n"+
					"test/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-new.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "old/xcode/path"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-new.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "old/xcode/path"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})

	})

	Convey("install Xcode ios mode with/without ios runtime", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		installArgs := InstallArgs{
			xcodeVersion:           "testVersion",
			xcodeAppPath:           "testdata/Xcode-old.app",
			acceptedLicensesFile:   "testdata/acceptedLicenses.plist",
			cipdPackagePrefix:      "test/prefix",
			kind:                   iosKind,
			serviceAccountJSON:     "",
			packageInstallerOnBots: "testdata/dummy_installer",
			withRuntime:            true,
		}

		Convey("install with runtime", func() {
			s.ReturnOutput = []string{
				"12.2.1",                // MacOS Version
				"12.2.1",                // MacOS Version
				"cipd dry run",          // 0 (index in s.Calls, same below)
				"cipd ensures",          // 1
				"chomod prints nothing", // 2
				"",
				"xcode-select -s prints nothing",
				"xcodebuild -license accept returns nothing",
				"",                               // 3: No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing", // 4
				"xcodebuild -runFirstLaunch installs packages",                // 5
				"xcrun simctl list prints a list of all simulators installed", // 6
				"",                                       // 7: successfully resolved (by returning no error)
				"cipd dry run",                           // 8
				"cipd ensures",                           // 9
				"chomod prints nothing",                  // 10
				"Developer mode is currently enabled.\n", // 11
			}
			installArgsForTest := installArgs
			installArgsForTest.withRuntime = true
			// Clean up the added runtime dir.
			defer os.RemoveAll("testdata/Xcode-old.app/Contents/Developer/Platforms")
			err := installXcode(ctx, installArgsForTest)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 17)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\ntest/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\ntest/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "testdata/Xcode-old.app",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testVersion",
			})

			// Normalize for win builder tests.
			runtimeInstallPath := filepath.FromSlash("testdata/Xcode-old.app/Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", runtimeInstallPath,
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", runtimeInstallPath,
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", runtimeInstallPath,
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})

		Convey("with runtime but runtime already exist", func() {
			s.ReturnOutput = []string{
				"12.2.1",                // MacOS Version
				"12.2.1",                // MacOS Version
				"cipd dry run",          // 0 (index in s.Calls, same below)
				"cipd ensures",          // 1
				"chomod prints nothing", // 2
				"",
				"xcode-select -s prints nothing",
				"xcodebuild -license accept returns nothing",
				"",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch installs packages",
				"xcrun simctl list prints a list of all simulators installed",
				"Developer mode is currently enabled.\n",
			}
			installArgsForTest := installArgs
			installArgsForTest.withRuntime = true
			installArgsForTest.xcodeAppPath = "testdata/Xcode-with-runtime.app"
			err := installXcode(ctx, installArgsForTest)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 13)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-with-runtime.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\ntest/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-with-runtime.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\ntest/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "testdata/Xcode-with-runtime.app",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-with-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-with-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})

		Convey("without runtime", func() {
			s.ReturnOutput = []string{
				"12.2.1",                // MacOS Version
				"12.2.1",                // MacOS Version
				"cipd dry run",          // 0 (index in s.Calls, same below)
				"cipd ensures",          // 1
				"chomod prints nothing", // 2
				"",
				"xcode-select -s prints nothing",
				"xcodebuild -license accept returns nothing",
				"",                               // 3: No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing", // 4
				"xcodebuild -runFirstLaunch installs packages",                // 5
				"xcrun simctl list prints a list of all simulators installed", // 6
				"Developer mode is currently enabled.\n",                      // 7
			}
			installArgsForTest := installArgs
			installArgsForTest.withRuntime = false
			installArgsForTest.xcodeAppPath = "testdata/Xcode-old.app"
			err := installXcode(ctx, installArgsForTest)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 13)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\ntest/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/mac testVersion\ntest/prefix/ios testVersion\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "testdata/Xcode-old.app",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})
	})

	Convey("installXcode on MacOS 13+", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		installArgs := InstallArgs{
			xcodeVersion:           "xcode-test-version",
			xcodeAppPath:           "testdata/Xcode-new.app",
			acceptedLicensesFile:   "testdata/acceptedLicenses.plist",
			cipdPackagePrefix:      "test/prefix",
			kind:                   macKind,
			serviceAccountJSON:     "",
			packageInstallerOnBots: "testdata/dummy_installer",
			withRuntime:            false,
		}

		Convey("install iOS Xcode on MacOS13+ should only install mac package", func() {
			s.ReturnOutput = []string{
				"13.2.1", // MacOS Version
				"13.2.1", // MacOS Version
				"cipd dry run",
				"cipd ensures",
				"chomod prints nothing",
				"", // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"license accpet",
				"testdata/Xcode-new.app",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch",
				"xcrun simctl list prints a list of all simulators installed",
				"xcode-select -s prints nothing",
				"Developer mode is currently enabled.\n",
			}
			installArgsForIOS := installArgs
			installArgsForIOS.kind = iosKind
			err := installXcode(ctx, installArgsForIOS)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 14)
		})

		Convey("install Xcode with runtime dmg when not already exists", func() {
			s.ReturnOutput = []string{
				"13.2.1", // MacOS Version
				"",       // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"license accpet",
				"testdata/Xcode-without-runtime.app",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch",
				"xcrun simctl list prints a list of all simulators installed",
				"xcode-select -s prints nothing",
				"testdata/Xcode-without-runtime.app",
				"xcode-select -s prints nothing",
				"xcrun simctl delete -d prints a list of runtime being deleted",
				"xcrun simctl list prints iOS 17.0 (21A5248u) - AAAAA (Deleting)",
				"xcrun simctl list prints nothing",
				"xcode-select -s prints nothing",
				"describe ios_runtime_dmg returns ios_runtime_version:ios-16-2",
				"describe ios_runtime_dmg returns ios_runtime_version:ios-17-0",
				"ios_runtime_build:21A5248u",
				"testdata/Xcode-without-runtime.app",
				"xcode-select -s prints nothing",
				ListRuntimeDiffBuildJson,
				"xcode-select -s prints nothing",
				"describe ios_runtime_dmg returns ios_runtime_version:ios-16-2",
				"describe ios_runtime_dmg returns ios_runtime_version:ios-17-0",
				"cipd dry run",
				"cipd ensures",
				"chomod prints nothing",
			}
			installArgsForTest := installArgs
			installArgsForTest.withRuntime = true
			installArgsForTest.xcodeAppPath = "testdata/Xcode-without-runtime.app"
			installArgsForTest.xcodeVersion = "TESTBUILDVERSION"
			err := installXcode(ctx, installArgsForTest)
			So(err, ShouldNotBeNil)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "delete", "-d", "14"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "TESTBUILDVERSION",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-17-0",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-17-0",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list", "-j"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})
		})

		Convey("install Xcode with runtime dmg when already exists", func() {
			s.ReturnOutput = []string{
				"13.2.1", // MacOS Version
				"",       // No original Xcode when running xcode-select -p
				"xcode-select -s prints nothing",
				"license accpet",
				"testdata/Xcode-without-runtime.app",
				"xcode-select -s prints nothing",
				"xcodebuild -runFirstLaunch",
				"xcrun simctl list prints a list of all simulators installed",
				"xcode-select -s prints nothing",
				"testdata/Xcode-without-runtime.app",
				"xcode-select -s prints nothing",
				"xcrun simctl delete -d prints a list of runtime being deleted",
				"xcrun simctl list prints nothing",
				"xcode-select -s prints nothing",
				"descrone ios_runtime_dmg returns ios_runtime_version:ios-17-0",
				"ios_runtime_build:21A5248u",
				"testdata/Xcode-without-runtime.app",
				"xcode-select -s prints nothing",
				ListRuntimeJson,
				"xcode-select -s prints nothing",
				"Developer mode is currently enabled.\n",
			}
			installArgsForTest := installArgs
			installArgsForTest.withRuntime = true
			installArgsForTest.xcodeAppPath = "testdata/Xcode-without-runtime.app"
			installArgsForTest.xcodeVersion = "TESTBUILDVERSION"
			err := installXcode(ctx, installArgsForTest)
			So(err, ShouldBeNil)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-license", "accept"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcodebuild", "-runFirstLaunch"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "delete", "-d", "14"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "TESTBUILDVERSION",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "TESTBUILDVERSION",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list", "-j"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/sbin/DevToolsSecurity")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-status"})
		})
	})

	Convey("describeRef works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		Convey("ref exists", func() {
			s.ReturnOutput = []string{
				"Package:       test/prefix/mac",
			}
			output, err := describeRef(ctx, "test/prefix/mac", "testXcodeVersion")
			So(err, ShouldBeNil)
			So(output, ShouldNotEqual, "")
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"describe", "test/prefix/mac", "-version", "testXcodeVersion",
			})
		})
		Convey("ref doesn't exist", func() {
			s.ReturnError = []error{errors.Reason("no such ref").Err()}
			output, err := describeRef(ctx, "test/prefix/mac", "testNonExistRef")
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"describe", "test/prefix/mac", "-version", "testNonExistRef",
			})
			So(output, ShouldEqual, "")
			So(err.Error(), ShouldContainSubstring, "Error when describing package path test/prefix/mac with ref testNonExistRef.")
		})
	})

	Convey("shouldReInstallXcode works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		Convey("Xcode doesn't exists so it needs to be re-intalled", func() {
			result, err := shouldReInstallXcode(ctx, "testdata/nonexistent.app", "testXcodeVersion")
			So(err, ShouldNotBeNil)
			So(result, ShouldEqual, true)
		})

		Convey("Xcode exists but expected version is different so it needs to be re-intalled", func() {
			result, err := shouldReInstallXcode(ctx, "testdata/Xcode-new.app", "testXcodeVersion")
			So(result, ShouldEqual, true)
			So(err, ShouldBeNil)
		})

		Convey("Xcode exists and expected version is the same so it doesn't need to be re-intalled", func() {
			result, err := shouldReInstallXcode(ctx, "testdata/Xcode-new.app", "TESTBUILDVERSION")
			So(result, ShouldEqual, false)
			So(err, ShouldBeNil)
		})

	})

	Convey("resolveRef works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		Convey("ref exists", func() {
			err := resolveRef(ctx, "test/prefix/ios_runtime", "testXcodeVersion", "")
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testXcodeVersion",
			})
		})
		Convey("ref doesn't exist", func() {
			s.ReturnError = []error{errors.Reason("input ref doesn't exist").Err()}
			err := resolveRef(ctx, "test/prefix/ios_runtime", "testNonExistRef", "")
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testNonExistRef",
			})
			So(err.Error(), ShouldContainSubstring, "Error when resolving package path test/prefix/ios_runtime with ref testNonExistRef.")
		})
	})

	Convey("resolveRuntimeRef works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		Convey("only input xcode version", func() {
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testXcodeVersion",
			})
			So(ver, ShouldEqual, "testXcodeVersion")
		})
		Convey("only input sim runtime version", func() {
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			})
			So(ver, ShouldEqual, "testSimVersion")
		})
		Convey("input both Xcode and sim version: default runtime exists", func() {
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			})
			So(ver, ShouldEqual, "testSimVersion_testXcodeVersion")
		})
		Convey("input both Xcode and sim version: fallback to uploaded runtime", func() {
			s.ReturnError = []error{errors.Reason("default runtime doesn't exist").Err()}
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 2)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			})
			So(s.Calls[1].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			})
			So(ver, ShouldEqual, "testSimVersion")
		})
		Convey("input both Xcode and sim version: fallback to any latest runtime", func() {
			s.ReturnError = []error{
				errors.Reason("default runtime doesn't exist").Err(),
				errors.Reason("uploaded runtime doesn't exist").Err(),
			}
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 3)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			})
			So(s.Calls[1].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			})
			So(s.Calls[2].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_latest",
			})
			So(ver, ShouldEqual, "testSimVersion_latest")
		})
		Convey("input both Xcode and sim version: raise when all fallbacks fail", func() {
			s.ReturnError = []error{
				errors.Reason("default runtime doesn't exist").Err(),
				errors.Reason("uploaded runtime doesn't exist").Err(),
				errors.Reason("any latest runtime doesn't exist").Err(),
			}
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			So(s.Calls, ShouldHaveLength, 3)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			})
			So(s.Calls[1].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			})
			So(s.Calls[2].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_latest",
			})
			So(err.Error(), ShouldContainSubstring, "Failed to resolve runtime ref given runtime version: testSimVersion, xcode version: testXcodeVersion.")
			So(ver, ShouldEqual, "")
		})

	})

	Convey("unzipXcodeArchive works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		Convey("unzipXcodeArchive should unzip xocde", func() {
			err := os.MkdirAll("test-Xcode.app", 0700)
			So(err, ShouldBeNil)
			err = os.MkdirAll("./testdata/Xcode-unarchive.app", 0700)
			So(err, ShouldBeNil)
			defer os.RemoveAll("./test-Xcode.app")
			defer os.RemoveAll("./testdata/Xcode-unarchive.app")
			err = unzipXcodeArchive(ctx, "./testdata/xcode-archive/", "./testdata/Xcode-unarchive.app")
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 1)
			So(s.Calls[0].Args, ShouldResemble, []string{
				"--expand", filepath.Join("./testdata/xcode-archive/", "xcode.xip"),
			})
		})
	})

	Convey("installRuntime works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("install an Xcode default runtime", func() {
			runtimeInstallArgs := RuntimeInstallArgs{
				runtimeVersion:     "",
				xcodeVersion:       "testVersion",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntime(ctx, runtimeInstallArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 4)
			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testVersion",
			})

			So(s.Calls[1].Executable, ShouldEqual, "cipd")
			So(s.Calls[1].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[1].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime testVersion\n")

			So(s.Calls[2].Executable, ShouldEqual, "cipd")
			So(s.Calls[2].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[2].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime testVersion\n")

			So(s.Calls[3].Executable, ShouldEqual, "chmod")
			So(s.Calls[3].Args, ShouldResemble, []string{
				"-R", "u+w", "test/path/to/install/runtimes",
			})
		})

		Convey("install an uploaded runtime", func() {
			runtimeInstallArgs := RuntimeInstallArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntime(ctx, runtimeInstallArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 4)
			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldResemble, []string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			})

			So(s.Calls[1].Executable, ShouldEqual, "cipd")
			So(s.Calls[1].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[1].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime testSimVersion\n")

			So(s.Calls[2].Executable, ShouldEqual, "cipd")
			So(s.Calls[2].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[2].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime testSimVersion\n")

			So(s.Calls[3].Executable, ShouldEqual, "chmod")
			So(s.Calls[3].Args, ShouldResemble, []string{
				"-R", "u+w", "test/path/to/install/runtimes",
			})
		})
	})

	Convey("installRuntimeDMG works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		Convey("install runtime DMG without xcode-version ref", func() {
			s.ReturnOutput = []string{
				"describe ios_runtime_dmg returns ios_runtime_version:ios-test-runtime",
				"dry run ensure file returns nothing",
				"ensure file returns nothing",
				"chmod returns nothing",
			}
			runtimeDMGInstallArgs := RuntimeDMGInstallArgs{
				runtimeVersion:     "ios-test-runtime",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntimeDMG(ctx, runtimeDMGInstallArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 4)

			callCounter := 0
			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-test-runtime",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime_dmg ios-test-runtime\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime_dmg ios-test-runtime\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "test/path/to/install/runtimes",
			})
		})

		Convey("install runtime DMG with xcode-version ref", func() {
			s.ReturnOutput = []string{
				"describe ios_runtime_dmg returns ios_runtime_version:ios-test-runtime",
				"dry run ensure file returns nothing",
				"ensure file returns nothing",
				"chmod returns nothing",
			}
			runtimeDMGInstallArgs := RuntimeDMGInstallArgs{
				runtimeVersion:     "ios-test-runtime",
				xcodeVersion:       "xcode-test-version",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntimeDMG(ctx, runtimeDMGInstallArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 4)

			callCounter := 0
			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "xcode-test-version",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime_dmg xcode-test-version\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime_dmg xcode-test-version\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "test/path/to/install/runtimes",
			})
		})

		Convey("install runtime DMG with xcode-version ref and mismatched runtime", func() {
			s.ReturnOutput = []string{
				"describe ios_runtime_dmg returns ios_runtime_version:mismatched-runtime",
				"describe ios_runtime_dmg returns ios_runtime_version:ios-test-runtime",
				"dry run ensure file returns nothing",
				"ensure file returns nothing",
				"chmod returns nothing",
			}
			runtimeDMGInstallArgs := RuntimeDMGInstallArgs{
				runtimeVersion:     "ios-test-runtime",
				xcodeVersion:       "xcode-test-version",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntimeDMG(ctx, runtimeDMGInstallArgs)
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 5)

			callCounter := 0
			So(s.Calls[0].Executable, ShouldEqual, "cipd")
			So(s.Calls[0].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "xcode-test-version",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-test-runtime",
			})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime_dmg ios-test-runtime\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "cipd")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			})
			So(s.Calls[callCounter].ConsumedStdin, ShouldEqual, "test/prefix/ios_runtime_dmg ios-test-runtime\n")

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "chmod")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{
				"-R", "u+w", "test/path/to/install/runtimes",
			})
		})
	})

	Convey("addRuntimeDMG works", t, func() {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		Convey("addRuntimeDMG should succeed", func() {
			s.ReturnOutput = []string{
				"testdata/Xcode-old.app",
				"xcode-select -s prints nothing",
				"xcrun simctl runtime add returns " + TestRuntimeId,
				ListRuntimeJson,
				ListRuntimeSDKJson,
				"xcrun simctl runtime match set returns nothing",
				"xcode-select -s prints nothing",
			}
			err := addRuntimeDMG(ctx, "testdata/Xcode-old.app", "random-path/runtime.dmg")
			So(err, ShouldBeNil)
			So(s.Calls, ShouldHaveLength, 7)

			callCounter := 0
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "add", "random-path/runtime.dmg"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list", "-j"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "match", "list", "-j"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "match", "set", "iphoneos17.0", "21A5248u", "--sdkBuild", "21A111112"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})
		})

		Convey("addRuntimeDMG runtime id not found", func() {
			s.ReturnOutput = []string{
				"testdata/Xcode-old.app",
				"xcode-select -s prints nothing",
				"xcrun simctl runtime add returns nothing",
				ListRuntimeJson,
				"xcode-select -s prints nothing",
			}
			err := addRuntimeDMG(ctx, "testdata/Xcode-old.app", "random-path/runtime.dmg")
			So(err, ShouldNotBeNil)
			So(s.Calls, ShouldHaveLength, 5)

			callCounter := 0
			So(s.Calls[callCounter].Executable, ShouldEqual, "/usr/bin/xcode-select")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-p"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "add", "random-path/runtime.dmg"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "xcrun")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"simctl", "runtime", "list", "-j"})

			callCounter++
			So(s.Calls[callCounter].Executable, ShouldEqual, "sudo")
			So(s.Calls[callCounter].Args, ShouldResemble, []string{"-n", "/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"})
		})
	})

	Convey("removeCipdFiles works", t, func() {
		Convey("remove cipd files whether it exists or not", func() {
			srcPath := "testdata/"
			tmpCipdPath, err := os.MkdirTemp(srcPath, "tmp")
			So(err, ShouldBeNil)
			defer os.RemoveAll(tmpCipdPath)

			// folder is empty but it should still succeed
			err = removeCipdFiles(tmpCipdPath)
			So(err, ShouldBeNil)

			// create cipd files so they can be removed
			dotCipdPath := filepath.Join(tmpCipdPath, ".cipd")
			dotXcodeVersionPath := filepath.Join(tmpCipdPath, ".xcode_versions")
			err = os.MkdirAll(dotCipdPath, 0700)
			So(err, ShouldBeNil)
			err = os.MkdirAll(dotXcodeVersionPath, 0700)
			So(err, ShouldBeNil)

			err = removeCipdFiles(tmpCipdPath)
			So(err, ShouldBeNil)

			// files should not exist after removing
			_, err = os.Stat(dotCipdPath)
			So(err, ShouldNotBeNil)
			_, err = os.Stat(dotXcodeVersionPath)
			So(err, ShouldNotBeNil)

		})
	})

	Convey("getIOSVersionWithoutPatch works", t, func() {
		Convey("Version without patch number should return original", func() {
			iosVersion := "17.0"
			trunctedVersion := getIOSVersionWithoutPatch(iosVersion)

			// folder is empty but it should still succeed
			So(trunctedVersion, ShouldEqual, iosVersion)
		})

		Convey("Version without patch number should return version without patch", func() {
			iosVersion := "17.0.1.2"
			trunctedVersion := getIOSVersionWithoutPatch(iosVersion)

			// folder is empty but it should still succeed
			So(trunctedVersion, ShouldEqual, "17.0")
		})
	})

}
