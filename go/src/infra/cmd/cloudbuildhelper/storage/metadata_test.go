// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"encoding/json"
	"testing"
	"time"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	raw := func() map[string]string {
		return map[string]string{
			"raw1":    "v1",
			"raw2":    "v2",
			"a@2":     "v3",
			"b@2":     "v4",
			"a@4":     "v5",
			"b@4":     "v6",
			"c@0003":  "v7", // edge case, non essential when using real timestamps
			"d@0":     "v8", // edge case, non essential when using real timestamps
			"bad@?":   "v9",
			"bad@":    "v10",
			"bad@1@2": "v11",
		}
	}

	parsed := &Metadata{
		d: map[string][]Metadatum{
			"bad@": {
				{Key: "bad@", Timestamp: 0, Value: "v10"},
			},
			"bad@1@2": {
				{Key: "bad@1@2", Timestamp: 0, Value: "v11"},
			},
			"bad@?": {
				{Key: "bad@?", Timestamp: 0, Value: "v9"},
			},
			"d": {
				{Key: "d", Timestamp: 0, Value: "v8"},
			},
			"raw1": {
				{Key: "raw1", Timestamp: 0, Value: "v1"},
			},
			"raw2": {
				{Key: "raw2", Timestamp: 0, Value: "v2"},
			},
			"a": {
				{Key: "a", Timestamp: 4, Value: "v5"},
				{Key: "a", Timestamp: 2, Value: "v3"},
			},
			"b": {
				{Key: "b", Timestamp: 4, Value: "v6"},
				{Key: "b", Timestamp: 2, Value: "v4"},
			},
			"c": {
				{Key: "c", Timestamp: 3, Value: "v7"},
			},
		},
	}

	ftt.Run("ParseMetadata", t, func(t *ftt.Test) {
		assert.Loosely(t, ParseMetadata(raw()), should.Resemble(parsed))
	})

	ftt.Run("Assemble", t, func(t *ftt.Test) {
		raw := raw()
		md := ParseMetadata(raw).Assemble()
		assert.Loosely(t, md, should.HaveLength(len(raw)))

		// Known edge cases we don't care about.
		assert.Loosely(t, md["c@3"], should.Equal("v7"))
		assert.Loosely(t, md["d"], should.Equal("v8"))
		delete(md, "c@3")
		delete(raw, "c@0003")
		delete(md, "d")
		delete(raw, "d@0")

		assert.Loosely(t, md, should.Resemble(raw))
	})

	ftt.Run("Keys", t, func(t *ftt.Test) {
		assert.Loosely(t, ParseMetadata(raw()).Keys(), should.Resemble([]string{
			"a", "b", "bad@", "bad@1@2", "bad@?", "c", "d", "raw1", "raw2",
		}))
	})

	ftt.Run("Equal", t, func(t *ftt.Test) {
		e := func(k string, ts Timestamp) Metadatum {
			return Metadatum{Key: k, Timestamp: ts}
		}

		md := func(e ...Metadatum) *Metadata {
			out := &Metadata{}
			for _, x := range e {
				out.Add(x)
			}
			return out
		}

		assert.Loosely(t, md().Equal(md()), should.BeTrue)
		assert.Loosely(t, md(e("1", 1)).Equal(md(e("1", 1))), should.BeTrue)
		assert.Loosely(t, md(e("1", 1)).Equal(md(e("1", 2))), should.BeFalse)
		assert.Loosely(t, md(e("1", 1), e("1", 2)).Equal(md(e("1", 1))), should.BeFalse)
		assert.Loosely(t, md(e("1", 1), e("2", 2)).Equal(md(e("1", 1), e("1", 2))), should.BeFalse)
	})

	ftt.Run("Clone", t, func(t *ftt.Test) {
		md := ParseMetadata(raw())
		assert.Loosely(t, md.Clone().Equal(md), should.BeTrue)
	})

	ftt.Run("Add", t, func(t *ftt.Test) {
		md := ParseMetadata(raw())

		md.Add(Metadatum{Key: "new", Timestamp: 5})
		md.Add(Metadatum{Key: "new", Timestamp: 3})
		md.Add(Metadatum{Key: "new", Timestamp: 4})
		md.Add(Metadatum{Key: "new", Timestamp: 6})
		md.Add(Metadatum{Key: "new", Timestamp: 4, Value: "z"})

		assert.Loosely(t, md.Values("new"), should.Resemble([]Metadatum{
			{Key: "new", Timestamp: 6},
			{Key: "new", Timestamp: 5},
			{Key: "new", Timestamp: 4, Value: "z"},
			{Key: "new", Timestamp: 3},
		}))
	})

	ftt.Run("TrimUnimportant", t, func(t *ftt.Test) {
		md := Metadata{}

		md.Add(Metadatum{Key: "k1", Timestamp: 1})
		md.Add(Metadatum{Key: "k1", Timestamp: 2})
		md.Add(Metadatum{Key: "k1", Timestamp: 3})
		md.Add(Metadatum{Key: "k2", Timestamp: 1})

		md.TrimUnimportant(2)
		assert.Loosely(t, md.Assemble(), should.Resemble(map[string]string{
			"k1@1": "", // oldest
			"k1@3": "", // most recent
			"k2@1": "",
		}))
	})

	ftt.Run("ToPretty", t, func(t *ftt.Test) {
		ts := testclock.TestRecentTimeUTC
		md := Metadata{}

		add := func(key string, age time.Duration, val interface{}) {
			blob, err := json.Marshal(val)
			if err != nil {
				panic(err)
			}
			md.Add(Metadatum{
				Key:       key,
				Timestamp: TimestampFromTime(ts.Add(-age)),
				Value:     string(blob),
			})
		}

		add("k1", 5*time.Second, "small1")
		add("k1", 10*time.Second, map[string]string{
			"long1": "loooooooooooooooooooooooong",
			"long2": "loooooooooooooooooooooooong",
		})
		add("k1", 15*time.Second, "small2")
		add("k2", 5*time.Second, "small1")
		add("k2", 10*time.Second, "small2")

		assert.Loosely(t, "\n"+md.ToPretty(ts, 10), should.Equal(`
k1 (5 seconds ago): "small1"
k1 (10 seconds ago):
  {
    "long1": "loooooooooooooooooooooooong",
    "long2": "loooooooooooooooooooooooong"
  }
k1 (15 seconds ago): "small2"
k2 (5 seconds ago): "small1"
k2 (10 seconds ago): "small2"
`))
	})
}
