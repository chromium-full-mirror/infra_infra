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

const TastServoInfoFile = "test_data/servo_info_tast.json"
const TautoServoInfoFile = "test_data/servo_info_tauto.json"

func TestServoInfos(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	emptyLogger := log.New(io.Discard, "", 0)

	ftt.Run(`ServoInfo works for tast test`, t, func(t *ftt.Test) {
		wantServoInfo, _ := anypb.New(&artifact.BuildMetadata_ServoInfo{
			ServodVersion: "v1.0.2388-4e980b5d 2024-10-31 14:47:15",
			ServoType:     "servo_v4p1_with_ccd_cr50",
		})

		gotServoInfo, _ := ReadServoInfo(ctx, emptyLogger, TastServoInfoFile)

		assert.Loosely(t, gotServoInfo, should.Match(wantServoInfo))
	})

	ftt.Run(`ServoInfo works for tauto test`, t, func(t *ftt.Test) {
		wantServoInfoTauto, _ := anypb.New(&artifact.BuildMetadata_ServoInfo{
			ServodVersion: "v1.0.2388-4e980b5d 2024-10-31 14:47:15",
			ServoType:     "servo_v4p1_with_ccd_cr50",
			ServoVersions: "fizz-labstation-release/R131-16063.25.0,servo_v4p1_v2.0.24152-0b36eb51a,0.5.261/cr50_v4.11_mp.76-bc730d0f41",
		})

		gotServoInfoTauto, _ := ReadServoInfo(ctx, emptyLogger, TautoServoInfoFile)

		assert.Loosely(t, gotServoInfoTauto, should.Match(wantServoInfoTauto))
	})

	ftt.Run(`Return empty if no servo info file is found`, t, func(t *ftt.Test) {
		wantEmptyServoInfo, _ := anypb.New(&artifact.BuildMetadata_ServoInfo{})
		gotEmptyServoInfo, _ := ReadServoInfo(ctx, emptyLogger, "")

		assert.Loosely(t, gotEmptyServoInfo, should.Match(wantEmptyServoInfo))
	})
}

func TestFindServoPath(t *testing.T) {
	t.Parallel()

	emptyLogger := log.New(io.Discard, "", 0)

	ftt.Run(`Returns empty if no tast servo path`, t, func(t *ftt.Test) {
		gotServoPath := TastServoFilePath(emptyLogger, "n/a")

		assert.Loosely(t, gotServoPath, should.BeEmpty)
	})
}
