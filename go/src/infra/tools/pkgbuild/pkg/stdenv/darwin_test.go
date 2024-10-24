// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stdenv

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/cipkg/base/generators"
	"go.chromium.org/luci/common/exec/execmock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestImportDarwin(t *testing.T) {
	ftt.Run("import darwin", t, func(t *ftt.Test) {
		ctx := execmock.Init(context.Background())
		xcodeSelectUses := execmock.Simple.WithArgs("xcode-select", "--print-path").Mock(ctx, execmock.SimpleInput{
			Stdout: "/path/to/xcode.app",
		})
		xcodeBuildUses := execmock.Simple.WithArgs(filepath.FromSlash("/path/to/xcode.app/usr/bin/xcodebuild"), "-version").Mock(ctx, execmock.SimpleInput{
			Stdout: "xcodeversion",
		})

		gs, err := importDarwin(ctx, &Config{
			FindBinary: func(bin string) (string, error) {
				return filepath.FromSlash(fmt.Sprintf("/bin/%s", bin)), nil
			},
			BuildPlatform: generators.NewPlatform("darwin", "arm64"),
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, xcodeSelectUses.Snapshot(), should.HaveLength(1))
		assert.Loosely(t, xcodeBuildUses.Snapshot(), should.HaveLength(1))
		assert.Loosely(t, gs, should.ContainMatch[generators.Generator](&generators.ImportTargets{
			Name: "xcode_import",
			Targets: map[string]generators.ImportTarget{
				"Developer": {Source: "/path/to/xcode.app", Mode: fs.ModeSymlink, Version: "xcodeversion"},
			},
		}))

		// All imports on Mac should be symlink.
		for _, g := range gs {
			if targets, ok := g.(*generators.ImportTargets); ok {
				for _, targ := range targets.Targets {
					t.Log("checking", t)
					assert.Loosely(t, targ.Mode&fs.ModeSymlink, should.NotEqual(0))
				}
			}
		}
	})
}
