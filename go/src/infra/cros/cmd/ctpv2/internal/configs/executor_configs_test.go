// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package configs

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/commonconfigs"
	"go.chromium.org/infra/cros/cmd/common_lib/commonexecutors"
	"go.chromium.org/infra/cros/cmd/common_lib/tools/crostoolrunner"
)

func TestGetExecutor_UnsupportedExecutorType(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported executor type", t, func(t *ftt.Test) {
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		contConfig := commonconfigs.NewContainerConfig(ctr, nil, false)
		execConfig := NewExecutorConfig(ctr, contConfig)
		executor, err := execConfig.GetExecutor(commonexecutors.NoExecutorType)
		assert.Loosely(t, executor, should.BeNil)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

// TODO: Fix tests
// func TestGetExecutor_SupportedExecutorType(t *testing.T) {
// 	t.Skip()
// 	t.Parallel()
// 	ftt.Run("Supported executor type", t, func(t *ftt.Test) {
// 		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
// 		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
// 		contConfig := commonconfigs.NewContainerConfig(ctr, getMockContainerImagesInfo(), false)
// 		execConfig := NewExecutorConfig(ctr, contConfig)
//
// 		executor, err := execConfig.GetExecutor(executors.NoExecutorType)
// 		assert.Loosely(t, executor, should.BeNil)
// 		assert.Loosely(t, err, should.NotBeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.InvServiceExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.CtrExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.CrosDutExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.CrosDutVmExecutorType)
// 		assert.Loosely(t, executor.GetExecutorType(), should.Equal(executors.CrosDutVmExecutorType))
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.CrosProvisionExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.CrosTestFinderExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.CacheServerExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
//
// 		executor, err = execConfig.GetExecutor(executors.SshTunnelExecutorType)
// 		assert.Loosely(t, executor, should.NotBeNil)
// 		assert.Loosely(t, err, should.BeNil)
// 	})
// }
