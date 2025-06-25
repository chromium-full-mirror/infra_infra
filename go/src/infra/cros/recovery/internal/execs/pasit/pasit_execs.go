// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pasit

import (
	"context"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/lab/api/passport"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cft"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// auditSwitchesExec checks that the switches described in the topology match those reported by PassPort.
func auditSwitchesExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.PassportSwitchClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "audit switches: get switch client").Err()
	}

	foundSwitches := make(map[string]bool)
	resp, err := client.GetSwitches(ctx, &passport.GetSwitchesRequest{})
	if err != nil {
		return errors.Annotate(err, "audit switches: call GetSwitches").Err()
	}

	for _, s := range resp.Switches {
		id := strings.ToUpper(s.GetId())
		log.Debugf(ctx, "Found Switch: %q", id)
		foundSwitches[id] = true
	}

	var missingSwitches []string
	for _, d := range info.GetChromeos().GetPasit().GetDevices() {
		if d.GetType() != tlw.Pasit_Device_SWITCH_FIXTURE {
			continue
		}
		id := strings.ToUpper(d.GetId())
		if !foundSwitches[id] {
			log.Debugf(ctx, "Missing Switch: %q", id)
			missingSwitches = append(missingSwitches, id)
		}
		delete(foundSwitches, id)
	}

	for id := range foundSwitches {
		log.Debugf(ctx, "Found extra switch: %q", id)
	}

	if len(missingSwitches) > 0 {
		return errors.Reason("audit switches: Missing switch with IDs %v", missingSwitches).Err()
	}

	return nil
}

// auditUSBTestersExec checks that the USB testers described in the topology match those reported by PassPort.
func auditUSBTestersExec(ctx context.Context, info *execs.ExecInfo) error {
	client, err := cft.PassportUSBTesterClientFromScope(ctx, info.GetDut())
	if err != nil {
		return errors.Annotate(err, "audit usb testers: get usb tester client").Err()
	}

	foundTesters := make(map[string]bool)
	resp, err := client.GetTesters(ctx, &passport.GetTestersRequest{})
	if err != nil {
		return errors.Annotate(err, "audit usb testers: call GetTesters").Err()
	}

	for _, s := range resp.GetTesters() {
		id := strings.ToUpper(s.GetId())
		log.Debugf(ctx, "Found usb testers: %q", id)
		foundTesters[id] = true
	}

	var missingTesters []string
	for _, d := range info.GetChromeos().GetPasit().GetDevices() {
		if d.GetType() != tlw.Pasit_Device_USB_TESTER {
			continue
		}
		id := strings.ToUpper(d.GetId())
		if !foundTesters[id] {
			log.Debugf(ctx, "Missing usb testers: %q in inventory records but found in testbed. Please update inventory records!", id)
			missingTesters = append(missingTesters, id)
		}
		delete(foundTesters, id)
	}

	for id := range foundTesters {
		log.Debugf(ctx, "Found a new USB tester: %q in  testbed. Please update inventory records!", id)
	}

	if len(missingTesters) > 0 {
		return errors.Reason("audit usb testers: Missing tester with IDs %v", missingTesters).Err()
	}

	return nil
}

func init() {
	execs.Register("pasit_audit_switches", auditSwitchesExec)
	execs.Register("pasit_audit_usb_testers", auditUSBTestersExec)
}
