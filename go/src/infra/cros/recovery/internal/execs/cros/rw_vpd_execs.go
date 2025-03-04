// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// RwVPDMap are the default requared values for RW_VPD
var RwVPDMap = map[string]string{
	"should_send_rlz_ping": "0",
	"gbind_attribute":      "=CikKIKxeOtv7AqiCHCDBHkyLN-HF0S7JRcZgsoIRvkPlfMqaEAAaA2V2ZRCX0O3NBg==",
	"ubind_attribute":      "=CikKILiLqJanLAzsXFuVPmfc_aOZxnyNirT9iesdM6kt59x6EAEaA2V2ZRCIkb3GCQ==",
}

const (
	// readRwVPDValuesCmdGlob reads the value of VPD key from RW_VPD partition by name.
	readRwVPDValuesCmdGlob = "vpd -i RW_VPD -g %s"
	//listRwVPDValuesCmd lists RW_VPD values
	listRwVPDValuesCmd = "vpd -i RW_VPD -l"
	// setRwVpdValueCmd sets a RW_VPD values
	setRwVpdValueCmd = "vpd -i RW_VPD -s %s=%s"
)

// areRequiredRWVPDKeysPresentExec confirms that there is no required RW_VPD keys missing on the device.
func areRequiredRWVPDKeysPresentExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	for k := range RwVPDMap {
		cmd := fmt.Sprintf(readRwVPDValuesCmdGlob, k)
		if _, err := r(ctx, time.Minute, cmd); err != nil {
			return errors.Annotate(err, "any rw vpd keys missing").Err()
		}
	}
	log.Infof(ctx, "no rw_vpd values missing")
	return nil
}

// restoreRWVPDKeys restores the values of RW VPD keys from the set of
// known values.
func restoreRWVPDKeysExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	for k, v := range RwVPDMap {
		if out, err := r(ctx, time.Minute, fmt.Sprintf(readRwVPDValuesCmdGlob, k)); err != nil {
			log.Debugf(ctx, "Restore RW VPD Keys: setting value  %s:%s", k, v)
			if _, err := r(ctx, time.Minute, fmt.Sprintf(setRwVpdValueCmd, k, v)); err != nil {
				log.Debugf(ctx, "Restore RW VPD Keys: failed to set value for %s", k)
				return errors.Reason("Restore RW VPD Keys: could not restore the value for key %s", k).Err()
			}
		} else {
			log.Debugf(ctx, "Restore RW VPD Keys: skipping fix for %s:%s", k, out)
		}
	}
	return nil
}

// canListRWVPDKeysExec checks whether any special errors are
// encountered during listing the RW VPD values.
func canListRWVPDKeysExec(ctx context.Context, info *execs.ExecInfo) error {
	r := info.DefaultRunner()
	_, err := r(ctx, time.Minute, listRwVPDValuesCmd)
	// We only care about error codes 11 and 12 following the logic in
	// legacy repair.
	if err != nil {
		errorCode, ok := execs.ErrCodeTag.Value(err)
		log.Debugf(ctx, "Can List RW VPD Keys: code, %v, Type: %T", errorCode, errorCode)
		if !ok {
			return errors.Annotate(err, "can list rw pd keys: cannot find error code.").Err()
		}
		if errorCode == 11 {
			log.Debugf(ctx, "Can List RW VPD Keys: Invalid VPD.")
			return errors.Annotate(err, "can list rw vpd keys").Err()
		} else if errorCode == 12 {
			log.Debugf(ctx, "Can List RW VPD Keys: Error when decoding VPD blob.")
			return errors.Annotate(err, "can list rw vpd keys").Err()
		}
		log.Debugf(ctx, "Not Critical: %s", err)
	}
	return nil
}

func init() {
	execs.Register("cros_are_required_rw_vpd_keys_present", areRequiredRWVPDKeysPresentExec)
	execs.Register("cros_can_list_rw_vpd_keys", canListRWVPDKeysExec)
	execs.Register("cros_restore_rw_vpd_keys", restoreRWVPDKeysExec)
}
