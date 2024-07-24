// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	ufspb "infra/unifiedfleet/api/v1/models"
	"infra/unifiedfleet/app/model/registration"
)

func TestCreateRackLSE(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("CreateRackLSE", t, func(t *ftt.Test) {
		t.Run("Create new rackLSE with non existing racks", func(t *ftt.Test) {
			rackLSE1 := &ufspb.RackLSE{
				Name:  "racklse-1",
				Racks: []string{"rack-1", "rack-2"},
			}
			resp, err := CreateRackLSE(ctx, rackLSE1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(CannotCreate))
		})
		t.Run("Create new rackLSE with existing racks", func(t *ftt.Test) {
			rack1 := &ufspb.Rack{
				Name: "rack-1",
			}
			mresp, merr := registration.CreateRack(ctx, rack1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Resemble(rack1))

			rackLSE2 := &ufspb.RackLSE{
				Name:  "racklse-2",
				Racks: []string{"rack-1"},
			}
			resp, err := CreateRackLSE(ctx, rackLSE2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(rackLSE2))
		})
	})
}
