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

	"infra/cros/cmd/common_lib/common"
	"infra/cros/cmd/cros_test_runner/data"
	"infra/cros/cmd/cros_test_runner/internal/commands"
)

func TestProcessResultsCmdDeps_UnsupportedSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Unsupported state keeper", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &UnsupportedStateKeeper{}
		cmd := commands.NewProcessResultsCmd()
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestProcessResultsCmdDeps_MissingDeps(t *testing.T) {
	t.Parallel()
	ftt.Run("Cmd missing deps", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		cmd := commands.NewProcessResultsCmd()
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestProcessResultsCmdDeps_UpdateSK(t *testing.T) {
	t.Parallel()
	ftt.Run("Update SK", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{CftTestRequest: nil}
		cmd := commands.NewProcessResultsCmd()
		err := cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
	})
}

func TestProcessResultsCmdDeps_Execute(t *testing.T) {
	t.Parallel()
	ftt.Run("BuildInputValidationCmd execute with passing values", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			CftTestRequest: &skylab_test_runner.CFTTestRequest{ParentBuildId: 12345678},
			GcsURL:         "some/url",
			TesthausURL:    "some/url",
			ProvisionResponses: map[string][]*api.InstallResponse{
				common.NewPrimaryDeviceIdentifier().Id: {
					{Status: api.InstallResponse_STATUS_SUCCESS},
				},
			},
			TestResponses: &api.CrosTestResponse{
				TestCaseResults: []*api.TestCaseResult{
					{
						TestCaseId: &api.TestCase_Id{Value: "testId"},
						Verdict:    &api.TestCaseResult_Pass_{},
					},
				},
			},
		}
		cmd := commands.NewProcessResultsCmd()

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Execute cmd
		err = cmd.Execute(ctx)
		assert.Loosely(t, err, should.BeNil)

		// Update SK
		err = cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.SkylabResult, should.NotBeNil)
	})

	ftt.Run("BuildInputValidationCmd execute with missing provision resp", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			CftTestRequest: &skylab_test_runner.CFTTestRequest{ParentBuildId: 12345678},
			GcsURL:         "some/url",
			TesthausURL:    "some/url",
			TestResponses: &api.CrosTestResponse{
				TestCaseResults: []*api.TestCaseResult{
					{
						TestCaseId: &api.TestCase_Id{Value: "testId"},
						Verdict:    &api.TestCaseResult_Pass_{},
					},
				},
			},
		}
		cmd := commands.NewProcessResultsCmd()

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Execute cmd
		err = cmd.Execute(ctx)
		assert.Loosely(t, err, should.BeNil)

		// Update SK
		err = cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.SkylabResult, should.NotBeNil)
	})

	ftt.Run("BuildInputValidationCmd execute with missing test results", t, func(t *ftt.Test) {
		ctx := context.Background()
		sk := &data.HwTestStateKeeper{
			CftTestRequest: &skylab_test_runner.CFTTestRequest{
				ParentBuildId: 12345678,
				TestSuites: []*api.TestSuite{
					{
						Name: "suite",
						Spec: &api.TestSuite_TestCaseIds{
							TestCaseIds: &api.TestCaseIdList{
								TestCaseIds: []*api.TestCase_Id{
									{
										Value: "test1",
									},
								},
							},
						},
					},
				},
			},
			GcsURL:      "some/url",
			TesthausURL: "some/url",
			ProvisionResponses: map[string][]*api.InstallResponse{
				common.NewPrimaryDeviceIdentifier().Id: {
					{Status: api.InstallResponse_STATUS_SUCCESS},
				},
			},
		}
		cmd := commands.NewProcessResultsCmd()

		// Extract deps first
		err := cmd.ExtractDependencies(ctx, sk)
		assert.Loosely(t, err, should.BeNil)

		// Execute cmd
		err = cmd.Execute(ctx)
		assert.Loosely(t, err, should.BeNil)

		// Update SK
		err = cmd.UpdateStateKeeper(ctx, sk)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, sk.SkylabResult, should.NotBeNil)
	})
}
