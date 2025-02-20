// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"path"

	"go.chromium.org/luci/common/data/text/sequence"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

type RustUpdate struct{}

func (c RustUpdate) HandleHook(oracle *Oracle, cwd string, hook *GclientHook) (handled bool, err error) {
	pat, err := sequence.NewPattern("src/tools/rust/update_rust.py", "$")
	if err != nil {
		panic(err)
	}
	if pat.In(hook.Action...) {
		handled = true
		LEAKY("src/tools/rust/update_rust.py")

		var rustVars map[string]string
		rustVars, err = badlyParsePythonGlobalVars(oracle, hook.Action[len(hook.Action)-1], []string{
			"RUST_REVISION",
			"RUST_SUB_REVISION",
		})
		if err != nil {
			return
		}

		LEAKY("src/tools/rust/update_rust.py reads CLANG_REVISION from src/tools/clang/scripts/update.py")
		var clangVar map[string]string
		clangVar, err = badlyParsePythonGlobalVars(oracle, "src/tools/clang/scripts/update.py", []string{
			"CLANG_REVISION",
		})
		if err != nil {
			return
		}

		rustVersion := fmt.Sprintf("%s-%s-%s", rustVars["RUST_REVISION"], rustVars["RUST_SUB_REVISION"], clangVar["CLANG_REVISION"])

		outdir := path.Join("src", "third_party", "rust-toolchain")

		LEAKY("The rust INSTALLED_VERSION is supposed to be a copy of VERSION from the archive, but we're cheating.")
		oracle.PinRawFile(path.Join(outdir, "INSTALLED_VERSION"), fmt.Sprintf("rustc 0 0 (%s chromium)", rustVersion), "RustUpdate hook")

		var bucket, object string
		bucket, object, err = ClangUpdate{}.GCSParams(oracle, fmt.Sprintf("rust-toolchain-%s.tar.xz", rustVersion))
		if err != nil {
			return
		}
		err = oracle.PinGCSArchive(outdir, bucket, object, nil, inputpb.GCSArchive_TAR_XZ, "")
	}
	return
}
