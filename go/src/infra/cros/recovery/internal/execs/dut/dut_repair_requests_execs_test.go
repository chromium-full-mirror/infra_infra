// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"testing"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/tlw"
)

var resetRepairRequestsExecCases = []struct {
	name     string
	requests []tlw.RepairRequest
}{
	{
		"empty",
		nil,
	},
	{
		"unknow",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN},
	},
	{
		"provision",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_PROVISION},
	},
	{
		"multiple",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_PROVISION, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
	},
}

func TestResetRepairRequestsExec(t *testing.T) {
	t.Parallel()
	for _, c := range resetRepairRequestsExecCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			ctx := context.Background()
			dut := &tlw.Dut{RepairRequests: c.requests}
			info := execs.NewExecInfo(&execs.RunArgs{DUT: dut}, "", nil, 0, nil)
			err := resetRepairRequestsExec(ctx, info)
			if err != nil {
				t.Errorf("%q -> unexpected error: %v", cs.name, err)
			}
			if len(dut.RepairRequests) > 0 {
				t.Errorf("%q -> unexpected repair requets after run", cs.name)
			}
		})
	}
}

var removeRepairRequestsExecCases = []struct {
	name       string
	actionArgs string
	requests   []tlw.RepairRequest
	expected   []tlw.RepairRequest
}{
	{
		"empty",
		"",
		nil,
		nil,
	},
	{
		"empty",
		"",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_PROVISION, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_PROVISION, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
	},
	{
		"unknow",
		"requests:unknow",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN},
	},
	{
		"exist provision",
		"requests:provision",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_PROVISION, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
	},
	{
		"not exist provision",
		"requests:provision",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY},
	},
}

func TestRemoveRepairRequestsExec(t *testing.T) {
	t.Parallel()
	for _, c := range removeRepairRequestsExecCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			ctx := context.Background()
			dut := &tlw.Dut{RepairRequests: c.requests}
			info := execs.NewExecInfo(&execs.RunArgs{DUT: dut}, "", []string{c.actionArgs}, 0, nil)
			err := removeRepairRequestsExec(ctx, info)
			if err != nil {
				t.Errorf("%q -> unexpected error: %v", cs.name, err)
			}
			if len(c.expected) > 0 {
				if len(dut.RepairRequests) != len(c.expected) {
					t.Errorf("%q -> mismatch collection size, expected %d but got %d", cs.name, len(c.expected), len(dut.RepairRequests))
				}
				for i, e := range c.expected {
					got := dut.RepairRequests[i]
					if e != got {
						t.Errorf("%q -> mismatch values, expected: %q, got %q", cs.name, e, got)
					}
				}
			} else {
				if len(dut.RepairRequests) > 0 {
					t.Errorf("%q -> unexpected collection, got %v", cs.name, dut.RepairRequests)
				}
			}
		})
	}
}

var addRepairRequestsExecCases = []struct {
	name       string
	actionArgs string
	requests   []tlw.RepairRequest
	expected   []tlw.RepairRequest
	fail       bool
}{
	{
		"empty",
		"",
		nil,
		nil,
		false,
	},
	{
		"unknow",
		"requests:unknow",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_UNKNOWN},
		true,
	},
	{
		"exist provision",
		"requests:provision,",
		[]tlw.RepairRequest{},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_PROVISION},
		false,
	},
	{
		"add only valid request",
		"requests:provision,bad,update_usbkey_image",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE, tlw.RepairRequest_REPAIR_REQUEST_PROVISION},
		true,
	},
	{
		"add only valid request",
		"requests:provision,update_usbkey_image",
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE},
		[]tlw.RepairRequest{tlw.RepairRequest_REPAIR_REQUEST_REIMAGE_BY_USBKEY, tlw.RepairRequest_REPAIR_REQUEST_UPDATE_USBKEY_IMAGE, tlw.RepairRequest_REPAIR_REQUEST_PROVISION},
		false,
	},
}

func TestAddRepairRequestsExec(t *testing.T) {
	t.Parallel()
	for _, c := range addRepairRequestsExecCases {
		cs := c
		t.Run(cs.name, func(t *testing.T) {
			ctx := context.Background()
			dut := &tlw.Dut{RepairRequests: c.requests}
			info := execs.NewExecInfo(&execs.RunArgs{DUT: dut}, "", []string{c.actionArgs}, 0, nil)
			err := addRepairRequestsExec(ctx, info)
			if c.fail {
				if err == nil {
					t.Errorf("%q -> expected to fail but not: %v", cs.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("%q -> unexpected error: %v", cs.name, err)
				}
				if len(c.expected) > 0 {
					if len(dut.RepairRequests) != len(c.expected) {
						t.Errorf("%q -> mismatch collection size, expected %d but got %d", cs.name, len(c.expected), len(dut.RepairRequests))
					}
					for i, e := range c.expected {
						got := dut.RepairRequests[i]
						if e != got {
							t.Errorf("%q -> mismatch values, expected: %q, got %q", cs.name, e, got)
						}
					}
				} else {
					if len(dut.RepairRequests) > 0 {
						t.Errorf("%q -> unexpected collection, got %v", cs.name, dut.RepairRequests)
					}
				}
			}
		})
	}
}
