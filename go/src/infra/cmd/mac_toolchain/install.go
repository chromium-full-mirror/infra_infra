// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/system/filesystem"
)

func getCipdFileNames() []string {
	return []string{".xcode_versions", ".cipd"}
}

func removeCipdFiles(xcodePackagePath string) error {
	for _, f := range getCipdFileNames() {
		packagePath := filepath.Join(xcodePackagePath, f)
		// remove if the file exists
		if _, err := os.Stat(packagePath); err == nil {
			err := os.RemoveAll(packagePath)
			if err != nil {
				return errors.Annotate(err, "failed to remove cipd file %s", packagePath).Err()
			}
		}
	}
	return nil
}

func getIOSVersionWithoutPatch(iosVersion string) string {
	parts := strings.Split(iosVersion, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return iosVersion
}

func getFileExtension(fileName string) string {
	return fileName[strings.LastIndex(fileName, ".")+1:]
}

func unzipXcodeArchive(ctx context.Context, rootDir, xcodePath string) error {
	// xcode needs to be removed first, if exists
	if _, err := os.Stat(xcodePath); !os.IsNotExist(err) {
		if err := os.RemoveAll(xcodePath); err != nil {
			return errors.Annotate(err, "Failed to remove existing xcode path %s", xcodePath).Err()
		}
	}

	// search for xcode.xip file
	files, err := os.ReadDir(rootDir)
	if err != nil {
		return errors.Annotate(err, "Unable to read root xip directory %s", rootDir).Err()
	}
	xipFilePath := ""
	for _, file := range files {
		if getFileExtension(file.Name()) == "xip" {
			xipFilePath = filepath.Join(rootDir, file.Name())
			break
		}
	}

	if err := RunCommand(ctx, "/usr/bin/xip", "--expand", xipFilePath); err != nil {
		return errors.Annotate(err, "Unable to unzip xip file %s", xipFilePath).Err()
	}

	// search for Xcode.app and move to the destination path
	files, err = os.ReadDir(".")
	if err != nil {
		return errors.Annotate(err, "Unable to read root xip directory %s", rootDir).Err()
	}
	// Iterate through the list of files and find the first file with the extension of `.app`.
	for _, file := range files {
		if strings.Contains(file.Name(), "Xcode") && getFileExtension(file.Name()) == "app" {
			unzippedXcodePath := filepath.Join("./", file.Name())
			err = os.Rename(unzippedXcodePath, xcodePath)
			if err != nil {
				os.RemoveAll(unzippedXcodePath)
				return errors.Annotate(err, "Failed to move Xcode to destination %s %s ", unzippedXcodePath, xcodePath).Err()
			}
			break
		}
	}
	return nil
}

// InstallPackagesArgs are the parameters for installPackages() to keep them manageable.
type InstallPackagesArgs struct {
	ref                string
	rootPath           string
	cipdPackagePrefix  string
	kind               KindType
	serviceAccountJSON string
}

// Installs the cpid package to |rootPath| of specified |kind|, find package
// as input |cipdPackagePrefix| & |ref|. These args are passed within
// |InstallPackagesArgs| struct.
func installPackages(ctx context.Context, args InstallPackagesArgs) error {
	cipdArgs := []string{
		"-ensure-file", "-",
		"-root", args.rootPath,
	}
	if args.serviceAccountJSON != "" {
		cipdArgs = append(cipdArgs, "-service-account-json", args.serviceAccountJSON)
	}
	cipdCheckArgs := append([]string{"puppet-check-updates"}, cipdArgs...)
	cipdEnsureArgs := append([]string{"ensure"}, cipdArgs...)

	ensureSpec := ""
	switch args.kind {
	case macKind:
		ensureSpec += fmt.Sprintf("%s/%s %s\n", args.cipdPackagePrefix, MacPackageName, args.ref)
	case iosKind:
		// TODO(crbug/1420480): on MacOS13+, Xcode is uploaded as one package in mac, so
		// we only need to download the mac package and there's no difference between
		// mac and ios kind. Clean up the if conditions below after all bots are upgraded
		// to MacOS13.
		onMacOS13OrLater, _ := isMacOS13OrLater(ctx)
		if onMacOS13OrLater {
			ensureSpec += fmt.Sprintf("%s/%s %s\n", args.cipdPackagePrefix, MacPackageName, args.ref)
		} else {
			ensureSpec += fmt.Sprintf("%s/%s %s\n%s/%s %s\n", args.cipdPackagePrefix, MacPackageName, args.ref, args.cipdPackagePrefix, IosPackageName, args.ref)
		}
	case iosRuntimeKind:
		ensureSpec += fmt.Sprintf("%s/%s %s\n", args.cipdPackagePrefix, IosRuntimePackageName, args.ref)
	case iosRuntimeDMGKind:
		ensureSpec += fmt.Sprintf("%s/%s %s\n", args.cipdPackagePrefix, IosRuntimeDMGPackageName, args.ref)
	case xcodeArchiveKind:
		ensureSpec += fmt.Sprintf("%s/%s %s\n", args.cipdPackagePrefix, XcodeArchivePackageName, args.ref)
	default:
		return errors.Reason("unknown package kind: %s", args.kind).Err()
	}

	// Check if `cipd ensure` will do something. Note: `cipd puppet-check-updates`
	// returns code 0 when `cipd ensure` has work to do, and "fails" otherwise.
	// TODO(sergeyberezin): replace this with a better option when
	// https://crbug.com/788032 is fixed.
	if err := RunWithStdin(ctx, ensureSpec, "cipd", cipdCheckArgs...); err != nil {
		// The rest logic ensures the Xcode is intact so it only applies to
		// iosKind or macKind.
		if args.kind != macKind && args.kind != iosKind {
			return nil
		}
		xcodeAppPath := args.rootPath
		// Sometimes Xcode cache in bots loses Contents/Developer/usr and CIPD
		// doesn't check if the package is intact. Add an additional check and
		// only return when the directory exists.
		binDirPath := filepath.Join(xcodeAppPath, "Contents", "Developer", "usr", "bin")
		if _, statErr := os.Stat(binDirPath); !os.IsNotExist(statErr) {
			return nil
		}
		logging.Warningf(ctx, "Contents/Developer/usr/bin doesn't exist in cached Xcode. Reinstalling Xcode.")
		// Remove and create an empty Xcode dir so `cipd ensure` will work to
		// download a new one.
		if removeErr := filesystem.RemoveAll(xcodeAppPath); removeErr != nil {
			return errors.Annotate(removeErr, "failed to remove corrupted Xcode package.").Err()
		}
		if err := os.MkdirAll(xcodeAppPath, 0700); err != nil {
			return errors.Annotate(err, "failed to create a folder %s", xcodeAppPath).Err()
		}
	}

	if err := RunWithStdin(ctx, ensureSpec, "cipd", cipdEnsureArgs...); err != nil {
		return errors.Annotate(err, "failed to install CIPD packages: %s", ensureSpec).Err()
	}
	// Xcode really wants its files to be user-writable (hangs mysteriously
	// otherwise). CIPD by default installs everything read-only. Update
	// permissions post-install.
	//
	// TODO(sergeyberezin): remove this once crbug.com/803158 is resolved and all
	// currently used Xcode versions are re-uploaded.
	if err := RunCommand(ctx, "chmod", "-R", "u+w", args.rootPath); err != nil {
		return errors.Annotate(err, "failed to update package permissions in %s for %s", args.rootPath, args.kind).Err()
	}
	return nil
}

func needToAcceptLicense(ctx context.Context, xcodeAppPath, acceptedLicensesFile string) bool {
	licenseInfoFile := filepath.Join(xcodeAppPath, "Contents", "Resources", "LicenseInfo.plist")

	licenseID, licenseType, err := getXcodeLicenseInfo(licenseInfoFile)
	if err != nil {
		errors.Log(ctx, err)
		return true
	}

	acceptedLicenseID, err := getXcodeAcceptedLicense(acceptedLicensesFile, licenseType)
	if err != nil {
		errors.Log(ctx, err)
		return true
	}

	// Historically all Xcode build numbers have been in the format of AANNNN, so
	// a simple string compare works.  If Xcode's build numbers change this may
	// need a more complex compare.
	if licenseID <= acceptedLicenseID {
		// Don't accept the license of older toolchain builds, this will break the
		// license of newer builds.
		return false
	}
	return true
}

func getXcodePath(ctx context.Context) string {
	path, err := RunOutput(ctx, "/usr/bin/xcode-select", "-p")
	if err != nil {
		return ""
	}
	return strings.Trim(path, " \n")
}

func setXcodePath(ctx context.Context, xcodeAppPath string) error {
	err := RunCommand(ctx, "sudo", "-n", "/usr/bin/xcode-select", "-s", xcodeAppPath)
	if err != nil {
		return errors.Annotate(err, "failed xcode-select -s %s", xcodeAppPath).Err()
	}
	return nil
}

// RunWithXcodeSelect temporarily sets the Xcode path with `sudo xcode-select
// -s` and runs a callback.
func RunWithXcodeSelect(ctx context.Context, xcodeAppPath string, f func() error) error {
	oldPath := getXcodePath(ctx)
	if oldPath != "" {
		defer setXcodePath(ctx, oldPath)
	}
	if err := setXcodePath(ctx, xcodeAppPath); err != nil {
		return err
	}
	if err := f(); err != nil {
		return err
	}
	return nil
}

func acceptLicense(ctx context.Context, xcodeAppPath string) error {
	err := RunWithXcodeSelect(ctx, xcodeAppPath, func() error {
		return RunCommand(ctx, "sudo", "-n", "/usr/bin/xcodebuild", "-license", "accept")
	})
	if err != nil {
		return errors.Annotate(err, "failed to accept new license").Err()
	}
	return nil
}

func finalizeInstall(ctx context.Context, xcodeAppPath, xcodeVersion, packageInstallerOnBots string) error {
	return RunWithXcodeSelect(ctx, xcodeAppPath, func() error {
		err := RunCommand(ctx, "sudo", "-n", "/usr/bin/xcodebuild", "-runFirstLaunch")
		if err != nil {
			return errors.Annotate(err, "failed when invoking xcodebuild -runFirstLaunch").Err()
		}
		// This command is needed to avoid a potential compile time issue.
		_, err = RunOutput(ctx, "xcrun", "simctl", "list")
		if err != nil {
			return err
		}
		return nil
	})
}

func checkDeveloperMode(ctx context.Context) error {
	out, err := RunOutput(ctx, "/usr/sbin/DevToolsSecurity", "-status")
	if err != nil {
		return errors.Annotate(err, "failed to run /usr/sbin/DevToolsSecurity -status").Err()
	}
	if !strings.Contains(out, "Developer mode is currently enabled.") {
		return errors.Reason("Developer mode is currently disabled! Please use `sudo /usr/sbin/DevToolsSecurity -enable` to enable.").Err()
	}
	return nil
}

func deleteUnusedIOSRuntime(ctx context.Context, xcodeAppPath string) error {
	return RunWithXcodeSelect(ctx, xcodeAppPath, func() error {
		// delete unused runtime after MaxIOSRuntimeKeepDays
		// -d is abbreviated flag for --notUsedSinceDays
		// More info can be found by appending the -help arg
		output, err := RunOutput(ctx, "xcrun", "simctl", "runtime", "delete", "-d", MaxIOSRuntimeKeepDays)
		logging.Warningf(ctx, "Unused runtimes delete command output: %s", output)
		if err != nil {
			return errors.Annotate(err, "failed when trying to delete unused runtimes.").Err()
		}

		logging.Warningf(ctx, "waiting for runtimes to finish deleting...")
		startTime := time.Now()
		endTime := startTime.Add(30 * time.Second)
		for time.Now().Before(endTime) {
			// Sleep for 2 seconds
			time.Sleep(2 * time.Second)
			out, err := RunOutput(ctx, "xcrun", "simctl", "runtime", "list")
			if err != nil {
				return errors.Annotate(err, "failed to list iOS runtimes").Err()
			}

			if !strings.Contains(out, "Deleting") {
				logging.Warningf(ctx, "Unused runtimes successfully deleted.")
				return nil
			}
		}
		return nil
	})
}

type ResolveRuntimeDMGRefArgs struct {
	runtimeVersion     string
	xcodeVersion       string
	packagePath        string
	serviceAccountJSON string
}

// Returns the best matched simulator runtime in CIPD with |runtimeVersion| and
// |xcodeVersion| as input.
// xcodeVersion will be used to search for runtime for best match.
// If not found, or if the returned cipd instance's ios_runtime_version is not
// the desired runtimeVersion, then runtimeVersion will be used as a second resort.
func resolveRuntimeDMGRef(ctx context.Context, args ResolveRuntimeDMGRefArgs) (string, error) {
	if args.xcodeVersion == "" && args.runtimeVersion == "" {
		err := errors.Reason("Empty Xcode and runtime version to resolve runtime dmg ref.").Err()
		return "", err
	}
	searchRefs := []string{}
	if args.xcodeVersion != "" {
		searchRefs = append(searchRefs, args.xcodeVersion)
	}
	if args.runtimeVersion != "" {
		searchRefs = append(searchRefs, args.runtimeVersion)
	}
	for _, searchRef := range searchRefs {
		if output, err := describeRef(ctx, args.packagePath, searchRef); err == nil {
			var runtimeVersionRegex = regexp.MustCompile(`ios_runtime_version:(.*)`)
			result := runtimeVersionRegex.FindStringSubmatch(output)
			if len(result) > 0 {
				if result[1] == args.runtimeVersion {
					logging.Warningf(ctx, "Using ref %s for runtime DMG", searchRef)
					return searchRef, nil
				} else {
					logging.Warningf(ctx, "Cannot use ref %s for runtime DMG, expected iOS runtime %s, but got %s", searchRef, args.runtimeVersion, result[1])
				}
			}
		} else {
			logging.Warningf(ctx, "Failed to describe ref: %s. Error: %s", searchRef, err.Error())
		}
	}
	err := errors.Reason("Failed to resolve runtime dmg ref given runtime version: %s, xcode version: %s.", args.runtimeVersion, args.xcodeVersion).Err()
	return "", err
}

// RuntimeDMGInstallArgs are the parameters for installRuntimeDMG() to keep them manageable.
type RuntimeDMGInstallArgs struct {
	runtimeVersion     string
	xcodeVersion       string
	installPath        string
	cipdPackagePrefix  string
	serviceAccountJSON string
}

// Resolves and installs the suitable runtime dmg.
func installRuntimeDMG(ctx context.Context, args RuntimeDMGInstallArgs) error {
	if err := os.MkdirAll(args.installPath, 0700); err != nil {
		return errors.Annotate(err, "failed to create a folder %s", args.installPath).Err()
	}

	packagePath := args.cipdPackagePrefix + "/" + IosRuntimeDMGPackageName
	resolveRuntimeDMGRefArgs := ResolveRuntimeDMGRefArgs{
		runtimeVersion:     args.runtimeVersion,
		xcodeVersion:       args.xcodeVersion,
		packagePath:        packagePath,
		serviceAccountJSON: args.serviceAccountJSON,
	}
	ref, err := resolveRuntimeDMGRef(ctx, resolveRuntimeDMGRefArgs)
	if err != nil {
		return errors.Annotate(err, "failed to resolve runtime dmg ref").Err()
	}

	installPackagesArgs := InstallPackagesArgs{
		ref:                ref,
		rootPath:           args.installPath,
		cipdPackagePrefix:  args.cipdPackagePrefix,
		kind:               iosRuntimeDMGKind,
		serviceAccountJSON: args.serviceAccountJSON,
	}
	if err := installPackages(ctx, installPackagesArgs); err != nil {
		return err
	}
	return nil
}

func addRuntimeDMG(ctx context.Context, xcodeAppPath string, dmgFilePath string) error {
	return RunWithXcodeSelect(ctx, xcodeAppPath, func() error {
		// add runtime dmg to Xcode
		addOutput, err := RunOutput(ctx, "xcrun", "simctl", "runtime", "add", dmgFilePath)
		if err != nil {
			return errors.Annotate(err, "failed to add runtime dmg to Xcode").Err()
		}
		logging.Warningf(ctx, "Runtime %s added to Xcode", addOutput)

		// get the build version of the added runtime dmg
		listOutput, err := RunOutput(ctx, "xcrun", "simctl", "runtime", "list", "-j")
		if err != nil {
			return errors.Annotate(err, "failed when invoking `xcrun simctl runtime list -j`").Err()
		}
		var runtimes map[string]IOSRuntime
		err = json.Unmarshal([]byte(listOutput), &runtimes)
		if err != nil {
			return errors.Annotate(err, "failed when parsing `xcrun simctl runtime list -j` output").Err()
		}
		overridingBuild := ""
		iosVersion := ""
		for id, runtime := range runtimes {
			if strings.Contains(addOutput, id) {
				overridingBuild = runtime.Build
				iosVersion = runtime.Version
				break
			}
		}
		if overridingBuild == "" {
			return errors.Reason("Unable to find the runtime build id to override with...").Err()
		}

		// get the build version of the default runtime
		matchListOutput, err := RunOutput(ctx, "xcrun", "simctl", "runtime", "match", "list", "-j")
		if err != nil {
			return errors.Annotate(err, "failed when invoking `xcrun simctl runtime match list -j`").Err()
		}
		var sdkRuntimes map[string]SDKRuntime
		err = json.Unmarshal([]byte(matchListOutput), &sdkRuntimes)
		if err != nil {
			return errors.Annotate(err, "failed when parsing `xcrun simctl runtime match list -j` output").Err()
		}
		overriddenBuild := ""
		// the iphoneSdk key only has the version without patch number
		// e.g. if the iosVersion is 17.0.1, then the key is 17.0
		truncatedVersion := getIOSVersionWithoutPatch(iosVersion)
		iphoneSdk := "iphoneos" + truncatedVersion
		for id, sdkRuntime := range sdkRuntimes {
			if id == iphoneSdk {
				overriddenBuild = sdkRuntime.SdkBuild
				break
			}
		}
		if overriddenBuild == "" {
			return errors.Reason("Unable to find the runtime build id to be overridden...").Err()
		}

		// Override the default runtime build with the desired one
		logging.Warningf(ctx, "Overriding runtime %s with %s", overriddenBuild, overridingBuild)
		err = RunCommand(ctx, "xcrun", "simctl", "runtime", "match", "set", iphoneSdk, overridingBuild, "--sdkBuild", overriddenBuild)
		if err != nil {
			return errors.Annotate(err, "failed when trying to override runtime %s with %s", overridingBuild, overriddenBuild).Err()
		}
		return nil
	})
}
func installAndAddRuntimeDMG(ctx context.Context, runtimeDMGInstallArgs RuntimeDMGInstallArgs, xcodeAppPath string) error {
	// install runtime
	if err := installRuntimeDMG(ctx, runtimeDMGInstallArgs); err != nil {
		return errors.Annotate(err, "failed to install runtime dmg %s", runtimeDMGInstallArgs.runtimeVersion).Err()
	}

	files, err := os.ReadDir(runtimeDMGInstallArgs.installPath)
	if err != nil {
		return errors.Annotate(err, "Unable to read runtime dmg directory %s", runtimeDMGInstallArgs.installPath).Err()
	}
	dmgFilePath := ""
	// Iterate through the list of files and find the first file with the extension of `dmg`.
	for _, file := range files {
		if getFileExtension(file.Name()) == "dmg" {
			dmgFilePath = filepath.Join(runtimeDMGInstallArgs.installPath, file.Name())
			break
		}
	}

	if dmgFilePath == "" {
		return errors.Reason("Unable to locate dmg file in directory %s", runtimeDMGInstallArgs.installPath).Err()
	}

	if err = addRuntimeDMG(ctx, xcodeAppPath, dmgFilePath); err != nil {
		return errors.Annotate(err, "failed to add runtime dmg %s to Xcode", dmgFilePath).Err()
	}
	return nil
}

// InstallArgs are the parameters for installXcode() to keep them manageable.
type InstallArgs struct {
	xcodeVersion           string
	xcodeAppPath           string
	acceptedLicensesFile   string
	cipdPackagePrefix      string
	kind                   KindType
	serviceAccountJSON     string
	packageInstallerOnBots string
	withRuntime            bool
	corruptedXcodePath     string
}

func describeRef(ctx context.Context, packagePath, ref string) (string, error) {
	resolveArgs := []string{"describe", packagePath, "-version", ref}
	output, err := RunOutput(ctx, "cipd", resolveArgs...)
	if err != nil {
		err = errors.Annotate(err, "Error when describing package path %s with ref %s.", packagePath, ref).Err()
		return "", err
	}
	return output, nil
}

// get the CFBundleVersion from the latest Xcode on cipd
func getLatestCFBundleVersion(ctx context.Context, xcodePackagePath, xcodeVersion string) (string, error) {
	output, err := describeRef(ctx, xcodePackagePath, xcodeVersion)
	if err != nil {
		err = errors.Annotate(err, "Error when getting latest CFBundleVersion from cipd").Err()
		return "", err
	}
	var cfBundleVersionRegex = regexp.MustCompile(`cf_bundle_version:(\d+\.?\d*)`)
	result := cfBundleVersionRegex.FindStringSubmatch(output)
	if len(result) > 0 {
		return result[1], nil
	}
	return "", errors.Reason("Unable to parse CFBundleVersion from cipd describe output %s", output).Err()
}
func shouldReInstallXcode(ctx context.Context, xcodeAppPath, xcodeVersion string) (bool, error) {
	_, _, buildVersion, err := getXcodeVersion(filepath.Join(xcodeAppPath, "Contents", "version.plist"))
	if err != nil {
		logging.Warningf(ctx, "Xcode should be re-installed due to error %s", err.Error())
		return true, err
	}
	if !strings.EqualFold(buildVersion, xcodeVersion) {
		logging.Warningf(ctx, "Xcode version mismatched between local %s and expected %s, Xcode should be re-installed", buildVersion, xcodeVersion)
		return true, nil
	}
	logging.Warningf(ctx, "Xcode version %s matches between local and expected. So it should not be re-installed", buildVersion)
	return false, nil
}

type IOSRuntime struct {
	Build   string `json:"build"`
	Version string `json:"version"`
}
type SDKRuntime struct {
	SdkBuild   string `json:"sdkBuild"`
	SdkVersion string `json:"sdkVersion"`
}

// get the runtime build string from the latest iOS runtime given an iOS version
func getLatestRuntimeBuild(ctx context.Context, runtimeDMGPackagePath, iosVersion, xcodeVersion string) (string, error) {
	fullIOSVersion := "ios-" + strings.Replace(iosVersion, ".", "-", -1)
	resolveRuntimeDMGRefArgs := ResolveRuntimeDMGRefArgs{
		runtimeVersion:     fullIOSVersion,
		xcodeVersion:       xcodeVersion,
		packagePath:        runtimeDMGPackagePath,
		serviceAccountJSON: "",
	}
	ref, err := resolveRuntimeDMGRef(ctx, resolveRuntimeDMGRefArgs)
	if err != nil {
		return "", errors.Annotate(err, "failed to resolve runtime dmg ref").Err()
	}
	output, err := describeRef(ctx, runtimeDMGPackagePath, ref)
	if err != nil {
		err = errors.Annotate(err, "Error when getting latest ios_runtime_build from cipd").Err()
		return "", err
	}
	var runtimeBuildVersionRegex = regexp.MustCompile(`ios_runtime_build:(.*)`)
	result := runtimeBuildVersionRegex.FindStringSubmatch(output)
	if len(result) > 0 {
		return result[1], nil
	}
	return "", errors.Reason("Unable to parse ios_runtime_build from cipd describe output %s", output).Err()
}

// The function takes in an iosVersion, e.g. 17.0, and check whether it has already existed
// by running `xcrun simctl runtime list`
func shouldInstallRuntime(ctx context.Context, cipdPackagePrefix, iosVersion, xcodeVersion, xcodeAppPath string) (bool, error) {
	shouldInstallRuntime := true
	runtimeDMGPackagePath := cipdPackagePrefix + "/" + IosRuntimeDMGPackageName
	runtimeBuildOnCipd, err := getLatestRuntimeBuild(ctx, runtimeDMGPackagePath, iosVersion, xcodeVersion)
	if err != nil {
		return shouldInstallRuntime, err
	}
	err = RunWithXcodeSelect(ctx, xcodeAppPath, func() error {
		output, err := RunOutput(ctx, "xcrun", "simctl", "runtime", "list", "-j")
		if err != nil {
			return errors.Annotate(err, "failed when invoking `xcrun simctl runtime list -j`").Err()
		}

		var runtimes map[string]IOSRuntime
		err = json.Unmarshal([]byte(output), &runtimes)
		if err != nil {
			return errors.Annotate(err, "failed when parsing `xcrun simctl runtime list -j` output").Err()
		}
		for _, runtime := range runtimes {
			if strings.EqualFold(runtimeBuildOnCipd, runtime.Build) {
				logging.Warningf(ctx, "Runtime %s Build %s should not be installed because it already exists", iosVersion, runtimeBuildOnCipd)
				shouldInstallRuntime = false
				return nil
			}
		}
		return nil
	})
	return shouldInstallRuntime, err
}

// Installs Xcode. The default runtime of the Xcode version will be installed
// unless |args.withRuntime| is False.
func installXcode(ctx context.Context, args InstallArgs) error {
	if err := os.MkdirAll(args.xcodeAppPath, 0700); err != nil {
		return errors.Annotate(err, "failed to create a folder %s", args.xcodeAppPath).Err()
	}
	shouldInstallXcode := true

	// if on MacOS13 or later, then we should use cipd tag to check for re-intall first
	// see crbug/1420480
	logging.Warningf(ctx, "Checking if the host is on MacOS13 or later")
	onMacOS13OrLater, err := isMacOS13OrLater(ctx)
	if err == nil {
		if onMacOS13OrLater {
			logging.Warningf(ctx, "Checking if Xcode should be re-installed")
			shouldInstallXcode, _ = shouldReInstallXcode(ctx, args.xcodeAppPath, args.xcodeVersion)
		}
	} else {
		logging.Warningf(ctx, "Failed to check MacOS version with the error: %s", err)
	}
	if shouldInstallXcode {
		if args.kind == xcodeArchiveKind {
			xcodeArchivePath, tmpDirErr := os.MkdirTemp(filepath.Join(args.xcodeAppPath, ".."), "tmp")
			if tmpDirErr != nil {
				return tmpDirErr
			}
			defer os.RemoveAll(xcodeArchivePath)
			installPackagesArgs := InstallPackagesArgs{
				ref:                args.xcodeVersion,
				rootPath:           xcodeArchivePath,
				cipdPackagePrefix:  args.cipdPackagePrefix,
				kind:               args.kind,
				serviceAccountJSON: args.serviceAccountJSON,
			}
			if err := installPackages(ctx, installPackagesArgs); err != nil {
				return err
			}
			if err := unzipXcodeArchive(ctx, xcodeArchivePath, args.xcodeAppPath); err != nil {
				return err
			}
		} else {
			installPackagesArgs := InstallPackagesArgs{
				ref:                args.xcodeVersion,
				rootPath:           args.xcodeAppPath,
				cipdPackagePrefix:  args.cipdPackagePrefix,
				kind:               args.kind,
				serviceAccountJSON: args.serviceAccountJSON,
			}
			if err := installPackages(ctx, installPackagesArgs); err != nil {
				return err
			}
		}
	}

	// crbug/1420480: on MacOS13+, cipd files are no longer allowed to be part of Xcode.app.
	// If Xcode is installed on MacOS13+, we need to remove them before runFirstLaunch.
	if onMacOS13OrLater {
		logging.Warningf(ctx, "Removing the hidden cipd files if exists to be compliant with MacOS13+ codesign check...")
		if err := removeCipdFiles(args.xcodeAppPath); err != nil {
			return err
		}
	}

	// Accept license and launch Xcode.
	// The steps are done async because the Xcode app can potentially be corrupted,
	// and cause the main process to hang. If the async process hangs, the corrupted
	// Xcode will be removed, and the main process will fail and exit.
	ch := make(chan error, 1)
	go func() {
		if err := acceptLicense(ctx, args.xcodeAppPath); err != nil {
			ch <- err
			return
		}
		if err = finalizeInstall(ctx, args.xcodeAppPath, args.xcodeVersion, args.packageInstallerOnBots); err != nil {
			ch <- err
		}
		ch <- nil
	}()
	select {
	case err := <-ch:
		if err != nil {
			if args.corruptedXcodePath != "" {
				logging.Warningf(ctx, "Attempting to rename %s to %s", args.xcodeAppPath, args.corruptedXcodePath)
				if renameErr := renameDirectory(args.xcodeAppPath, args.corruptedXcodePath); renameErr != nil {
					logging.Warningf(ctx, "Error renaming corrupted Xcode directory: %s", renameErr)
				}
			}
			// if fail to accept license or launch, remove xcode
			os.RemoveAll(args.xcodeAppPath)
			return err
		} else {
			close(ch)
		}
	case <-time.After(MaxXcodeLaunchWaitTime):
		if args.corruptedXcodePath != "" {
			logging.Warningf(ctx, "Attempting to rename %s to %s", args.xcodeAppPath, args.corruptedXcodePath)
			if renameErr := renameDirectory(args.xcodeAppPath, args.corruptedXcodePath); renameErr != nil {
				logging.Warningf(ctx, "Error renaming corrupted Xcode directory: %s", renameErr)
			}
		}
		err := os.RemoveAll(args.xcodeAppPath)
		if err != nil {
			return errors.Annotate(err, "failed to remove corrupted Xcode %s", args.xcodeAppPath).Err()
		}
		return errors.Reason("The downloaded Xcode app is possibly corrupted. The app has been deleted. Please retry...").Err()
	}

	simulatorDirPath := filepath.Join(args.xcodeAppPath, XcodeIOSSimulatorRuntimeRelPath)
	simulatorFilePath := filepath.Join(simulatorDirPath, XcodeIOSSimulatorRuntimeFilename)
	_, statErr := os.Stat(simulatorFilePath)
	// Only install the default runtime when |withRuntime| arg is true and the
	// Xcode package installed doesn't have runtime file (backwards
	// compatibility for former Xcode packages).
	if args.withRuntime && os.IsNotExist(statErr) {
		if onMacOS13OrLater {
			logging.Warningf(ctx, "Deleting unused runtimes (if there are any) to free up disk spaces...")
			if err := deleteUnusedIOSRuntime(ctx, args.xcodeAppPath); err != nil {
				logging.Warningf(ctx, "error: %s. There are probably no runtimes to delete", err)
			}
			cfBundleVersion, err := getiOSRuntimeVersion(filepath.Join(args.xcodeAppPath, XcodeIOSSimulatorRuntimeVersionRelPath))
			if err != nil {
				return err
			}
			shouldInstallRuntime, err := shouldInstallRuntime(ctx, args.cipdPackagePrefix, cfBundleVersion, args.xcodeVersion, args.xcodeAppPath)
			if err != nil {
				return err
			}
			if shouldInstallRuntime {
				runtimeVersion := "ios-" + strings.Replace(cfBundleVersion, ".", "-", -1)
				// creating a temp dir to install ios runtime dmg. Will be removed later
				runtimeDMGPath, tmpDirErr := os.MkdirTemp(filepath.Join(args.xcodeAppPath, ".."), "tmp")
				if tmpDirErr != nil {
					return tmpDirErr
				}
				defer os.RemoveAll(runtimeDMGPath)
				runtimeDMGInstallArgs := RuntimeDMGInstallArgs{
					runtimeVersion:     runtimeVersion,
					xcodeVersion:       args.xcodeVersion,
					installPath:        runtimeDMGPath,
					cipdPackagePrefix:  args.cipdPackagePrefix,
					serviceAccountJSON: args.serviceAccountJSON,
				}
				logging.Warningf(ctx, "Installing and adding runtime %s dmg to Xcode...", runtimeVersion)
				if err := installAndAddRuntimeDMG(ctx, runtimeDMGInstallArgs, args.xcodeAppPath); err != nil {
					return err
				}
			}
		} else {
			runtimeInstallArgs := RuntimeInstallArgs{
				runtimeVersion:     "",
				xcodeVersion:       args.xcodeVersion,
				installPath:        simulatorDirPath,
				cipdPackagePrefix:  args.cipdPackagePrefix,
				serviceAccountJSON: args.serviceAccountJSON,
			}
			if err := installRuntime(ctx, runtimeInstallArgs); err != nil {
				return err
			}
		}
	}

	return checkDeveloperMode(ctx)
}

// Tests whether the input |ref| exists as a ref in CIPD |packagePath|.
func resolveRef(ctx context.Context, packagePath, ref, serviceAccountJSON string) error {
	resolveArgs := []string{"resolve", packagePath, "-version", ref}
	if serviceAccountJSON != "" {
		resolveArgs = append(resolveArgs, "-service-account-json", serviceAccountJSON)
	}
	err := RunCommand(ctx, "cipd", resolveArgs...)
	if err != nil {
		err = errors.Annotate(err, "Error when resolving package path %s with ref %s.", packagePath, ref).Err()
		return err
	}
	return nil
}

// ResolveRuntimeRefArgs are the parameters for resolveRuntimeRef() to keep them manageable.
type ResolveRuntimeRefArgs struct {
	runtimeVersion     string
	xcodeVersion       string
	packagePath        string
	serviceAccountJSON string
}

// Returns the best simulator runtime in CIPD with |runtimeVersion| and
// |xcodeVersion| as input. Args are passed in within |ResolveRuntimeRefArgs|:
//   - If only |xcodeVersion| is provided, only finds the default runtime coming
//     with the Xcode.
//   - If only |runtimeVersion| is provided, only finds the manually uploaded
//     runtime of the version.
//   - If both are provided, find a runtime using the following priority:
//     1. Satisfying both Xcode and runtime version,
//     2. A manually uploaded runtime of the version,
//     3. The latest uploaded runtime of the version, regardless of whether it's
//     from another Xcode or manually uploaded.
//
// Details: go/ios-runtime-cipd
func resolveRuntimeRef(ctx context.Context, args ResolveRuntimeRefArgs) (string, error) {
	if args.xcodeVersion == "" && args.runtimeVersion == "" {
		err := errors.Reason("Empty Xcode and runtime version to resolve runtime ref.").Err()
		return "", err
	}
	searchRefs := []string{}
	if args.xcodeVersion != "" && args.runtimeVersion == "" {
		searchRefs = append(searchRefs, args.xcodeVersion)
	}
	if args.xcodeVersion == "" && args.runtimeVersion != "" {
		searchRefs = append(searchRefs, args.runtimeVersion)
	}
	if args.xcodeVersion != "" && args.runtimeVersion != "" {
		searchRefs = append(searchRefs,
			args.runtimeVersion+"_"+args.xcodeVersion, // Xcode default runtime.
			args.runtimeVersion,                       // Uploaded runtime.
			args.runtimeVersion+"_latest")             // Latest uploaded runtime.
	}
	for _, searchRef := range searchRefs {
		if err := resolveRef(ctx, args.packagePath, searchRef, args.serviceAccountJSON); err == nil { // if NO error
			return searchRef, nil
		} else {
			logging.Warningf(ctx, "Failed to resolve ref: %s. Error: %s", searchRef, err.Error())
		}
	}
	err := errors.Reason("Failed to resolve runtime ref given runtime version: %s, xcode version: %s.", args.runtimeVersion, args.xcodeVersion).Err()
	return "", err
}

// RuntimeInstallArgs are the parameters for installRuntime() to keep them manageable.
type RuntimeInstallArgs struct {
	runtimeVersion     string
	xcodeVersion       string
	installPath        string
	cipdPackagePrefix  string
	serviceAccountJSON string
}

// Resolves and installs the suitable runtime.
func installRuntime(ctx context.Context, args RuntimeInstallArgs) error {
	if err := os.MkdirAll(args.installPath, 0700); err != nil {
		return errors.Annotate(err, "failed to create a folder %s", args.installPath).Err()
	}

	packagePath := args.cipdPackagePrefix + "/" + IosRuntimePackageName
	resolveRuntimeRefArgs := ResolveRuntimeRefArgs{
		runtimeVersion:     args.runtimeVersion,
		xcodeVersion:       args.xcodeVersion,
		packagePath:        packagePath,
		serviceAccountJSON: args.serviceAccountJSON,
	}
	ref, err := resolveRuntimeRef(ctx, resolveRuntimeRefArgs)
	if err != nil {
		return errors.Annotate(err, "failed to resolve runtime cipd ref. Xcode version: %s, runtime version: %s", args.xcodeVersion, args.runtimeVersion).Err()
	}
	installPackagesArgs := InstallPackagesArgs{
		ref:                ref,
		rootPath:           args.installPath,
		cipdPackagePrefix:  args.cipdPackagePrefix,
		kind:               iosRuntimeKind,
		serviceAccountJSON: args.serviceAccountJSON,
	}
	if err := installPackages(ctx, installPackagesArgs); err != nil {
		return err
	}
	return nil
}
