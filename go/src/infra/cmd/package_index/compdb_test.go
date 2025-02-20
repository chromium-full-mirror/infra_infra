// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	kpb "go.chromium.org/infra/cmd/package_index/kythe/proto"
)

func TestGetClangUtil(t *testing.T) {
	t.Parallel()
	cwd, _ := os.Getwd()
	clangInfo := &clangUnitInfo{
		unit: clangUnit{
			Command: "foo clang++ bar baz",
		},
		filepathsFn: filepath.Join(cwd, "package_index_testdata",
			"input", "src", "out", "Debug", "gen", "main.pb.h"),
	}
	ftt.Run("linux", t, func(t *ftt.Test) {
		cu, err := getClangUnit(
			context.Background(),
			clangInfo,
			"rootPath",
			"out/dir",
			"corpus",
			"linux",
			"",
			&FileHashMap{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cu, should.NotBeNil)

		details := &kpb.BuildDetails{
			BuildConfig: "linux",
		}
		detail, _ := anypb.New(details)
		detail.TypeUrl = "kythe.io/proto/kythe.proto.BuildDetails"
		assert.Loosely(t, cu.Argument, should.Resemble(
			[]string{"clang++", "bar", "baz",
				"-DKYTHE_IS_RUNNING=1", "-w"}))
	})
	ftt.Run("linux-arm64", t, func(t *ftt.Test) {
		cu, err := getClangUnit(
			context.Background(),
			clangInfo,
			"rootPath",
			"out/dir",
			"corpus",
			"linux",
			"arm64",
			&FileHashMap{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cu, should.NotBeNil)

		details := &kpb.BuildDetails{
			BuildConfig: "linux",
		}
		detail, _ := anypb.New(details)
		detail.TypeUrl = "kythe.io/proto/kythe.proto.BuildDetails"
		assert.Loosely(t, cu.Argument, should.Resemble(
			[]string{"clang++", "bar", "baz",
				"-target", "arm64",
				"-DKYTHE_IS_RUNNING=1", "-w"}))
	})
	ftt.Run("mac", t, func(t *ftt.Test) {
		cu, err := getClangUnit(
			context.Background(),
			clangInfo,
			"rootPath",
			"out/dir",
			"corpus",
			"mac",
			"",
			&FileHashMap{})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cu, should.NotBeNil)

		details := &kpb.BuildDetails{
			BuildConfig: "linux",
		}
		detail, _ := anypb.New(details)
		detail.TypeUrl = "kythe.io/proto/kythe.proto.BuildDetails"
		assert.Loosely(t, cu.Argument, should.Resemble(
			[]string{"clang++", "bar", "baz",
				"-DKYTHE_IS_RUNNING=1", "-w"}))
	})
}
