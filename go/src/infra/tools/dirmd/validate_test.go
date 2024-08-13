// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"os"
	"path/filepath"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestValidateFile(t *testing.T) {
	t.Parallel()

	ftt.Run(`ValidateFile`, t, func(t *ftt.Test) {
		suite := func(path string, valid bool) {
			err := filepath.Walk(path, func(fullName string, info os.FileInfo, err error) error {
				switch {
				case err != nil:
					return err
				case info.IsDir():
					return nil
				}

				t.Run(fullName, func(t *ftt.Test) {
					err := ValidateFile(fullName)
					if valid {
						assert.Loosely(t, err, should.BeNil)
					} else {
						assert.Loosely(t, err, should.NotBeNil)
					}
				})
				return nil
			})
			assert.Loosely(t, err, should.BeNil)
		}

		suite("testdata/validation/valid", true)
		suite("testdata/validation/invalid", false)
	})
}
