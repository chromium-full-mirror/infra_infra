// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package costserver_test

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	fleetcostAPI "go.chromium.org/infra/cros/fleetcost/api/rpc"
	testsupport "go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
)

// TestPing tests the ping API, which does nothing
func TestPing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tf := testsupport.NewFixture(ctx, t)

	response, err := tf.Frontend.Ping(tf.Ctx, &fleetcostAPI.PingRequest{})
	if err != nil {
		t.Error(err)
	}
	assert.That(t, response, should.Match(&fleetcostAPI.PingResponse{}))
}
