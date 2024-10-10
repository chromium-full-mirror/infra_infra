// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	ufsAPI "infra/unifiedfleet/api/v1/rpc"
	"infra/unifiedfleet/app/model/caching"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/util"
)

func mockCachingService(name string) *ufspb.CachingService {
	return &ufspb.CachingService{
		Name: util.AddPrefix(util.CachingServiceCollection, name),
	}
}

func TestCreateCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("CreateCachingService", t, func(t *ftt.Test) {
		t.Run("Create new CachingService with cachingServiceId - happy path", func(t *ftt.Test) {
			cs := mockCachingService("")
			cs.PrimaryNode = "127.0.0.2"
			cs.SecondaryNode = "127.0.0.3"
			cs.ServingSubnets = []string{"1.1.1.0/24"}
			req := &ufsAPI.CreateCachingServiceRequest{
				CachingService:   cs,
				CachingServiceId: "127.0.0.1",
			}
			resp, err := tf.Fleet.CreateCachingService(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cs))
		})

		t.Run("Create new CachingService with nil entity", func(t *ftt.Test) {
			req := &ufsAPI.CreateCachingServiceRequest{
				CachingService:   nil,
				CachingServiceId: "128.0.0.1",
			}
			_, err := tf.Fleet.CreateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new CachingService without cachingServiceId", func(t *ftt.Test) {
			cs := mockCachingService("")
			cs.ServingSubnets = []string{"1.1.1.0/24"}
			req := &ufsAPI.CreateCachingServiceRequest{
				CachingService: cs,
			}
			_, err := tf.Fleet.CreateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new CachingService with empty primary node", func(t *ftt.Test) {
			cs := mockCachingService("")
			cs.PrimaryNode = ""
			cs.ServingSubnets = []string{"1.1.1.0/24"}
			req := &ufsAPI.CreateCachingServiceRequest{
				CachingService:   cs,
				CachingServiceId: "id",
			}
			_, err := tf.Fleet.CreateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Empty primary node name."))
		})

		t.Run("Create new CachingService with empty secondary node", func(t *ftt.Test) {
			cs := mockCachingService("")
			cs.PrimaryNode = "primary-node-name"
			cs.SecondaryNode = ""
			cs.ServingSubnets = []string{"1.1.1.0/24"}
			req := &ufsAPI.CreateCachingServiceRequest{
				CachingService:   cs,
				CachingServiceId: "id",
			}
			_, err := tf.Fleet.CreateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Empty secondary node name."))
		})

		t.Run("Create new CachingService with both subnets and zones", func(t *ftt.Test) {
			cs := mockCachingService("")
			cs.PrimaryNode = "primary-node-name"
			cs.SecondaryNode = "secondary-node-name"
			cs.ServingSubnets = []string{"1.1.1.0/24"}
			cs.Zones = []ufspb.Zone{ufspb.Zone_ZONE_CHROMEOS1}
			req := &ufsAPI.CreateCachingServiceRequest{
				CachingService:   cs,
				CachingServiceId: "id",
			}
			_, err := tf.Fleet.CreateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Cannot specify both subnets and zones"))
		})
	})
}

func TestUpdateCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("UpdateCachingService", t, func(t *ftt.Test) {
		t.Run("Update existing CachingService - happy path", func(t *ftt.Test) {
			caching.CreateCachingService(ctx, &ufspb.CachingService{
				Name: "127.0.0.1",
			})

			cs1 := mockCachingService("127.0.0.1")
			cs1.Port = 30000
			ureq := &ufsAPI.UpdateCachingServiceRequest{
				CachingService: cs1,
			}
			resp, err := tf.Fleet.UpdateCachingService(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Resemble(cs1))
		})

		t.Run("Update CachingService - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateCachingServiceRequest{
				CachingService: nil,
			}
			resp, err := tf.Fleet.UpdateCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update CachingService - Invalid input empty name", func(t *ftt.Test) {
			cs := mockCachingService("")
			cs.Name = ""
			req := &ufsAPI.UpdateCachingServiceRequest{
				CachingService: cs,
			}
			resp, err := tf.Fleet.UpdateCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update CachingService - Invalid input invalid name", func(t *ftt.Test) {
			cs := mockCachingService("a.b)7&")
			req := &ufsAPI.UpdateCachingServiceRequest{
				CachingService: cs,
			}
			resp, err := tf.Fleet.UpdateCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.CachingServiceNameFormat))
		})

		t.Run("Update new CachingService with invalid primary node", func(t *ftt.Test) {
			cs := mockCachingService("128.0.0.1")
			cs.PrimaryNode = "invalid name"
			req := &ufsAPI.UpdateCachingServiceRequest{
				CachingService: cs,
			}
			_, err := tf.Fleet.UpdateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidHostname))
		})

		t.Run("Update new CachingService with invalid secondary node", func(t *ftt.Test) {
			cs := mockCachingService("129.0.0.1")
			cs.SecondaryNode = "invalid name"
			req := &ufsAPI.UpdateCachingServiceRequest{
				CachingService: cs,
			}
			_, err := tf.Fleet.UpdateCachingService(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidHostname))
		})

	})
}

func TestGetCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	cs, _ := caching.CreateCachingService(ctx, &ufspb.CachingService{
		Name: "127.0.0.1",
	})
	ftt.Run("GetCachingService", t, func(t *ftt.Test) {
		t.Run("Get CachingService by existing ID - happy path", func(t *ftt.Test) {
			req := &ufsAPI.GetCachingServiceRequest{
				Name: util.AddPrefix(util.CachingServiceCollection, "127.0.0.1"),
			}
			resp, _ := tf.Fleet.GetCachingService(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			resp.Name = util.RemovePrefix(resp.Name)
			assert.Loosely(t, resp, should.Resemble(cs))
		})

		t.Run("Get CachingService - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetCachingServiceRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Get CachingService - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetCachingServiceRequest{
				Name: util.AddPrefix(util.CachingServiceCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.CachingServiceNameFormat))
		})
	})
}

func TestDeleteCachingService(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	caching.CreateCachingService(ctx, &ufspb.CachingService{
		Name: "127.0.0.1",
	})
	ftt.Run("DeleteCachingService", t, func(t *ftt.Test) {
		t.Run("Delete CachingService by existing ID - happy path", func(t *ftt.Test) {
			req := &ufsAPI.DeleteCachingServiceRequest{
				Name: util.AddPrefix(util.CachingServiceCollection, "127.0.0.1"),
			}
			_, err := tf.Fleet.DeleteCachingService(tf.C, req)
			assert.Loosely(t, err, should.BeNil)

			res, err := caching.GetCachingService(tf.C, "127.0.0.1")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete CachingService - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteCachingServiceRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete CachingService - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteCachingServiceRequest{
				Name: util.AddPrefix(util.CachingServiceCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteCachingService(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.CachingServiceNameFormat))
		})
	})
}

func TestListCachingServices(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	cachingServices := make([]*ufspb.CachingService, 0, 4)
	for i := 0; i < 4; i++ {
		cs := mockCachingService("")
		cs.Name = fmt.Sprintf("cs-%d", i)
		resp, _ := caching.CreateCachingService(tf.C, cs)
		resp.Name = util.AddPrefix(util.CachingServiceCollection, resp.Name)
		cachingServices = append(cachingServices, resp)
	}
	ftt.Run("ListCachingServices", t, func(t *ftt.Test) {
		t.Run("ListCachingServices - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListCachingServicesRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListCachingServices(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListCachingServices - Full listing with no pagination - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListCachingServicesRequest{}
			resp, err := tf.Fleet.ListCachingServices(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.CachingServices, should.Resemble(cachingServices))
		})

		t.Run("ListCachingServices - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListCachingServicesRequest{
				Filter: "state=x|state=y",
			}
			_, err := tf.Fleet.ListCachingServices(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}
