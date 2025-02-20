// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/servertest"

	"go.chromium.org/infra/cros/fleetcost/cmd/fleetcost/cli"
	"go.chromium.org/infra/cros/fleetcost/cmd/fleetcostserver/serverlib"
)

// TestPing tests starting a server, pinging it, and then tearing it down.
func TestPing(t *testing.T) {
	// t.Parallel()

	ctx := context.Background()
	testServer, err := servertest.RunServer(ctx, &servertest.Settings{
		Modules: serverlib.GetModules(),
		Init:    serverlib.ServerMain,
	})

	assert.That(t, err, should.ErrLike(nil))
	assert.Loosely(t, testServer, should.NotBeNil)

	clitool := cli.GetApplication()

	exitStatus := subcommands.Run(clitool, []string{"ping", "-local", fmt.Sprintf("-address=%s", testServer.HTTPAddr())})
	assert.That(t, exitStatus, should.Equal(0))
}
