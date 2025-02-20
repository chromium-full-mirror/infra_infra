// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"go.chromium.org/luci/cipkg/base/actions"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/vpython/application"
	"go.chromium.org/luci/vpython/common"
	"go.chromium.org/luci/vpython/python"
	"go.chromium.org/luci/vpython/wheels"

	"go.chromium.org/infra/tools/vpython/vpythoncommon"
)

const DefaultPythonVersion = "3.8"

type PythonRuntime struct {
	// The version of the python interpreter bundle.
	//
	// Should be a value like "3.8" or "3.11" (i.e. MAJOR.MINOR, no patch
	// version).
	Version string

	// The version of the "go.chromium.org/infra/3pp/tools/virtualenv" CIPD package to use for
	// virtualenv support.
	//
	// Should be a CIPD tag for this package.
	Virtualenv string
}

func GetPythonRuntime(ver string) *PythonRuntime {
	switch ver {
	case "3.8":
		return &PythonRuntime{
			Version:    "3.8",
			Virtualenv: vpythoncommon.Virtualenv38Version,
		}
	case "3.11":
		return &PythonRuntime{
			Version:    ver,
			Virtualenv: vpythoncommon.Virtualenv311Version,
		}
	default:
		return &PythonRuntime{
			Version:    ver,
			Virtualenv: vpythoncommon.Virtualenv311Version,
		}
	}
}

func Main(ctx context.Context) error {
	rt := GetPythonRuntime(DefaultPythonVersion)
	app := &application.Application{
		PruneThreshold: 7 * 24 * time.Hour, // One week.

		// double the worst-case scenario (cpython, empty venv, pep425_tags, wheels, venv)
		MaxPrunesPerSweep: 10,

		DefaultSpecPattern: ".vpython3",

		Environments: os.Environ(),
		Arguments:    os.Args[1:],

		PythonExecutable: "python3",
	}

	// Intercept must be called after capturing Environments to avoid
	// NoDefaultCurrentDirectoryInExePath being inherited by python.
	reexecRegistry := actions.NewReexecRegistry()
	wheels.MustSetExecutor(reexecRegistry)
	reexecRegistry.Intercept(ctx)

	ctx = app.Initialize(ctx)
	if err := app.ParseEnvs(ctx); err != nil {
		return err
	}
	if err := app.ParseArgs(ctx); err != nil {
		return err
	}
	ctx = app.SetLogLevel(ctx)

	actionProcessor := actions.NewActionProcessor()
	wheels.MustSetTransformer(app.CIPDCacheDir, actionProcessor)

	if app.Bypass {
		// no-op for tool mode if we are bypassing vpython
		if app.ToolMode != "" {
			return nil
		}
		return app.ExecutePython(ctx)
	}

	if err := app.LoadSpec(ctx); err != nil {
		return err
	}

	// Update the Python Runtime based on vpython spec, if specified.
	if v := app.VpythonSpec.PythonVersion; v != "" {
		rt = GetPythonRuntime(v)
	}

	bundle := common.DefaultBundleDir(rt.Version)
	if app.InterpreterPath != "" {
		bundle = app.InterpreterPath
	}
	cpython, err := python.CPythonFromPath(bundle, "cpython3")
	if err != nil {
		return err
	}

	env := python.Environment{
		Executable: app.PythonExecutable,
		CPython:    cpython,
		Virtualenv: python.VirtualenvFromCIPD(rt.Virtualenv),
	}
	venv := env.WithWheels(wheels.FromSpec(app.VpythonSpec, env.Pep425Tags()))

	if !app.Help && app.ToolMode != "" {
		switch app.ToolMode {
		case "install":
			app.PruneThreshold = 0
			return app.BuildVENV(ctx, actionProcessor, venv)
		case "verify":
			return wheels.Verify(app.VpythonSpec)
		default:
			return errors.Reason("unknown -vpython-tool command: %s", app.ToolMode).Err()
		}
	}

	if app.Help {
		// Continue to execute python to print its help message after vpython's.
		fmt.Println(app.Usage)
	}
	if err := app.BuildVENV(ctx, actionProcessor, venv); err != nil {
		return err
	}
	if err := app.ExecutePython(ctx); err != nil {
		return err
	}
	return errors.New("unreachable")
}

func main() {
	ctx := context.Background()
	if err := Main(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
