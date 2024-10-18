// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cipd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	cipdpkg "go.chromium.org/luci/cipd/client/cipd/pkg"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestListFiles(t *testing.T) {
	ftt.Run("In a temporary directory", t, func(t *ftt.Test) {
		path, err := ioutil.TempDir("", "cipd-test")
		assert.Loosely(t, err, should.BeNil)
		defer os.RemoveAll(path)

		t.Run("finds a file called CIPD_VERSION.json", func(t *ftt.Test) {
			err := ioutil.WriteFile(filepath.Join(path, "CIPD_VERSION.json"), []byte{}, 0644)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, listCIPDVersionFiles(path), should.Match([]string{
				filepath.Join(path, "CIPD_VERSION.json"),
			}))
		})

		t.Run("finds a file called foo.cipd_version", func(t *ftt.Test) {
			err := ioutil.WriteFile(filepath.Join(path, "foo.cipd_version"), []byte{}, 0644)
			assert.Loosely(t, err, should.BeNil)

			assert.Loosely(t, listCIPDVersionFiles(path), should.Match([]string{
				filepath.Join(path, "foo.cipd_version"),
			}))
		})

		t.Run("reads a file", func(t *ftt.Test) {
			err := ioutil.WriteFile(filepath.Join(path, "foo.cipd_version"), []byte(`
        {
          "package_name": "Hello",
          "instance_id": "World"
        }
      `), 0644)
			assert.Loosely(t, err, should.BeNil)

			f, err := readCIPDVersionFile(filepath.Join(path, "foo.cipd_version"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, f, should.Match(cipdpkg.VersionFile{
				PackageName: "Hello",
				InstanceID:  "World",
			}))
		})

		t.Run("file doesn't exist", func(t *ftt.Test) {
			f, err := readCIPDVersionFile(filepath.Join(path, "does not exist"))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, f, should.Match(cipdpkg.VersionFile{}))
		})
	})
}
