// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"testing"
)

func TestIsInternal(t *testing.T) {
	testCases := []struct {
		name      string
		accountID string
		want      bool
	}{
		{
			name:      "internal",
			accountID: "1",
			want:      true,
		},
		{
			name:      "external",
			accountID: "2",
			want:      false,
		},
		{
			name:      "missing",
			accountID: "",
			want:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isInternal(tc.accountID)
			if got != tc.want {
				t.Errorf("Unexpected. want %v got %v", tc.want, got)
			}
		})
	}
}
