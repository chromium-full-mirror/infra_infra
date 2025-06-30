// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"database/sql"
	"fmt"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

const FIELDS_PER_DEVICE = 5

func updateAndroidDevicesTable(ctx context.Context, tx *sql.Tx, devices []device) error {
	query := fmt.Sprintf(`
		INSERT INTO android_devices
			(id, lab_name, host_group, run_target, state)
		VALUES %s
		ON CONFLICT (id) DO UPDATE SET
			id=EXCLUDED.id,
			lab_name=EXCLUDED.lab_name,
			host_group=EXCLUDED.host_group,
			run_target=EXCLUDED.run_target,
			state=EXCLUDED.state
		`,
		queryutils.ValuesString(len(devices)*FIELDS_PER_DEVICE, FIELDS_PER_DEVICE),
	)

	values := make([]any, len(devices)*FIELDS_PER_DEVICE)
	for i, device := range devices {
		values[i*FIELDS_PER_DEVICE] = device.id
		values[i*FIELDS_PER_DEVICE+1] = device.labName
		values[i*FIELDS_PER_DEVICE+2] = device.hostGroup
		values[i*FIELDS_PER_DEVICE+3] = device.runTarget
		values[i*FIELDS_PER_DEVICE+4] = device.state
	}

	_, err := tx.ExecContext(
		ctx,
		query,
		values...,
	)
	if err != nil {
		return errors.Annotate(err, "failed to update android device").Err()
	}
	return nil
}
