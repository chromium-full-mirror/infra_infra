// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ufs

import (
	"context"
	"net/http"
	"testing"
)

// TestNewClient tests that NewClient responds in appropriate ways
// to ill-formed arguments. Not a deep test.
func TestNewClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hc := http.DefaultClient

	_, err := NewClient(ctx, hc, "")
	if err == nil {
		t.Errorf("expected error to not be nil")
	}
}
