// Copyright (c) 2023 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"path"
	"strings"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/data/text/sequence"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/experimental/crderiveinputs/inputpb"
)

type ClangUpdate struct{}

func badlyParsePythonGlobalVars(oracle *Oracle, pyFile string, wantedVars []string) (map[string]string, error) {
	updatePyContent, err := oracle.ReadFullString(pyFile)
	if err != nil {
		return nil, err
	}
	// NOTE: there are several better choices here for parsing, but recall that
	// the purpose of this tool is to inform where we should just be pinning
	// things in a better way anyway (e.g. just put the pertinent pin
	// information in a JSON pin file, rather than as global variables in
	// a python script). Accordingly, we parse the file in a very dumb way here:
	wantedVarSet := stringset.NewFromSlice(wantedVars...)
	scn := bufio.NewScanner(strings.NewReader(updatePyContent))
	gotVars := map[string]string{}
	for scn.Scan() {
		line := scn.Text()
		toks := strings.Split(line, "=")
		if len(toks) == 2 {
			lhs, rhs := toks[0], toks[1]
			lhs = strings.TrimSpace(lhs)
			if wantedVarSet.Has(lhs) {
				gotVars[lhs] = strings.Trim(strings.TrimSpace(rhs), `'"`)
			}
			if len(gotVars) == len(wantedVarSet) {
				break
			}
		}
	}
	if len(gotVars) != len(wantedVarSet) {
		return nil, errors.Reason("Unable to find all wanted vars %q, got %q", wantedVarSet, gotVars).Err()
	}

	return gotVars, nil
}

func (c ClangUpdate) GCSParams(oracle *Oracle, file string) (bucket, object string, err error) {
	hostOS, ok := map[string]string{
		"linux":     "Linux_x64",
		"mac":       "Mac",
		"mac-arm64": "Mac_arm64",
		"win":       "Win",
	}[oracle.HostOS]
	if !ok {
		err = errors.Reason("unknown HostOS for clang update script: %s", oracle.HostOS).Err()
		return
	}
	bucket = "chromium-browser-clang"
	object = fmt.Sprintf("%s/%s", hostOS, file)
	return
}

func (c ClangUpdate) HandleHook(oracle *Oracle, cwd string, hook *GclientHook) (handled bool, err error) {
	pat, err := sequence.NewPattern("src/tools/clang/scripts/update.py", "$")
	if err != nil {
		panic(err)
	}
	if pat.In(hook.Action...) {
		handled = true
		LEAKY("clang/scripts/update.py")

		var clangVars map[string]string
		clangVars, err = badlyParsePythonGlobalVars(oracle, hook.Action[len(hook.Action)-1], []string{
			"CLANG_REVISION",     // a python string
			"CLANG_SUB_REVISION", // a number
			"RELEASE_VERSION",    // a string
		})
		if err != nil {
			return
		}

		outdir := path.Join("src", "third_party", "llvm-build", "Release+Asserts")
		stampFile := path.Join(outdir, "cr_build_revision")
		LEAKY("The real clang/scripts/update.py relies on finding and exec'ing the .gclient file at the root to find `target_os`.")
		packageVersion := fmt.Sprintf("%s-%s", clangVars["CLANG_REVISION"], clangVars["CLANG_SUB_REVISION"])
		expectedStamp := fmt.Sprintf("%s,%s", packageVersion, oracle.TargetOS)
		oracle.PinRawFile(stampFile, expectedStamp, "ClangUpdate hook")

		if oracle.HostOS != "linux" {
			TODO("clang update.py has extra runtimes to fetch on non-linux")
			return
		}

		var bucket, object string
		if bucket, object, err = c.GCSParams(oracle, fmt.Sprintf("clang-%s.tar.xz", packageVersion)); err != nil {
			return
		}
		err = oracle.PinGCSArchive(outdir, bucket, object, nil, inputpb.GCSArchive_TAR_XZ, "")
	}
	return
}
