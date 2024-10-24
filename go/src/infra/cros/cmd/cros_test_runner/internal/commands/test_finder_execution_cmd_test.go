// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands_test

import (
	"context"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/infra/proto/go/test_platform/skylab_test_runner"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/cmd/common_lib/containers"
	"infra/cros/cmd/common_lib/tools/crostoolrunner"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
	"infra/cros/cmd/cros_test_runner/internal/executors"
)

func TestTestFinderExecutionCmd_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestFinderTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestFinderExecutor(cont)
		cmd := commands.NewTestFinderExecutionCmd(exec)
		sk := &UnsupportedStateKeeper{}
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestTestFinderExecutionCmd_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestFinderTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestFinderExecutor(cont)
		cmd := commands.NewTestFinderExecutionCmd(exec)
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestTestFinderExecutionCmd_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd with no updates", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestFinderTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestFinderExecutor(cont)
		cmd := commands.NewTestFinderExecutionCmd(exec)
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestTestFinderExecutionCmd_ExtractDepsSuccess(t *testing.T) {
	t.Parallel()

	ftt.Run("TestFinderExecutionCmd extract deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{
			Tests:       []string{"test1"},
			Tags:        []string{"group:test"},
			TagsExclude: []string{"group:nottest"},
		}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestFinderTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestFinderExecutor(cont)
		cmd := commands.NewTestFinderExecutionCmd(exec)

		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, cmd.Tests, should.Resemble(sk.Tests))
		assert.Loosely(t, cmd.Tags, should.Resemble(sk.Tags))
		assert.Loosely(t, cmd.TagsExclude, should.Resemble(sk.TagsExclude))
	})
}

func TestTestFinderExecutionCmd_UpdateSKSuccess(t *testing.T) {
	t.Parallel()
	ftt.Run("TestFinderExecutionCmd update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.LocalTestStateKeeper{HwTestStateKeeper: data.HwTestStateKeeper{CftTestRequest: &skylab_test_runner.CFTTestRequest{}}}
		ctrCipd := crostoolrunner.CtrCipdInfo{Version: "prod"}
		ctr := &crostoolrunner.CrosToolRunner{CtrCipdInfo: ctrCipd}
		cont := containers.NewCrosTestFinderTemplatedContainer("container/image/path", ctr)
		exec := executors.NewCrosTestFinderExecutor(cont)
		cmd := commands.NewTestFinderExecutionCmd(exec)

		cmd.TestSuites = []*api.TestSuite{
			{
				Spec: &api.TestSuite_TestCases{
					TestCases: &api.TestCaseList{
						TestCases: []*api.TestCase{
							{
								Id: &api.TestCase_Id{
									Value: "test1",
								},
							},
							{
								Id: &api.TestCase_Id{
									Value: "test2",
								},
							},
						},
					},
				},
			},
		}

		expectedResponse := []*api.TestSuite{
			{
				Spec: &api.TestSuite_TestCaseIds{
					TestCaseIds: &api.TestCaseIdList{
						TestCaseIds: []*api.TestCase_Id{
							{
								Value: "test1",
							},
							{
								Value: "test2",
							},
						},
					},
				},
			},
		}

		// Update SK
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.CftTestRequest.TestSuites, should.Resemble(expectedResponse))
	})
}
