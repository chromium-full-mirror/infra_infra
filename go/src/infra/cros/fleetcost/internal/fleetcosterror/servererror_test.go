// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fleetcosterror_test

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/common/testing/typed"

	"go.chromium.org/infra/cros/fleetcost/internal/fleetcosterror"
)

func TestWithDefaultCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		code    codes.Code
		in      error
		outCode codes.Code
	}{
		{
			name:    "nil",
			code:    codes.Unimplemented,
			in:      nil,
			outCode: codes.OK,
		},
		{
			name:    "error with no status",
			code:    codes.Unimplemented,
			in:      errors.New("whoa"),
			outCode: codes.Unimplemented,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := tt.outCode
			actual := status.Code(fleetcosterror.WithDefaultCode(tt.code, tt.in))
			if diff := typed.Got(actual).Want(expected).Diff(); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
