// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package android

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/cros/recovery/internal/execs"
	"infra/cros/recovery/tlw"
)

func TestHasDutBoardExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("hasDutBoardExec", t, func(t *ftt.Test) {
		t.Run("Attached DUT board is present - no error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:              "board",
							Model:              "model",
							SerialNumber:       "serialNumber",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "some name", nil, 0, nil)
			assert.Loosely(t, hasDutBoardExec(ctx, info), should.BeNil)
		})
		t.Run("Missing attached DUT board - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Model:              "model",
							SerialNumber:       "serialNumber",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutBoardExec(ctx, info), should.NotBeNil)
		})
		t.Run("ChromeOs DUT  with board - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Chromeos: &tlw.ChromeOS{
							Board:        "board",
							Model:        "model",
							SerialNumber: "serialNumber",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutBoardExec(ctx, info), should.NotBeNil)
		})
	})
}

func TestHasDutModelExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("hasDutModelExec", t, func(t *ftt.Test) {
		t.Run("Attached DUT model is present - no error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:              "board",
							Model:              "model",
							SerialNumber:       "serialNumber",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutModelExec(ctx, info), should.BeNil)
		})
		t.Run("Missing attached DUT model - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:              "board",
							SerialNumber:       "serialNumber",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "name", nil, 0, nil)
			assert.Loosely(t, hasDutModelExec(ctx, info), should.NotBeNil)
		})
		t.Run("ChromeOs DUT with model - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Chromeos: &tlw.ChromeOS{
							Board:        "board",
							Model:        "model",
							SerialNumber: "serialNumber",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutModelExec(ctx, info), should.NotBeNil)
		})
	})
}

func TestHasDutSerialNumberExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("hasDutSerialNumberExec", t, func(t *ftt.Test) {
		t.Run("Attached DUT serial number is present - no error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:              "board",
							Model:              "model",
							SerialNumber:       "serialNumber",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutSerialNumberExec(ctx, info), should.BeNil)
		})
		t.Run("Missing attached DUT serial number - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:              "board",
							Model:              "model",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutSerialNumberExec(ctx, info), should.NotBeNil)
		})
		t.Run("ChromeOs DUT with serial number - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Chromeos: &tlw.ChromeOS{
							Board:        "board",
							Model:        "model",
							SerialNumber: "serialNumber",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutSerialNumberExec(ctx, info), should.NotBeNil)
		})
	})
}

func TestHasDutAssociatedHostExec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ftt.Run("hasDutAssociatedHostExec", t, func(t *ftt.Test) {
		t.Run("Attached DUT associated hostname is present - no error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:              "board",
							Model:              "model",
							SerialNumber:       "serialNumber",
							AssociatedHostname: "associatedHostname",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutAssociatedHostExec(ctx, info), should.BeNil)
		})
		t.Run("Missing attached DUT associated hostname - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Android: &tlw.Android{
							Board:        "board",
							Model:        "model",
							SerialNumber: "serialNumber",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutAssociatedHostExec(ctx, info), should.NotBeNil)
		})
		t.Run("ChromeOs DUT - returns error", func(t *ftt.Test) {
			info := execs.NewExecInfo(
				&execs.RunArgs{
					DUT: &tlw.Dut{
						Chromeos: &tlw.ChromeOS{
							Board:        "board",
							Model:        "model",
							SerialNumber: "serialNumber",
						},
					},
				}, "", nil, 0, nil)
			assert.Loosely(t, hasDutAssociatedHostExec(ctx, info), should.NotBeNil)
		})
	})
}
