// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"bytes"
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestWrite(t *testing.T) {
	t.Parallel()

	ftt.Run(`Write`, t, func(t *ftt.Test) {
		buf := &bytes.Buffer{}
		w := writer{
			textMode: true,
			w:        buf,
		}

		test := func(g *Graph, expected ...string) {
			err := w.writeGraph(g)
			assert.Loosely(t, err, should.BeNil)
			actual := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			assert.Loosely(t, actual, should.Resemble(expected))
		}

		t.Run(`Zero`, func(t *ftt.Test) {
			test(&Graph{},
				"54", // header
				"0",  // version
				"",   // commit hash
				"0",  // number of root commits
				"0",  // number of root children
				"0",  // total number of edges
				"0",  // number of root edges
			)
		})

		t.Run(`Two direct children`, func(t *ftt.Test) {
			foo := &node{probSumDenominator: 1}
			bar := &node{probSumDenominator: 2}
			foo.edges = []edge{{to: bar, probSum: probOne}}
			bar.edges = []edge{{to: foo, probSum: probOne}}
			g := &Graph{
				Commit: "deadbeef",
				root: node{
					children: map[string]*node{
						"foo": foo,
						"bar": bar,
					},
				},
			}

			test(g,
				"54",       // header
				"0",        // version
				"deadbeef", // commit hash

				"0", // root's probSumDenominator
				"2", // number of root children

				"bar", // name of a root child
				"2",   // bar's probSumDenominator
				"0",   // number of bar children

				"foo", // name of a root child
				"1",   // foo's probSumDenominator
				"0",   // number of foo children

				"2", // total number of edges

				"0", // number of root edges

				"1",        // number of bar edges
				"2",        // index of foo
				"16777216", // probSum for bar->foo

				"1",        // number of foo edges
				"1",        // index of bar
				"16777216", // probSum for foo->bar
			)
		})
	})
}
