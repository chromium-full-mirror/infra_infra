// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/client"
	"go.chromium.org/infra/cros/karte/internal/site"
)

// CreateAction is a CLI command that creates an action on the Karte server.
var CreateAction = &subcommands.Command{
	UsageLine: `create-action`,
	ShortDesc: "create action",
	LongDesc:  "Create an action on the karte server.",
	CommandRun: func() subcommands.CommandRun {
		r := &createActionRun{}
		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		// TODO(gregorynisbet): add envFlags
		r.Flags.StringVar(&r.kind, "kind", "", "The action kind")
		r.Flags.StringVar(&r.swarmingTaskID, "task-id", "", "The ID of the swarming task")
		r.Flags.StringVar(&r.assetTag, "asset-tag", "", "The asset tag")
		r.Flags.StringVar(&r.failReason, "fail-reason", "", "The fail reason")
		r.Flags.StringVar(&r.model, "model", "", "The model of the DUT")
		r.Flags.StringVar(&r.board, "board", "", "The board of the DUT")
		r.Flags.StringVar(&r.recoveredBy, "recovered-by", "", "The action that recovered this action")
		r.Flags.IntVar(&r.restarts, "restarts", 0, "The number of times we restarted the plan")
		r.Flags.StringVar(&r.planName, "plan-name", "", "The name of the current plan")
		r.Flags.StringVar(&r.allowFail, "allow-fail", "", `whether the current action is allow to fail or not {"", "f", "u", "t"}`)
		r.Flags.StringVar(&r.actionType, "action-type", "", `what the action type is {"", "u", "v", "c", "r"}`)
		return r
	},
}

// createActionRun runs create-action.
type createActionRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	// Action fields
	kind           string
	swarmingTaskID string
	assetTag       string
	// TODO(gregorynisbet): Support times.
	// startTime      string
	// stopTime       string
	// TODO(gregorynisbet): Support status.
	// status     string
	failReason string
	// TODO(gregorynisbet): Support times.
	// sealTime string
	model       string
	board       string
	recoveredBy string
	restarts    int
	planName    string
	allowFail   string
	actionType  string
}

// nontrivialActionFields counts the number of fields in the action to be created with a non-default value.
func (c *createActionRun) nontrivialActionFields() int {
	tally := 0
	if c.kind != "" {
		tally++
	}
	if c.swarmingTaskID != "" {
		tally++
	}
	if c.assetTag != "" {
		tally++
	}
	if c.failReason != "" {
		tally++
	}
	if c.model != "" {
		tally++
	}
	if c.board != "" {
		tally++
	}
	if c.recoveredBy != "" {
		tally++
	}
	if c.restarts != 0 {
		tally++
	}
	if c.planName != "" {
		tally++
	}
	if c.allowFail != "" {
		tally++
	}
	if c.actionType != "" {
		tally++
	}
	return tally
}

// Run creates an action and returns an exit status.
func (c *createActionRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun creates an action and returns an error.
func (c *createActionRun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 0 {
		return errors.Reason("positional arguments are not accepted").Err()
	}
	tally := c.nontrivialActionFields()
	if tally == 0 {
		return errors.Reason("refusing to create empty action").Err()
	}
	authOptions, err := c.authFlags.Options()
	if err != nil {
		return errors.Annotate(err, "create action").Err()
	}
	kClient, err := client.NewClient(ctx, client.DevConfig(authOptions))
	if err != nil {
		return errors.Annotate(err, "create action").Err()
	}
	// TODO(gregorynisbet): Factor this into a separate function.
	action := &kartepb.Action{}
	action.Kind = c.kind
	action.SwarmingTaskId = c.swarmingTaskID
	action.AssetTag = c.assetTag
	action.FailReason = c.failReason
	action.Model = c.model
	action.Board = c.board
	action.RecoveredBy = c.recoveredBy
	action.Restarts = int32(c.restarts)
	action.PlanName = c.planName
	switch c.allowFail {
	case "f":
		action.AllowFail = kartepb.Action_NO_ALLOW_FAIL
	case "", "u":
		action.AllowFail = kartepb.Action_ALLOW_FAIL_UNSPECIFIED
	case "t":
		action.AllowFail = kartepb.Action_ALLOW_FAIL
	default:
		return errors.Reason("invalid allowFail value %q", c.allowFail).Err()
	}
	switch c.actionType {
	case "", "u":
		action.ActionType = kartepb.Action_ACTION_TYPE_UNSPECIFIED
	case "v":
		action.ActionType = kartepb.Action_ACTION_TYPE_VERIFIER
	case "c":
		action.ActionType = kartepb.Action_ACTION_TYPE_CONDITION
	case "r":
		action.ActionType = kartepb.Action_ACTION_TYPE_RECOVERY
	default:
		return errors.Reason("invalid allowFail value %q", c.actionType).Err()
	}
	out, err := kClient.CreateAction(ctx, &kartepb.CreateActionRequest{Action: action})
	if err != nil {
		return errors.Annotate(err, "create action").Err()
	}
	fmt.Fprintf(a.GetOut(), "%s\n", out.String())
	return nil
}
