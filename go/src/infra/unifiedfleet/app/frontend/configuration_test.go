// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"go.chromium.org/chromiumos/config/go/api"
	"go.chromium.org/chromiumos/config/go/payload"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "go.chromium.org/infra/unifiedfleet/api/v1/models"
	ufsAPI "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	"go.chromium.org/infra/unifiedfleet/app/model/configuration"
	"go.chromium.org/infra/unifiedfleet/app/model/datastore"
	"go.chromium.org/infra/unifiedfleet/app/model/inventory"
	"go.chromium.org/infra/unifiedfleet/app/model/registration"
	"go.chromium.org/infra/unifiedfleet/app/util"
)

func mockChromePlatform(id, desc string) *ufspb.ChromePlatform {
	return &ufspb.ChromePlatform{
		Name:        util.AddPrefix(util.ChromePlatformCollection, id),
		Description: desc,
	}
}

func mockMachineLSEPrototype(id string) *ufspb.MachineLSEPrototype {
	return &ufspb.MachineLSEPrototype{
		Name: util.AddPrefix(util.MachineLSEPrototypeCollection, id),
	}
}

func mockRackLSEPrototype(id string) *ufspb.RackLSEPrototype {
	return &ufspb.RackLSEPrototype{
		Name: util.AddPrefix(util.RackLSEPrototypeCollection, id),
	}
}

func mockVlan(id string) *ufspb.Vlan {
	return &ufspb.Vlan{
		Name: util.AddPrefix(util.VlanCollection, id),
	}
}

func mockConfigBundle(id string, programId string, name string) *payload.ConfigBundle {
	return &payload.ConfigBundle{
		DesignList: []*api.Design{
			{
				Id: &api.DesignId{
					Value: id,
				},
				ProgramId: &api.ProgramId{
					Value: programId,
				},
				Name: name,
			},
		},
	}
}

func TestCreateChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	chromePlatform1 := mockChromePlatform("", "Phone")
	chromePlatform2 := mockChromePlatform("", "Camera")
	chromePlatform3 := mockChromePlatform("", "Sensor")
	ftt.Run("CreateChromePlatform", t, func(t *ftt.Test) {
		t.Run("Create new chromePlatform with chromePlatform_id", func(t *ftt.Test) {
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform1,
				ChromePlatformId: "ChromePlatform-1",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
		})

		t.Run("Create existing chromePlatform", func(t *ftt.Test) {
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform3,
				ChromePlatformId: "ChromePlatform-1",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.AlreadyExists))
		})

		t.Run("Create new chromePlatform - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform: nil,
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new chromePlatform - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform2,
				ChromePlatformId: "",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new chromePlatform - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform2,
				ChromePlatformId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateChromePlatform(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	chromePlatform1 := mockChromePlatform("", "Camera")
	chromePlatform2 := mockChromePlatform("chromePlatform-1", "Phone")
	chromePlatform3 := mockChromePlatform("chromePlatform-3", "Sensor")
	chromePlatform4 := mockChromePlatform("a.b)7&", "Printer")
	ftt.Run("UpdateChromePlatform", t, func(t *ftt.Test) {
		t.Run("Update existing chromePlatform", func(t *ftt.Test) {
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform1,
				ChromePlatformId: "chromePlatform-1",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
			ureq := &ufsAPI.UpdateChromePlatformRequest{
				ChromePlatform: chromePlatform2,
			}
			resp, err = tf.Fleet.UpdateChromePlatform(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform2))
		})

		t.Run("Update non-existing chromePlatform", func(t *ftt.Test) {
			ureq := &ufsAPI.UpdateChromePlatformRequest{
				ChromePlatform: chromePlatform3,
			}
			resp, err := tf.Fleet.UpdateChromePlatform(tf.C, ureq)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("There is no ChromePlatform with ChromePlatformID chromePlatform-3 in the system"))
		})

		t.Run("Update chromePlatform - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateChromePlatformRequest{
				ChromePlatform: nil,
			}
			resp, err := tf.Fleet.UpdateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update chromePlatform - Invalid input empty name", func(t *ftt.Test) {
			chromePlatform3.Name = ""
			req := &ufsAPI.UpdateChromePlatformRequest{
				ChromePlatform: chromePlatform3,
			}
			resp, err := tf.Fleet.UpdateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update chromePlatform - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateChromePlatformRequest{
				ChromePlatform: chromePlatform4,
			}
			resp, err := tf.Fleet.UpdateChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetChromePlatform(t *testing.T) {
	t.Parallel()
	ftt.Run("GetChromePlatform", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		chromePlatform1 := mockChromePlatform("chromePlatform-1", "Camera")
		req := &ufsAPI.CreateChromePlatformRequest{
			ChromePlatform:   chromePlatform1,
			ChromePlatformId: "chromePlatform-1",
		}
		resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(chromePlatform1))
		t.Run("Get chromePlatform by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-1"),
			}
			resp, err := tf.Fleet.GetChromePlatform(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))
		})
		t.Run("Get chromePlatform by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-2"),
			}
			resp, err := tf.Fleet.GetChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("Get chromePlatform - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetChromePlatformRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get chromePlatform - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListChromePlatforms(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	chromePlatforms := make([]*ufspb.ChromePlatform, 0, 4)
	for i := range 4 {
		chromePlatform1 := mockChromePlatform("", "Camera")
		chromePlatform1.Name = fmt.Sprintf("chromePlatform-%d", i)
		resp, _ := configuration.CreateChromePlatform(tf.C, chromePlatform1)
		resp.Name = util.AddPrefix(util.ChromePlatformCollection, resp.Name)
		chromePlatforms = append(chromePlatforms, resp)
	}
	ftt.Run("ListChromePlatforms", t, func(t *ftt.Test) {
		t.Run("ListChromePlatforms - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListChromePlatformsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListChromePlatforms(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListChromePlatforms - Full listing with no pagination - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListChromePlatformsRequest{}
			resp, err := tf.Fleet.ListChromePlatforms(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.ChromePlatforms, should.Match(chromePlatforms))
		})

		t.Run("ListChromePlatforms - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListChromePlatformsRequest{
				Filter: "machine=mac-1|kvm=kvm-2",
			}
			_, err := tf.Fleet.ListChromePlatforms(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestDeleteChromePlatform(t *testing.T) {
	t.Parallel()
	ftt.Run("DeleteChromePlatform", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		t.Run("Delete chromePlatform by existing ID with references", func(t *ftt.Test) {
			chromePlatform1 := mockChromePlatform("", "Camera")
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform1,
				ChromePlatformId: "chromePlatform-1",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform1))

			chromeBrowserMachine1 := &ufspb.Machine{
				Name: util.AddPrefix(util.MachineCollection, "machine-1"),
				Device: &ufspb.Machine_ChromeBrowserMachine{
					ChromeBrowserMachine: &ufspb.ChromeBrowserMachine{
						ChromePlatform: "chromePlatform-1",
					},
				},
			}
			mresp, merr := registration.CreateMachine(tf.C, chromeBrowserMachine1)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(chromeBrowserMachine1))

			/* TODO(eshwarn) : Remove comment when kvm create/get is added
			kvm1 := &ufspb.KVM{
				Name: util.AddPrefix(kvmCollection, "kvm-1"),
				ChromePlatform: "chromePlatform-1",
			}
			kreq := &ufsAPI.CreateKVMMachineRequest{
				Kvm:   kvm1,
				KvmId: "kvm-1",
			}
			kresp, kerr := tf.Fleet.CreateKVM(tf.C, kreq)
			So(kerr, ShouldBeNil)
			So(kresp, ShouldResembleProto, kvm1)
			*/

			dreq := &ufsAPI.DeleteChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-1"),
			}
			_, err = tf.Fleet.DeleteChromePlatform(tf.C, dreq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.CannotDelete))

			greq := &ufsAPI.GetChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-1"),
			}
			res, err := tf.Fleet.GetChromePlatform(tf.C, greq)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(chromePlatform1))
		})

		t.Run("Delete chromePlatform by existing ID without references", func(t *ftt.Test) {
			chromePlatform2 := mockChromePlatform("", "Camera")
			req := &ufsAPI.CreateChromePlatformRequest{
				ChromePlatform:   chromePlatform2,
				ChromePlatformId: "chromePlatform-2",
			}
			resp, err := tf.Fleet.CreateChromePlatform(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(chromePlatform2))

			dreq := &ufsAPI.DeleteChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-2"),
			}
			_, err = tf.Fleet.DeleteChromePlatform(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			greq := &ufsAPI.GetChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-2"),
			}
			res, err := tf.Fleet.GetChromePlatform(tf.C, greq)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete chromePlatform by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "chromePlatform-2"),
			}
			_, err := tf.Fleet.DeleteChromePlatform(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete chromePlatform - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteChromePlatformRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete chromePlatform - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteChromePlatformRequest{
				Name: util.AddPrefix(util.ChromePlatformCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteChromePlatform(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machineLSEPrototype1 := mockMachineLSEPrototype("")
	machineLSEPrototype2 := mockMachineLSEPrototype("")
	machineLSEPrototype3 := mockMachineLSEPrototype("")
	ftt.Run("CreateMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Create new machineLSEPrototype with machineLSEPrototype_id", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype1,
				MachineLSEPrototypeId: "MachineLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
		})

		t.Run("Create existing machineLSEPrototype", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype3,
				MachineLSEPrototypeId: "MachineLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.AlreadyExists))
		})

		t.Run("Create new machineLSEPrototype - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype: nil,
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new machineLSEPrototype - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype2,
				MachineLSEPrototypeId: "",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new machineLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype2,
				MachineLSEPrototypeId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machineLSEPrototype1 := mockMachineLSEPrototype("")
	machineLSEPrototype2 := mockMachineLSEPrototype("machineLSEPrototype-1")
	machineLSEPrototype3 := mockMachineLSEPrototype("machineLSEPrototype-3")
	machineLSEPrototype4 := mockMachineLSEPrototype("a.b)7&")
	ftt.Run("UpdateMachineLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Update existing machineLSEPrototype", func(t *ftt.Test) {
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype1,
				MachineLSEPrototypeId: "machineLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
			ureq := &ufsAPI.UpdateMachineLSEPrototypeRequest{
				MachineLSEPrototype: machineLSEPrototype2,
			}
			resp, err = tf.Fleet.UpdateMachineLSEPrototype(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype2))
		})

		t.Run("Update non-existing machineLSEPrototype", func(t *ftt.Test) {
			ureq := &ufsAPI.UpdateMachineLSEPrototypeRequest{
				MachineLSEPrototype: machineLSEPrototype3,
			}
			resp, err := tf.Fleet.UpdateMachineLSEPrototype(tf.C, ureq)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Update machineLSEPrototype - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateMachineLSEPrototypeRequest{
				MachineLSEPrototype: nil,
			}
			resp, err := tf.Fleet.UpdateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update machineLSEPrototype - Invalid input empty name", func(t *ftt.Test) {
			machineLSEPrototype3.Name = ""
			req := &ufsAPI.UpdateMachineLSEPrototypeRequest{
				MachineLSEPrototype: machineLSEPrototype3,
			}
			resp, err := tf.Fleet.UpdateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update machineLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateMachineLSEPrototypeRequest{
				MachineLSEPrototype: machineLSEPrototype4,
			}
			resp, err := tf.Fleet.UpdateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ftt.Run("GetMachineLSEPrototype", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		machineLSEPrototype1 := mockMachineLSEPrototype("machineLSEPrototype-1")
		req := &ufsAPI.CreateMachineLSEPrototypeRequest{
			MachineLSEPrototype:   machineLSEPrototype1,
			MachineLSEPrototypeId: "machineLSEPrototype-1",
		}
		resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
		t.Run("Get machineLSEPrototype by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-1"),
			}
			resp, err := tf.Fleet.GetMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))
		})
		t.Run("Get machineLSEPrototype by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-2"),
			}
			resp, err := tf.Fleet.GetMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("Get machineLSEPrototype - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEPrototypeRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get machineLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListMachineLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	machineLSEPrototypes := make([]*ufspb.MachineLSEPrototype, 0, 4)
	for i := range 4 {
		machineLSEPrototype1 := mockMachineLSEPrototype("")
		machineLSEPrototype1.Name = fmt.Sprintf("machineLSEPrototype-%d", i)
		resp, _ := configuration.CreateMachineLSEPrototype(tf.C, machineLSEPrototype1)
		resp.Name = util.AddPrefix(util.MachineLSEPrototypeCollection, resp.Name)
		machineLSEPrototypes = append(machineLSEPrototypes, resp)
	}
	ftt.Run("ListMachineLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("ListMachineLSEPrototypes - page_size negative", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEPrototypesRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListMachineLSEPrototypes(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListMachineLSEPrototypes - Full listing", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEPrototypesRequest{}
			resp, err := tf.Fleet.ListMachineLSEPrototypes(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.MachineLSEPrototypes, should.Match(machineLSEPrototypes))
		})

		t.Run("ListMachineLSEPrototypes - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListMachineLSEPrototypesRequest{
				Filter: "machine=mac-1|kvm=kvm-2",
			}
			_, err := tf.Fleet.ListMachineLSEPrototypes(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestDeleteMachineLSEPrototype(t *testing.T) {
	t.Parallel()
	ftt.Run("DeleteMachineLSEPrototype", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		t.Run("Delete machineLSEPrototype by existing ID with references", func(t *ftt.Test) {
			machineLSEPrototype1 := mockMachineLSEPrototype("")
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype1,
				MachineLSEPrototypeId: "machineLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype1))

			machineLSE1 := &ufspb.MachineLSE{
				Name:                util.AddPrefix(util.MachineLSECollection, "machinelse-1"),
				MachineLsePrototype: "machineLSEPrototype-1",
				Hostname:            "machinelse-1",
			}
			machineLSE1, err = inventory.CreateMachineLSE(ctx, machineLSE1)
			assert.Loosely(t, machineLSE1, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)

			dreq := &ufsAPI.DeleteMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-1"),
			}
			_, err = tf.Fleet.DeleteMachineLSEPrototype(tf.C, dreq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.CannotDelete))

			greq := &ufsAPI.GetMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-1"),
			}
			res, err := tf.Fleet.GetMachineLSEPrototype(tf.C, greq)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(machineLSEPrototype1))
		})

		t.Run("Delete machineLSEPrototype by existing ID without references", func(t *ftt.Test) {
			machineLSEPrototype2 := mockMachineLSEPrototype("")
			req := &ufsAPI.CreateMachineLSEPrototypeRequest{
				MachineLSEPrototype:   machineLSEPrototype2,
				MachineLSEPrototypeId: "machineLSEPrototype-2",
			}
			resp, err := tf.Fleet.CreateMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(machineLSEPrototype2))

			dreq := &ufsAPI.DeleteMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-2"),
			}
			_, err = tf.Fleet.DeleteMachineLSEPrototype(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			greq := &ufsAPI.GetMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-2"),
			}
			res, err := tf.Fleet.GetMachineLSEPrototype(tf.C, greq)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete machineLSEPrototype by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "machineLSEPrototype-2"),
			}
			_, err := tf.Fleet.DeleteMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete machineLSEPrototype - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineLSEPrototypeRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete machineLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteMachineLSEPrototypeRequest{
				Name: util.AddPrefix(util.MachineLSEPrototypeCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteMachineLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rackLSEPrototype1 := mockRackLSEPrototype("")
	rackLSEPrototype2 := mockRackLSEPrototype("")
	rackLSEPrototype3 := mockRackLSEPrototype("")
	ftt.Run("CreateRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Create new rackLSEPrototype with rackLSEPrototype_id", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype1,
				RackLSEPrototypeId: "RackLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
		})

		t.Run("Create existing rackLSEPrototype", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype3,
				RackLSEPrototypeId: "RackLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.AlreadyExists))
		})

		t.Run("Create new rackLSEPrototype - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype: nil,
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new rackLSEPrototype - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype2,
				RackLSEPrototypeId: "",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new rackLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype2,
				RackLSEPrototypeId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rackLSEPrototype1 := mockRackLSEPrototype("")
	rackLSEPrototype2 := mockRackLSEPrototype("rackLSEPrototype-1")
	rackLSEPrototype3 := mockRackLSEPrototype("rackLSEPrototype-3")
	rackLSEPrototype4 := mockRackLSEPrototype("a.b)7&")
	ftt.Run("UpdateRackLSEPrototype", t, func(t *ftt.Test) {
		t.Run("Update existing rackLSEPrototype", func(t *ftt.Test) {
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype1,
				RackLSEPrototypeId: "rackLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
			ureq := &ufsAPI.UpdateRackLSEPrototypeRequest{
				RackLSEPrototype: rackLSEPrototype2,
			}
			resp, err = tf.Fleet.UpdateRackLSEPrototype(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype2))
		})

		t.Run("Update non-existing rackLSEPrototype", func(t *ftt.Test) {
			ureq := &ufsAPI.UpdateRackLSEPrototypeRequest{
				RackLSEPrototype: rackLSEPrototype3,
			}
			resp, err := tf.Fleet.UpdateRackLSEPrototype(tf.C, ureq)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Update rackLSEPrototype - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRackLSEPrototypeRequest{
				RackLSEPrototype: nil,
			}
			resp, err := tf.Fleet.UpdateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update rackLSEPrototype - Invalid input empty name", func(t *ftt.Test) {
			rackLSEPrototype3.Name = ""
			req := &ufsAPI.UpdateRackLSEPrototypeRequest{
				RackLSEPrototype: rackLSEPrototype3,
			}
			resp, err := tf.Fleet.UpdateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update rackLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateRackLSEPrototypeRequest{
				RackLSEPrototype: rackLSEPrototype4,
			}
			resp, err := tf.Fleet.UpdateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ftt.Run("GetRackLSEPrototype", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		rackLSEPrototype1 := mockRackLSEPrototype("rackLSEPrototype-1")
		req := &ufsAPI.CreateRackLSEPrototypeRequest{
			RackLSEPrototype:   rackLSEPrototype1,
			RackLSEPrototypeId: "rackLSEPrototype-1",
		}
		resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
		t.Run("Get rackLSEPrototype by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-1"),
			}
			resp, err := tf.Fleet.GetRackLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))
		})
		t.Run("Get rackLSEPrototype by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-2"),
			}
			resp, err := tf.Fleet.GetRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("Get rackLSEPrototype - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSEPrototypeRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get rackLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListRackLSEPrototypes(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	rackLSEPrototypes := make([]*ufspb.RackLSEPrototype, 0, 4)
	for i := range 4 {
		rackLSEPrototype1 := mockRackLSEPrototype("")
		rackLSEPrototype1.Name = fmt.Sprintf("rackLSEPrototype-%d", i)
		resp, _ := configuration.CreateRackLSEPrototype(tf.C, rackLSEPrototype1)
		resp.Name = util.AddPrefix(util.RackLSEPrototypeCollection, resp.Name)
		rackLSEPrototypes = append(rackLSEPrototypes, resp)
	}
	ftt.Run("ListRackLSEPrototypes", t, func(t *ftt.Test) {
		t.Run("ListRackLSEPrototypes - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEPrototypesRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListRackLSEPrototypes(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListRackLSEPrototypes - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEPrototypesRequest{
				PageSize: 2000,
			}
			resp, err := tf.Fleet.ListRackLSEPrototypes(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.RackLSEPrototypes, should.Match(rackLSEPrototypes))
		})

		t.Run("ListRackLSEPrototypes - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListRackLSEPrototypesRequest{
				Filter: "machine=mac-1|kvm=kvm-2",
			}
			_, err := tf.Fleet.ListRackLSEPrototypes(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestDeleteRackLSEPrototype(t *testing.T) {
	t.Parallel()
	ftt.Run("DeleteRackLSEPrototype", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		t.Run("Delete rackLSEPrototype by existing ID with references", func(t *ftt.Test) {
			rackLSEPrototype1 := mockRackLSEPrototype("")
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype1,
				RackLSEPrototypeId: "rackLSEPrototype-1",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype1))

			rackLSE1 := &ufspb.RackLSE{
				Name:             util.AddPrefix(util.RackLSECollection, "racklse-1"),
				RackLsePrototype: "rackLSEPrototype-1",
			}
			mreq := &ufsAPI.CreateRackLSERequest{
				RackLSE:   rackLSE1,
				RackLSEId: "racklse-1",
			}
			mresp, merr := tf.Fleet.CreateRackLSE(tf.C, mreq)
			assert.Loosely(t, merr, should.BeNil)
			assert.Loosely(t, mresp, should.Match(rackLSE1))

			dreq := &ufsAPI.DeleteRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-1"),
			}
			_, err = tf.Fleet.DeleteRackLSEPrototype(tf.C, dreq)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.CannotDelete))

			greq := &ufsAPI.GetRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-1"),
			}
			res, err := tf.Fleet.GetRackLSEPrototype(tf.C, greq)
			assert.Loosely(t, res, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res, should.Match(rackLSEPrototype1))
		})

		t.Run("Delete rackLSEPrototype by existing ID without references", func(t *ftt.Test) {
			rackLSEPrototype2 := mockRackLSEPrototype("")
			req := &ufsAPI.CreateRackLSEPrototypeRequest{
				RackLSEPrototype:   rackLSEPrototype2,
				RackLSEPrototypeId: "rackLSEPrototype-2",
			}
			resp, err := tf.Fleet.CreateRackLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(rackLSEPrototype2))

			dreq := &ufsAPI.DeleteRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-2"),
			}
			_, err = tf.Fleet.DeleteRackLSEPrototype(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			greq := &ufsAPI.GetRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-2"),
			}
			res, err := tf.Fleet.GetRackLSEPrototype(tf.C, greq)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete rackLSEPrototype by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "rackLSEPrototype-2"),
			}
			_, err := tf.Fleet.DeleteRackLSEPrototype(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete rackLSEPrototype - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSEPrototypeRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete rackLSEPrototype - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteRackLSEPrototypeRequest{
				Name: util.AddPrefix(util.RackLSEPrototypeCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteRackLSEPrototype(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestCreateVlan(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	vlan1 := mockVlan("")
	vlan2 := mockVlan("")
	vlan3 := mockVlan("")
	ftt.Run("CreateVlan", t, func(t *ftt.Test) {
		t.Run("Create new vlan with vlan_id", func(t *ftt.Test) {
			vlan1.VlanAddress = "192.168.255.248/27"
			req := &ufsAPI.CreateVlanRequest{
				Vlan:   vlan1,
				VlanId: "Vlan-1",
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
		})

		t.Run("Create existing vlan", func(t *ftt.Test) {
			vlan3.VlanAddress = "192.168.255.248/27"
			req := &ufsAPI.CreateVlanRequest{
				Vlan:   vlan3,
				VlanId: "Vlan-1",
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already exists"))
		})

		t.Run("Create new vlan - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.CreateVlanRequest{
				Vlan: nil,
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Create new vlan - Invalid input empty ID", func(t *ftt.Test) {
			req := &ufsAPI.CreateVlanRequest{
				Vlan:   vlan2,
				VlanId: "",
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyID))
		})

		t.Run("Create new vlan - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.CreateVlanRequest{
				Vlan:   vlan2,
				VlanId: "a.b)7&",
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestUpdateVlan(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	vlan1 := mockVlan("")
	vlan2 := mockVlan("vlan-1")
	vlan3 := mockVlan("vlan-3")
	vlan4 := mockVlan("a.b)7&")
	ftt.Run("UpdateVlan", t, func(t *ftt.Test) {
		t.Run("Update existing vlan", func(t *ftt.Test) {
			vlan1.VlanAddress = "3.3.3.3/27"
			req := &ufsAPI.CreateVlanRequest{
				Vlan:   vlan1,
				VlanId: "vlan-1",
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
			ureq := &ufsAPI.UpdateVlanRequest{
				Vlan: vlan2,
			}
			resp, err = tf.Fleet.UpdateVlan(tf.C, ureq)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan2))
		})

		t.Run("Update non-existing vlan", func(t *ftt.Test) {
			ureq := &ufsAPI.UpdateVlanRequest{
				Vlan: vlan3,
			}
			resp, err := tf.Fleet.UpdateVlan(tf.C, ureq)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Update vlan - Invalid input nil", func(t *ftt.Test) {
			req := &ufsAPI.UpdateVlanRequest{
				Vlan: nil,
			}
			resp, err := tf.Fleet.UpdateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.NilEntity))
		})

		t.Run("Update vlan - Invalid input empty name", func(t *ftt.Test) {
			vlan3.Name = ""
			req := &ufsAPI.UpdateVlanRequest{
				Vlan: vlan3,
			}
			resp, err := tf.Fleet.UpdateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Update vlan - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.UpdateVlanRequest{
				Vlan: vlan4,
			}
			resp, err := tf.Fleet.UpdateVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestGetVlan(t *testing.T) {
	t.Parallel()
	ftt.Run("GetVlan", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		vlan1 := mockVlan("vlan-1")
		vlan1.VlanAddress = "3.3.3.4/27"
		req := &ufsAPI.CreateVlanRequest{
			Vlan:   vlan1,
			VlanId: "vlan-1",
		}
		resp, err := tf.Fleet.CreateVlan(tf.C, req)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, resp, should.Match(vlan1))
		t.Run("Get vlan by existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "vlan-1"),
			}
			resp, err := tf.Fleet.GetVlan(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan1))
		})
		t.Run("Get vlan by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.GetVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "vlan-2"),
			}
			resp, err := tf.Fleet.GetVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})
		t.Run("Get vlan - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.GetVlanRequest{
				Name: "",
			}
			resp, err := tf.Fleet.GetVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})
		t.Run("Get vlan - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.GetVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.GetVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestListVlans(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	vlans := make([]*ufspb.Vlan, 0, 4)
	for i := range 4 {
		vlan1 := mockVlan("")
		vlan1.Name = fmt.Sprintf("vlan-%d", i)
		resp, _ := configuration.CreateVlan(tf.C, vlan1)
		resp.Name = util.AddPrefix(util.VlanCollection, resp.Name)
		vlans = append(vlans, resp)
	}
	ftt.Run("ListVlans", t, func(t *ftt.Test) {
		t.Run("ListVlans - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListVlansRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListVlans(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListVlans - Full listing - happy path", func(t *ftt.Test) {
			req := &ufsAPI.ListVlansRequest{}
			resp, err := tf.Fleet.ListVlans(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp.Vlans, should.Match(vlans))
		})

		t.Run("ListVlans - page_size negative - filter format invalid format OR - error", func(t *ftt.Test) {
			req := &ufsAPI.ListVlansRequest{
				Filter: "machine=mac-1|kvm=kvm-2",
			}
			_, err := tf.Fleet.ListVlans(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidFilterFormat))
		})
	})
}

func TestListIPs(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("ListIPs", t, func(t *ftt.Test) {
		t.Run("ListIPs - page_size negative - error", func(t *ftt.Test) {
			req := &ufsAPI.ListIPsRequest{
				PageSize: -5,
			}
			resp, err := tf.Fleet.ListIPs(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidPageSize))
		})

		t.Run("ListIPs - unsupported filter", func(t *ftt.Test) {
			req := &ufsAPI.ListIPsRequest{
				Filter: "machine=mac-1",
			}
			_, err := tf.Fleet.ListIPs(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("Invalid field name"))
		})

		t.Run("ListIPs - happy path", func(t *ftt.Test) {
			vlan1 := mockVlan("listip-vlan-1")
			vlan1.VlanAddress = "192.168.100.0/27"
			_, err := configuration.CreateVlan(ctx, vlan1)
			assert.Loosely(t, err, should.BeNil)
			ips := []*ufspb.IP{
				{
					Id:       "listip1",
					Occupied: true,
					Ipv4Str:  "192.168.100.1",
					Vlan:     "listip-vlan-1",
					Ipv4:     uint32(100),
				},
				{
					Id:      "listip2",
					Ipv4Str: "192.168.100.2",
					Vlan:    "listip-vlan-1",
					Ipv4:    uint32(100),
				},
			}
			_, err = configuration.BatchUpdateIPs(ctx, ips)
			assert.Loosely(t, err, should.BeNil)

			req := &ufsAPI.ListIPsRequest{
				Filter: "vlan=listip-vlan-1",
			}
			resp, err := tf.Fleet.ListIPs(tf.C, req)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetIps(), should.HaveLength(2))
			assert.Loosely(t, err, should.BeNil)
		})
	})
}

func TestDeleteVlan(t *testing.T) {
	t.Parallel()
	ftt.Run("DeleteVlan", t, func(t *ftt.Test) {
		ctx := testingContext()
		tf, validate := newTestFixtureWithContext(ctx, t.T)
		defer validate()
		t.Run("Delete vlan by existing ID without references", func(t *ftt.Test) {
			vlan2 := mockVlan("")
			vlan2.VlanAddress = "192.168.110.0/27"
			req := &ufsAPI.CreateVlanRequest{
				Vlan:   vlan2,
				VlanId: "vlan-2",
			}
			resp, err := tf.Fleet.CreateVlan(tf.C, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(vlan2))

			dreq := &ufsAPI.DeleteVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "vlan-2"),
			}
			_, err = tf.Fleet.DeleteVlan(tf.C, dreq)
			assert.Loosely(t, err, should.BeNil)

			greq := &ufsAPI.GetVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "vlan-2"),
			}
			res, err := tf.Fleet.GetVlan(tf.C, greq)
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete vlan by non-existing ID", func(t *ftt.Test) {
			req := &ufsAPI.DeleteVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "vlan-2"),
			}
			_, err := tf.Fleet.DeleteVlan(tf.C, req)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(datastore.NotFound))
		})

		t.Run("Delete vlan - Invalid input empty name", func(t *ftt.Test) {
			req := &ufsAPI.DeleteVlanRequest{
				Name: "",
			}
			resp, err := tf.Fleet.DeleteVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.EmptyName))
		})

		t.Run("Delete vlan - Invalid input invalid characters", func(t *ftt.Test) {
			req := &ufsAPI.DeleteVlanRequest{
				Name: util.AddPrefix(util.VlanCollection, "a.b)7&"),
			}
			resp, err := tf.Fleet.DeleteVlan(tf.C, req)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(ufsAPI.InvalidCharacters))
		})
	})
}

