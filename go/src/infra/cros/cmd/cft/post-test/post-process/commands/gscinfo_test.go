// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"io"
	"log"
	"testing"

	"google.golang.org/protobuf/types/known/anypb"

	"go.chromium.org/chromiumos/config/go/test/artifact"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

const GSCInfoJSONFile = "test_data/gsc_info.json"

func TestGSCInfos(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	test := "storage.LowPowerStateResidence"
	emptyLogger := log.New(io.Discard, "", 0)

	ftt.Run(`GSCInfos works`, t, func(t *ftt.Test) {
		wantGSCInfo, _ := anypb.New(&artifact.GscInfo{
			GscRoVersion:   "0.0.59",
			GscRwVersion:   "0.26.112",
			GscRwBranch:    "tot:v0.0",
			GscRwRev:       "1408",
			GscRwSha:       "-ac00db8d",
			GscBuildurl:    "gs://chromeos-image-archive/firmware-ti50-postsubmit/R129-15969.0.0-102267-8741384054848642993",
			GscTestbedType: "gsc_dt_shield",
			GscCcdSerial:   "1482101a-4c2ac261",
		})
		want := map[string]*anypb.Any{
			test: wantGSCInfo,
		}
		gscFiles := map[string]string{
			test: GSCInfoJSONFile,
		}

		got := gscInfos(ctx, emptyLogger, gscFiles)

		assert.Loosely(t, got, should.Match(want))
	})

	ftt.Run(`Return empty if no GSC devboard info file is found`, t, func(t *ftt.Test) {
		wantGSCInfo, _ := anypb.New(&artifact.GscInfo{})
		want := map[string]*anypb.Any{
			test: wantGSCInfo,
		}
		gscFiles := map[string]string{
			test: "",
		}

		got := gscInfos(ctx, emptyLogger, gscFiles)

		assert.Loosely(t, got, should.Match(want))
	})
}
