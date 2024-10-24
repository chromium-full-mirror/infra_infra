// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostoolrunner

import (
	"context"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

const (
	ctrCipdPackagePackageForTest = "chromiumos/infra/cros-tool-runner/linux-amd64"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	ftt.Run("CTR cipd validate without version", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{CtrCipdPackage: ctrCipdPackagePackageForTest}
		err := ctrCipd.Validate(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("CTR cipd validate with version", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{Version: "Version1234"}
		err := ctrCipd.Validate(ctx)
		assert.Loosely(t, err, should.BeNil)
	})
}

// TODO (azrahman): fix these tests for other platform (mac)

// func TestEnsure(t *testing.T) {
// 	t.Parallel()

// 	Convey("CTR cipd ensure error without version", t, func() {
// 		ctx := context.Background()
// 		ctrCipd := CtrCipdInfo{CtrCipdPackage: ctrCipdPackagePackageForTest}
// 		err := ctrCipd.ensure(ctx)
// 		So(err, ShouldNotBeNil)
// 	})

// 	Convey("CTR cipd ensure success", t, func() {
// 		ctx := context.Background()
// 		ctrCipd := CtrCipdInfo{Version: "prod", CtrCipdPackage: ctrCipdPackagePackageForTest}
// 		err := ctrCipd.ensure(ctx)
// 		So(err, ShouldBeNil)
// 	})
// }

func TestInitialize(t *testing.T) {
	t.Parallel()

	ftt.Run("CTR cipd initialize with already initialized package", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{CtrCipdPackage: ctrCipdPackagePackageForTest}
		ctrCipd.IsInitialized = true
		err := ctrCipd.Initialize(ctx)
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("CTR cipd initialize with validation error", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := CtrCipdInfo{CtrCipdPackage: ctrCipdPackagePackageForTest}
		err := ctrCipd.Initialize(ctx)
		assert.Loosely(t, err, should.NotBeNil)
	})

	// Convey("CTR cipd initialize with ensure error", t, func() {
	// 	ctx := context.Background()
	// 	ctrCipd := CtrCipdInfo{Version: "invalidversion", CtrCipdPackage: ctrCipdPackagePackageForTest}
	// 	err := ctrCipd.Initialize(ctx)
	// 	So(err, ShouldNotBeNil)
	// })

	// Convey("CTR cipd initialize success", t, func() {
	// 	ctx := context.Background()
	// 	ctrCipd := CtrCipdInfo{Version: "prod", CtrCipdPackage: ctrCipdPackagePackageForTest}
	// 	err := ctrCipd.Initialize(ctx)
	// 	So(err, ShouldBeNil)
	// })
}
