// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	dirmdpb "infra/tools/dirmd/proto"
)

// TODO(crbug.com/1505875) - Remove this once migration is complete.
func TestHandleMetadata(t *testing.T) {
	t.Parallel()

	// The code will translate the .textproto mapping into a map with lower-cased
	// keys so that the checks remain case insensitive.
	cm := map[string]int64{
		"test>component": 12345,
	}
	dir := "/some/path/to/dir/"

	ftt.Run(`Invalid`, t, func(t *ftt.Test) {
		t.Run(`Monorail Nil`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				TeamEmail: "team@sample.com",
			}
			md, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, md, should.BeNil)
			assert.Loosely(t, err, should.ErrLike(MonorailMissingError))
		})

		t.Run(`Monorail Component Nil`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project: "chromium",
				},
			}
			md, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, md, should.BeNil)
			assert.Loosely(t, err, should.ErrLike(MonorailMissingError))
		})
		t.Run(`Missing Component`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "Random>Component",
				},
			}
			md, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, md, should.BeNil)
			assert.Loosely(t, err, should.ErrLike("Random>Component is missing from the provided mapping"))
		})
	})

	ftt.Run(`Valid`, t, func(t *ftt.Test) {
		t.Run(`Buganizer Defined`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Buganizer: &dirmdpb.Buganizer{
					ComponentId: 123,
				},
			}
			newMd, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newMd, should.Resemble(md))
		})
		t.Run(`Buganizer Public Defined`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				BuganizerPublic: &dirmdpb.Buganizer{
					ComponentId: 123,
				},
			}
			newMd, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, newMd, should.Resemble(md))
		})
		t.Run(`1:1 mapping`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "test>component",
				},
			}
			newMd, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, err, should.BeNil)

			expected := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "test>component",
				},
				Buganizer: &dirmdpb.Buganizer{
					ComponentId: 12345,
				},
			}
			assert.Loosely(t, newMd, should.Resemble(expected))
		})
		t.Run(`Case insensitive`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "tEsT>CompOnent",
				},
			}
			newMd, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, err, should.BeNil)

			expected := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "tEsT>CompOnent",
				},
				Buganizer: &dirmdpb.Buganizer{
					ComponentId: 12345,
				},
			}
			assert.Loosely(t, newMd, should.Resemble(expected))
		})
		t.Run(`Non Chromium Monorail Project`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "v8",
					Component: "Random>Component",
				},
			}
			md, err := HandleMetadata(md, cm, dir)
			assert.Loosely(t, err, should.BeNil)

			// Should remain unchanged.
			assert.Loosely(t, md, should.Resemble(md))
		})
	})
}

func TestCanSkipMixin(t *testing.T) {
	t.Parallel()

	ftt.Run(`Skip`, t, func(t *ftt.Test) {
		t.Run(`Buganizer Present`, func(t *ftt.Test) {
			mixin := &dirmdpb.Metadata{
				Buganizer: &dirmdpb.Buganizer{
					ComponentId: 12345,
				},
			}
			assert.Loosely(t, canSkipMixin(mixin), should.BeTrue)
		})
		t.Run(`Buganizer & Monorail Present`, func(t *ftt.Test) {
			mixin := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "tEsT>CompOnent",
				},
				Buganizer: &dirmdpb.Buganizer{
					ComponentId: 12345,
				},
			}
			assert.Loosely(t, canSkipMixin(mixin), should.BeTrue)
		})
		t.Run(`No Monorail`, func(t *ftt.Test) {
			mixin := &dirmdpb.Metadata{}
			assert.Loosely(t, canSkipMixin(mixin), should.BeTrue)
		})
		t.Run(`Non Chromium`, func(t *ftt.Test) {
			mixin := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "v8",
					Component: "tEsT>CompOnent",
				},
			}
			assert.Loosely(t, canSkipMixin(mixin), should.BeTrue)
		})
	})

	ftt.Run(`No Skip`, t, func(t *ftt.Test) {
		t.Run(`Monorail Only`, func(t *ftt.Test) {
			mixin := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "tEsT>CompOnent",
				},
			}
			assert.Loosely(t, canSkipMixin(mixin), should.BeFalse)
		})
	})

	cm := map[string]int64{
		"test>component": 12345,
	}

	ftt.Run(`Handle Mixins`, t, func(t *ftt.Test) {
		t.Run(`Nil md`, func(t *ftt.Test) {
			mixins, err := HandleMixins(nil, cm, "/root")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mixins), should.BeZero)
		})
		t.Run(`No Mixins`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "tEsT>CompOnent",
				},
			}

			mixins, err := HandleMixins(md, cm, "/root")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mixins), should.BeZero)
		})
		t.Run(`Empty Mixins`, func(t *ftt.Test) {
			md := &dirmdpb.Metadata{
				Mixins: make([]string, 0),
			}

			mixins, err := HandleMixins(md, cm, "/root")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(mixins), should.BeZero)
		})
	})
}
