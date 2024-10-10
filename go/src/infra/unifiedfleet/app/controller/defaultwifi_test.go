// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package controller

import (
	"testing"

	"google.golang.org/genproto/protobuf/field_mask"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	ufspb "infra/unifiedfleet/api/v1/models"
	. "infra/unifiedfleet/app/model/datastore"
	"infra/unifiedfleet/app/model/history"
)

func TestCreateDefaultWifi(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("CreateDefaultWifi", t, func(t *ftt.Test) {
		t.Run("Create new DefaultWifi - happy path", func(t *ftt.Test) {
			wifi := &ufspb.DefaultWifi{Name: "zone_sfo36_os"}
			resp, err := CreateDefaultWifi(ctx, wifi)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(wifi))
		})
		t.Run("Create new DefaultWifi - already existing", func(t *ftt.Test) {
			w1 := &ufspb.DefaultWifi{Name: "pool1"}
			_, _ = CreateDefaultWifi(ctx, w1)

			dup := &ufspb.DefaultWifi{Name: "pool1"}
			_, err := CreateDefaultWifi(ctx, dup)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("already exists"))
		})
	})
}

func TestDeleteDefaultWifi(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	CreateDefaultWifi(ctx, &ufspb.DefaultWifi{Name: "pool"})
	ftt.Run("DeleteDefaultWifi", t, func(t *ftt.Test) {
		t.Run("Delete DefaultWifi by existing ID - happy path", func(t *ftt.Test) {
			err := DeleteDefaultWifi(ctx, "pool")
			assert.Loosely(t, err, should.BeNil)

			res, err := GetDefaultWifi(ctx, "pool")
			assert.Loosely(t, res, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})

		t.Run("Delete DefaultWifi by non-existing ID", func(t *ftt.Test) {
			err := DeleteDefaultWifi(ctx, "non-existing")
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))
		})
	})
}

func TestUpdateDefaultWifi(t *testing.T) {
	t.Parallel()
	ctx := testingContext()
	ftt.Run("UpdateDefaultWifi", t, func(t *ftt.Test) {
		t.Run("Update DefaultWifi for existing DefaultWifi - happy path", func(t *ftt.Test) {
			CreateDefaultWifi(ctx, &ufspb.DefaultWifi{
				Name: "zone_sfo36_os",
				WifiSecret: &ufspb.Secret{
					ProjectId:  "p1",
					SecretName: "s1",
				},
			})
			w2 := &ufspb.DefaultWifi{
				Name: "zone_sfo36_os",
				WifiSecret: &ufspb.Secret{
					ProjectId:  "p1",
					SecretName: "s2",
				}}
			resp, err := UpdateDefaultWifi(ctx, w2, nil)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp, should.Match(w2))
			changes, err := history.QueryChangesByPropertyName(ctx, "name", "defaultwifis/zone_sfo36_os")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(2))
			assert.Loosely(t, changes[1].GetEventLabel(), should.Equal("defaultwifi.secret.secret_name"))
			assert.Loosely(t, changes[1].GetOldValue(), should.Equal("s1"))
			assert.Loosely(t, changes[1].GetNewValue(), should.Equal("s2"))
		})

		t.Run("Update DefaultWifi for non-existing DefaultWifi", func(t *ftt.Test) {
			resp, err := UpdateDefaultWifi(ctx, &ufspb.DefaultWifi{Name: "pool3"}, nil)
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring(NotFound))

			changes, err := history.QueryChangesByPropertyName(ctx, "name", "defaultwifis/pool3")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, changes, should.HaveLength(0))
		})

		t.Run("Update DefaultWifi for existing DefaultWifi with field mask - happy path", func(t *ftt.Test) {
			w3 := &ufspb.DefaultWifi{
				Name: "zone_sfo36_os",
				WifiSecret: &ufspb.Secret{
					ProjectId:  "ppppp",
					SecretName: "s3",
				}}
			resp, _ := UpdateDefaultWifi(ctx, w3, &field_mask.FieldMask{Paths: []string{"wifi_secret.secret_name"}})
			assert.Loosely(t, resp, should.NotBeNil)
			assert.Loosely(t, resp.GetWifiSecret().GetProjectId(), should.Equal("p1"))
			assert.Loosely(t, resp.GetWifiSecret().GetSecretName(), should.Equal("s3"))
		})

		t.Run("Update DefaultWifi for existing DefaultWifi with field mask - failure", func(t *ftt.Test) {
			w4 := &ufspb.DefaultWifi{
				Name: "zone_sfo36_os",
				WifiSecret: &ufspb.Secret{
					ProjectId:  "p1",
					SecretName: "s4",
				}}
			resp, err := UpdateDefaultWifi(ctx, w4, &field_mask.FieldMask{Paths: []string{"wifi_secret.non-existing-field"}})
			assert.Loosely(t, resp, should.BeNil)
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
