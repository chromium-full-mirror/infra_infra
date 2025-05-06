// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testsupport_test

import (
	"context"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/fleetcost/internal/costserver/testsupport"
)

// TestClockSmokeTest tests that grabbing the clock and advancing it one hour
// causes the time to be one hour later when reading.
func TestClockSmokeTest(t *testing.T) {
	t.Parallel()

	tf := testsupport.NewFixture(context.Background(), t)
	now := tf.Clock().Now()
	tf.AdvanceClock(1 * time.Hour)
	later := tf.Clock().Now()

	delta := later.Sub(now)
	assert.That(t, delta, should.Match(time.Hour))
}
