// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package stdenv

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/base/workflow"
)

func importWindows(cfg *Config) (gs []generators.Generator, err error) {
	// Copy windows sdk
	winSDK := cfg.WinSDK
	if winSDK == nil {
		vsDir := os.Getenv("VSINSTALLDIR")
		if vsDir == "" {
			return nil, fmt.Errorf("failed to find visual studio: VSINSTALLDIR not set")
		}
		mf, err := os.Open(filepath.Join(vsDir, "win_sdk", "SDKManifest.xml"))
		if err != nil {
			return nil, fmt.Errorf("failed to open sdk manifest: %w", err)
		}
		defer mf.Close()
		ver, err := io.ReadAll(mf)
		if err != nil {
			return nil, fmt.Errorf("failed to read sdk manifest: %w", err)
		}

		winSDK = &generators.ImportTargets{
			Name: "winsdk_files",
			Targets: map[string]generators.ImportTarget{
				"/": {Source: vsDir, Version: string(ver), Mode: fs.ModeDir},
			},
		}
	}
	gs = append(gs, winSDK)

	// Import platform-specific tools
	g, err := generators.FromPathBatch("windows_import", cfg.FindBinary,
		"attrib",
		"cmd",
		"fc",
		"where",
	)
	// Bat shim is preferable for executables in most cases because mingw symlink
	// won't work with standard windows executables (e.g nmake, python...).
	// Cmd is handled differently in setup-hook by setting COMSPEC to its real
	// path.
	for k, v := range g.Targets {
		if k == "cmd.exe" {
			v.MinGWSymlink = true
		} else {
			v.GenerateBatShim = true
		}
		g.Targets[k] = v
	}
	gs = append(gs, g)

	return
}

func (g *Generator) generateWindows(plats generators.Platforms, tmpl *workflow.Generator) error {
	procArch := plats.Build.Arch()
	if procArch == "386" {
		procArch = "x86"
	}

	sdk_arch := map[string]string{
		"386":   "x86",
		"amd64": "x64",
		"arm64": "arm64",
	}[plats.Host.Arch()]
	if sdk_arch == "" {
		return fmt.Errorf("host architecture not supported yet: %s", plats.Host)
	}

	tmpl.Env.Set("PROCESSOR_ARCHITECTURE", procArch)
	tmpl.Env.Set("winsdk_root", "{{.winsdk_files}}")
	tmpl.Env.Set("sdk_arch", sdk_arch)
	return nil
}
