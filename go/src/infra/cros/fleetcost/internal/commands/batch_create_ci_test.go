// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"context"
	"strings"
	"testing"
)

func TestUploadRow(t *testing.T) {
	t.Parallel()

	err := uploadRow(context.Background(), nil, map[string]string{
		"primary": "quota-faft-rack-setup",
	})
	switch {
	case err == nil:
		t.Error("err cannot be nil")
	case strings.Contains(err.Error(), "cannot be used in"):
		// do nothing
	default:
		t.Errorf("unexpected error: %s", err.Error())
	}
}
