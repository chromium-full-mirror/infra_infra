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

	ufspb "infra/unifiedfleet/api/v1/models"
	. "infra/unifiedfleet/app/model/datastore"
)

func mockKVM(id string) *ufspb.KVM {
	return &ufspb.KVM{
		Name: id,
	}
}

func TestCreateKVM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	KVM1 := mockKVM("KVM-1")
	KVM2 := mockKVM("")
	ftt.Run("CreateKVM", t, func(t *ftt.Test) {
		t.Run("Create new KVM", func(t *ftt.Test) {
			resp, err := CreateKVM(ctx, KVM1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVM1))
		})
		t.Run("Create existing KVM", func(t *ftt.Test) {
			resp, err := CreateKVM(ctx, KVM1)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(AlreadyExists))
		})
		t.Run("Create KVM - invalid ID", func(t *ftt.Test) {
			resp, err := CreateKVM(ctx, KVM2)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestUpdateKVM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	KVM1 := mockKVM("KVM-1")
	KVM2 := mockKVM("KVM-1")
	KVM3 := mockKVM("KVM-3")
	KVM4 := mockKVM("")
	ftt.Run("UpdateKVM", t, func(t *ftt.Test) {
		t.Run("Update existing KVM", func(t *ftt.Test) {
			resp, err := CreateKVM(ctx, KVM1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVM1))

			resp, err = UpdateKVM(ctx, KVM2)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVM2))
		})
		t.Run("Update non-existing KVM", func(t *ftt.Test) {
			resp, err := UpdateKVM(ctx, KVM3)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Update KVM - invalid ID", func(t *ftt.Test) {
			resp, err := UpdateKVM(ctx, KVM4)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestGetKVM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	KVM1 := mockKVM("KVM-1")
	ftt.Run("GetKVM", t, func(t *ftt.Test) {
		t.Run("Get KVM by existing ID", func(t *ftt.Test) {
			resp, err := CreateKVM(ctx, KVM1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVM1))
			resp, err = GetKVM(ctx, "KVM-1")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVM1))
		})
		t.Run("Get KVM by non-existing ID", func(t *ftt.Test) {
			resp, err := GetKVM(ctx, "KVM-2")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Get KVM - invalid ID", func(t *ftt.Test) {
			resp, err := GetKVM(ctx, "")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestListKVMs(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	KVMs := make([]*ufspb.KVM, 0, 4)
	for i := 0; i < 4; i++ {
		KVM1 := mockKVM(fmt.Sprintf("KVM-%d", i))
		resp, _ := CreateKVM(ctx, KVM1)
		KVMs = append(KVMs, resp)
	}
	ftt.Run("ListKVMs", t, func(t *ftt.Test) {
		t.Run("List KVMs - page_token invalid", func(t *ftt.Test) {
			resp, nextPageToken, err := ListKVMs(ctx, 5, "abc", nil, false)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, nextPageToken, should.BeEmpty)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InvalidPageToken))
		})

		t.Run("List KVMs - Full listing with no pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListKVMs(ctx, 4, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVMs))
		})

		t.Run("List KVMs - listing with pagination", func(t *ftt.Test) {
			resp, nextPageToken, err := ListKVMs(ctx, 3, "", nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, nextPageToken, should.NotBeEmpty)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVMs[:3]))

			resp, _, err = ListKVMs(ctx, 2, nextPageToken, nil, false)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVMs[3:]))
		})
	})
}

func TestDeleteKVM(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")
	datastore.GetTestable(ctx).Consistent(true)
	KVM4 := mockKVM("KVM-4")
	ftt.Run("DeleteKVM", t, func(t *ftt.Test) {
		t.Run("Delete KVM successfully by existing ID", func(t *ftt.Test) {
			resp, cerr := CreateKVM(ctx, KVM4)
			assert.Loosely(t, cerr, should.BeNil)
			assert.Loosely(t, resp, should.Match(KVM4))

			err := DeleteKVM(ctx, "KVM-4")
			assert.Loosely(t, err, should.BeNil)

			resp, cerr = GetKVM(ctx, "KVM-4")
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, cerr, should.NotBeNil)
			assert.Loosely(t, cerr.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete KVM by non-existing ID", func(t *ftt.Test) {
			err := DeleteKVM(ctx, "KVM-6")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
		t.Run("Delete KVM - invalid ID", func(t *ftt.Test) {
			err := DeleteKVM(ctx, "")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestBatchUpdateKVMs(t *testing.T) {
	t.Parallel()
	ftt.Run("BatchUpdateKVMs", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		kvms := make([]*ufspb.KVM, 0, 4)
		for i := 0; i < 4; i++ {
			kvm1 := mockKVM(fmt.Sprintf("kvm-%d", i))
			kvm1.ChromePlatform = "chromePlatform-1"
			resp, err := CreateKVM(ctx, kvm1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(kvm1))
			kvms = append(kvms, resp)
		}
		t.Run("BatchUpdate all kvms", func(t *ftt.Test) {
			resp, err := BatchUpdateKVMs(ctx, kvms)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(kvms))
		})
		t.Run("BatchUpdate existing and non-existing kvms", func(t *ftt.Test) {
			KVM5 := mockKVM("")
			kvms = append(kvms, KVM5)
			resp, err := BatchUpdateKVMs(ctx, kvms)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(InternalError))
		})
	})
}

func TestQueryKVMByPropertyName(t *testing.T) {
	t.Parallel()
	ftt.Run("QueryKVMByPropertyName", t, func(t *ftt.Test) {
		ctx := gaetesting.TestingContextWithAppID("go-test")
		datastore.GetTestable(ctx).Consistent(true)
		dummyKVM := &ufspb.KVM{
			Name: "kvm-15",
		}
		kvm1 := mockKVM("kvm-15")
		kvm1.ChromePlatform = "chromePlatform-1"
		resp, cerr := CreateKVM(ctx, kvm1)
		assert.Loosely(t, cerr, should.BeNil)
		assert.Loosely(t, resp, should.Match(kvm1))

		kvms := make([]*ufspb.KVM, 0, 1)
		kvms = append(kvms, kvm1)

		kvms1 := make([]*ufspb.KVM, 0, 1)
		kvms1 = append(kvms1, dummyKVM)
		t.Run("Query By existing ChromePlatform keysonly", func(t *ftt.Test) {
			resp, err := QueryKVMByPropertyName(ctx, "chrome_platform_id", "chromePlatform-1", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(kvms1))
		})
		t.Run("Query By non-existing ChromePlatform", func(t *ftt.Test) {
			resp, err := QueryKVMByPropertyName(ctx, "chrome_platform_id", "chromePlatform-2", true)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.BeNil)
		})
		t.Run("Query By existing ChromePlatform", func(t *ftt.Test) {
			resp, err := QueryKVMByPropertyName(ctx, "chrome_platform_id", "chromePlatform-1", false)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.Match(kvms))
		})
	})
}
