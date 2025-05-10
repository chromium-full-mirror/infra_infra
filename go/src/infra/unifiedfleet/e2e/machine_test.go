// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build linux && (integration || e2e)

package e2e_test

import (
	"context"
	"testing"
)

func TestAddMachine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ufs, err := NewLocalUFSEnv(ctx)
	if err != nil {
		t.Fatalf("Failed to setup local UFS env: %s", err)
	}
	t.Cleanup(func() {
		if err := ufs.Close(); err != nil {
			t.Errorf("failed to cleanup local UFS env: %s", err)
		}
	})

	c, out, err := ufs.Shivas(ctx, []string{"add", "machine", "-name", "machine1", "-zone", "sfo36_browser"}, WithBrowser)
	t.Logf("Add machine: code %d, output: %s", c, out)
	if err != nil {
		t.Errorf("add machine: %s", err)
	}
	c, out, err = ufs.Shivas(ctx, []string{"get", "machine"}, WithBrowser, WithStdin())
	t.Logf("Verify added machine: code %d, output: %s", c, out)
	if err != nil {
		t.Errorf("get machine: %s", err)
	}
}
