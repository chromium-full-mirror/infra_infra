// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/recovery/internal/components"
)

type runnerResponse struct {
	out string
	err error
}

var detectInternalStorageTests = []struct {
	name  string
	out   string
	calls map[string]runnerResponse
}{
	{
		"nothing",
		"",
		nil,
	},
	{
		"mmc",
		"/dev/mmc1",
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_mmc_disks": {out: "mmc1"},
		},
	},
	{
		"nvme",
		"/dev/disk4",
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_nvme_nss": {out: "disk4"},
		},
	},
	{
		"ufs",
		"/dev/disk2",
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_mmc_disks": {},
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_nvme_nss":  {},
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_ufs_disks": {out: "disk2"},
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_ata_disks": {},
		},
	},
	{
		"ata",
		"/dev/disk1",
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_mmc_disks": {},
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_nvme_nss":  {},
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_ufs_disks": {},
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; list_fixed_ata_disks": {out: "disk1"},
		},
	},
}

func TestDetectInternalStorage(t *testing.T) {
	t.Parallel()
	for _, tt := range detectInternalStorageTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			got := DetectInternalStorage(ctx, fakeRunner(tt.calls))
			if diff := cmp.Diff(got, tt.out); diff != "" {
				t.Errorf("TestDetectInternalStorage %q: diff: %v got:%q; expected:%q", tt.name, diff, got, tt.out)
			}
		})
	}
}

var deviceMainStoragePathTests = []struct {
	name      string
	out       string
	expectErr bool
	calls     map[string]runnerResponse
}{
	{
		"nothing",
		"",
		false,
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; get_fixed_dst_drive": {out: ""},
		},
	},
	{
		"found drive",
		"/dev/sdb1",
		false,
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; get_fixed_dst_drive": {out: "/dev/sdb1"},
		},
	},
	{
		"fail with error",
		"",
		true,
		map[string]runnerResponse{
			". /usr/sbin/write_gpt.sh; . /usr/share/misc/chromeos-common.sh; load_base_vars; get_fixed_dst_drive": {
				out: "/dev/sdb1",
				err: fmt.Errorf("fail"),
			},
		},
	},
}

func TestDeviceMainStoragePath(t *testing.T) {
	t.Parallel()
	for _, tt := range deviceMainStoragePathTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			got, err := DeviceMainStoragePath(ctx, fakeRunner(tt.calls))
			if tt.expectErr && err == nil {
				t.Errorf("TestDeviceMainStoragePath %q: expected error but not received it", tt.name)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("TestDeviceMainStoragePath %q: not expected error but got one %s", tt.name, err)
			}
			if diff := cmp.Diff(got, tt.out); diff != "" {
				t.Errorf("TestDeviceMainStoragePath %q: diff: %v got:%q; expected:%q", tt.name, diff, got, tt.out)
			}
		})
	}
}

func fakeRunner(calls map[string]runnerResponse) components.Runner {
	return func(_ context.Context, _ time.Duration, c string, args ...string) (string, error) {
		cmd := c + strings.Join(args, " ")
		if v, ok := calls[cmd]; ok {
			return v.out, v.err
		}
		return "", fmt.Errorf("Not implemented")
	}
}
