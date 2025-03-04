// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCache(t *testing.T) {
	t.Parallel()

	ftt.Run("With temp dir", t, func(t *ftt.Test) {
		tmp, err := os.MkdirTemp("", "gaedeploy_test")
		assert.Loosely(t, err, should.BeNil)
		t.Cleanup(func() { os.RemoveAll(tmp) })

		testTime := testclock.TestRecentTimeLocal.Round(time.Second)
		ctx, tc := testclock.UseTime(context.Background(), testTime)

		cache := Cache{Root: filepath.Join(tmp, "cache")}

		scan := func() []string {
			files, err := os.ReadDir(cache.Root)
			assert.Loosely(t, err, should.BeNil)
			names := make([]string, len(files))
			for i, f := range files {
				names[i] = f.Name()
			}
			return names
		}

		t.Run("WithTarball happy path", func(t *ftt.Test) {
			src := testSrc{
				data: map[string]string{
					"dir/":     "",
					"dir/file": "hi",
				},
			}

			callback := func(path string) error {
				blob, err := os.ReadFile(filepath.Join(path, "dir", "file"))
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, string(blob), should.Match("hi"))
				return nil
			}

			err := cache.WithTarball(ctx, &src, callback)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, src.calls, should.Equal(1))

			// Updated the metadata
			entryDir := filepath.Join(cache.Root, hex.EncodeToString(src.SHA256()))
			m, err := readMetadata(ctx, entryDir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.Created.Equal(testTime), should.BeTrue)
			assert.Loosely(t, m.Touched.Equal(testTime), should.BeTrue)

			tc.Add(time.Minute)

			err = cache.WithTarball(ctx, &src, callback)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, src.calls, should.Equal(1)) // didn't touch the source

			// Updated the metadata
			m, err = readMetadata(ctx, entryDir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, m.Created.Equal(testTime), should.BeTrue)
			assert.Loosely(t, m.Touched.Equal(testTime.Add(time.Minute)), should.BeTrue)
		})

		t.Run("WithTarball wrong hash", func(t *ftt.Test) {
			src := testSrc{
				data: map[string]string{
					"dir/":     "",
					"dir/file": "hi",
				},
				sha256: bytes.Repeat([]byte{0}, 32),
			}
			err := cache.WithTarball(ctx, &src, func(path string) error {
				panic("must not be called")
			})
			assert.Loosely(t, err, should.ErrLike("tarball hash mismatch"))
		})

		t.Run("Trim works", func(t *ftt.Test) {
			var created []string // oldest to newest
			for i := range 3 {
				src := testSrc{
					data: map[string]string{"file": fmt.Sprintf("file %d", i)},
				}
				created = append(created, hex.EncodeToString(src.SHA256()))
				err := cache.WithTarball(ctx, &src, func(path string) error { return nil })
				assert.Loosely(t, err, should.BeNil)
				tc.Add(time.Minute)
			}

			assert.Loosely(t, scan(), should.HaveLength(len(created)))

			// Kick two oldest ones (keep only one newest).
			assert.Loosely(t, cache.Trim(ctx, 1), should.BeNil)

			// Worked!
			assert.Loosely(t, scan(), should.Match([]string{created[2]}))
		})
	})
}

type testSrc struct {
	data  map[string]string
	calls int

	blob   []byte
	sha256 []byte
}

func (src *testSrc) asTarGz() []byte {
	if src.blob != nil {
		return src.blob
	}

	keys := make([]string, 0, len(src.data))
	for k := range src.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf := bytes.Buffer{}
	gz := gzip.NewWriter(&buf)
	tr := tar.NewWriter(gz)

	for _, key := range keys {
		body := src.data[key]
		hdr := &tar.Header{
			Name: key,
			Mode: 0600,
			Size: int64(len(body)),
		}
		if err := tr.WriteHeader(hdr); err != nil {
			panic(err)
		}
		if _, err := tr.Write([]byte(body)); err != nil {
			panic(err)
		}
	}

	if err := tr.Close(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

func (src *testSrc) SHA256() []byte {
	if src.sha256 != nil {
		return src.sha256
	}
	h := sha256.New()
	h.Write(src.asTarGz())
	src.sha256 = h.Sum(nil)
	return src.sha256
}

func (src *testSrc) Open(ctx context.Context, tmp string) (io.ReadCloser, error) {
	src.calls++
	return io.NopCloser(bytes.NewReader(src.asTarGz())), nil
}
