// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"database/sql"

	"go.chromium.org/luci/common/errors"
)

func updateAndroidHostsTable(ctx context.Context, tx *sql.Tx, host host) error {
	query := `
		INSERT INTO android_hosts
			(hostname, host_group, state)
		VALUES ($1, $2, $3)
		ON CONFLICT (hostname) DO UPDATE SET
			hostname=EXCLUDED.hostname,
			host_group=EXCLUDED.host_group,
			state=EXCLUDED.state
		`
	_, err := tx.ExecContext(
		ctx,
		query,
		host.hostname, host.host_group, host.state,
	)
	if err != nil {
		return errors.Annotate(err, "failed to update android hosts").Err()
	}
	return nil
}
