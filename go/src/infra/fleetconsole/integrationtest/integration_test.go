// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package integrationtest

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/servertest"

	"infra/fleetconsole/cmd/consoleadmin/clilib"
	"infra/fleetconsole/cmd/fleetconsoleserver/serverlib"
)

// TestNothing just loads the server and the command line application.
//
// Future CLs will make this test actually do something.
func TestNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testServer, err := servertest.RunServer(ctx, &servertest.Settings{
		Modules: serverlib.Modules(),
		Init:    serverlib.ServerMain,
	})

	assert.That(t, err, should.ErrLike(nil))
	assert.Loosely(t, testServer, should.NotBeNil)

	_ = clilib.Application()
}
