// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"encoding/json"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"infra/cmdsupport/cmdlib"
	"infra/cros/cmd/paris/internal/site"
	"infra/cros/recovery"
	"infra/cros/recovery/config/tree"
	"infra/cros/recovery/tlw"
	"infra/libs/skylab/buildbucket"
)

// RecoveryConfig subcommand: For now, print the config file content to terminal/file.
var RecoveryConfig = &subcommands.Command{
	UsageLine: "config [-task-name TASK] [-device DEV] [-plan PLAN] [-tree]",
	ShortDesc: "print the JSON plan configuration file",
	LongDesc:  "print the JSON plan configuration file.",
	CommandRun: func() subcommands.CommandRun {
		c := &printConfigRun{}
		c.authFlags.Register(&c.Flags, site.DefaultAuthOptions)
		c.Flags.StringVar(&c.taskName, "task-name", "recovery", "Task name of the configuration we print.")
		c.Flags.StringVar(&c.deviceType, "device", "cros", "Device type supported 'cros', 'labstation'.")
		c.Flags.StringVar(&c.planName, "plan", "", "Print only plan instead of config.")
		c.Flags.BoolVar(&c.asTree, "tree", false, "Print data as tree.")
		c.Flags.BoolVar(&c.asShortTree, "short", false, "Print a short version of tree.")
		return c
	},
}

type printConfigRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags

	taskName    string
	deviceType  string
	planName    string
	asTree      bool
	asShortTree bool
}

// Run output the content of the recovery config file.
func (c *printConfigRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		cmdlib.PrintError(a, err)
		return 1
	}
	return 0
}

// innerRun executes internal logic of output file content.
func (c *printConfigRun) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	ctx := cli.GetContext(a, c, env)
	tn, err := buildbucket.NormalizeTaskName(c.taskName)
	if err != nil {
		return errors.Annotate(err, "local recovery").Err()
	}
	var ds tlw.DUTSetupType
	switch c.deviceType {
	case "labstation":
		ds = tlw.DUTSetupType_LABSTATION
	case "android":
		ds = tlw.DUTSetupType_ANDROID
	case "cros":
		ds = tlw.DUTSetupType_CROS
	case "browser":
		ds = tlw.DUTSetupType_CROS_BROWSER
	default:
		return errors.Reason("upsupported device type %s", c.deviceType).Err()
	}
	config, err := recovery.ParsedDefaultConfiguration(ctx, tn, ds)
	if err != nil {
		return errors.Annotate(err, "inner run").Err()
	}
	var obj interface{}
	if c.planName == "" {
		if c.asTree {
			obj = tree.ConvertConfiguration(config, c.asShortTree)
		} else {
			obj = config
		}
	} else {
		plan := config.GetPlans()[c.planName]
		if c.asTree {
			obj = tree.ConvertPlan(c.planName, plan, c.asShortTree)
		} else {
			obj = plan
		}
	}
	if s, err := json.MarshalIndent(obj, "", "\t"); err != nil {
		return errors.Annotate(err, "inner run").Err()
	} else {
		a.GetOut().Write(s)
	}
	return nil
}
