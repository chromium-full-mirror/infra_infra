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

func TestDeleteAndroidDevicesOfHostNotInList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Delete devices", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		assert.Loosely(t, err, should.BeNil)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM android_devices WHERE host = $1 AND id NOT IN ($2, $3)")).WithArgs(
			"host1", "device1", "device2",
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		tx, err := db.BeginTx(ctx, nil)
		assert.Loosely(t, err, should.BeNil)

		devices := []device{
			{id: "device1"},
			{id: "device2"},
		}

		err = deleteAndroidDevicesOfHostNotInList(ctx, tx, "host1", devices)
		assert.Loosely(t, err, should.BeNil)

		err = tx.Commit()
		assert.Loosely(t, err, should.BeNil)

		err = mock.ExpectationsWereMet()
		assert.Loosely(t, err, should.BeNil)
	})
}
