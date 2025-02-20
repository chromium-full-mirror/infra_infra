// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"fmt"
	"strings"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/cros_test_runner/data"
)

// ParseArgsCmd represents build input validation command.
type ParseArgsCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	Args *data.LocalArgs

	// Updates
	Tests                           []string
	Tags                            []string
	TagsExclude                     []string
	ContainerKeysRequestedForUpdate []string
	DutAddress                      *labapi.IpEndpoint
	DutCacheAddress                 *labapi.IpEndpoint
	CacheAddress                    *labapi.IpEndpoint
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *ParseArgsCmd) ExtractDependencies(ctx context.Context, ski interfaces.StateKeeperInterface) error {
	var err error
	switch sk := ski.(type) {
	case *data.PreLocalTestStateKeeper:
		err = cmd.extractDepsFromPreLocalTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *ParseArgsCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.PreLocalTestStateKeeper:
		err = cmd.updatePreLocalTestStateKeeper(ctx, sk)

	default:
		return fmt.Errorf("StateKeeper '%T' is not supported by cmd type %s.", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// Execute executes the command.
func (cmd *ParseArgsCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Parse Arguments")
	defer func() { step.End(err) }()

	if cmd.Args.Tests != "" {
		cmd.Tests = strings.Split(cmd.Args.Tests, ",")
	}
	if cmd.Args.Tags != "" {
		cmd.Tags = strings.Split(cmd.Args.Tags, ",")
	}
	if cmd.Args.TagsExclude != "" {
		cmd.TagsExclude = strings.Split(cmd.Args.TagsExclude, ",")
	}
	if cmd.Args.ContainerKeysRequestedForUpdate != "" {
		cmd.ContainerKeysRequestedForUpdate = strings.Split(cmd.Args.ContainerKeysRequestedForUpdate, ",")
	}
	if cmd.Args.DutAddress != "" {
		cmd.DutAddress, err = common.GetIPEndpoint(cmd.Args.DutAddress)
		if err != nil {
			return errors.Annotate(err, "error while parsing DutAddress: ").Err()
		}
	}
	if cmd.Args.DutCacheAddress != "" {
		cmd.DutCacheAddress, err = common.GetIPEndpoint(cmd.Args.DutCacheAddress)
		if err != nil {
			return errors.Annotate(err, "error while parsing DutCacheAddress: ").Err()
		}
	}
	if cmd.Args.CacheAddress != "" {
		cmd.CacheAddress, err = common.GetIPEndpoint(cmd.Args.CacheAddress)
		if err != nil {
			return errors.Annotate(err, "error while parsing CacheAddress: ").Err()
		}
	}

	return nil

}

func (cmd *ParseArgsCmd) extractDepsFromPreLocalTestStateKeeper(ctx context.Context, sk *data.PreLocalTestStateKeeper) error {
	if sk.Args == nil {
		return fmt.Errorf("Cmd %q missing dependency: Args", cmd.GetCommandType())
	}
	cmd.Args = sk.Args

	return nil
}

func (cmd *ParseArgsCmd) updatePreLocalTestStateKeeper(
	ctx context.Context,
	sk *data.PreLocalTestStateKeeper) error {

	sk.Tests = cmd.Tests
	sk.Tags = cmd.Tags
	sk.TagsExclude = cmd.TagsExclude
	sk.ContainerKeysRequestedForUpdate = cmd.ContainerKeysRequestedForUpdate
	sk.DutAddress = cmd.DutAddress
	sk.DutCacheAddress = cmd.DutCacheAddress
	sk.CacheAddress = cmd.CacheAddress

	return nil
}

func NewParseArgsCmd() *ParseArgsCmd {
	abstractCmd := interfaces.NewAbstractCmd(ParseArgsCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &ParseArgsCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
