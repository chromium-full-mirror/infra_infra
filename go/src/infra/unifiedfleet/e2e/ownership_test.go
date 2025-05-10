// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build linux && (integration || e2e)

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOwnership(t *testing.T) {
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
	cmd := "add machine-prototype -f /dev/stdin"
	c, out, err := ufs.ShivasStdin(ctx, strings.Split(cmd, " "), `{"name":"browser:prototype1"}`, WithBrowser)
	t.Logf("%q: code %d, output: %s", cmd, c, out)
	if err != nil {
		t.Errorf("prototype: %s", err)
	}

	cmds := []string{
		"add machine -name machine1 -zone sfo36_browser",
		"add host -name build1-h9 -machine machine1 -prototype browser:prototype1",
		"add adm -name adm1 -zone sfo36_browser -serial 123456 -man Google -devicetype android_phone -build-target unknown -model unknown",
		"add adh -name build1-h9--device1 -machine adm1 -associated-hostname build1-h9",
		"admin cron ufs.sync_bot_config.sync",
	}
	for _, cmd := range cmds {
		c, out, err := ufs.Shivas(ctx, strings.Split(cmd, " "), WithBrowser)
		t.Logf("%s: code %d, output: %s", cmd, c, out)
		if err != nil {
			t.Errorf("%q: %s", cmd, err)
		}
	}
	time.Sleep(5 * time.Second)
	cmd = "get host -json build1-h9--device1"
	c, out, err = ufs.Shivas(ctx, strings.Split(cmd, " "), WithBrowser)
	t.Logf("%q: code %d, output: %s", cmd, c, out)
	if err != nil {
		t.Errorf("prototype: %s", err)
	}
}
