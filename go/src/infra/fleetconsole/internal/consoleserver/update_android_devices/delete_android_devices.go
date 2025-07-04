// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"database/sql"
	"fmt"

	"go.chromium.org/infra/fleetconsole/internal/database/queryutils"
)

func deleteAndroidDevicesOfHostNotInList(ctx context.Context, tx *sql.Tx, hostname string, devices []device) error {
	if len(devices) == 0 {
		return nil
	}

	q := fmt.Sprintf("DELETE FROM android_devices WHERE host = $1 AND id NOT IN %s", queryutils.ValuesStringWithOffset(len(devices), 0, 1))

	args := make([]any, len(devices)+1)
	args[0] = hostname
	for i, d := range devices {
		args[i+1] = d.id
	}

	_, err := tx.ExecContext(ctx, q, args...)
	return err
}
