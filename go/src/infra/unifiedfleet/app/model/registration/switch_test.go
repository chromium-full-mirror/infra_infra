// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registration

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
)

func TestCreateSwitch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	switch1 := mockSwitch("Switch-1")
	switch2 := mockSwitch("")
	ftt.Run("CreateSwitch", t, func(t *ftt.Test) {
		t.Run("Create new switch", func(t *ftt.Test) {
			resp, err := CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch1))
		})
		t.Run("Create existing switch", func(t *ftt.Test) {
			resp, err := CreateSwitch(ctx, switch1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create switch - invalid ID", func(t *ftt.Test) {
			resp, err := CreateSwitch(ctx, switch2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateSwitch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	switch1 := mockSwitch("Switch-1")
	switch2 := mockSwitch("Switch-1")
	switch3 := mockSwitch("Switch-3")
	switch4 := mockSwitch("")
	ftt.Run("UpdateSwitch", t, func(t *ftt.Test) {
		t.Run("Update existing switch", func(t *ftt.Test) {
			resp, err := CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch1))

			resp, err = UpdateSwitch(ctx, switch2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch2))
		})
		t.Run("Update non-existing switch", func(t *ftt.Test) {
			resp, err := UpdateSwitch(ctx, switch3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update switch - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateSwitch(ctx, switch4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetSwitch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	switch1 := mockSwitch("Switch-1")
	ftt.Run("GetSwitch", t, func(t *ftt.Test) {
		t.Run("Get switch by existing ID", func(t *ftt.Test) {
			resp, err := CreateSwitch(ctx, switch1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch1))
			resp, err = GetSwitch(ctx, "Switch-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch1))
		})
		t.Run("Get switch by non-existing ID", func(t *ftt.Test) {
			resp, err := GetSwitch(ctx, "switch-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get switch - invalid ID", func(t *ftt.Test) {
			resp, err := GetSwitch(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListSwitches(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	switches := make([]*ufspb.Switch, 0, 4)
	for i := 0; i < 4; i++ {
		switch1 := mockSwitch(fmt.Sprintf("switch-%d", i))
		resp, _ := CreateSwitch(ctx, switch1)
		switches = append(switches, resp)
	}
	ftt.Run("ListSwitches", t, func(t *ftt.Test) {
		t.Run("List switches - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListSwitches(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List switches - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListSwitches(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switches))
		})

		t.Run("List switches - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListSwitches(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switches[:3]))

			resp, _, err = ListSwitches(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(switches[3:]))
		})
	})
}

func TestDeleteSwitch(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	switch2 := mockSwitch("switch-2")
	ftt.Run("DeleteSwitch", t, func(t *ftt.Test) {
		t.Run("Delete switch successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateSwitch(ctx, switch2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(switch2))

			err := DeleteSwitch(ctx, "switch-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetSwitch(ctx, "switch-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete switch by non-existing ID", func(t *ftt.Test) {
			err := DeleteSwitch(ctx, "switch-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete switch - invalid ID", func(t *ftt.Test) {
			err := DeleteSwitch(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func mockSwitch(id string) *ufspb.Switch {
	return &ufspb.Switch{
		Name: id,
	}
}
