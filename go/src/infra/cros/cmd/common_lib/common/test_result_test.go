// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetTesthausURL(t *testing.T) {
	t.Parallel()

	Convey("Test get Testhaus URL", t, func() {
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
		}
		for _, tc := range tests {

			Convey(fmt.Sprintf("When the invocation is: %q and gcsURL is: %q should get Testhaus URL postfix: %q", tc.invocationName, tc.gcsURL, tc.wantTesthausPostfix), func() {
				wantTesthausURL := fmt.Sprintf("%s%s", TesthausURLPrefix, tc.wantTesthausPostfix)
				gotTesthausURL := GetTesthausURL(tc.invocationName, tc.gcsURL)
				So(gotTesthausURL, ShouldEqual, wantTesthausURL)
			})
		}
	})
}
