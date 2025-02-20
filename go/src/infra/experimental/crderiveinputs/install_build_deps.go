// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/exec"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

func (e EmbedTools) ExtractInstallBuildDeps(ctx context.Context, oracle *Oracle, root string) (*inputpb.LinuxSystemDeps, error) {
	IMPROVE("EmbedTools.ExtractInstallBuildDeps should directly update manifest.")

	LEAKY("linux-install-build-deps-extractor.py assumes an EXCESSIVE amount of stuff re: install-build-deps.py contents.")
	LEAKY("Assuming --arm and --nacl for install-build-deps.py")

	// pull install-build-deps.py locally, pass it to
	ibd, err := oracle.ReadFullString(path.Join(root, "build", "install-build-deps.py"))
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "python3", filepath.Join(
		string(e), "scripts", "linux-install-build-deps-extractor.py",
	))
	cmd.Stdin = strings.NewReader(ibd)
	extracted, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	ret := &inputpb.LinuxSystemDeps{}
	return ret, protojson.Unmarshal(extracted, ret)
}
