// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dockerfile

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestPins(t *testing.T) {
	t.Parallel()

	pinsYAML := `{"pins": [
		{"image": "library/yyy", "tag": "old", "digest": "sha256:456"},
		{"image": "xxx", "digest": "sha256:123", "comment": "zzz", "freeze": "yyy"},
		{"image": "gcr.io/example/zzz", "tag": "1.2.3", "digest": "sha256:789"}
	]}`

	ftt.Run("Works", t, func(t *ftt.Test) {
		p, err := ReadPins(strings.NewReader(pinsYAML))
		assert.Loosely(t, err, should.BeNil)

		r := p.Resolver()

		d, err := r.ResolveTag("xxx", "")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, d, should.Equal("sha256:123"))

		// The same exact pin.
		d, err = r.ResolveTag("library/xxx", "latest")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, d, should.Equal("sha256:123"))

		// And this one too.
		d, err = r.ResolveTag("docker.io/library/xxx", "latest")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, d, should.Equal("sha256:123"))

		d, err = r.ResolveTag("yyy", "old")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, d, should.Equal("sha256:456"))

		d, err = r.ResolveTag("gcr.io/example/zzz", "1.2.3")
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, d, should.Equal("sha256:789"))

		// Missing image.
		_, err = r.ResolveTag("zzz", "1.2.3")
		assert.Loosely(t, err, should.ErrLike("no such pinned <image>:<tag> combination in pins YAML"))
		assert.Loosely(t, IsMissingPinErr(err), should.Resemble(&Pin{
			Image: "docker.io/library/zzz",
			Tag:   "1.2.3",
		}))

		// Missing tag.
		_, err = r.ResolveTag("yyy", "blah")
		assert.Loosely(t, err, should.ErrLike("no such pinned <image>:<tag> combination in pins YAML"))
		assert.Loosely(t, IsMissingPinErr(err), should.Resemble(&Pin{
			Image: "docker.io/library/yyy",
			Tag:   "blah",
		}))
	})

	ftt.Run("Duplicate pins YAML", t, func(t *ftt.Test) {
		_, err := ReadPins(strings.NewReader(`{"pins": [
			{"image": "library/xxx", "tag": "tag", "digest": "sha256:456"},
			{"image": "library/xxx", "tag": "another", "digest": "sha256:456"},  # OK
			{"image": "xxx", "tag": "tag", "digest": "sha256:456"}               # dup
		]}`))
		assert.Loosely(t, err, should.ErrLike(`pin #3: duplicate entry for "docker.io/library/xxx:tag"`))
	})

	ftt.Run("Incomplete pins", t, func(t *ftt.Test) {
		_, err := ReadPins(strings.NewReader(`{"pins": [
      {"digest": "sha256:123"}
    ]}`))
		assert.Loosely(t, err, should.ErrLike("pin #1: 'image' field is required"))

		_, err = ReadPins(strings.NewReader(`{"pins": [
      {"image": "xxx"}
    ]}`))
		assert.Loosely(t, err, should.ErrLike("pin #1: 'digest' field is required"))
	})

	ftt.Run("Empty", t, func(t *ftt.Test) {
		r := (&Pins{}).Resolver()
		_, err := r.ResolveTag("img", "tag")
		assert.Loosely(t, err, should.ErrLike("not using pins YAML, the Dockerfile must use @<digest> refs"))
	})

	ftt.Run("WritePins", t, func(t *ftt.Test) {
		p, err := ReadPins(strings.NewReader(pinsYAML))
		assert.Loosely(t, err, should.BeNil)

		out := bytes.Buffer{}
		assert.Loosely(t, WritePins(&out, p), should.BeNil)
		assert.Loosely(t, out.String(), should.Equal(`# Managed by cloudbuildhelper.
#
# All comments or unrecognized fields will be overwritten. To comment an entry
# use "comment" field.
#
# To update digests of all entries:
#   $ cloudbuildhelper pins-update <path-to-this-file>
#
# To add an entry (or update an existing one):
#   $ cloudbuildhelper pins-add <path-to-this-file> <image>[:<tag>]
#
# To remove an entry just delete it from the file.
#
# To prevent an entry from being updated by pins-update, add "freeze" field with
# an explanation why it is frozen.

pins:
- comment: zzz
  image: docker.io/library/xxx
  tag: latest
  digest: sha256:123
  freeze: yyy
- image: docker.io/library/yyy
  tag: old
  digest: sha256:456
- image: gcr.io/example/zzz
  tag: 1.2.3
  digest: sha256:789
`))
	})

	ftt.Run("Add", t, func(t *ftt.Test) {
		p := Pins{}

		// Adds new, normalizing it.
		assert.Loosely(t, p.Add(Pin{Image: "xxx", Digest: "yyy"}), should.BeNil)
		assert.Loosely(t, p.Pins, should.Resemble([]Pin{
			{Image: "docker.io/library/xxx", Tag: "latest", Digest: "yyy"},
		}))

		// Overwrites existing.
		assert.Loosely(t, p.Add(Pin{Image: "library/xxx", Digest: "zzz"}), should.BeNil)
		assert.Loosely(t, p.Pins, should.Resemble([]Pin{
			{Image: "docker.io/library/xxx", Tag: "latest", Digest: "zzz"},
		}))

		// Handle bad pins.
		assert.Loosely(t, p.Add(Pin{}), should.ErrLike(`'image' field is required`))
	})

	ftt.Run("Visit success", t, func(t *ftt.Test) {
		p := Pins{Pins: []Pin{
			{Image: "example.com/repo/img1", Tag: "t1", Digest: "d1"},
			{Image: "example.com/repo/img1", Tag: "t2", Digest: "d2"},
			{Image: "example.com/repo/img2", Tag: "t3", Digest: "d3"},
			{Image: "example.com/repo/img2", Tag: "t4", Digest: "d4"},
		}}

		err := p.Visit(func(p *Pin) error {
			p.Digest += "_new"
			return nil
		})
		assert.Loosely(t, err, should.BeNil)

		assert.Loosely(t, p.Pins, should.Resemble([]Pin{
			{Image: "example.com/repo/img1", Tag: "t1", Digest: "d1_new"},
			{Image: "example.com/repo/img1", Tag: "t2", Digest: "d2_new"},
			{Image: "example.com/repo/img2", Tag: "t3", Digest: "d3_new"},
			{Image: "example.com/repo/img2", Tag: "t4", Digest: "d4_new"},
		}))
	})

	ftt.Run("Visit failure", t, func(t *ftt.Test) {
		p := Pins{Pins: []Pin{
			{Image: "example.com/repo/img1", Tag: "t1", Digest: "d1"},
			{Image: "example.com/repo/img1", Tag: "t2", Digest: "d2"},
			{Image: "example.com/repo/img2", Tag: "t3", Digest: "d3"},
			{Image: "example.com/repo/img2", Tag: "t4", Digest: "d4"},
		}}

		err := p.Visit(func(p *Pin) error {
			p.Digest += "_new"
			if p.Image == "example.com/repo/img2" {
				return errors.New("blarg")
			}
			return nil
		})
		assert.Loosely(t, err, should.ErrLike(`blarg (and 1 other error)`))

		// Updated only img1 ones.
		assert.Loosely(t, p.Pins, should.Resemble([]Pin{
			{Image: "example.com/repo/img1", Tag: "t1", Digest: "d1_new"},
			{Image: "example.com/repo/img1", Tag: "t2", Digest: "d2_new"},
			{Image: "example.com/repo/img2", Tag: "t3", Digest: "d3"},
			{Image: "example.com/repo/img2", Tag: "t4", Digest: "d4"},
		}))
	})
}
