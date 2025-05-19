// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// TestFinderExecutionCmd represents test execution cmd.
type TestFinderExecutionCmd struct {
	*interfaces.SingleCmdByExecutor

	// Deps (optional)
	Tests       []string
	Tags        []string
	TagsExclude []string

	// Updates
	TestSuites []*testapi.TestSuite
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *TestFinderExecutionCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.LocalTestStateKeeper:
		err = cmd.extractDepsFromLocalTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *TestFinderExecutionCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.LocalTestStateKeeper:
		err = cmd.updateLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *TestFinderExecutionCmd) extractDepsFromLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if sk.Tests == nil {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: Tests", cmd.GetCommandType())
	}
	if sk.Tags == nil {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: Tags", cmd.GetCommandType())
	}
	if sk.TagsExclude == nil {
		logging.Infof(ctx, "Warning: cmd %q missing non-critical dependency: TagsExclude", cmd.GetCommandType())
	}

	cmd.Tests = sk.Tests
	cmd.Tags = sk.Tags
	cmd.TagsExclude = sk.TagsExclude

	return nil
}

func (cmd *TestFinderExecutionCmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.LocalTestStateKeeper) error {

	if cmd.TestSuites != nil {
		sk.CftTestRequest.TestSuites = []*testapi.TestSuite{}
		for _, testSuite := range cmd.TestSuites {
			testCaseIds := []*testapi.TestCase_Id{}
			for _, testCase := range testSuite.GetTestCases().TestCases {
				testCaseIds = append(testCaseIds, testCase.Id)
			}
			sk.CftTestRequest.TestSuites = append(sk.CftTestRequest.TestSuites, &testapi.TestSuite{
				Spec: &testapi.TestSuite_TestCaseIds{
					TestCaseIds: &testapi.TestCaseIdList{
						TestCaseIds: testCaseIds,
					},
				},
			})
		}
	}

	return nil
}

func NewTestFinderExecutionCmd(executor interfaces.ExecutorInterface) *TestFinderExecutionCmd {
	singleCmdByExec := interfaces.NewSingleCmdByExecutor(TestFinderExecutionCmdType, executor)
	cmd := &TestFinderExecutionCmd{SingleCmdByExecutor: singleCmdByExec}
	cmd.ConcreteCmd = cmd
	return cmd
}
