// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/luciexe/build"

	"infra/experimental/golangbuild/golangbuildpb"
)

// CIPD dependencies for non-coordinator mode builds.
//
// N.B. We assume a few tools are already available on the machine we're
// running on. Namely:
// - For non-Windows, a C/C++ toolchain
const cipdBuildDeps = `
@Subdir
infra/3pp/tools/git/${platform} version:2@2.39.2.chromium.11
@Subdir cc/${os=windows}
golang/third_party/llvm-mingw-msvcrt/${platform} latest
`

// CIPD tool dependencies only. Used for coordinator builds.
const cipdToolDeps = `
@Subdir bin
infra/tools/bb/${platform} latest
infra/tools/rdb/${platform} latest
infra/tools/luci/cas/${platform} latest
infra/tools/result_adapter/${platform} latest
`

// CIPD dependency for Xcode.
const cipdXcodeDep = `
@Subdir
infra/tools/mac_toolchain/${platform} latest
`

func installTools(ctx context.Context, inputs *golangbuildpb.Inputs, experiments map[string]struct{}) (toolsRoot string, err error) {
	step, ctx := build.StartStep(ctx, "install tools")
	defer endInfraStep(step, &err) // Any failure in this function is an infrastructure failure.

	// Construct the CIPD ensure file.
	var cipdDeps string
	gotXcode := false

	switch inputs.GetMode() {
	case golangbuildpb.Mode_MODE_COORDINATOR:
		cipdDeps = cipdToolDeps
	case golangbuildpb.Mode_MODE_ALL, golangbuildpb.Mode_MODE_BUILD, golangbuildpb.Mode_MODE_TEST, golangbuildpb.Mode_MODE_PERF:
		// Don't install git from CIPD on less-common platforms. We'll get it from the external OS as needed.
		if _, bestEffortPlatform := experiments["luci.best_effort_platform"]; !bestEffortPlatform {
			cipdDeps = cipdBuildDeps
		}

		cipdDeps += cipdToolDeps + fmt.Sprintf(`
@Subdir go_bootstrap
golang/bootstrap-go/${platform} %v
`, inputs.BootstrapVersion)
		if inputs.XcodeVersion != "" {
			gotXcode = true
			cipdDeps += cipdXcodeDep
		}
		if inputs.ClangVersion != "" {
			cipdDeps += fmt.Sprintf(`
@Subdir clang
golang/third_party/clang/${platform} version:%s
`, inputs.ClangVersion)
		}
	}
	// Append build-only dependencies.
	switch inputs.GetMode() {
	case golangbuildpb.Mode_MODE_ALL, golangbuildpb.Mode_MODE_BUILD, golangbuildpb.Mode_MODE_PERF:
		if extraBuild := inputs.GetToolsExtraBuild(); extraBuild != "" {
			cipdDeps += "\n" + extraBuild
		}
	}
	// Append test-only dependencies.
	switch inputs.GetMode() {
	case golangbuildpb.Mode_MODE_ALL, golangbuildpb.Mode_MODE_TEST, golangbuildpb.Mode_MODE_PERF:
		const wasmRuntimeDep = `
@Subdir %[1]s
infra/3pp/tools/%[1]s/${platform} version:%[2]s
`
		if v := inputs.NodeVersion; v != "" {
			cipdDeps += fmt.Sprintf(wasmRuntimeDep, "nodejs", v)
		}
		if v := inputs.WasmtimeVersion; v != "" {
			cipdDeps += fmt.Sprintf(wasmRuntimeDep, "wasmtime", v)
		}
		if v := inputs.WazeroVersion; v != "" {
			cipdDeps += fmt.Sprintf(wasmRuntimeDep, "wazero", v)
		}

		if extraTest := inputs.GetToolsExtraTest(); extraTest != "" {
			cipdDeps += "\n" + extraTest
		}
	}
	// Append perf-only dependencies.
	if inputs.GetMode() == golangbuildpb.Mode_MODE_PERF {
		cipdDeps += `
@Subdir bin
golang/benchstat/${platform} latest
`
	}

	if _, err := io.WriteString(step.Log("ensure file"), cipdDeps); err != nil {
		return "", err
	}

	// Store in the named cache specified in Inputs. This is shared across
	// builder types, allowing reuse across builds if the dependencies
	// versions are the same.
	luciExe := lucictx.GetLUCIExe(ctx)
	if luciExe == nil {
		return "", fmt.Errorf("missing LUCI_CONTEXT")
	}

	cache := inputs.ToolsCache
	if cache == "" {
		return "", fmt.Errorf("inputs missing ToolsCache: %+v", inputs)
	}
	if !filepath.IsLocal(cache) {
		return "", fmt.Errorf("ToolsCache %q must be relative", cache)
	}

	toolsRoot = filepath.Join(luciExe.GetCacheDir(), cache)

	if _, err := io.WriteString(step.Log("tools root"), toolsRoot); err != nil {
		return "", err
	}

	// Install packages.
	cmd := exec.CommandContext(ctx, "cipd",
		"ensure", "-root", toolsRoot, "-ensure-file", "-",
		"-json-output", filepath.Join(os.TempDir(), "ensure_results.json"))
	cmd.Stdin = strings.NewReader(cipdDeps)
	if err := cmdStepRun(ctx, "cipd ensure", cmd, true); err != nil {
		return "", err
	}

	// Set up Xcode.
	// See https://source.corp.google.com/h/chromium/infra/infra/+/main:go/src/infra/cmd/mac_toolchain/README.md and
	// https://chromium.googlesource.com/chromium/tools/depot_tools/+/HEAD/recipes/recipe_modules/osx_sdk/api.py
	if gotXcode {
		xcodeInstall := exec.CommandContext(ctx, filepath.Join(toolsRoot, "mac_toolchain"), "install", "-xcode-version", inputs.XcodeVersion, "-output-dir", filepath.Join(toolsRoot, inputs.XcodeVersion, "Xcode.app"))
		if err := cmdStepRun(ctx, "install Xcode "+inputs.XcodeVersion, xcodeInstall, true); err != nil {
			return "", err
		}
		xcodeSelect := exec.CommandContext(ctx, "sudo", "xcode-select", "--switch", filepath.Join(toolsRoot, inputs.XcodeVersion, "Xcode.app"))
		if err := cmdStepRun(ctx, "select Xcode "+inputs.XcodeVersion, xcodeSelect, true); err != nil {
			return "", err
		}
	}
	return toolsRoot, nil
}

func withToolsRoot(ctx context.Context, toolsRoot string) context.Context {
	return context.WithValue(ctx, toolsRootKey{}, toolsRoot)
}

type toolsRootKey struct{}

func toolsRoot(ctx context.Context) string {
	return ctx.Value(toolsRootKey{}).(string)
}

func toolPath(ctx context.Context, tool string) string {
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	return filepath.Join(toolsRoot(ctx), "bin", tool)
}

func toolCmd(ctx context.Context, tool string, args ...string) *exec.Cmd {
	return command(ctx, toolPath(ctx, tool), args...)
}
