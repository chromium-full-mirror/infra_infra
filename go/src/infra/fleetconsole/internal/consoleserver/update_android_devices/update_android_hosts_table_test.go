// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateandroiddevices

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// TODO b/425632753: Turn this in a proper integration test
// Instead of testing the exact "insert" queries being run we should
// Inspect the state of the db after the execution and checking
// That the rows we expect to find are really there
func TestUpdateAndroidHostsTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Insert new host", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)
		defer db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO android_hosts
				(hostname, host_group, state)
			VALUES ($1, $2, $3)
			ON CONFLICT (hostname) DO UPDATE SET
				hostname=EXCLUDED.hostname,
				host_group=EXCLUDED.host_group,
				state=EXCLUDED.state
		`)).WithArgs("host1", "group1", "state1").WillReturnResult(sqlmock.NewResult(1, 1))

		tx, err := db.BeginTx(ctx, nil)
		assert.Loosely(t, err, should.BeNil)

		mockHost := host{
			hostname:   "host1",
			host_group: "group1",
			state:      "state1",
		}
		err = updateAndroidHostsTable(ctx, tx, mockHost)
		assert.Loosely(t, err, should.BeNil)

		err = mock.ExpectationsWereMet()
		assert.Loosely(t, err, should.BeNil)
	})
}
