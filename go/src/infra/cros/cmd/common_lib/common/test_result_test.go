// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"testing"
)

func TestGetTesthausURL(t *testing.T) {
	t.Parallel()

	ftt.Run("Test get Testhaus URL", t, func(t *ftt.Test) {
		tests := []struct {
			invocationName      string
			gcsURL              string
			wantTesthausPostfix string
		}{
			{
				invocationName:      "invocations/inv-123",
				gcsURL:              "gs://test_bucket/test_prefix",
				wantTesthausPostfix: "invocations/inv-123",
			},
			{
				invocationName:      "invocations/inv-123",
				gcsURL:              "",
				wantTesthausPostfix: "invocations/inv-123",
			},
			{
				invocationName:      "",
				gcsURL:              "gs://test_bucket/test_prefix",
				wantTesthausPostfix: "test_bucket/test_prefix",
			},
			{
				invocationName:      "",
				gcsURL:              "gs://",
				wantTesthausPostfix: "",
			},
			{
				invocationName:      "",
				gcsURL:              "",
				wantTesthausPostfix: "",
			},
		}
		for _, tc := range tests {

			t.Run(fmt.Sprintf("When the invocation is: %q and gcsURL is: %q should get Testhaus URL postfix: %q", tc.invocationName, tc.gcsURL, tc.wantTesthausPostfix), func(t *ftt.Test) {
				wantTesthausURL := fmt.Sprintf("%s%s", TesthausURLPrefix, tc.wantTesthausPostfix)
				gotTesthausURL := GetTesthausURL(tc.invocationName, tc.gcsURL)
				assert.Loosely(t, gotTesthausURL, should.Equal(wantTesthausURL))
			})
		}
	})
}
