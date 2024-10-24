// Copyright 2020 The Chromium Authors
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

func TestReduce(t *testing.T) {
	t.Parallel()

	ftt.Run(`Reduce`, t, func(t *ftt.Test) {
		t.Run(`Works`, func(t *ftt.Test) {
			input := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						TeamEmail: "team@example.com",
						Monorail: &dirmdpb.Monorail{
							Project: "chromium",
						},
					},
					"a": {
						TeamEmail: "team@example.com", // redundant
						Wpt:       &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
						Monorail: &dirmdpb.Monorail{
							Project:   "chromium", // redundant
							Component: "Component",
						},
					},
				},
			}

			actual := input.Clone()
			assert.Loosely(t, actual.Reduce(), should.BeNil)
			assert.Loosely(t, actual.Proto(), should.Resemble(&dirmdpb.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": input.Dirs["."], // did not change
					"a": {
						Wpt: &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
						Monorail: &dirmdpb.Monorail{
							Component: "Component",
						},
					},
				},
			}))
		})

		t.Run(`Deep nesting`, func(t *ftt.Test) {
			m := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {TeamEmail: "team@example.com"},
					"a": {
						TeamEmail: "team@example.com",
						// One more additional field, to avoid "a" being removed completely.
						Os: dirmdpb.OS_ANDROID,
					},
					"a/b": {TeamEmail: "team@example.com"},
				},
			}
			assert.Loosely(t, m.Reduce(), should.BeNil)
			assert.Loosely(t, m.Dirs["a"].GetTeamEmail(), should.BeEmpty)
			assert.Loosely(t, m.Dirs["a"].GetOs(), should.Equal(dirmdpb.OS_ANDROID))
			assert.Loosely(t, m.Dirs, should.NotContainKey("a/b"))
		})

		t.Run(`Mixins`, func(t *ftt.Test) {
			input := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						TeamEmail: "team@example.com",
						Wpt:       &dirmdpb.WPT{Notify: dirmdpb.Trinary_NO},
					},
					"a": {
						Mixins: []string{"//mixin1"},
					},
					"a/b": {
						Mixins:    []string{"//mixin2"},
						Buganizer: &dirmdpb.Buganizer{ComponentId: 54},
						TeamEmail: "team@example.com", // redundant
						Wpt:       &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
						Monorail: &dirmdpb.Monorail{
							Project:   "chromium", // redundant
							Component: "Component",
						},
					},
				},
				Repos: map[string]*dirmdpb.Repo{
					".": {
						Mixins: map[string]*dirmdpb.Metadata{
							"//mixin1": {
								Monorail: &dirmdpb.Monorail{
									Project: "chromium",
								},
							},
							"//mixin2": {
								Buganizer: &dirmdpb.Buganizer{ComponentId: 54},
							},
						},
					},
				},
			}

			actual := input.Clone()
			assert.Loosely(t, actual.Reduce(), should.BeNil)
			assert.Loosely(t, actual.Proto(), should.Resemble(&dirmdpb.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": input.Dirs["."], // did not change
					"a": {
						Mixins: []string{"//mixin1"},
					},
					"a/b": {
						Mixins: []string{"//mixin2"},
						Wpt:    &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
						Monorail: &dirmdpb.Monorail{
							Component: "Component",
						},
					},
				},
				Repos: input.Repos,
			}))
		})

		t.Run(`Nothing to reduce`, func(t *ftt.Test) {
			m := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a": {TeamEmail: "team@example.com"},
					"b": {TeamEmail: "team@example.com"},
				},
			}
			assert.Loosely(t, m.Reduce(), should.BeNil)
			assert.Loosely(t, m.Proto(), should.Resemble(&dirmdpb.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a": {TeamEmail: "team@example.com"},
					"b": {TeamEmail: "team@example.com"},
				},
			}))
		})
	})
}

