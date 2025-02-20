// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package topology

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// IsItemGood checks whether a ServoTopologyItem has
// minimum required data.
func IsItemGood(ctx context.Context, c *tlw.ServoTopologyItem) bool {
	return c != nil && c.Serial != "" && c.Type != "" && c.UsbHubPort != ""
}

// VerifyServoTopologyItems verifies whether the servo topology items
// are all valid and ready for use.
func VerifyServoTopologyItems(ctx context.Context, devices []*tlw.ServoTopologyItem) error {
	if devices == nil {
		log.Debugf(ctx, "Verify Servo Topology Items: the servo topology items slice is nil.")
		return errors.Reason("verify servo topology items: the servo topology items slice is nil.").Err()
	}
	for _, d := range devices {
		if d == nil {
			log.Debugf(ctx, "Verify Servo Topology Items: a single servo topology item is nil")
			return errors.Reason("verify servo topology items: a single servo topology item is nil").Err()
		}
		if !IsItemGood(ctx, d) {
			log.Debugf(ctx, "Verify Servo Topology Items: a servo topology item %q has missing components.", d)
			return errors.Reason("verify servo topology items: a servo topology item %q has missing components.", d).Err()
		}
	}
	log.Debugf(ctx, "Verify Servo Topology Items: the topology is fine, count of topology items is :%d", len(devices))
	return nil
}
