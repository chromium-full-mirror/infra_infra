// Copyright 2025 The Chromium Authors
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

const StressInfoFile = "test_data/stress_info.json"

func TestStressInfo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	test := "firmware.ECPDCCD.normal"
	emptyLogger := log.New(io.Discard, "", 0)

	ftt.Run(`stressTestInfos works`, t, func(t *ftt.Test) {
		stressTestFiles := map[string]string{
			test: StressInfoFile,
		}
		got := stressTestInfos(ctx, emptyLogger, stressTestFiles)
		wantStressTestInfo, _ := anypb.New(&artifact.StressTestInfo{
			Iterations: 10,
		})
		want := map[string]*anypb.Any{
			test: wantStressTestInfo,
		}

		assert.Loosely(t, got, should.Match(want))
	})

	ftt.Run(`Return nil if no stress info file is found`, t, func(t *ftt.Test) {
		stressTestFiles := map[string]string{
			test: "",
		}
		got := stressTestInfos(ctx, emptyLogger, stressTestFiles)
		wantStressInfo, _ := anypb.New(&artifact.StressTestInfo{})
		want := map[string]*anypb.Any{
			test: wantStressInfo,
		}

		assert.Loosely(t, got, should.Match(want))
	})
}
