// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"math"
	"sort"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/rts/filegraph"
)

func TestGraph(t *testing.T) {
	t.Parallel()

	ftt.Run(`Graph`, t, func(t *ftt.Test) {
		t.Run(`Root of zero value`, func(t *ftt.Test) {
			g := &Graph{}
			root := g.Node("//")
			assert.Loosely(t, root, should.NotBeNil)
			assert.Loosely(t, root.Name(), should.Equal("//"))
		})

		t.Run(`node()`, func(t *ftt.Test) {
			g := &Graph{
				root: node{
					children: map[string]*node{
						"dir": {
							children: map[string]*node{
								"foo": {},
							},
						},
					},
				},
			}

			t.Run(`//`, func(t *ftt.Test) {
				assert.Loosely(t, g.node("//"), should.Equal(&g.root))
			})

			t.Run(`//dir`, func(t *ftt.Test) {
				assert.Loosely(t, g.node("//dir"), should.Equal(g.root.children["dir"]))
			})
			t.Run(`//dir/foo`, func(t *ftt.Test) {
				assert.Loosely(t, g.node("//dir/foo"), should.Equal(g.root.children["dir"].children["foo"]))
			})
			t.Run(`//dir/bar`, func(t *ftt.Test) {
				assert.Loosely(t, g.node("//dir/bar"), should.BeNil)
			})
		})

		t.Run(`ensureNode`, func(t *ftt.Test) {
			g := &Graph{}
			t.Run("//foo/bar", func(t *ftt.Test) {
				bar := g.ensureNode("//foo/bar")
				assert.Loosely(t, bar, should.NotBeNil)
				assert.Loosely(t, bar.name, should.Equal("//foo/bar"))
				assert.Loosely(t, g.node("//foo/bar"), should.Equal(bar))

				foo := g.node("//foo")
				assert.Loosely(t, foo, should.NotBeNil)
				assert.Loosely(t, foo.name, should.Equal("//foo"))
				assert.Loosely(t, foo.children["bar"], should.Equal(bar))
			})

			t.Run("already exists", func(t *ftt.Test) {
				assert.Loosely(t, g.ensureNode("//foo/bar"), should.Equal(g.ensureNode("//foo/bar")))
			})

			t.Run("//", func(t *ftt.Test) {
				root := g.ensureNode("//")
				assert.Loosely(t, root, should.Equal(&g.root))
			})
		})

		t.Run(`sortedChildKeys()`, func(t *ftt.Test) {
			node := &node{
				children: map[string]*node{
					"foo": {},
					"bar": {},
				},
			}
			assert.Loosely(t, node.sortedChildKeys(), should.Resemble([]string{"bar", "foo"}))
		})

		t.Run(`Node(non-existent) returns nil`, func(t *ftt.Test) {
			g := &Graph{}
			// Do not use ShouldBeNil - it checks for interface{} with nil inside,
			// and we need exact nil.
			assert.Loosely(t, g.Node("//a/b") == nil, should.BeTrue)
		})

		t.Run(`EdgeReader`, func(t *ftt.Test) {
			root := &node{name: "//"}
			bar := &node{parent: root, name: "//foo", probSumDenominator: 4}
			foo := &node{parent: root, name: "//bar", probSumDenominator: 2}
			foo.edges = []edge{{to: bar, probSum: probOne}}
			bar.edges = []edge{{to: foo, probSum: probOne}}
			root.children = map[string]*node{
				"foo": foo,
				"bar": bar,
			}

			type outgoingEdge struct {
				to       filegraph.Node
				distance float64
			}
			var actual []outgoingEdge
			callback := func(other filegraph.Node, distance float64) bool {
				actual = append(actual, outgoingEdge{to: other, distance: distance})
				return true
			}

			r := &EdgeReader{}
			t.Run(`Works`, func(t *ftt.Test) {
				r.ReadEdges(foo, callback)
				assert.Loosely(t, actual, should.HaveLength(1))
				assert.Loosely(t, actual[0].to, should.Equal(bar))
				assert.Loosely(t, actual[0].distance, should.AlmostEqual(
					-math.Log(0.25), 0.00000000000001))
			})
			t.Run(`Double ChangeLogFactor`, func(t *ftt.Test) {
				r.ChangeLogDistanceFactor = 2
				r.ReadEdges(foo, callback)
				assert.Loosely(t, actual, should.HaveLength(1))
				assert.Loosely(t, actual[0].to, should.Equal(bar))
				assert.Loosely(t, actual[0].distance, should.AlmostEqual(
					-2*math.Log(0.25), 0.00000000000001))
			})
			t.Run(`File structure distance only`, func(t *ftt.Test) {
				r.FileStructureDistanceFactor = 1
				t.Run(`parent`, func(t *ftt.Test) {
					r.ReadEdges(foo, callback)
					assert.Loosely(t, actual, should.Resemble([]outgoingEdge{
						{to: root, distance: 1},
					}))
				})
				t.Run(`children`, func(t *ftt.Test) {
					r.ReadEdges(root, callback)
					sort.Slice(actual, func(i, j int) bool {
						return actual[i].to == foo
					})
					assert.Loosely(t, actual, should.Resemble([]outgoingEdge{
						{to: foo, distance: 1},
						{to: bar, distance: 1},
					}))
				})
			})
			t.Run(`Both distances`, func(t *ftt.Test) {
				r.ChangeLogDistanceFactor = 1
				r.FileStructureDistanceFactor = 1
				r.ReadEdges(foo, callback)
				assert.Loosely(t, actual, should.HaveLength(2))
				assert.Loosely(t, actual[0].to, should.Equal(bar))
				assert.Loosely(t, actual[0].distance, should.AlmostEqual(
					-math.Log(0.25), 0.00000000000001))
				assert.Loosely(t, actual[1].to, should.Equal(root))
				assert.Loosely(t, actual[1].distance, should.Equal(1.0))
			})
		})

		t.Run(`splitName`, func(t *ftt.Test) {
			t.Run("//foo/bar.cc", func(t *ftt.Test) {
				assert.Loosely(t, splitName("//foo/bar.cc"), should.Resemble([]string{"foo", "bar.cc"}))
			})
			t.Run("//", func(t *ftt.Test) {
				assert.Loosely(t, splitName("//"), should.Resemble([]string(nil)))
			})
		})
	})
}
