// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build linux && (integration || e2e)

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestShivasHostBasic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ufs := setupLocalUFS(ctx, t)

	cmd := "add machine-prototype -f /dev/stdin"
	rc, _, err := ufs.ShivasStdin(ctx, strings.Split(cmd, " "), `{"name":"browser:prototype1"}`, WithBrowser)
	assert.Loosely(t, err, should.BeNil)
	assert.Loosely(t, rc, should.BeZero)

	type runAndCheck struct {
		command      string
		outputRegexp string
	}
	// TODO(guocb) add delete command and check the result. Currently the Shivas()
	// doesn't support stdin well.
	cases := []struct {
		name   string
		checks []runAndCheck
	}{
		{
			"basic operation on a machine lse",
			[]runAndCheck{
				{"add machine -name machine1 -zone sfo36_browser", ""},
				{"get mahcine machine1", "Machine Name.*\nmachine1"},
				{"add host -name lse1 -machine machine1 -prototype browser:prototype1", ""},
				{"get host lse1", "Host .*\nlse1"},
				{"get host -json lse1", `"name": "lse1"`},
				{"update host -name lse1 -state serving", ""},
				{"internal-print-bot-info lse1", `\{"Dimensions":\{"dut_state":\["ready"\],"ufs_state":\["ready"\],"ufs_zone":\["ZONE_SFO36_BROWSER"\]\},"State":null\}`},
				{"internal-sync-dut-info -download-dut-info lse1", `\{"Dimensions":\{"dut_state":\["ready"\],"ufs_state":\["ready"\],"ufs_zone":\["ZONE_SFO36_BROWSER"\]\},"State":null\}`},
				{"internal-sync-dut-info -upload-health-status unhealthy -download-dut-info lse1", `"dut_state":.*"needs_manual_repair"`},
				{"internal-sync-dut-info -upload-health-status healthy -download-dut-info lse1", `"dut_state":.*"ready"`},
				// Test forced state updates.
				{"internal-sync-dut-info -upload-health-status unhealthy -download-dut-info lse1", `"dut_state":.*"needs_manual_repair"`},
				{"update host -name lse1 -state serving -force-state", `"resourceState":.*"STATE_SERVING"`},
				{"get host lse1", "Host .*\nlse1.*serving"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var rc int
			var out string
			var err error
			for _, chk := range tc.checks {
				rc, out, err = ufs.Shivas(ctx, strings.Split(chk.command, " "), WithBrowser)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, rc, should.BeZero)
				if chk.outputRegexp != "" {
					assert.Loosely(t, out, should.MatchRegexp(chk.outputRegexp))
				}
			}
		})
	}
}

func setupLocalUFS(ctx context.Context, t *testing.T) *LocalUFSEnv {
	t.Helper()
	ufs, err := NewLocalUFSEnv(ctx)
	if err != nil {
		t.Fatalf("Failed to setup local UFS env: %s", err)
	}
	t.Cleanup(func() {
		if err := ufs.Close(); err != nil {
			t.Errorf("failed to cleanup local UFS env: %s", err)
		}
	})
	return ufs
}
