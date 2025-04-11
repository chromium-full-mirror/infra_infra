// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package stdenv

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"

	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/cipkg/base/workflow"
	"go.chromium.org/luci/common/exec"
)

func importDarwin(ctx context.Context, cfg *Config, bins ...string) (gs []generators.Generator, err error) {
	// Import posix utilities
	g, err := generators.FromPathBatch("posix_import", cfg.FindBinary, bins...)
	if err != nil {
		return nil, err
	}
	gs = append(gs, g)

	// Import xcode directory
	xcode := cfg.XcodeDeveloper
	if xcode == nil {
		cmd := exec.Command(ctx, "xcode-select", "--print-path")
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		path := string(bytes.TrimSpace(out))

		cmd = exec.Command(ctx, filepath.Join(path, "usr", "bin", "xcodebuild"), "-version")
		out, err = cmd.Output()
		if err != nil {
			return nil, err
		}
		ver := string(bytes.TrimSpace(out))

		xcode = &generators.ImportTargets{
			Name: "xcode_import",
			Targets: map[string]generators.ImportTarget{
				"Developer": {Source: path, Mode: fs.ModeSymlink, Version: ver},
			},
		}
	}
	gs = append(gs, xcode)

	// Import platform-specific tools
	gs = append(gs, &generators.ImportTargets{
		Name: "darwin_import",
		Targets: map[string]generators.ImportTarget{
			"bin/codesign":     {Source: "/usr/bin/codesign", Mode: fs.ModeSymlink},
			"bin/hdiutil":      {Source: "/usr/bin/hdiutil", Mode: fs.ModeSymlink},
			"bin/pkgbuild":     {Source: "/usr/bin/pkgbuild", Mode: fs.ModeSymlink},
			"bin/plutil":       {Source: "/usr/bin/plutil", Mode: fs.ModeSymlink},
			"bin/productbuild": {Source: "/usr/bin/productbuild", Mode: fs.ModeSymlink},
			"bin/sw_vers":      {Source: "/usr/bin/sw_vers", Mode: fs.ModeSymlink},
			"bin/vm_stat":      {Source: "/usr/bin/vm_stat", Mode: fs.ModeSymlink},
			"bin/xcode-select": {Source: "/usr/bin/xcode-select", Mode: fs.ModeSymlink},
			"bin/xcrun":        {Source: "/usr/bin/xcrun", Mode: fs.ModeSymlink},

			// Using compilers without wrappers require configuring Apple Framework properly, which isn't trivial.
			// See also: https://github.com/NixOS/nixpkgs/tree/master/pkgs/os-specific/darwin/apple-sdk
			"bin/cc":      {Source: "/usr/bin/cc", Mode: fs.ModeSymlink},
			"bin/c++":     {Source: "/usr/bin/c++", Mode: fs.ModeSymlink},
			"bin/clang":   {Source: "/usr/bin/clang", Mode: fs.ModeSymlink},
			"bin/clang++": {Source: "/usr/bin/clang++", Mode: fs.ModeSymlink},
			"bin/gcc":     {Source: "/usr/bin/gcc", Mode: fs.ModeSymlink},
			"bin/g++":     {Source: "/usr/bin/g++", Mode: fs.ModeSymlink},
		},
	})

	return
}

func (g *Generator) generateDarwin(plats generators.Platforms, tmpl *workflow.Generator) error {
	tmpl.Env.Set("DEVELOPER_DIR", "{{.xcode_import}}/Developer")

	// Env GREP added here to skip the configure testing them.
	// TODO(fancl): Update the specs to include gnu grep in the tools if
	// configure.ac expects gnu tools.
	tmpl.Env.Set("GREP", "{{.posix_import}}/bin/grep")
	return nil
}