func TestMerge(t *testing.T) {
	ftt.Run(`Merge`, t, func(t *ftt.Test) {
		t.Run(`wpt.notify false value is not overwritten/ignored`, func(t *ftt.Test) {
			inherited := &dirmdpb.Metadata{
				Wpt: &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
			}
			own := &dirmdpb.Metadata{
				Wpt: &dirmdpb.WPT{Notify: dirmdpb.Trinary_NO},
			}
			Merge(inherited, own)
			assert.Loosely(t, inherited.Wpt.Notify, should.Equal(dirmdpb.Trinary_NO))
		})
	})
}

func TestComputeAll(t *testing.T) {
	t.Parallel()

	ftt.Run(`Nearest ancestor`, t, func(t *ftt.Test) {
		m := &Mapping{
			Dirs: map[string]*dirmdpb.Metadata{
				".": {TeamEmail: "0"},
			},
		}
		assert.Loosely(t, m.nearestAncestor("a/b/c").TeamEmail, should.Equal("0"))
		assert.Loosely(t, m.nearestAncestor("."), should.BeNil)
	})

	ftt.Run(`ComputeAll`, t, func(t *ftt.Test) {
		t.Run(`Works`, func(t *ftt.Test) {
			m := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						TeamEmail: "team@example.com",
						// Will be inherited entirely.
						Wpt: &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},

						// Will be inherited partially.
						Monorail: &dirmdpb.Monorail{
							Project: "chromium",
						},
					},
					"a": {
						TeamEmail: "team-email@chromium.org",
						Monorail: &dirmdpb.Monorail{
							Component: "Component",
						},
					},
				},
			}
			assert.Loosely(t, m.ComputeAll(), should.BeNil)
			assert.Loosely(t, m.Proto(), should.Resemble(&dirmdpb.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": m.Dirs["."], // did not change
					"a": {
						TeamEmail: "team-email@chromium.org",
						Wpt:       &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
						Monorail: &dirmdpb.Monorail{
							Project:   "chromium",
							Component: "Component",
						},
					},
				},
			}))
		})

		t.Run(`Deep nesting`, func(t *ftt.Test) {
			m := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".":   {TeamEmail: "team@example.com"},
					"a":   {},
					"a/b": {},
				},
			}
			assert.Loosely(t, m.ComputeAll(), should.BeNil)
			assert.Loosely(t, m.Dirs["a/b"].TeamEmail, should.Equal("team@example.com"))
		})

		t.Run(`Mixins`, func(t *ftt.Test) {
			m := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": {
						TeamEmail: "root@example.com",
						// Will be inherited entirely.
						Wpt: &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},

						// Will be inherited partially.
						Monorail: &dirmdpb.Monorail{
							Project: "chromium",
						},
					},
					"a": {
						TeamEmail: "a@chromium.org",
						Mixins:    []string{"//FOO_METADATA", "//BAR_METADATA"},
					},
				},
				Repos: map[string]*dirmdpb.Repo{
					".": {
						Mixins: map[string]*dirmdpb.Metadata{
							"//FOO_METADATA": {
								Monorail: &dirmdpb.Monorail{
									Component: "foo",
								},
								Os: dirmdpb.OS_ANDROID,
							},
							"//BAR_METADATA": {
								TeamEmail: "bar@chromium.org",
								Monorail: &dirmdpb.Monorail{
									Component: "bar",
								},
							},
						},
					},
				},
			}
			assert.Loosely(t, m.ComputeAll(), should.BeNil)
			assert.Loosely(t, m.Proto(), should.Resemble(&dirmdpb.Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					".": m.Dirs["."], // did not change
					"a": {
						TeamEmail: "a@chromium.org",
						Wpt:       &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
						Monorail: &dirmdpb.Monorail{
							Project:   "chromium",
							Component: "bar",
						},
						Os: dirmdpb.OS_ANDROID,
					},
				},
				Repos: m.Repos,
			}))
		})

		t.Run(`No root`, func(t *ftt.Test) {
			input := &Mapping{
				Dirs: map[string]*dirmdpb.Metadata{
					"a": {TeamEmail: "a"},
					"b": {TeamEmail: "b"},
				},
			}

			actual := input.Clone()
			assert.Loosely(t, actual.ComputeAll(), should.BeNil)
			assert.Loosely(t, input.Proto(), should.Resemble(input.Proto()))
		})
	})
}
