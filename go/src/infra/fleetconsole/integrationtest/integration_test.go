// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package integrationtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/lucictx"
	"go.chromium.org/luci/server/servertest"

	"infra/fleetconsole/cmd/consoleadmin/clilib"
	"infra/fleetconsole/cmd/fleetconsoleserver/serverlib"
)

// TestPing tests the ping RPC.
func TestPing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testServer, err := servertest.RunServer(ctx, &servertest.Settings{
		Options: serverlib.Options(),
		Modules: serverlib.Modules(),
		Init:    serverlib.ServerMain,
	})
	assert.That(t, err, should.ErrLike(nil))
	assert.Loosely(t, testServer, should.NotBeNil)

	clientCtx := lucictx.SetLocalAuth(ctx, testServer.FakeClientRPCAuth())

	cli := clilib.Application(func(context.Context) context.Context {
		return clientCtx
	})

	exitCode := subcommands.Run(cli, []string{"ping", "-local", fmt.Sprintf("-address=%s", testServer.HTTPAddr())})

	assert.That(t, exitCode, should.Equal(0))
}
