// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package handler

import (
	"testing"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model"
)

func TestRenderSettingsPage(t *testing.T) {
	t.Parallel()

	ftt.Run("render settings", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		s := SettingsPage{}

		t.Run("Title", func(t *ftt.Test) {
			title, err := SettingsPage.Title(s, c)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, title, should.Equal("Admin SOM settings"))
		})

		tree := &model.Tree{
			Name:                       "oak",
			DisplayName:                "Great Oaakk",
			BugQueueLabel:              "test",
			AlertStreams:               []string{"hello", "world"},
			HelpLink:                   "http://google.com/",
			GerritProject:              "some/project/name",
			GerritInstance:             "some-gerrit-instance",
			DefaultMonorailProjectName: "oak-project",
			BuildBucketProjectFilter:   "oak-build",
		}

		assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
		datastore.GetTestable(c).CatchupIndexes()

		t.Run("Fields", func(t *ftt.Test) {
			fields, err := SettingsPage.Fields(s, c)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(fields), should.Equal(8))
		})

		t.Run("ReadSettings", func(t *ftt.Test) {
			settings, err := SettingsPage.ReadSettings(s, c)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(settings), should.Equal(8))
			assert.Loosely(t, settings["Trees"], should.Equal("oak:Great Oaakk"))
			assert.Loosely(t, settings["BugQueueLabels"], should.Equal("oak:test"))
			assert.Loosely(t, settings["AlertStreams-oak"], should.Equal("hello,world"))
			assert.Loosely(t, settings["HelpLink-oak"], should.Equal("http://google.com/"))
			assert.Loosely(t, settings["GerritProject-oak"], should.Equal("some/project/name"))
			assert.Loosely(t, settings["GerritInstance-oak"], should.Equal("some-gerrit-instance"))
			assert.Loosely(t, settings["DefaultMonorailProjectName-oak"], should.Equal("oak-project"))
			assert.Loosely(t, settings["BuildBucketProjectFilter-oak"], should.Equal("oak-build"))
		})
	})
}

func TestWriteAllValues(t *testing.T) {
	t.Parallel()

	ftt.Run("write settings", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()

		t.Run("writeTrees", func(t *ftt.Test) {
			t.Run("basic", func(t *ftt.Test) {
				values := map[string]string{
					"Trees": "foo",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				tree := &model.Tree{
					Name: "foo",
				}
				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Foo"))
			})

			tree := &model.Tree{
				Name:        "oak",
				DisplayName: "Great Oaakk",
			}

			assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
			datastore.GetTestable(c).CatchupIndexes()

			t.Run("overwrite tree", func(t *ftt.Test) {
				values := map[string]string{
					"Trees": "oak",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
			})

			t.Run("overwrite tree with new display name", func(t *ftt.Test) {
				values := map[string]string{
					"Trees": "oak:Oaakk",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Oaakk"))
			})
		})

		t.Run("update AlertStreams", func(t *ftt.Test) {
			tree := &model.Tree{
				Name:        "oak",
				DisplayName: "Oak",
			}

			assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
			datastore.GetTestable(c).CatchupIndexes()

			t.Run("basic", func(t *ftt.Test) {
				values := map[string]string{
					"AlertStreams-oak": "thing,hello",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
				assert.Loosely(t, tree.AlertStreams, should.Resemble([]string{"thing", "hello"}))
			})

			t.Run("delete", func(t *ftt.Test) {
				values := map[string]string{
					"AlertStreams-oak": "",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
				assert.Loosely(t, tree.AlertStreams, should.Resemble([]string(nil)))
			})
		})

		t.Run("splitBugQueueLabels", func(t *ftt.Test) {
			t.Run("single", func(t *ftt.Test) {
				labelMap, err := splitBugQueueLabels(c, "oak:thing")
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, labelMap["oak"], should.Equal("thing"))
			})

			t.Run("mutiple", func(t *ftt.Test) {
				labelMap, err := splitBugQueueLabels(c, "oak:thing,maple:syrup,haha:haha")
				assert.Loosely(t, err, should.BeNil)

				assert.Loosely(t, labelMap["oak"], should.Equal("thing"))
				assert.Loosely(t, labelMap["maple"], should.Equal("syrup"))
				assert.Loosely(t, labelMap["haha"], should.Equal("haha"))
			})
		})

		t.Run("update BugQueueLabel", func(t *ftt.Test) {
			tree := &model.Tree{
				Name:          "oak",
				DisplayName:   "Oak",
				BugQueueLabel: "test",
			}

			assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
			datastore.GetTestable(c).CatchupIndexes()

			t.Run("basic", func(t *ftt.Test) {
				values := map[string]string{
					"BugQueueLabels": "oak:thing",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.Name, should.Equal("oak"))
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
				assert.Loosely(t, tree.BugQueueLabel, should.Equal("thing"))
			})

			t.Run("remove label", func(t *ftt.Test) {
				values := map[string]string{
					"BugQueueLabels": "oak:",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.Name, should.Equal("oak"))
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
				assert.Loosely(t, tree.BugQueueLabel, should.BeEmpty)
			})
		})

		t.Run("update HelpLink", func(t *ftt.Test) {
			tree := &model.Tree{
				Name:          "oak",
				DisplayName:   "Oak",
				HelpLink:      "Mwuhaha",
				BugQueueLabel: "ShouldNotChange",
			}

			assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
			datastore.GetTestable(c).CatchupIndexes()

			t.Run("basic", func(t *ftt.Test) {
				values := map[string]string{
					"HelpLink-oak": "http://google.com",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
				assert.Loosely(t, tree.HelpLink, should.Equal("http://google.com"))
				assert.Loosely(t, tree.BugQueueLabel, should.Equal("ShouldNotChange"))
			})

			t.Run("delete", func(t *ftt.Test) {
				values := map[string]string{
					"HelpLink-oak": "",
				}
				err := writeAllValues(c, values)
				assert.Loosely(t, err, should.BeNil)
				datastore.GetTestable(c).CatchupIndexes()

				assert.Loosely(t, datastore.Get(c, tree), should.BeNil)
				assert.Loosely(t, tree.DisplayName, should.Equal("Oak"))
				assert.Loosely(t, tree.HelpLink, should.BeEmpty)
				assert.Loosely(t, tree.BugQueueLabel, should.Equal("ShouldNotChange"))
			})
		})
	})
}
