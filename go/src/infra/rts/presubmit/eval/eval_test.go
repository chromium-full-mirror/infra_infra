// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package eval

import (
	"bytes"
	"container/heap"
	"fmt"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/rts"
	evalpb "go.chromium.org/infra/rts/presubmit/eval/proto"
)

func TestBucketSlice(t *testing.T) {
	t.Parallel()
	ftt.Run(`bucketSlice`, t, func(t *ftt.Test) {
		t.Run(`inc`, func(t *ftt.Test) {
			thresholds := make([]*evalpb.Threshold, 10)
			for i := 0; i < len(thresholds); i++ {
				thresholds[i] = &evalpb.Threshold{
					MaxDistance: float32(i),
				}
			}
			b := make(bucketSlice, len(thresholds)+1)

			t.Run(`2`, func(t *ftt.Test) {
				b.inc(thresholds, rts.Affectedness{Distance: 2}, 1)
				assert.Loosely(t, b[2], should.Equal(1))
			})
			t.Run(`3`, func(t *ftt.Test) {
				b.inc(thresholds, rts.Affectedness{Distance: 3}, 1)
				assert.Loosely(t, b[3], should.Equal(1))
			})
			t.Run(`10`, func(t *ftt.Test) {
				b.inc(thresholds, rts.Affectedness{Distance: 10}, 1)
				assert.Loosely(t, b[10], should.Equal(1))
			})
			t.Run(`0`, func(t *ftt.Test) {
				// This data point was not lost by any threshold.
				b.inc(thresholds, rts.Affectedness{Distance: 0}, 1)
				assert.Loosely(t, b[0], should.Equal(1))
			})
			t.Run(`11`, func(t *ftt.Test) {
				// This data point was lost by all thresholds.
				b.inc(thresholds, rts.Affectedness{Distance: 11}, 1)
				assert.Loosely(t, b[10], should.Equal(1))
			})
		})
		t.Run(`makeCumulative`, func(t *ftt.Test) {
			b := make(bucketSlice, 10)

			assert10 := func(expected string) {
				expected = strings.TrimPrefix(expected, "\n")
				expected = strings.ReplaceAll(expected, "\t", "")

				var buf bytes.Buffer
				for _, v := range b {
					fmt.Fprintf(&buf, "%d", v)
				}
				assert.Loosely(t, buf.String(), should.Equal(expected))
			}

			t.Run(`b[0] = 1`, func(t *ftt.Test) {
				b[0] = 1
				b.makeCumulative()
				assert10(`1000000000`)
			})

			t.Run(`b[5] = 1`, func(t *ftt.Test) {
				b[5] = 1
				b.makeCumulative()
				assert10(`1111110000`)
			})

			t.Run(`b[2] = 1, b[4] = 2`, func(t *ftt.Test) {
				b[2] = 1
				b[4] = 2
				b.makeCumulative()
				assert10(`3332200000`)
			})
		})
	})
}

func TestMostAffected(t *testing.T) {
	t.Parallel()
	ftt.Run(`Test[]rts.Affectedness`, t, func(t *ftt.Test) {
		t.Run(`mostAffected`, func(t *ftt.Test) {
			t.Run(`Works`, func(t *ftt.Test) {
				most, err := mostAffected([]rts.Affectedness{
					{Distance: 1},
					{Distance: 0},
				})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, most, should.Resemble(rts.Affectedness{Distance: 0}))
			})

			t.Run(`Empty`, func(t *ftt.Test) {
				_, err := mostAffected(nil)
				assert.Loosely(t, err, should.ErrLike("empty"))
			})

			t.Run(`Single`, func(t *ftt.Test) {
				most, err := mostAffected([]rts.Affectedness{{Distance: 0}})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, most, should.Resemble(rts.Affectedness{Distance: 0}))
			})
		})
	})
}

func TestQuantiles(t *testing.T) {
	t.Parallel()
	ftt.Run(`Quantiles`, t, func(t *ftt.Test) {
		t.Run(`median of 1, 2, 3, 4`, func(t *ftt.Test) {
			afs := []rts.Affectedness{
				{Distance: 1},
				{Distance: 2},
				{Distance: 3},
				{Distance: 4},
			}
			assert.Loosely(t, distanceQuantiles(afs, 2), should.Resemble([]float32{2, 4}))
		})
		t.Run(`4-quantiles of 1, 2, 3, 4`, func(t *ftt.Test) {
			afs := []rts.Affectedness{
				{Distance: 1},
				{Distance: 2},
				{Distance: 3},
				{Distance: 4},
			}
			assert.Loosely(t, distanceQuantiles(afs, 4), should.Resemble([]float32{1, 2, 3, 4}))
		})
		t.Run(`10-quantiles of 1, 2, 3, 4`, func(t *ftt.Test) {
			afs := []rts.Affectedness{
				{Distance: 1},
				{Distance: 2},
				{Distance: 3},
				{Distance: 4},
			}
			assert.Loosely(t, distanceQuantiles(afs, 10), should.Resemble([]float32{1, 1, 2, 2, 2, 3, 3, 4, 4, 4}))
		})
	})
}

func TestFurthestRejections(t *testing.T) {
	ftt.Run("FurthestRejections", t, func(t *ftt.Test) {
		furthest := make(furthestRejections, 3)
		furthest.Consider(affectedRejection{MostAffected: rts.Affectedness{Distance: 1}})
		furthest.Consider(affectedRejection{MostAffected: rts.Affectedness{Distance: 2}})
		furthest.Consider(affectedRejection{MostAffected: rts.Affectedness{Distance: 3}})
		furthest.Consider(affectedRejection{MostAffected: rts.Affectedness{Distance: 4}})
		furthest.Consider(affectedRejection{MostAffected: rts.Affectedness{Distance: 5}})

		assert.Loosely(t, len(furthest), should.Equal(3))
		assert.Loosely(t, heap.Pop(&furthest), should.Resemble(affectedRejection{MostAffected: rts.Affectedness{Distance: 3}}))
		assert.Loosely(t, heap.Pop(&furthest), should.Resemble(affectedRejection{MostAffected: rts.Affectedness{Distance: 4}}))
		assert.Loosely(t, heap.Pop(&furthest), should.Resemble(affectedRejection{MostAffected: rts.Affectedness{Distance: 5}}))
	})
}
