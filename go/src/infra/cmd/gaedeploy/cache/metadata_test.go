// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := time.Now().Round(time.Millisecond) // convert to wall clock

	Convey("modifyMetadata Works", t, func() {
		tmp, err := os.MkdirTemp("", "gaedeploy_test")
		So(err, ShouldBeNil)
		defer os.RemoveAll(tmp)

		// Creates new file.
		err = modifyMetadata(ctx, tmp, func(m *cacheMetadata) {
			So(m.Created.IsZero(), ShouldBeTrue)
			So(m.Touched.IsZero(), ShouldBeTrue)
			m.Created = ts
			m.Touched = ts
		})
		So(err, ShouldBeNil)

		// Reads the existing file, writes back modifications.
		err = modifyMetadata(ctx, tmp, func(m *cacheMetadata) {
			So(m.Created.Equal(ts), ShouldBeTrue)
			So(m.Touched.Equal(ts), ShouldBeTrue)
			m.Touched = ts.Add(10 * time.Second)
		})
		So(err, ShouldBeNil)

		// Verify it is updated.
		m, err := readMetadata(ctx, tmp)
		So(err, ShouldBeNil)
		So(m.Created.Equal(ts), ShouldBeTrue)
		So(m.Touched.Equal(ts.Add(10*time.Second)), ShouldBeTrue)
	})
}
