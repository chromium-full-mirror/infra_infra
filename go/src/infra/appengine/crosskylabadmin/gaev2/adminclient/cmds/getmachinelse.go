// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmds

import (
	"context"
	"fmt"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/appengine/crosskylabadmin/internal/ufs"
	"go.chromium.org/infra/appengine/crosskylabadmin/site"
	shivasUtils "go.chromium.org/infra/cmd/shivas/utils"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

// GetMachineLSE calls the GetMachineLSE RPC of UFS the way that CrOSSkylabAdmin would.
var GetMachineLSE = &subcommands.Command{
	UsageLine: `get-machine-lse`,
	ShortDesc: `Get the stable version`,
	CommandRun: func() subcommands.CommandRun {
		r := &getMachineLSERun{}
		r.authFlags.Register(&r.Flags, site.DefaultAuthOptions)
		r.Flags.StringVar(&r.ufs, "ufs", site.ProdUFS, "the UFS server")
		r.Flags.StringVar(&r.name, "name", "", "the device name")
		return r
	},
}

// GetMachineLSERun runs the get-machine-lse command.
type getMachineLSERun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	name      string
	ufs       string
}

// Run runs the command and returns an exit status.
func (c *getMachineLSERun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	if err := c.innerRun(ctx, a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// InnerRun runs the command and returns an error.
func (c *getMachineLSERun) innerRun(ctx context.Context, a subcommands.Application, args []string, env subcommands.Env) error {
	ctx = shivasUtils.SetupContext(ctx, ufsUtil.OSNamespace)
	if len(args) != 0 {
		return errors.New("get machine lse: positional arguments are unacceptable")
	}
	authOptions, err := c.authFlags.Options()
	if err != nil {
		return errors.Fmt("get machine lse: authenticating: %w", err)
	}
	hc, err := auth.NewAuthenticator(ctx, auth.InteractiveLogin, authOptions).Client()
	if err != nil {
		return errors.Fmt("get machine lse: %w", err)
	}
	client, err := ufs.NewClient(ctx, hc, c.ufs)
	if err != nil {
		return errors.Fmt("get machine lse: creating client: %w", err)
	}
	if c.name == "" {
		return errors.New("name cannot be empty")
	}
	req := &ufsAPI.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, c.name),
	}
	jsonMarshaler.Marshal(a.GetErr(), req)
	res, err := client.GetMachineLSE(ctx, req)
	if err != nil {
		return errors.Fmt("get machine lse: inner run: %w", err)
	}
	jsonMarshaler.Marshal(a.GetOut(), res)
	return nil
}
