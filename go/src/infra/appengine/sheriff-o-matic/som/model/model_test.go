// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"
)

var _ = fmt.Printf

func TestAnnotation(t *testing.T) {
	t.Parallel()

	ftt.Run("Annotation", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		cl := testclock.New(testclock.TestTimeUTC)
		c = clock.Set(c, cl)

		ann := &Annotation{
			Key:              "foobar",
			KeyDigest:        fmt.Sprintf("%x", sha1.Sum([]byte("foobar"))),
			ModificationTime: cl.Now(),
		}
		cl.Add(time.Hour)

		t.Run("allows weird keys", func(t *ftt.Test) {
			ann.Key = "hihih\"///////%20     lol"
			ann.KeyDigest = fmt.Sprintf("%x", sha1.Sum([]byte(ann.Key)))
			assert.Loosely(t, datastore.Put(c, ann), should.BeNil)
		})

		t.Run("allows long keys", func(t *ftt.Test) {
			// App engine key size limit is 500 characters
			ann.Key = strings.Repeat("annnn", 200)
			ann.KeyDigest = fmt.Sprintf("%x", sha1.Sum([]byte(ann.Key)))
			assert.Loosely(t, datastore.Put(c, ann), should.BeNil)
		})

		t.Run("with mocked checkAndGetBug", func(t *ftt.Test) {
			t.Run("add", func(t *ftt.Test) {
				t.Run("time", func(t *ftt.Test) {
					changeS := `{"snoozeTime":123123}`
					needRefresh, err := ann.Add(c, strings.NewReader(changeS))

					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, needRefresh, should.BeFalse)
					assert.Loosely(t, ann.SnoozeTime, should.Equal(123123))
					assert.Loosely(t, ann.Bugs, should.BeNil)
					assert.Loosely(t, ann.Comments, should.BeNil)
					assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now()))
				})

				t.Run("bugs", func(t *ftt.Test) {
					changeString := `{"bugs":[{"id": "123123", "projectId": "chromium"}]}`
					t.Run("basic", func(t *ftt.Test) {
						needRefresh, err := ann.Add(c, strings.NewReader(changeString))

						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, needRefresh, should.BeTrue)
						assert.Loosely(t, ann.SnoozeTime, should.BeZero)
						assert.Loosely(t, ann.Bugs, should.Resemble([]MonorailBug{{BugID: "123123", ProjectID: "chromium"}}))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now()))

						t.Run("duplicate bugs", func(t *ftt.Test) {
							cl.Add(time.Hour)
							needRefresh, err = ann.Add(c, strings.NewReader(changeString))
							assert.Loosely(t, err, should.BeNil)
							assert.Loosely(t, needRefresh, should.BeFalse)

							assert.Loosely(t, ann.SnoozeTime, should.BeZero)
							assert.Loosely(t, ann.Bugs, should.Resemble([]MonorailBug{{BugID: "123123", ProjectID: "chromium"}}))
							assert.Loosely(t, ann.Comments, should.BeNil)
							// We aren't changing the annotation, so the modification time shouldn't update.
							assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now().Add(-time.Hour)))
						})
					})

					t.Run("bug error", func(t *ftt.Test) {
						needRefresh, err := ann.Add(c, strings.NewReader("hi"))

						assert.Loosely(t, err, should.NotBeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.BeZero)
						assert.Loosely(t, ann.Bugs, should.BeNil)
						assert.Loosely(t, ann.Comments, should.BeNil)
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now().Add(-time.Hour)))
					})
				})

				t.Run("comments", func(t *ftt.Test) {
					changeString := `{"comments":["woah", "man", "comments"]}`
					t.Run("basic", func(t *ftt.Test) {
						needRefresh, err := ann.Add(c, strings.NewReader(changeString))
						tm := cl.Now()

						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.BeZero)
						assert.Loosely(t, ann.Bugs, should.BeNil)

						assert.Loosely(t, ann.Comments, should.Resemble([]Comment{{"woah", "", tm}, {"man", "", tm}, {"comments", "", tm}}))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(tm))
					})

					t.Run("comments error", func(t *ftt.Test) {
						needRefresh, err := ann.Add(c, strings.NewReader("plz don't add me"))

						assert.Loosely(t, err, should.NotBeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.BeZero)
						assert.Loosely(t, ann.Bugs, should.BeNil)
						assert.Loosely(t, ann.Comments, should.BeNil)
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now().Add(-time.Hour)))
					})
				})
			})

			t.Run("remove", func(t *ftt.Test) {
				tm := cl.Now()
				fakeComments := []Comment{{"hello", "", tm}, {"world", "", tm}, {"hehe", "", tm}}
				fakeBugs := []MonorailBug{{BugID: "123123", ProjectID: "chromium"}, {BugID: "bug2", ProjectID: "fuchsia"}}
				ann.SnoozeTime = 100
				ann.Bugs = fakeBugs
				ann.Comments = fakeComments

				t.Run("time", func(t *ftt.Test) {
					changeS := `{"snoozeTime":true}`
					needRefresh, err := ann.Remove(c, strings.NewReader(changeS))

					assert.Loosely(t, err, should.BeNil)
					assert.Loosely(t, needRefresh, should.BeFalse)
					assert.Loosely(t, ann.SnoozeTime, should.BeZero)
					assert.Loosely(t, ann.Bugs, should.Resemble(fakeBugs))
					assert.Loosely(t, ann.Comments, should.Resemble(fakeComments))
					assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now()))
				})

				t.Run("bugs", func(t *ftt.Test) {
					changeString := `{"bugs":[{"id": "123123", "projectId": "chromium"}]}`
					t.Run("basic", func(t *ftt.Test) {
						needRefresh, err := ann.Remove(c, strings.NewReader(changeString))

						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.Equal(100))
						assert.Loosely(t, ann.Comments, should.Resemble(fakeComments))
						assert.Loosely(t, ann.Bugs, should.Resemble([]MonorailBug{{BugID: "bug2", ProjectID: "fuchsia"}}))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now()))
					})

					t.Run("bug error", func(t *ftt.Test) {
						needRefresh, err := ann.Remove(c, strings.NewReader("badbugzman"))

						assert.Loosely(t, err, should.NotBeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.Equal(100))
						assert.Loosely(t, ann.Bugs, should.Resemble(fakeBugs))
						assert.Loosely(t, ann.Comments, should.Resemble(fakeComments))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now().Add(-time.Hour)))
					})
				})

				t.Run("comments", func(t *ftt.Test) {
					t.Run("basic", func(t *ftt.Test) {
						changeString := `{"comments":[1]}`
						needRefresh, err := ann.Remove(c, strings.NewReader(changeString))

						assert.Loosely(t, err, should.BeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.Equal(100))
						assert.Loosely(t, ann.Bugs, should.Resemble(fakeBugs))
						assert.Loosely(t, ann.Comments, should.Resemble([]Comment{{"hello", "", tm}, {"hehe", "", tm}}))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now()))
					})

					t.Run("bad format", func(t *ftt.Test) {
						needRefresh, err := ann.Remove(c, strings.NewReader("don't do this"))

						assert.Loosely(t, err, should.NotBeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.Equal(100))
						assert.Loosely(t, ann.Bugs, should.Resemble(fakeBugs))
						assert.Loosely(t, ann.Comments, should.Resemble(fakeComments))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now().Add(-time.Hour)))
					})

					t.Run("invalid index", func(t *ftt.Test) {
						changeString := `{"comments":[3]}`
						needRefresh, err := ann.Remove(c, strings.NewReader(changeString))

						assert.Loosely(t, err, should.NotBeNil)
						assert.Loosely(t, needRefresh, should.BeFalse)
						assert.Loosely(t, ann.SnoozeTime, should.Equal(100))
						assert.Loosely(t, ann.Bugs, should.Resemble(fakeBugs))
						assert.Loosely(t, ann.Comments, should.Resemble(fakeComments))
						assert.Loosely(t, ann.ModificationTime, should.Resemble(cl.Now().Add(-time.Hour)))
					})
				})

			})
		})
	})
}

