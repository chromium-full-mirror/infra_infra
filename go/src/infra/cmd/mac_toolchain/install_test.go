// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
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

	ftt.Run("installXcode works", t, func(t *ftt.Test) {
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

		t.Run("for accepted license, mac", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(12))
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "testdata/Xcode-old.app",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})

		t.Run("for already installed package with Developer mode enabled and -runFirstLaunch needs to run", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(11))
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\n"))
			assert.Loosely(t, s.Calls[callCounter].Env, should.Resemble([]string(nil)))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "original/Xcode.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))

		})

		t.Run("for already installed package with Developer mode disabled", func(t *ftt.Test) {
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
			assert.Loosely(t, err.Error(), should.ContainSubstring("Developer mode is currently disabled! Please use `sudo /usr/sbin/DevToolsSecurity -enable` to enable."))
			assert.Loosely(t, s.Calls, should.HaveLength(11))

			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "original/Xcode.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})

		t.Run("with a service account", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(12))
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
				"-service-account-json", "test/service-account.json",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
				"-service-account-json", "test/service-account.json",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "testdata/Xcode-old.app",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})

		t.Run("for new license, ios", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(s.Calls), should.Equal(13))
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-new.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal(
				"test/prefix/mac testVersion\n"+
					"test/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-new.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "old/xcode/path"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-new.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "old/xcode/path"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})

	})

	ftt.Run("install Xcode ios mode with/without ios runtime", t, func(t *ftt.Test) {
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

		t.Run("install with runtime", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(17))
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\ntest/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\ntest/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "testdata/Xcode-old.app",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testVersion",
			}))

			// Normalize for win builder tests.
			runtimeInstallPath := filepath.FromSlash("testdata/Xcode-old.app/Contents/Developer/Platforms/iPhoneOS.platform/Library/Developer/CoreSimulator/Profiles/Runtimes")

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", runtimeInstallPath,
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", runtimeInstallPath,
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", runtimeInstallPath,
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})

		t.Run("with runtime but runtime already exist", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(13))
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-with-runtime.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\ntest/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-with-runtime.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\ntest/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "testdata/Xcode-with-runtime.app",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-with-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-with-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})

		t.Run("without runtime", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(13))
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\ntest/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "testdata/Xcode-old.app",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/mac testVersion\ntest/prefix/ios testVersion\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "testdata/Xcode-old.app",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})
	})

	ftt.Run("installXcode on MacOS 13+", t, func(t *ftt.Test) {
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

		t.Run("install iOS Xcode on MacOS13+ should only install mac package", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(14))
		})

		t.Run("install Xcode with runtime dmg when not already exists", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.NotBeNil)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "delete", "-d", "14"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "TESTBUILDVERSION",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-17-0",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-17-0",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list", "-j"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))
		})

		t.Run("install Xcode with runtime dmg when already exists", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			callCounter := 0
			// skip MacOS version check calls
			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-license", "accept"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcodebuild", "-runFirstLaunch"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "delete", "-d", "14"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "TESTBUILDVERSION",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "TESTBUILDVERSION",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list", "-j"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-without-runtime.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/sbin/DevToolsSecurity"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-status"}))
		})
	})

	ftt.Run("describeRef works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		t.Run("ref exists", func(t *ftt.Test) {
			s.ReturnOutput = []string{
				"Package:       test/prefix/mac",
			}
			output, err := describeRef(ctx, "test/prefix/mac", "testXcodeVersion")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, output, should.NotEqual(""))
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"describe", "test/prefix/mac", "-version", "testXcodeVersion",
			}))
		})
		t.Run("ref doesn't exist", func(t *ftt.Test) {
			s.ReturnError = []error{errors.Reason("no such ref").Err()}
			output, err := describeRef(ctx, "test/prefix/mac", "testNonExistRef")
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"describe", "test/prefix/mac", "-version", "testNonExistRef",
			}))
			assert.Loosely(t, output, should.BeEmpty)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Error when describing package path test/prefix/mac with ref testNonExistRef."))
		})
	})

	ftt.Run("shouldReInstallXcode works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		t.Run("Xcode doesn't exists so it needs to be re-intalled", func(t *ftt.Test) {
			result, err := shouldReInstallXcode(ctx, "testdata/nonexistent.app", "testXcodeVersion")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, result, should.Equal(true))
		})

		t.Run("Xcode exists but expected version is different so it needs to be re-intalled", func(t *ftt.Test) {
			result, err := shouldReInstallXcode(ctx, "testdata/Xcode-new.app", "testXcodeVersion")
			assert.Loosely(t, result, should.Equal(true))
			assert.Loosely(t, err, should.BeNil)
		})

		t.Run("Xcode exists and expected version is the same so it doesn't need to be re-intalled", func(t *ftt.Test) {
			result, err := shouldReInstallXcode(ctx, "testdata/Xcode-new.app", "TESTBUILDVERSION")
			assert.Loosely(t, result, should.Equal(false))
			assert.Loosely(t, err, should.BeNil)
		})

	})

	ftt.Run("resolveRef works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		t.Run("ref exists", func(t *ftt.Test) {
			err := resolveRef(ctx, "test/prefix/ios_runtime", "testXcodeVersion", "")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testXcodeVersion",
			}))
		})
		t.Run("ref doesn't exist", func(t *ftt.Test) {
			s.ReturnError = []error{errors.Reason("input ref doesn't exist").Err()}
			err := resolveRef(ctx, "test/prefix/ios_runtime", "testNonExistRef", "")
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testNonExistRef",
			}))
			assert.Loosely(t, err.Error(), should.ContainSubstring("Error when resolving package path test/prefix/ios_runtime with ref testNonExistRef."))
		})
	})

	ftt.Run("resolveRuntimeRef works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		t.Run("only input xcode version", func(t *ftt.Test) {
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testXcodeVersion",
			}))
			assert.Loosely(t, ver, should.Equal("testXcodeVersion"))
		})
		t.Run("only input sim runtime version", func(t *ftt.Test) {
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			}))
			assert.Loosely(t, ver, should.Equal("testSimVersion"))
		})
		t.Run("input both Xcode and sim version: default runtime exists", func(t *ftt.Test) {
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			}))
			assert.Loosely(t, ver, should.Equal("testSimVersion_testXcodeVersion"))
		})
		t.Run("input both Xcode and sim version: fallback to uploaded runtime", func(t *ftt.Test) {
			s.ReturnError = []error{errors.Reason("default runtime doesn't exist").Err()}
			resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "testXcodeVersion",
				packagePath:        "test/prefix/ios_runtime",
				serviceAccountJSON: "",
			}
			ver, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(2))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			}))
			assert.Loosely(t, s.Calls[1].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			}))
			assert.Loosely(t, ver, should.Equal("testSimVersion"))
		})
		t.Run("input both Xcode and sim version: fallback to any latest runtime", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(3))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			}))
			assert.Loosely(t, s.Calls[1].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			}))
			assert.Loosely(t, s.Calls[2].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_latest",
			}))
			assert.Loosely(t, ver, should.Equal("testSimVersion_latest"))
		})
		t.Run("input both Xcode and sim version: raise when all fallbacks fail", func(t *ftt.Test) {
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
			assert.Loosely(t, s.Calls, should.HaveLength(3))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_testXcodeVersion",
			}))
			assert.Loosely(t, s.Calls[1].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			}))
			assert.Loosely(t, s.Calls[2].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion_latest",
			}))
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to resolve runtime ref given runtime version: testSimVersion, xcode version: testXcodeVersion."))
			assert.Loosely(t, ver, should.BeEmpty)
		})

	})

	ftt.Run("unzipXcodeArchive works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		t.Run("unzipXcodeArchive should unzip xocde", func(t *ftt.Test) {
			err := os.MkdirAll("test-Xcode.app", 0700)
			assert.Loosely(t, err, should.BeNil)
			err = os.MkdirAll("./testdata/Xcode-unarchive.app", 0700)
			assert.Loosely(t, err, should.BeNil)
			defer os.RemoveAll("./test-Xcode.app")
			defer os.RemoveAll("./testdata/Xcode-unarchive.app")
			err = unzipXcodeArchive(ctx, "./testdata/xcode-archive/", "./testdata/Xcode-unarchive.app")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(1))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"--expand", filepath.Join("./testdata/xcode-archive/", "xcode.xip"),
			}))
		})
	})

	ftt.Run("installRuntime works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("install an Xcode default runtime", func(t *ftt.Test) {
			runtimeInstallArgs := RuntimeInstallArgs{
				runtimeVersion:     "",
				xcodeVersion:       "testVersion",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntime(ctx, runtimeInstallArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(4))
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testVersion",
			}))

			assert.Loosely(t, s.Calls[1].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[1].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[1].ConsumedStdin, should.Equal("test/prefix/ios_runtime testVersion\n"))

			assert.Loosely(t, s.Calls[2].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[2].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[2].ConsumedStdin, should.Equal("test/prefix/ios_runtime testVersion\n"))

			assert.Loosely(t, s.Calls[3].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[3].Args, should.Resemble([]string{
				"-R", "u+w", "test/path/to/install/runtimes",
			}))
		})

		t.Run("install an uploaded runtime", func(t *ftt.Test) {
			runtimeInstallArgs := RuntimeInstallArgs{
				runtimeVersion:     "testSimVersion",
				xcodeVersion:       "",
				installPath:        "test/path/to/install/runtimes",
				cipdPackagePrefix:  "test/prefix",
				serviceAccountJSON: "",
			}
			err := installRuntime(ctx, runtimeInstallArgs)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(4))
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"resolve", "test/prefix/ios_runtime", "-version", "testSimVersion",
			}))

			assert.Loosely(t, s.Calls[1].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[1].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[1].ConsumedStdin, should.Equal("test/prefix/ios_runtime testSimVersion\n"))

			assert.Loosely(t, s.Calls[2].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[2].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[2].ConsumedStdin, should.Equal("test/prefix/ios_runtime testSimVersion\n"))

			assert.Loosely(t, s.Calls[3].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[3].Args, should.Resemble([]string{
				"-R", "u+w", "test/path/to/install/runtimes",
			}))
		})
	})

	ftt.Run("installRuntimeDMG works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("install runtime DMG without xcode-version ref", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(4))

			callCounter := 0
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-test-runtime",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime_dmg ios-test-runtime\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime_dmg ios-test-runtime\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "test/path/to/install/runtimes",
			}))
		})

		t.Run("install runtime DMG with xcode-version ref", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(4))

			callCounter := 0
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "xcode-test-version",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime_dmg xcode-test-version\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime_dmg xcode-test-version\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "test/path/to/install/runtimes",
			}))
		})

		t.Run("install runtime DMG with xcode-version ref and mismatched runtime", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(5))

			callCounter := 0
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "xcode-test-version",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"describe", "test/prefix/ios_runtime_dmg", "-version", "ios-test-runtime",
			}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"puppet-check-updates", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime_dmg ios-test-runtime\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("cipd"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"ensure", "-ensure-file", "-", "-root", "test/path/to/install/runtimes",
			}))
			assert.Loosely(t, s.Calls[callCounter].ConsumedStdin, should.Equal("test/prefix/ios_runtime_dmg ios-test-runtime\n"))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("chmod"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{
				"-R", "u+w", "test/path/to/install/runtimes",
			}))
		})
	})

	ftt.Run("addRuntimeDMG works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)
		t.Run("addRuntimeDMG should succeed", func(t *ftt.Test) {
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
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(7))

			callCounter := 0
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "add", "random-path/runtime.dmg"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list", "-j"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "match", "list", "-j"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "match", "set", "iphoneos17.0", "21A5248u", "--sdkBuild", "21A111112"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))
		})

		t.Run("addRuntimeDMG runtime id not found", func(t *ftt.Test) {
			s.ReturnOutput = []string{
				"testdata/Xcode-old.app",
				"xcode-select -s prints nothing",
				"xcrun simctl runtime add returns nothing",
				ListRuntimeJson,
				"xcode-select -s prints nothing",
			}
			err := addRuntimeDMG(ctx, "testdata/Xcode-old.app", "random-path/runtime.dmg")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, s.Calls, should.HaveLength(5))

			callCounter := 0
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("/usr/bin/xcode-select"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"-p"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "add", "random-path/runtime.dmg"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("xcrun"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"simctl", "runtime", "list", "-j"}))

			callCounter++
			assert.Loosely(t, s.Calls[callCounter].Executable, should.Equal("sudo"))
			assert.Loosely(t, s.Calls[callCounter].Args, should.Resemble([]string{"/usr/bin/xcode-select", "-s", "testdata/Xcode-old.app"}))
		})
	})

	ftt.Run("removeCipdFiles works", t, func(t *ftt.Test) {
		t.Run("remove cipd files whether it exists or not", func(t *ftt.Test) {
			srcPath := "testdata/"
			tmpCipdPath, err := os.MkdirTemp(srcPath, "tmp")
			assert.Loosely(t, err, should.BeNil)
			defer os.RemoveAll(tmpCipdPath)

			// folder is empty but it should still succeed
			err = removeCipdFiles(tmpCipdPath)
			assert.Loosely(t, err, should.BeNil)

			// create cipd files so they can be removed
			dotCipdPath := filepath.Join(tmpCipdPath, ".cipd")
			dotXcodeVersionPath := filepath.Join(tmpCipdPath, ".xcode_versions")
			err = os.MkdirAll(dotCipdPath, 0700)
			assert.Loosely(t, err, should.BeNil)
			err = os.MkdirAll(dotXcodeVersionPath, 0700)
			assert.Loosely(t, err, should.BeNil)

			err = removeCipdFiles(tmpCipdPath)
			assert.Loosely(t, err, should.BeNil)

			// files should not exist after removing
			_, err = os.Stat(dotCipdPath)
			assert.Loosely(t, err, should.NotBeNil)
			_, err = os.Stat(dotXcodeVersionPath)
			assert.Loosely(t, err, should.NotBeNil)

		})
	})

	ftt.Run("getIOSVersionWithoutPatch works", t, func(t *ftt.Test) {
		t.Run("Version without patch number should return original", func(t *ftt.Test) {
			iosVersion := "17.0"
			trunctedVersion := getIOSVersionWithoutPatch(iosVersion)

			// folder is empty but it should still succeed
			assert.Loosely(t, trunctedVersion, should.Equal(iosVersion))
		})

		t.Run("Version without patch number should return version without patch", func(t *ftt.Test) {
			iosVersion := "17.0.1.2"
			trunctedVersion := getIOSVersionWithoutPatch(iosVersion)

			// folder is empty but it should still succeed
			assert.Loosely(t, trunctedVersion, should.Equal("17.0"))
		})
	})

}
