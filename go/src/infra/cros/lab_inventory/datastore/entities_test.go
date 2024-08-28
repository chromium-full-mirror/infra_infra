// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package datastore

import (
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
)

func TestGetLastScannedTime(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")

	ftt.Run("Get last scanned time from datastore", t, func(t *ftt.Test) {
		t.Run("Get existing metadata entity from datastore", func(t *ftt.Test) {
			lastScannedTime := time.Date(2020, 01, 01, 12, 34, 56, 0, time.UTC)
			e := &MRMetadataEntity{
				ID:          MRLastScannedID,
				LastScanned: lastScannedTime,
			}
			err := datastore.Put(ctx, e)
			assert.Loosely(t, err, should.BeNil)

			res, err := GetLastScannedTime(ctx)
			assert.Loosely(t, res.LastScanned, should.Equal(e.LastScanned))
			assert.Loosely(t, err, should.BeNil)

			// Clean up test
			err = datastore.Delete(ctx, e)
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("No metadata entity exists in datastore", func(t *ftt.Test) {
			_, err := GetLastScannedTime(ctx)
			assert.Loosely(t, err, should.NotBeNil)
			assert.Loosely(t, err.Error(), should.ContainSubstring("datastore: no such entity"))
		})
	})
}

func TestSaveLastScannedTime(t *testing.T) {
	t.Parallel()
	ctx := gaetesting.TestingContextWithAppID("go-test")

	ftt.Run("Save last scanned time to datastore", t, func(t *ftt.Test) {
		t.Run("Save new metadata entity", func(t *ftt.Test) {
			lastScannedTime := time.Date(2020, 01, 01, 12, 34, 56, 0, time.UTC)
			err := SaveLastScannedTime(ctx, lastScannedTime)
			assert.Loosely(t, err, should.BeNil)

			res, err := GetLastScannedTime(ctx)
			assert.Loosely(t, res.LastScanned, should.Equal(lastScannedTime))
			assert.Loosely(t, err, should.BeNil)
		})
		t.Run("Update metadata entity", func(t *ftt.Test) {
			oldScannedTime := time.Date(2020, 01, 01, 12, 34, 56, 0, time.UTC)
			res, err := GetLastScannedTime(ctx)
			assert.Loosely(t, res.LastScanned, should.Equal(oldScannedTime))
			assert.Loosely(t, err, should.BeNil)

			newScannedTime := time.Date(2020, 01, 01, 01, 00, 00, 0, time.UTC)
			err = SaveLastScannedTime(ctx, newScannedTime)
			assert.Loosely(t, err, should.BeNil)

			res, err = GetLastScannedTime(ctx)
			assert.Loosely(t, res.LastScanned, should.Equal(newScannedTime))
			assert.Loosely(t, err, should.BeNil)
		})
	})
}
