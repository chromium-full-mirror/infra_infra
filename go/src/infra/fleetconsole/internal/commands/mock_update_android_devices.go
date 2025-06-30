// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"time"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/api/fleetconsolerpc"
	"go.chromium.org/infra/fleetconsole/internal/consoleserver/testdata"
	"go.chromium.org/infra/fleetconsole/internal/site"
)

// MockUpdateAndroidDevicesCommand mocks update android devices.
var MockUpdateAndroidDevicesCommand *subcommands.Command = &subcommands.Command{
	UsageLine: "mock-update-android-devices [options...]",
	ShortDesc: "Mock update android devices.",
	LongDesc:  "Mocks a pubsub message to update android devices.",
	CommandRun: func() subcommands.CommandRun {
		c := &mockUpdateAndroidDevicesRun{}
		c.Init()
		return c
	},
}

type mockUpdateAndroidDevicesRun struct {
	site.Subcommand
}

func (c *mockUpdateAndroidDevicesRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := cli.GetContext(a, c, env)
	err := c.innerRun(ctx, a, args, env)
	return c.Done(ctx, err)
}

func (c *mockUpdateAndroidDevicesRun) innerRun(ctx context.Context, a subcommands.Application, _ []string, _ subcommands.Env) error {
	host, err := c.CommonFlags.Host()
	if err != nil {
		return errors.Annotate(err, "mock update android devices").Err()
	}
	client, err := consoleClient(ctx, host, c.AuthFlags, c.CommonFlags.HTTP(), 30*time.Minute)
	if err != nil {
		return err
	}

	_, err = client.UpdateAndroidDevices(ctx, &fleetconsolerpc.UpdateAndroidDevicesRequest{
		Host: testdata.MockLabResource,
	})
	if err != nil {
		return err
	}
	return nil
}
