// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git

import (
	"fmt"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestApply(t *testing.T) {
	t.Parallel()

	ftt.Run(`apply`, t, func(t *ftt.Test) {
		g := &Graph{}
		g.ensureInitialized()

		applyChanges := func(changes []fileChange) {
			err := g.apply(changes, 100)
			assert.Loosely(t, err, should.BeNil)
		}

		t.Run(`Empty change`, func(t *ftt.Test) {
			applyChanges(nil)
			assert.Loosely(t, g.root, should.Resemble(node{name: "//"}))
		})

		t.Run(`Add one file`, func(t *ftt.Test) {
			applyChanges([]fileChange{
				{Path: "a", Status: 'A'},
			})
			// The file is registered, but the commit is otherwise ignored.
			assert.Loosely(t, g.root, should.Resemble(node{
				name: "//",
				children: map[string]*node{
					"a": {
						name:   "//a",
						parent: &g.root,
					},
				},
			}))
		})

		t.Run(`Add two files`, func(t *ftt.Test) {
			applyChanges([]fileChange{
				{Path: "a", Status: 'A'},
				{Path: "b", Status: 'A'},
			})
			assert.Loosely(t, g.root, should.Resemble(node{
				name: "//",
				children: map[string]*node{
					"a": {
						name:               "//a",
						parent:             &g.root,
						probSumDenominator: 1,
						edges:              []edge{{to: g.node("//b"), probSum: probOne}},
					},
					"b": {
						name:               "//b",
						parent:             &g.root,
						probSumDenominator: 1,
						edges:              []edge{{to: g.node("//a"), probSum: probOne}},
					},
				},
			}))

			t.Run(`Add two more`, func(t *ftt.Test) {
				applyChanges([]fileChange{
					{Path: "b", Status: 'A'},
					{Path: "c/d", Status: 'A'},
				})
				assert.Loosely(t, g.root, should.Resemble(node{
					name: "//",
					children: map[string]*node{
						"a": {
							name:               "//a",
							parent:             &g.root,
							probSumDenominator: 1,
							edges:              []edge{{to: g.node("//b"), probSum: probOne}},
						},
						"b": {
							name:               "//b",
							parent:             &g.root,
							probSumDenominator: 2,
							edges: []edge{
								{to: g.node("//a"), probSum: probOne},
								{to: g.node("//c/d"), probSum: probOne},
							},
						},
						"c": {
							name:   "//c",
							parent: &g.root,
							children: map[string]*node{
								"d": {
									name:               "//c/d",
									parent:             g.node("//c"),
									probSumDenominator: 1,
									edges:              []edge{{to: g.node("//b"), probSum: probOne}},
								},
							},
						},
					},
				}))
			})

			t.Run(`Modify them again`, func(t *ftt.Test) {
				applyChanges([]fileChange{
					{Path: "a", Status: 'M'},
					{Path: "b", Status: 'M'},
				})
				assert.Loosely(t, g.root, should.Resemble(node{
					name: "//",
					children: map[string]*node{
						"a": {
							name:               "//a",
							parent:             &g.root,
							probSumDenominator: 2,
							edges:              []edge{{to: g.node("//b"), probSum: 2 * probOne}},
						},
						"b": {
							name:               "//b",
							parent:             &g.root,
							probSumDenominator: 2,
							edges:              []edge{{to: g.node("//a"), probSum: 2 * probOne}},
						},
					},
				}))

			})

			t.Run(`Modify one and add another`, func(t *ftt.Test) {
				applyChanges([]fileChange{
					{Path: "b", Status: 'M'},
					{Path: "c", Status: 'M'},
				})
				assert.Loosely(t, g.root, should.Resemble(node{
					name: "//",
					children: map[string]*node{
						"a": {
							name:               "//a",
							parent:             &g.root,
							probSumDenominator: 1,
							edges:              []edge{{to: g.node("//b"), probSum: probOne}},
						},
						"b": {
							name:               "//b",
							parent:             &g.root,
							probSumDenominator: 2,
							edges: []edge{
								{to: g.node("//a"), probSum: probOne},
								{to: g.node("//c"), probSum: probOne},
							},
						},
						"c": {
							name:               "//c",
							parent:             &g.root,
							probSumDenominator: 1,
							edges:              []edge{{to: g.node("//b"), probSum: probOne}},
						},
					},
				}))
			})

			t.Run(`Rename one`, func(t *ftt.Test) {
				applyChanges([]fileChange{
					{Path: "b", Path2: "c", Status: 'R'},
				})
				assert.Loosely(t, g.root, should.Resemble(node{
					name: "//",
					children: map[string]*node{
						"a": {
							name:               "//a",
							parent:             &g.root,
							probSumDenominator: 1,
							edges:              []edge{{to: g.node("//b"), probSum: probOne}},
						},
						"b": {
							name:               "//b",
							parent:             &g.root,
							probSumDenominator: 1,
							edges: []edge{
								{to: g.node("//a"), probSum: probOne},
								{to: g.node("//c")},
							},
						},
						"c": {
							name:   "//c",
							parent: &g.root,
							edges:  []edge{{to: g.node("//b")}},
						},
					},
				}))
			})

			t.Run(`Remove one`, func(t *ftt.Test) {
				applyChanges([]fileChange{
					{Path: "b", Status: 'D'},
				})
				assert.Loosely(t, g.root, should.Resemble(node{
					name: "//",
					children: map[string]*node{
						"a": {
							name:               "//a",
							parent:             &g.root,
							probSumDenominator: 1,
							edges:              []edge{{to: g.node("//b"), probSum: probOne}},
						},
						"b": {
							name:               "//b",
							parent:             &g.root,
							probSumDenominator: 1,
							edges:              []edge{{to: g.node("//a"), probSum: probOne}},
						},
					},
				}))
			})
		})

		t.Run(`Great migration`, func(t *ftt.Test) {
			addFiles := make([]fileChange, 1000)
			for i := range addFiles {
				addFiles[i] = fileChange{Path: fmt.Sprintf("%d", i), Status: 'A'}
			}
			applyChanges(addFiles)

			greatMigration := make([]fileChange, len(addFiles))
			for i, add := range addFiles {
				greatMigration[i] = fileChange{
					Path:   add.Path,
					Path2:  "new/" + add.Path,
					Status: 'R',
				}
			}
			applyChanges(greatMigration)

			old54 := g.node("//54")
			new54 := g.node("//new/54")
			assert.Loosely(t, new54, should.NotBeNil)
			assert.Loosely(t, new54.edges, should.Resemble([]edge{{to: old54}}))
		})
	})
}
