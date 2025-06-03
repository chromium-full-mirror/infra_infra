// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/common_lib/interfaces"
	"go.chromium.org/infra/cros/cmd/ctpv2/data"
)

// PrepareFilterContainersInfoCmd represents prepare filter containers info cmd.
type PrepareFilterContainersInfoCmd struct {
	*interfaces.AbstractSingleCmdByNoExecutor

	// Deps
	CtpReq       *testapi.CTPRequest
	CredsFile    string
	CTPversion   string
	Environment  string
	Experiments  []string
	IsAlRun      bool
	IsPartnerRun bool
	HasAshChrome bool
	// Updates
	FiltersQueue *list.List
}

// ExtractDependencies extracts all the command dependencies from state keeper.
func (cmd *PrepareFilterContainersInfoCmd) ExtractDependencies(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.extractDepsFromFilterStateKeepr(ctx, sk)

	default:
		return fmt.Errorf("stateKeeper '%T' is not supported by cmd type %s", sk, cmd.GetCommandType())
	}

	if err != nil {
		return errors.Annotate(err, "error during extracting dependencies for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

// UpdateStateKeeper updates the state keeper with info from the cmd.
func (cmd *PrepareFilterContainersInfoCmd) UpdateStateKeeper(
	ctx context.Context,
	ski interfaces.StateKeeperInterface) error {

	var err error
	switch sk := ski.(type) {
	case *data.FilterStateKeeper:
		err = cmd.updateLocalTestStateKeeper(ctx, sk)
	}

	if err != nil {
		return errors.Annotate(err, "error during updating for command %s: ", cmd.GetCommandType()).Err()
	}

	return nil
}

func (cmd *PrepareFilterContainersInfoCmd) extractDepsFromFilterStateKeepr(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if sk.CtpReq == nil {
		return fmt.Errorf("cmd %q missing dependency: CtpV2Req", cmd.GetCommandType())
	}
	cmd.Experiments = sk.BuildState.Build().Input.Experiments
	cmd.CTPversion = sk.CTPversion
	cmd.CredsFile = sk.DockerKeyFile
	cmd.CtpReq = sk.CtpReq
	cmd.IsAlRun = sk.IsAlRun
	cmd.IsPartnerRun = sk.IsPartnerRun
	cmd.HasAshChrome = sk.HasAshChrome
	cmd.Environment = sk.Environment
	return nil
}

func (cmd *PrepareFilterContainersInfoCmd) updateLocalTestStateKeeper(
	ctx context.Context,
	sk *data.FilterStateKeeper) error {

	if cmd.FiltersQueue != nil {
		sk.FiltersQueue = cmd.FiltersQueue
	}

	return nil
}

// Execute executes the command.
func (cmd *PrepareFilterContainersInfoCmd) Execute(ctx context.Context) error {
	var err error
	step, ctx := build.StartStep(ctx, "Prepare containers for filters")
	defer func() { step.End(err) }()

	logging.Infof(ctx, "ctpreq:", cmd.CtpReq)

	defaultFilters := common.MakeDefaultFilters(ctx, cmd.CtpReq.GetSuiteRequest(), cmd.Experiments, cmd.IsPartnerRun, cmd.IsAlRun, cmd.HasAshChrome)

	ctpFilters := common.ConstructCtpFilters(ctx, defaultFilters, cmd.CtpReq.GetKarbonFilters())

	// -- Create container info queue --
	filtersQueue := list.New()

	firestoreDB := common.TestPlatformFireStore
	if cmd.IsAlRun && cmd.IsPartnerRun {
		firestoreDB = common.PartnerTestPlatformFireStore
	}
	for _, filter := range ctpFilters {
		if cmd.IsAlRun && cmd.IsPartnerRun {
			filter.GetContainerInfo().BinaryArgs = append(filter.GetContainerInfo().BinaryArgs, "-firestore", firestoreDB)
		}
		filter.GetContainerInfo().BinaryArgs = append(filter.GetContainerInfo().BinaryArgs, "-env", cmd.Environment)
		filtersQueue.PushBack(filter)
	}

	common.WriteStringToStepLog(ctx, step, string(common.ListToJSON(filtersQueue)), "Filters Queue")

	cmd.FiltersQueue = filtersQueue

	filterData, err := json.MarshalIndent(ctpFilters, "", "\t")
	if err != nil {
		logging.Infof(
			ctx,
			"error during writing ctp filters to log: %s",
			err.Error())
	}
	common.WriteStringToStepLog(ctx, step, string(filterData), "Final Ctp filters list")

	return nil
}

func NewPrepareFilterContainersInfoCmd() *PrepareFilterContainersInfoCmd {
	abstractCmd := interfaces.NewAbstractCmd(PrepareFilterContainersCmdType)
	abstractSingleCmdByNoExecutor := &interfaces.AbstractSingleCmdByNoExecutor{AbstractCmd: abstractCmd}
	return &PrepareFilterContainersInfoCmd{AbstractSingleCmdByNoExecutor: abstractSingleCmdByNoExecutor}
}
