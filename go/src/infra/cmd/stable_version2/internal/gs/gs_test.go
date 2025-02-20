// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gs

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/cmd/stable_version2/internal/utils"
)

func TestParseOmahaStatus(t *testing.T) {
	ftt.Run("Parse omaha status file", t, func(t *ftt.Test) {
		ctx := context.Background()
		bt, err := ioutil.ReadFile(testDataPath("omaha_status.json"))
		assert.Loosely(t, err, should.BeNil)

		res, err := ParseOmahaStatus(ctx, bt)
		assert.Loosely(t, err, should.BeNil)

		t.Run("Parse normal board", func(t *ftt.Test) {
			v := utils.GetCrOSSVByBuildtarget(res, "normal")
			assert.Loosely(t, v, should.Equal("R59-9414.0.0"))
		})

		t.Run("Parse board name with dash, dash should be replaced by underscore", func(t *ftt.Test) {
			v := utils.GetCrOSSVByBuildtarget(res, "board-with-dash")
			assert.Loosely(t, v, should.BeEmpty)

			v = utils.GetCrOSSVByBuildtarget(res, "board_with_dash")
			assert.Loosely(t, v, should.Equal("R59-9414.0.0"))
		})

		t.Run("Parse board with 2 chromeos versions, shoud return the new one", func(t *ftt.Test) {
			v := utils.GetCrOSSVByBuildtarget(res, "board_with_new_version")
			assert.Loosely(t, v, should.Equal("R59-9514.0.0"))
		})

		t.Run("Parse board with 2 milestones, shoud return the new one", func(t *ftt.Test) {
			v := utils.GetCrOSSVByBuildtarget(res, "board_with_new_milestone")
			assert.Loosely(t, v, should.Equal("R60-9414.0.0"))
		})

		t.Run("Parse non-beta channel board, no return", func(t *ftt.Test) {
			v := utils.GetCrOSSVByBuildtarget(res, "canaryboard")
			assert.Loosely(t, v, should.BeEmpty)
		})
	})
}

func TestParseMetaData(t *testing.T) {
	ftt.Run("Parse meta data file", t, func(t *ftt.Test) {

		t.Run("Parse non-unibuild", func(t *ftt.Test) {
			bt, err := ioutil.ReadFile((testDataPath("meta_data.json")))
			assert.Loosely(t, err, should.BeNil)

			res, err := ParseMetadata(bt)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.FirmwareVersions, should.HaveLength(1))
			assert.Loosely(t, res.FirmwareVersions[0].GetKey().GetBuildTarget().GetName(), should.Equal("arkham"))
			assert.Loosely(t, res.FirmwareVersions[0].GetKey().GetModelId().GetValue(), should.Equal("arkham"))
			assert.Loosely(t, res.FirmwareVersions[0].GetVersion(), should.Equal("v1"))
		})

		t.Run("Parse unibuild", func(t *ftt.Test) {
			bt, err := ioutil.ReadFile((testDataPath("meta_data_unibuild.json")))
			assert.Loosely(t, err, should.BeNil)

			res, err := ParseMetadata(bt)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, res.FirmwareVersions, should.HaveLength(2))
			assert.Loosely(t, res.FirmwareVersions[0].GetKey().GetBuildTarget().GetName(), should.Equal("reef"))
			assert.Loosely(t, res.FirmwareVersions[1].GetKey().GetBuildTarget().GetName(), should.Equal("reef"))
			models := map[string]string{
				res.FirmwareVersions[0].GetKey().GetModelId().GetValue(): res.FirmwareVersions[0].GetVersion(),
				res.FirmwareVersions[1].GetKey().GetModelId().GetValue(): res.FirmwareVersions[1].GetVersion(),
			}
			assert.Loosely(t, models["model1"], should.Equal("v1"))
			assert.Loosely(t, models["model2"], should.Equal("v2"))
		})
	})
}

func testDataPath(p string) string {
	return filepath.Join("testdata", p)
}
