// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commoncommands_test

import (
	"container/list"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cros/cmd/common_lib/commoncommands"
)

func TestPopQueueInstantiation_BadCast(t *testing.T) {
	ftt.Run("Bad Type Cast", t, func(t *ftt.Test) {
		queue := list.New()
		queue.PushBack(&api.TestTask{
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "key",
					Value: "value",
				},
			},
		})
		err := commoncommands.InstantiatePopFromQueue(queue, func(element any) {
			_ = element.(*api.ProvisionTask)
		})
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestPopQueueInstantiation_GoodCast(t *testing.T) {
	ftt.Run("Good Type Cast", t, func(t *ftt.Test) {
		queue := list.New()
		queue.PushBack(&api.TestTask{
			DynamicDeps: []*api.DynamicDep{
				{
					Key:   "key",
					Value: "value",
				},
			},
		})
		var testRequest *api.TestTask
		err := commoncommands.InstantiatePopFromQueue(queue, func(element any) {
			testRequest = element.(*api.TestTask)
		})
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, testRequest, should.NotBeNil)
		assert.Loosely(t, testRequest.DynamicDeps, should.HaveLength(1))
		assert.Loosely(t, testRequest.DynamicDeps[0].Key, should.Equal("key"))
	})
}
