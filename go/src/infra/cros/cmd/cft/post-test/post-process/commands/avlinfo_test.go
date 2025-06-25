// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"io"
	"log"
	"testing"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const AvlInfoFile = "test_data/avl_info.json"

func TestAvlInfos(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	test := "storage.LowPowerStateResidence"
	emptyLogger := log.New(io.Discard, "", 0)

	ftt.Run("AVLInfos works", t, func(t *ftt.Test) {
		want := map[string]*api.AvlInfo{
			test: {
				AvlPartModel:     "0x0000f5 MMC32G",
				AvlPartFirmware:  "0xa200000000000000",
				AvlComponentType: "storage",
			},
		}
		avlFiles := map[string]string{
			test: AvlInfoFile,
		}

		got := avlInfos(ctx, emptyLogger, avlFiles)

		assert.Loosely(t, len(got), should.Equal(1))
		assert.Loosely(t, got[test], should.Match(want[test]))
	})

	ftt.Run("Return empty if no avl info file is found", t, func(t *ftt.Test) {
		want := map[string]*api.AvlInfo{
			test: {},
		}
		avlFiles := map[string]string{
			test: "",
		}

		got := avlInfos(ctx, emptyLogger, avlFiles)

		assert.Loosely(t, len(got), should.Equal(1))
		assert.Loosely(t, got[test], should.Match(want[test]))
	})
}
