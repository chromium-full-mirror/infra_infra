// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package controller

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	. "go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
)

func mockChromePlatform(id, desc string) *ufspb.ChromePlatform {
	return &ufspb.ChromePlatform{
		Name:        id,
		Description: desc,
	}
}

func TestListChromePlatforms(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	chromePlatforms := make([]*ufspb.ChromePlatform, 0, 4)
	for i := range 4 {
		chromePlatform1 := mockChromePlatform("", "Camera")
		chromePlatform1.Name = fmt.Sprintf("chromePlatform-%d", i)
		resp, _ := configuration.CreateChromePlatform(ctx, chromePlatform1)
		chromePlatforms = append(chromePlatforms, resp)
	}
	ftt.Run("ListChromePlatforms", t, func(t *ftt.Test) {
		t.Run("List chromePlatforms - filter invalid", func(t *ftt.Test) {
			_, _, err := ListChromePlatforms(ctx, 5, "", "machine=mx-1", false)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Failed to read filter for listing chromeplatforms"))
		})

		t.Run("ListChromePlatforms - Full listing - happy path", func(t *ftt.Test) {
			resp, _, _ := ListChromePlatforms(ctx, 5, "", "", false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(chromePlatforms))
		})
	})
}

func TestDeleteChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	chromePlatform1 := mockChromePlatform("chromePlatform-1", "Camera")
	chromePlatform2 := mockChromePlatform("chromePlatform-2", "Camera")
	chromePlatform3 := mockChromePlatform("chromePlatform-3", "Sensor")
	ftt.Run("DeleteChromePlatform", t, func(t *ftt.Test) {
		t.Run("Delete chromePlatform by existing ID with machine reference", func(t *ftt.Test) {
			resp, cerr := configuration.CreateChromePlatform(ctx, chromePlatform1)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))

			chromeBrowserMachine1 := &ufspb.Machine{
				Name: "machine-1",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-1",
					},
				},
			}
			mresp, merr := registration.CreateMachine(ctx, chromeBrowserMachine1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(chromeBrowserMachine1))

			err := DeleteChromePlatform(ctx, "chromePlatform-1")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(CannotDelete))

			resp, cerr = configuration.GetChromePlatform(ctx, "chromePlatform-1")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
		})
		t.Run("Delete chromePlatform by existing ID with KVM reference", func(t *ftt.Test) {
			resp, cerr := configuration.CreateChromePlatform(ctx, chromePlatform3)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform3))

			kvm1 := &ufspb.KVM{
				Name:           "kvm-1",
				ChromePlatform: "chromePlatform-3",
			}
			kresp, kerr := registration.CreateKVM(ctx, kvm1)
			assert.Loosely(t, kerr, should.BeNil)
			assert.Loosely(t, kresp, should.Match(kvm1))

			err := DeleteChromePlatform(ctx, "chromePlatform-3")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(CannotDelete))

			resp, cerr = configuration.GetChromePlatform(ctx, "chromePlatform-3")
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform3))
		})
		t.Run("Delete chromePlatform successfully by existing ID without references", func(t *ftt.Test) {
			resp, cerr := configuration.CreateChromePlatform(ctx, chromePlatform2)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform2))

			err := DeleteChromePlatform(ctx, "chromePlatform-2")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = configuration.GetChromePlatform(ctx, "chromePlatform-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestUpdateChromePlatforms(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	chromePlatform1 := mockChromePlatform("chromePlatform-update", "Camera")
	chromePlatform1.Manufacturer = "fake"
	chromePlatform2 := mockChromePlatform("chromePlatform-update-2", "Camera")
	chromePlatform2.Manufacturer = "apple"
	configuration.BatchUpdateChromePlatforms(ctx, []*ufspb.ChromePlatform{chromePlatform1, chromePlatform2})
	ftt.Run("UpdateChromePlatforms", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			p2 := mockChromePlatform("chromePlatform-update", "Camera")
			p2.Manufacturer = "non-fake"
			newP, err := UpdateChromePlatform(ctx, p2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newP, should.Match(p2))
		})

		t.Run("happy path with updating manufacturer", func(t *ftt.Test) {
			inventory.CreateMachineLSE(ctx, &ufspb.MachineLSE{
				Name:         "platform-host",
				Hostname:     "platform-host",
				Manufacturer: "apple",
				Machines:     []string{"platform-machine"},
			})
			registration.CreateMachine(ctx, &ufspb.Machine{
				Name: "platform-machine",
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-update-2",
					},
				},
			})
			p2 := mockChromePlatform("chromePlatform-update-2", "Camera")
			p2.Manufacturer = "dell"
			newP, err := UpdateChromePlatform(ctx, p2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newP, should.Match(p2))

			lse, err := inventory.GetMachineLSE(ctx, "platform-host")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lse.GetManufacturer(), should.Equal("dell"))
		})
	})
}

func TestBatchGetChromePlatforms(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("BatchGetChromePlatforms", t, func(t *ftt.Test) {
		t.Run("Batch get chrome platforms - happy path", func(t *ftt.Test) {
			platforms := make([]*ufspb.ChromePlatform, 4)
			for i := range 4 {
				platforms[i] = &ufspb.ChromePlatform{
					Name: fmt.Sprintf("platform-batchGet-%d", i),
				}
			}
			_, err := configuration.BatchUpdateChromePlatforms(ctx, platforms)
			assert.Loosely(t, err, should.BeNil)
			resp, err := configuration.BatchGetChromePlatforms(ctx, []string{"platform-batchGet-0", "platform-batchGet-1", "platform-batchGet-2", "platform-batchGet-3"})
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(4))
			assert.Loosely(t, resp, should.Match(platforms))
		})
		t.Run("Batch get chrome platforms  - missing id", func(t *ftt.Test) {
			resp, err := configuration.BatchGetChromePlatforms(ctx, []string{"platform-batchGet-non-existing"})
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("platform-batchGet-non-existing"))
		})
		t.Run("Batch get chrome platforms  - empty input", func(t *ftt.Test) {
			resp, err := configuration.BatchGetChromePlatforms(ctx, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))

			input := make([]string, 0)
			resp, err = configuration.BatchGetChromePlatforms(ctx, input)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.HaveLength(0))
		})
	})
}
