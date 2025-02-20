// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/experimental/golangbuild/golangbuildpb"
)

// buildRunner ensures a prebuilt toolchain exists for the current platform and the
// the sourceSpec this build was invoked with. It builds one if necessary and uploads
// it to CAS.
//
// This implements "build mode" for golangbuild.
type buildRunner struct {
	props *golangbuildpb.BuildMode
}

// newBuildRunner creates a new BuildMode runner.
func newBuildRunner(props *golangbuildpb.BuildMode) *buildRunner {
	return &buildRunner{props: props}
}

// Run implements the runner interface for buildRunner.
func (r *buildRunner) Run(ctx context.Context, spec *buildSpec, _ runOptions) error {
	// N.B. We ignore the fetchOnly run option becauase the only thing Run does is fetch.

	// Grab a prebuilt toolchain or build one and upload it.
	return getGo(ctx, spec, "", spec.goroot, spec.goSrc, false)
}

// getGo fetches and/or builds a Go toolchain from the provided sourceSpec and installs it at goroot.
// The fetched and/or built toolchain is constructed according to spec. goName is the display name for
// this toolchain, and is useful for disambiguating multiple toolchains if multiple are installed.
func getGo(ctx context.Context, spec *buildSpec, goName, goroot string, goSrc *sourceSpec, requirePrebuilt bool) (err error) {
	stepName := "get go"
	if goName != "" {
		stepName = fmt.Sprintf("get %s go", goName)
	}
	step, ctx := build.StartStep(ctx, stepName)
	defer endStep(step, &err)

	defer func() {
		if err != nil {
			return
		}

		// Run `go env` on the resulting toolchain for debugging purposes.
		_ = cmdStepRun(ctx, "go env", goCmd(ctx, goroot, goroot, "env"), true)

		// If requested, reinstall the compiler and linker in race mode.
		if spec.inputs.CompilerLinkerRaceMode {
			cmd := goCmd(ctx, goroot, goroot, "install", "-race", "cmd/compile", "cmd/link")
			if r := cmdStepRun(ctx, "go install -race cmd/compile cmd/link", cmd, false); r != nil {
				err = r
				return
			}
		}
	}()

	// Check to see if we might have a prebuilt Go in CAS.
	digest, err := checkForPrebuiltGo(ctx, goSrc, spec.inputs)
	if err != nil {
		return err
	}
	if digest != "" {
		// Try to fetch from CAS. Note this might fail if the digest is stale enough.
		ok, err := fetchGoFromCAS(ctx, digest, goroot)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	if requirePrebuilt {
		return infraErrorf("no prebuilt Go found, but this builder requires it")
	}

	// There was no prebuilt toolchain we could grab. Fetch Go and build it.
	//
	// If you make any changes here, consider if it's also necessary to bump
	// prebuiltGoVersion so that when golangbuild consumes the prebuilt toolchain,
	// it won't encounter any unexpected contents.

	// Fetch the main Go repository into goroot.
	if err := fetchRepo(ctx, goSrc, goroot, spec.inputs); err != nil {
		return err
	}

	// Possibly update the version file.
	if err := maybeUpdateVersionFile(ctx, goSrc, goroot, spec.inputs); err != nil {
		return err
	}

	// Build Go.
	ext := scriptExt(spec.inputs.Host)
	testID := "make" + ext
	if goName != "" {
		testID += " (" + goName + ")"
	}
	if err := cmdStepTest(ctx, spec, "make"+ext, testID, goScriptCmd(ctx, goroot, "make"+ext)); err != nil {
		return err
	}

	// Upload to CAS.
	return uploadGoToCAS(ctx, goSrc, spec.inputs, goroot)
}

// maybeUpdateVersionFile possibly updates the VERSION file in goroot. It ensures that after it runs
// that *some* valid VERSION file exists in goroot.
//
// The precise semantics of maybeUpdateVersionFile are:
//   - If the input property "version_file" is present, it always overwrites
//     the VERSION file with that value.
//   - If no VERSION file is present or the VERSION file is empty, then the
//     VERSION file is written with contents `devel <commit>` or
//     `devel <change>/<patchset>` (existing behavior).
//   - If a VERSION file is present AND the first line matches `go1.X.Y`,
//     then only the first line is kept, and we append `-devel_<commit>` or
//     `-devel_<change>_<patchset>` to the version.
//   - If a VERSION file is present otherwise, it is left alone.
//
// The purpose of retaining existing version files, and possibly appending
// a suffix to the version, is to retain invariants about toolchain versions
// for downstream tooling.
func maybeUpdateVersionFile(ctx context.Context, goSrc *sourceSpec, goroot string, inputs *golangbuildpb.Inputs) error {
	versionPath := filepath.Join(goroot, "VERSION")
	if inputs.VersionFile != "" {
		return writeFile(ctx, versionPath, inputs.VersionFile)
	}

	// Load VERSION file.
	version, _, err := readFile(ctx, versionPath)
	if err != nil {
		return err
	}
	// Strip metadata from the version.
	version = versionWithoutMetadata(version)

	// Check the version and update it if necessary.
	var newVersion string
	if versionRegexp.MatchString(version) {
		// On release branches, there may already be a version file of
		// the form "go1.X.Y". Preserve this version so that tests can
		// rely on the version comparing correctly with other Go versions.
		// Add a suffix, however, just to delineate that this is likely a
		// released version with a few extra commits patched on top.
		switch {
		case goSrc.change != nil:
			newVersion = fmt.Sprintf("%s-devel_%d_%d", version, goSrc.change.Change, goSrc.change.Patchset)
		case goSrc.commit != nil:
			newVersion = fmt.Sprintf("%s-devel_%s", version, goSrc.commit.Id)
		}
	} else if version == "" {
		switch {
		case goSrc.change != nil:
			newVersion = fmt.Sprintf("devel %d/%d", goSrc.change.Change, goSrc.change.Patchset)
		case goSrc.commit != nil:
			newVersion = fmt.Sprintf("devel %s", goSrc.commit.Id)
		}
	}

	// Write out the VERSION file if necessary.
	if newVersion != "" && newVersion != version {
		if err := writeFile(ctx, versionPath, newVersion); err != nil {
			return err
		}
	}
	return nil
}

var versionRegexp = regexp.MustCompile(`^go1([.]\d+){2}$`)

func versionWithoutMetadata(v string) string {
	s, _, _ := strings.Cut(v, "\n")
	return s
}

// scriptExt returns the extension to use for
// GOROOT/src/{make,all} scripts on this GOOS.
func scriptExt(host *golangbuildpb.Port) string {
	switch host.Goos {
	case "windows":
		return ".bat"
	case "plan9":
		return ".rc"
	default:
		return ".bash"
	}
}