func TestAlertJSONNonGroupingGetStepName(t *testing.T) {
	ftt.Run("valid ID", t, func(t *ftt.Test) {
		ann := &AlertJSONNonGrouping{
			ID: "tree$!project$!bucket$!builder$!step$!0",
		}
		stepName, err := ann.GetStepName()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, stepName, should.Equal("step"))
	})
	ftt.Run("invalid ID", t, func(t *ftt.Test) {
		ann := &AlertJSONNonGrouping{
			ID: "my key",
		}
		_, err := ann.GetStepName()
		assert.Loosely(t, err, should.NotBeNil)
	})
	ftt.Run("invalid ID 1", t, func(t *ftt.Test) {
		ann := &AlertJSONNonGrouping{
			ID: "1$!2$!3$!4$!5$!6$!7",
		}
		_, err := ann.GetStepName()
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestAnnotationGetStepName(t *testing.T) {
	c := gaetesting.TestingContext()
	ftt.Run("Get step name valid", t, func(t *ftt.Test) {
		ann := &Annotation{
			Tree: datastore.MakeKey(c, "Tree", "chromium"),
			Key:  "chromium.step_name",
		}
		stepName, err := ann.GetStepName()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, stepName, should.Equal("step_name"))
	})
	ftt.Run("Get step name when tree name containing dot", t, func(t *ftt.Test) {
		ann := &Annotation{
			Tree: datastore.MakeKey(c, "Tree", "chromium.clang"),
			Key:  "chromium.clang.step_name",
		}
		stepName, err := ann.GetStepName()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, stepName, should.Equal("step_name"))
	})
	ftt.Run("Get step name invalid", t, func(t *ftt.Test) {
		ann := &Annotation{
			Tree: datastore.MakeKey(c, "Tree", "chromium"),
			Key:  "step_name",
		}
		_, err := ann.GetStepName()
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func TestIsGroupAnnotation(t *testing.T) {
	ftt.Run("is group annotation returns true", t, func(t *ftt.Test) {
		ann := &Annotation{
			Key: "e2d935ac-c623-4c10-b1e3-73bb54584f8f",
		}
		assert.Loosely(t, ann.IsGroupAnnotation(), should.BeTrue)
	})
	ftt.Run("is group annotation returns false", t, func(t *ftt.Test) {
		ann := &Annotation{
			Key: "abcd",
		}
		assert.Loosely(t, ann.IsGroupAnnotation(), should.BeFalse)
	})
}
