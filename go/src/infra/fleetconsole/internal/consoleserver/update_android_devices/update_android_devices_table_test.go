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
func TestUpdateAndroidDevicesTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Insert new devices", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO android_devices
				(id, lab_name, host_group, run_target, state)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				id=EXCLUDED.id,
				lab_name=EXCLUDED.lab_name,
				host_group=EXCLUDED.host_group,
				run_target=EXCLUDED.run_target,
				state=EXCLUDED.state
		`)).WithArgs("device1", "lab1", "hostgroup1", "runtarget1", "state1").WillReturnResult(sqlmock.NewResult(0, 1))

		tx, err := db.BeginTx(ctx, nil)
		assert.Loosely(t, err, should.BeNil)
		mockDevices := []device{
			{
				id:        "device1",
				labName:   "lab1",
				hostGroup: "hostgroup1",
				runTarget: "runtarget1",
				state:     "state1",
			},
		}
		err = updateAndroidDevicesTable(ctx, tx, mockDevices)
		assert.Loosely(t, err, should.BeNil)

		err = mock.ExpectationsWereMet()
		assert.Loosely(t, err, should.BeNil)
	})
}
