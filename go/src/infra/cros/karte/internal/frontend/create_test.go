// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"context"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	kartepb "go.chromium.org/infra/cros/karte/api"
	"go.chromium.org/infra/cros/karte/internal/identifiers"
	"go.chromium.org/infra/cros/karte/internal/scalars"
	"go.chromium.org/infra/cros/karte/internal/testsupport"
)

// TestCreateActionWithClock tests creating an action with the testing clock set to 10 seconds after
// the beginning of time (UTC midnight on 1970-01-01).
func TestCreateActionWithClock(t *testing.T) {
	ftt.Run("test create action with clock", t, func(t *ftt.Test) {
		ctx := testsupport.NewTestingContext(context.Background())
		ctx = identifiers.Use(ctx, identifiers.NewDefault())
		k := NewKarteFrontend("")

		action, err := k.CreateAction(ctx, &kartepb.CreateActionRequest{
			Action: &kartepb.Action{},
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, action.Name[0:10], should.Match("zzzzUzzzzz"))
		action.Name = ""
		assert.Loosely(t, action, should.Match(&kartepb.Action{
			CreateTime: scalars.ConvertTimeToTimestampPtr(time.Unix(10, 0)),
			SealTime:   scalars.ConvertTimeToTimestampPtr(time.Unix(43210, 0)),
		}))
	})
}
