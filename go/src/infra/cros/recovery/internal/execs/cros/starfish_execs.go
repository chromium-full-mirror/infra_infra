// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components/cros/cellular/starfish"
	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

func init() {
	execs.Register("cros_audit_cellular_starfish", auditStarfishExec)
}

// auditStarfishExec ensures that we are able to probe the SIMs in the starfish slots
// and that every slot in the dut conifg can be found.
func auditStarfishExec(ctx context.Context, info *execs.ExecInfo) error {
	c := info.GetChromeos().GetCellular()
	if c == nil {
		return errors.Reason("audit starfish: cellular data is not present in dut info").Err()
	}

	if c.GetCarrier() == "" {
		c.StarfishState = tlw.Cellular_STARFISH_STATE_NOT_APPLICABLE
		log.Infof(ctx, "audit starfish: carrier name is empty, setting starfish state to NA")
		return nil
	}

	runner := info.DefaultRunner()
	slots, err := starfish.GetOccupiedSlots(ctx, runner)
	if err != nil {
		c.StarfishState = tlw.Cellular_STARFISH_STATE_BROKEN
		return errors.Reason("audit starfish: failed to get occupied starfish slots").Err()
	}

	occupiedSlots := make(map[int]bool)
	for _, slot := range slots {
		// Starfish slots are 0 indexed while SimInfo SlotIds are 1 indexed.
		occupiedSlots[slot+1] = true
	}

	// Verify, that for each SIM info declared in the DUT config, we find a corresponding
	// SIM in the starfish slot. We do not need to check the reverse condition since there
	// may be additional SIMs in the starfish that are not included in the DUT config.
	for _, s := range c.GetSimInfos() {
		if len(s.GetProfileInfos()) > 0 && !occupiedSlots[int(s.GetSlotId())] {
			c.StarfishState = tlw.Cellular_STARFISH_STATE_BROKEN
			return errors.Reason("audit starfish: unable to find SIM in starfish slot %d", s.GetSlotId()).Err()
		}
		occupiedSlots[int(s.GetSlotId())] = false
	}

	c.StarfishState = tlw.Cellular_STARFISH_STATE_WORKING
	return nil
}
