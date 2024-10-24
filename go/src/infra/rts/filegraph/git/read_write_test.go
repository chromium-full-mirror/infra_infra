// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestReadWrite(t *testing.T) {
	t.Parallel()

	ftt.Run(`ReadWrite`, t, func(t *ftt.Test) {
		test := func(g *Graph) {
			g.ensureInitialized()

			buf := &bytes.Buffer{}
			w := writer{w: buf}
			err := w.writeGraph(g)
			assert.Loosely(t, err, should.BeNil)

			r := reader{r: bufio.NewReader(buf)}
			g2 := &Graph{}
			g2.ensureInitialized()
			err = r.readGraph(g2)
			assert.Loosely(t, err, should.BeNil)

			g2.root.visit(func(n *node) bool {
				n.copyEdgesOnAppend = false
				return true
			})
			assert.Loosely(t, g, should.Resemble(g2))
		}

		t.Run(`Zero`, func(t *ftt.Test) {
			test(&Graph{})
		})

		t.Run(`Two direct children`, func(t *ftt.Test) {
			g := &Graph{
				Commit: "deadbeef",
				root:   node{name: "//"},
			}
			foo := &node{parent: &g.root, name: "//foo", probSumDenominator: 1}
			bar := &node{parent: &g.root, name: "//bar", probSumDenominator: 2}
			foo.edges = []edge{{to: bar, probSum: probOne}}
			bar.edges = []edge{{to: foo, probSum: probOne}}
			g.root.children = map[string]*node{
				"foo": foo,
				"bar": bar,
			}
			test(g)
		})
	})
}