func TestOSImportVlans(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()
	ftt.Run("Import OS vlan-related infos", t, func(t *ftt.Test) {
		t.Run("happy path", func(t *ftt.Test) {
			req := &ufsAPI.ImportOSVlansRequest{
				Source: &ufsAPI.ImportOSVlansRequest_MachineDbSource{
					MachineDbSource: &ufsAPI.MachineDBSource{
						Host: "fake_host",
					},
				},
			}
			tf.Fleet.importPageSize = 25
			res, err := tf.Fleet.ImportOSVlans(ctx, req)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.Code, should.Equal(code.Code_OK))
		})
	})
}

func getReturnedPlatformNames(res datastore.OpResults) []string {
	gets := make([]string, len(res))
	for i, r := range res {
		gets[i] = r.Data.(*ufspb.ChromePlatform).GetName()
	}
	return gets
}

func TestUpdateConfigBundle(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	tf, validate := newTestFixtureWithContext(ctx, t)
	defer validate()

	t.Run("update non-existent ConfigBundle", func(t *testing.T) {
		cb1 := mockConfigBundle("design1", "program1", "name1")
		cb1Bytes, err := proto.Marshal(cb1)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}
		want := &ufsAPI.UpdateConfigBundleResponse{
			ConfigBundle: cb1Bytes,
		}

		got, err := tf.Fleet.UpdateConfigBundle(tf.C, &ufsAPI.UpdateConfigBundleRequest{
			ConfigBundle: cb1Bytes,
			UpdateMask:   nil,
			AllowMissing: true,
		})
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}

		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("UpdateConfigBundle returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("update existent ConfigBundle", func(t *testing.T) {
		cb2 := mockConfigBundle("design2", "program2", "name2")
		cb2Bytes, err := proto.Marshal(cb2)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}

		_, _ = tf.Fleet.UpdateConfigBundle(tf.C, &ufsAPI.UpdateConfigBundleRequest{
			ConfigBundle: cb2Bytes,
			UpdateMask:   nil,
			AllowMissing: true,
		})

		// Update cb2
		cb2update := mockConfigBundle("design2", "program2", "name2update")
		cb2updateBytes, err := proto.Marshal(cb2update)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}
		want := &ufsAPI.UpdateConfigBundleResponse{
			ConfigBundle: cb2updateBytes,
		}

		got, err := tf.Fleet.UpdateConfigBundle(tf.C, &ufsAPI.UpdateConfigBundleRequest{
			ConfigBundle: cb2updateBytes,
			UpdateMask:   nil,
			AllowMissing: true,
		})
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}

		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("UpdateConfigBundle returned unexpected diff (-want +got):\n%s", diff)
		}
	})

	t.Run("update existent ConfigBundle", func(t *testing.T) {
		cb3 := mockConfigBundle("", "", "")
		cb3Bytes, err := proto.Marshal(cb3)
		if err != nil {
			t.Fatalf("UpdateConfigBundle failed: %s", err)
		}

		got, err := tf.Fleet.UpdateConfigBundle(tf.C, &ufsAPI.UpdateConfigBundleRequest{
			ConfigBundle: cb3Bytes,
			UpdateMask:   nil,
			AllowMissing: true,
		})
		if err == nil {
			t.Errorf("UpdateConfigBundle succeeded with empty IDs")
		}
		if c := status.Code(err); c != codes.InvalidArgument {
			t.Errorf("Unexpected error when calling GetConfigBundle: %s", err)
		}

		var respNil *ufsAPI.UpdateConfigBundleResponse = nil
		if diff := cmp.Diff(respNil, got); diff != "" {
			t.Errorf("UpdateConfigBundle returned unexpected diff (-want +got):\n%s", diff)
		}
	})
}
