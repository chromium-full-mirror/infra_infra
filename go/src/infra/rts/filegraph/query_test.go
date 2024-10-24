// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filegraph

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/registry"
	"go.chromium.org/luci/common/testing/truth"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func init() {
	registry.RegisterCmpOption(cmp.AllowUnexported(testNode{}))
}

type testGraph struct {
	nodes map[string]*testNode
}

func (g *testGraph) node(name string) *testNode {
	n := g.nodes[name]
	if n == nil {
		n = &testNode{
			name:  name,
			edges: map[*testNode]float64{},
		}
		g.nodes[name] = n
	}
	return n
}

func (g *testGraph) ReadEdges(from Node, callback func(to Node, distance float64) (keepGoing bool)) {
	for other, dist := range from.(*testNode).edges {
		if !callback(other, dist) {
			return
		}
	}
}

func run(t testing.TB, q *Query) map[string]*ShortestPath {
	t.Helper()

	ret := map[string]*ShortestPath{}
	q.Run(func(sp *ShortestPath) bool {
		name := sp.Node.Name()
		assert.Loosely(t, ret[name], should.BeNil, truth.LineContext())
		ret[name] = sp
		return true
	})
	return ret
}

func (g *testGraph) query(sources ...string) *Query {
	q := &Query{
		Sources:    make([]Node, len(sources)),
		EdgeReader: g,
	}
	for i, src := range sources {
		q.Sources[i] = g.node(src)
	}
	return q
}

type testNode struct {
	name  string
	edges map[*testNode]float64
}

func (n *testNode) Name() string {
	return n.name
}

func initGraph(edges ...testEdge) *testGraph {
	g := &testGraph{
		nodes: map[string]*testNode{},
	}

	for _, e := range edges {
		g.node(e.from).edges[g.node(e.to)] = e.distance
	}

	return g
}

type testEdge struct {
	from     string
	to       string
	distance float64
}

func TestQuery(t *testing.T) {
	t.Parallel()

	ftt.Run(`Query`, t, func(t *ftt.Test) {
		t.Run(`Run`, func(t *ftt.Test) {
			t.Run(`Works`, func(t *ftt.Test) {
				g := initGraph(
					testEdge{from: "//a", to: "//b/1", distance: 1},
					testEdge{from: "//a", to: "//b/2", distance: 2},
					testEdge{from: "//b/1", to: "//c", distance: 3},
					testEdge{from: "//b/2", to: "//c", distance: 3},
				)

				sps := run(t, g.query("//a"))
				assert.Loosely(t, sps, should.Resemble(map[string]*ShortestPath{
					"//a": {
						Node:     g.node("//a"),
						Distance: 0,
					},
					"//b/1": {
						Prev:     sps["//a"],
						Node:     g.node("//b/1"),
						Distance: 1,
					},
					"//b/2": {
						Prev:     sps["//a"],
						Node:     g.node("//b/2"),
						Distance: 2,
					},
					"//c": {
						Prev:     sps["//b/1"],
						Node:     g.node("//c"),
						Distance: 4,
					},
				}))
			})

			t.Run(`MaxDistance`, func(t *ftt.Test) {
				g := initGraph(
					testEdge{from: "//a", to: "//b/1", distance: 1},
					testEdge{from: "//a", to: "//b/2", distance: 2},
					testEdge{from: "//b/1", to: "//c", distance: 3},
					testEdge{from: "//b/2", to: "//c", distance: 3},
				)
				q := g.query("//a")
				q.MaxDistance = 3
				sps := run(t, q)
				assert.Loosely(t, sps, should.Resemble(map[string]*ShortestPath{
					"//a": {
						Node:     g.node("//a"),
						Distance: 0,
					},
					"//b/1": {
						Prev:     sps["//a"],
						Node:     g.node("//b/1"),
						Distance: 1,
					},
					"//b/2": {
						Prev:     sps["//a"],
						Node:     g.node("//b/2"),
						Distance: 2,
					},
				}))
			})

			t.Run(`Unreachable`, func(t *ftt.Test) {
				g := initGraph(
					testEdge{from: "//a", to: "//b", distance: 1},
					testEdge{from: "//c", to: "//d"},
				)

				sps := run(t, g.query("//a"))
				assert.Loosely(t, sps, should.Resemble(map[string]*ShortestPath{
					"//a": {
						Node:     g.node("//a"),
						Distance: 0,
					},
					"//b": {
						Prev:     sps["//a"],
						Node:     g.node("//b"),
						Distance: 1,
					},
				}))
			})

			t.Run(`Visiting the same node multiple times`, func(t *ftt.Test) {
				g := initGraph(
					testEdge{from: "//a", to: "//b", distance: 1},
					testEdge{from: "//a", to: "//c", distance: 10},
					testEdge{from: "//b", to: "//c", distance: 1},
				)
				g.query("//a") // asserts that each node is reported once
			})
			t.Run(`Duplicate sources`, func(t *ftt.Test) {
				g := initGraph(
					testEdge{from: "//a", to: "//b", distance: 1},
				)
				g.query("//a", "//a") // asserts that each node is reported once
			})
		})

		t.Run(`ShortestPath`, func(t *ftt.Test) {
			g := initGraph(
				testEdge{from: "//a", to: "//b/1", distance: 1},
				testEdge{from: "//a", to: "//b/2", distance: 2},
				testEdge{from: "//b/1", to: "//c", distance: 3},
				testEdge{from: "//b/2", to: "//c", distance: 3},
				testEdge{from: "//unreachable/1", to: "//unreachable/2"},
			)
			q := g.query("//a")

			t.Run(`Works`, func(t *ftt.Test) {
				sp := q.ShortestPath(g.node("//c"))
				assert.Loosely(t, sp, should.Resemble(&ShortestPath{
					Node:     g.node("//c"),
					Distance: 4,
					Prev: &ShortestPath{
						Node:     g.node("//b/1"),
						Distance: 1,
						Prev: &ShortestPath{
							Node:     g.node("//a"),
							Distance: 0,
						},
					},
				}))
				assert.Loosely(t, sp.Path(), should.Resemble([]*ShortestPath{sp.Prev.Prev, sp.Prev, sp}))
			})

			t.Run(`Unreachable`, func(t *ftt.Test) {
				assert.Loosely(t, q.ShortestPath(g.node("//unreachable/1")), should.BeNil)
			})
		})
	})
}
