// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.chromium.org/chromiumos/infra/proto/go/lab_platform"
	"go.chromium.org/luci/gae/impl/memory"
)

func fakeVersion(i int, board, model, pool string) *lab_platform.StableVersion {
	return &lab_platform.StableVersion{
		Target: &lab_platform.StableVersionTarget{
			Board: board,
			Model: model,
			Pool:  pool,
		},
		OsVersion:           fmt.Sprintf("xxx-%d-cros-version", i),
		OsImagePath:         fmt.Sprintf("xxx-%d-cros-version-path", i),
		FirmwareRoVersion:   fmt.Sprintf("xxx-%d-fw-ro-version", i),
		FirmwareRoImagePath: fmt.Sprintf("xxx-%d-fw-ro-version-path", i),
	}
}

func TestVersions(t *testing.T) {
	ctx := context.Background()
	ctx = memory.Use(ctx)

	Convey("StableVersion datastore", t, func() {
		Convey("Good cases", func() {
			board := "my-board"
			model := "my-model"
			board2 := "yours-board"
			model2 := "yours-model"
			pools := []string{"pool1", "pool2"}
			version1 := fakeVersion(1, board, model, "")
			version2 := fakeVersion(2, board2, model2, "pool1")

			item, err := FindVersion(ctx, board, model, pools)
			So(err, ShouldNotBeNil)
			So(item, ShouldBeNil)

			err = WriteVersions(ctx, []*lab_platform.StableVersion{version1, version2})
			So(err, ShouldBeNil)

			// Find version1 without pools.
			item, err = FindVersion(ctx, board, model, nil)
			So(err, ShouldBeNil)
			So(item.OsVersion, ShouldEqual, version1.OsVersion)
			So(item.OsVersion, ShouldNotEqual, version2.OsVersion)
			So(item.OsImagePath, ShouldEqual, version1.OsImagePath)
			So(item.FirmwareRoVersion, ShouldEqual, version1.FirmwareRoVersion)
			So(item.FirmwareRoImagePath, ShouldEqual, version1.FirmwareRoImagePath)

			// Find version1 as default when request with pools.
			item, err = FindVersion(ctx, board, model, pools)
			So(err, ShouldBeNil)
			So(item.OsVersion, ShouldEqual, version1.OsVersion)
			So(item.OsVersion, ShouldNotEqual, version2.OsVersion)
			So(item.OsImagePath, ShouldEqual, version1.OsImagePath)
			So(item.FirmwareRoVersion, ShouldEqual, version1.FirmwareRoVersion)
			So(item.FirmwareRoImagePath, ShouldEqual, version1.FirmwareRoImagePath)

			// Find version2 with by a pool.
			item, err = FindVersion(ctx, board2, model2, pools)
			So(err, ShouldBeNil)
			So(item.OsVersion, ShouldNotEqual, version1.OsVersion)
			So(item.OsVersion, ShouldEqual, version2.OsVersion)
			So(item.OsImagePath, ShouldEqual, version2.OsImagePath)
			So(item.FirmwareRoVersion, ShouldEqual, version2.FirmwareRoVersion)
			So(item.FirmwareRoImagePath, ShouldEqual, version2.FirmwareRoImagePath)
		})
		Convey("key ignores cases", func() {
			board := "my1-Board"
			model := "my1-Model"
			version := fakeVersion(10, strings.ToLower(board), strings.ToLower(model), "")

			item, err := FindVersion(ctx, board, model, nil)
			So(err, ShouldNotBeNil)
			So(item, ShouldBeNil)

			err = WriteVersions(ctx, []*lab_platform.StableVersion{version})
			So(err, ShouldBeNil)

			// Find version1 without pools.
			item, err = FindVersion(ctx, board, model, nil)
			So(err, ShouldBeNil)
			So(item.OsVersion, ShouldEqual, version.OsVersion)
			So(item.OsImagePath, ShouldEqual, version.OsImagePath)
			So(item.FirmwareRoVersion, ShouldEqual, version.FirmwareRoVersion)
			So(item.FirmwareRoImagePath, ShouldEqual, version.FirmwareRoImagePath)
		})
	})
}

func TestRemoveBadVersions(t *testing.T) {
	versions := []*lab_platform.StableVersion{
		fakeVersion(10, "board-10", "model-10", "pool-10"),
		fakeVersion(11, "board-11", "model-11", "pool-11"),
		fakeVersion(12, "board-12", "model-12", "pool-12"),
		fakeVersion(13, "board-13", "model-13", "pool-13"),
		fakeVersion(14, "board-14", "model-14", "pool-14"),
		fakeVersion(15, "board-15", "model-15", "pool-15"),
		fakeVersion(16, "board-16", "model-16", "pool-16"),
		// Duplicate version
		fakeVersion(50, "board-50", "model-50", "pool-50"),
		fakeVersion(51, "board-50", "model-50", "pool-50"),
		// Bad versions.
		fakeVersion(100, "", "model-100", "pool-100"),
		fakeVersion(101, "board-101", "", "pool-101"),
		fakeVersion(102, "board-102", "model-102", ""),
	}
	keys := []string{
		"board=board-10;model=model-10;pool=pool-10",
		"board=board-11;model=model-11;pool=pool-11",
		"board=board-12;model=model-12;pool=pool-12",
		"board=board-13;model=model-13;pool=pool-13",
		"board=board-14;model=model-14;pool=pool-14",
		"board=board-15;model=model-15;pool=pool-15",
		"board=board-16;model=model-16;pool=pool-16",
		"board=board-50;model=model-50;pool=pool-50",
		"board=board-102;model=model-102;pool=",
	}
	ctx := context.Background()
	versionMap := removeBadVersions(ctx, versions)
	for _, k := range keys {
		if _, ok := versionMap[k]; !ok {
			t.Errorf("TestRemoveBadVersions the key %q is missed, but expecetd", k)
		}
	}
}
