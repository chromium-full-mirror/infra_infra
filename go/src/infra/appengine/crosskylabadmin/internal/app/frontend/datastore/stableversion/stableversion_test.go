// Copyright 2019 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stableversion

import (
	"context"
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/impl/memory"
	"go.chromium.org/luci/gae/service/datastore"
)

func TestUpdateAndGet(t *testing.T) {
	// TODO(gregorynisbet): convert to test fixture
	ctx := context.Background()
	ctx = memory.Use(ctx)

	model := "xxx-model"
	buildTarget := "xxx-buildTarget"
	crosVersion := "xxx-cros-version"
	firmwareVersion := "xxx-firmware-version"
	faftVersion := "xxx-faft-version"

	ftt.Run("StableVersion datastore", t, func(t *ftt.Test) {
		t.Run("Cros", func(t *ftt.Test) {
			t.Run("Cros not present initially", func(t *ftt.Test) {
				item, err := GetCrosStableVersion(ctx, buildTarget, model)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, item, should.BeEmpty)
			})
			t.Run("Cros write should succeed", func(t *ftt.Test) {
				err := PutSingleCrosStableVersion(ctx, buildTarget, model, crosVersion)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("Cros present after write", func(t *ftt.Test) {
				item, err := GetCrosStableVersion(ctx, buildTarget, model)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, item, should.Equal(crosVersion))
			})
			t.Run("Cros is not case-sensitive", func(t *ftt.Test) {
				err := PutSingleCrosStableVersion(ctx, "AAA", "model", crosVersion)
				assert.Loosely(t, err, should.BeNil)
				item, err := GetCrosStableVersion(ctx, "Aaa", "model")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, item, should.Equal(crosVersion))
			})
		})
		t.Run("Faft", func(t *ftt.Test) {
			t.Run("Faft not present initially", func(t *ftt.Test) {
				item, err := GetFaftStableVersion(ctx, buildTarget, model)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, item, should.BeEmpty)
			})
			t.Run("Faft write should succeed", func(t *ftt.Test) {
				err := PutSingleFaftStableVersion(ctx, buildTarget, model, faftVersion)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("Faft present after write", func(t *ftt.Test) {
				item, err := GetFaftStableVersion(ctx, buildTarget, model)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, item, should.Equal(faftVersion))
			})
			t.Run("Faft is not case-sensitive", func(t *ftt.Test) {
				err := PutSingleFaftStableVersion(ctx, "AAA", "BBB", faftVersion)
				assert.Loosely(t, err, should.BeNil)
				item, err := GetFaftStableVersion(ctx, "Aaa", "Bbb")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, item, should.Equal(faftVersion))
			})
		})
		t.Run("Firmware", func(t *ftt.Test) {
			t.Run("Firmware not present initially", func(t *ftt.Test) {
				item, err := GetFirmwareStableVersion(ctx, buildTarget, model)
				assert.Loosely(t, err, should.NotBeNil)
				assert.Loosely(t, item, should.BeEmpty)
			})
			t.Run("Firmware write should succeed", func(t *ftt.Test) {
				err := PutSingleFirmwareStableVersion(ctx, buildTarget, model, firmwareVersion)
				assert.Loosely(t, err, should.BeNil)
			})
			t.Run("Firmware present after write", func(t *ftt.Test) {
				item, err := GetFirmwareStableVersion(ctx, buildTarget, model)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, item, should.Equal(firmwareVersion))
			})
			t.Run("Firmware is not case-sensitive", func(t *ftt.Test) {
				err := PutSingleFirmwareStableVersion(ctx, "AAA", "BBB", firmwareVersion)
				assert.Loosely(t, err, should.BeNil)
				item, err := GetFirmwareStableVersion(ctx, "Aaa", "Bbb")
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, item, should.Equal(firmwareVersion))
			})
		})
	})
}

func TestRemoveEmptyKeyOrValue(t *testing.T) {
	ftt.Run("remove non-conforming keys and values", t, func(t *ftt.Test) {
		ctx := context.Background()
		ctx = memory.Use(ctx)
		t.Run("remove empty key good", func(t *ftt.Test) {
			m := map[string]string{"": "a"}
			removeEmptyKeyOrValue(ctx, m)
			assert.Loosely(t, len(m), should.BeZero)
		})
		t.Run("remove empty value good", func(t *ftt.Test) {
			m := map[string]string{"a": ""}
			removeEmptyKeyOrValue(ctx, m)
			assert.Loosely(t, len(m), should.BeZero)
		})
		t.Run("remove empty key and value good", func(t *ftt.Test) {
			m := map[string]string{"": ""}
			removeEmptyKeyOrValue(ctx, m)
			assert.Loosely(t, len(m), should.BeZero)
		})
		t.Run("remove conforming key and value bad", func(t *ftt.Test) {
			m := map[string]string{"k": "v"}
			removeEmptyKeyOrValue(ctx, m)
			assert.Loosely(t, len(m), should.Equal(1))
		})
	})
}

// TestImposeVersion tests updating and deleting a stable version entry in datastore through the ImposeVersion interface.
func TestImposeVersion(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContext()
	datastore.GetTestable(ctx).Consistent(true)
	ftt.Run("test impose version", t, func(t *ftt.Test) {
		t.Run("cros", func(t *ftt.Test) {
			e := &CrosStableVersionEntity{
				ID:   "eve;eve",
				Cros: "a",
			}
			assert.Loosely(t, datastore.Put(ctx, e), should.BeNil)
			assert.Loosely(t, e.ImposeVersion(ctx, "b"), should.BeNil)
			var ents []*CrosStableVersionEntity
			assert.Loosely(t, datastore.GetAll(ctx, datastore.NewQuery(CrosStableVersionKind), &ents), should.BeNil)
			assert.Loosely(t, len(ents), should.Equal(1))
			assert.Loosely(t, ents[0].Cros, should.Equal("b"))
			assert.Loosely(t, e.ImposeVersion(ctx, ""), should.BeNil)
		})
		t.Run("faft", func(t *ftt.Test) {
			e := &FaftStableVersionEntity{
				ID:   "eve;eve",
				Faft: "a",
			}
			assert.Loosely(t, datastore.Put(ctx, e), should.BeNil)
			assert.Loosely(t, e.ImposeVersion(ctx, "b"), should.BeNil)
			var ents []*FaftStableVersionEntity
			assert.Loosely(t, datastore.GetAll(ctx, datastore.NewQuery(FaftStableVersionKind), &ents), should.BeNil)
			assert.Loosely(t, len(ents), should.Equal(1))
			assert.Loosely(t, ents[0].Faft, should.Equal("b"))
			assert.Loosely(t, e.ImposeVersion(ctx, ""), should.BeNil)
		})
		t.Run("firmware", func(t *ftt.Test) {
			e := &FirmwareStableVersionEntity{
				ID:       "eve;eve",
				Firmware: "a",
			}
			assert.Loosely(t, datastore.Put(ctx, e), should.BeNil)
			assert.Loosely(t, e.ImposeVersion(ctx, "b"), should.BeNil)
			var ents []*FirmwareStableVersionEntity
			assert.Loosely(t, datastore.GetAll(ctx, datastore.NewQuery(FirmwareStableVersionKind), &ents), should.BeNil)
			assert.Loosely(t, len(ents), should.Equal(1))
			assert.Loosely(t, ents[0].Firmware, should.Equal("b"))
			assert.Loosely(t, e.ImposeVersion(ctx, ""), should.BeNil)
		})
	})
}
