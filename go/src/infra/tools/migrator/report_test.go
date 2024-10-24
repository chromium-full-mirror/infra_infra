// Copyright 2020 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package migrator

import (
	"reflect"
	"testing"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/config"
)

func TestReportID(t *testing.T) {
	t.Parallel()

	ftt.Run(`ReportID`, t, func(t *ftt.Test) {
		t.Run(`ConfigSet`, func(t *ftt.Test) {
			assert.Loosely(t, ReportID{Project: "foo"}.ConfigSet(), should.Resemble(
				config.Set("projects/foo")))
			assert.Loosely(t, ReportID{Project: "foo", ConfigFile: "irrelevant"}.ConfigSet(), should.Resemble(
				config.Set("projects/foo")))
		})

		t.Run(`String`, func(t *ftt.Test) {
			assert.Loosely(t, ReportID{Checkout: "checkout"}.String(), should.Match("checkout"))
			assert.Loosely(t, ReportID{Checkout: "checkout", Project: "foo"}.String(), should.Match("checkout|foo"))
			assert.Loosely(t, ReportID{Checkout: "checkout", Project: "foo", ConfigFile: "file"}.String(), should.Match("checkout|foo|file"))
		})
	})
}

func TestReport(t *testing.T) {
	t.Parallel()

	ftt.Run(`Report`, t, func(t *ftt.Test) {
		r := &Report{
			ReportID: ReportID{"checkout", "proj-foo", "config.file"},
			Tag:      "SOME_TAG",
			Problem:  "This is a problem.",
			Metadata: map[string]stringset.Set{
				"meta": stringset.NewFromSlice("value"),
			},
		}

		t.Run(`Clone`, func(t *ftt.Test) {
			ptr := func(a any) uintptr {
				return reflect.ValueOf(a).Pointer()
			}

			newR := r.Clone()
			assert.Loosely(t, r.ReportID, should.Resemble(newR.ReportID))
			assert.Loosely(t, r.Tag, should.Resemble(newR.Tag))
			assert.Loosely(t, r.Problem, should.Resemble(newR.Problem))
			assert.Loosely(t, ptr(r.Metadata), should.NotEqual(ptr(newR.Metadata)))                 // different maps
			assert.Loosely(t, ptr(r.Metadata["meta"]), should.NotEqual(ptr(newR.Metadata["meta"]))) // different sets
			assert.Loosely(t, r.Metadata["meta"].ToSlice(), should.Resemble(newR.Metadata["meta"].ToSlice()))
		})

		t.Run(`ToCSVRow`, func(t *ftt.Test) {
			assert.Loosely(t, r.ToCSVRow(), should.Resemble([]string{
				"checkout", "proj-foo", "config.file", "SOME_TAG", "This is a problem.", "false",
				"meta:value",
			}))

			r.Actionable = true
			assert.Loosely(t, r.ToCSVRow(), should.Resemble([]string{
				"checkout", "proj-foo", "config.file", "SOME_TAG", "This is a problem.", "true",
				"meta:value",
			}))
		})

		t.Run(`NewReportFromCSVRow`, func(t *ftt.Test) {
			t.Run(`Good`, func(t *ftt.Test) {
				report, err := NewReportFromCSVRow([]string{
					"checkout", "proj-foo", "config.file", "SOME_TAG", "This is a problem.",
					"true", "meta:value", "meta:other_value", "other_meta:1",
				})
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, report.ReportID, should.Resemble(ReportID{"checkout", "proj-foo", "config.file"}))
				assert.Loosely(t, report.Tag, should.Match("SOME_TAG"))
				assert.Loosely(t, report.Problem, should.Match("This is a problem."))
				assert.Loosely(t, report.Actionable, should.BeTrue)
				assert.Loosely(t, report.Metadata, should.Resemble(map[string]stringset.Set{
					"meta":       stringset.NewFromSlice("value", "other_value"),
					"other_meta": stringset.NewFromSlice("1"),
				}))
			})

			t.Run(`Bad`, func(t *ftt.Test) {
				t.Run(`no Checkout`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow(nil)
					assert.Loosely(t, err, should.ErrLike("Checkout field"))

					_, err = NewReportFromCSVRow([]string{""})
					assert.Loosely(t, err, should.ErrLike("Checkout field"))
				})

				t.Run(`no Project`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow([]string{"checkout"})
					assert.Loosely(t, err, should.ErrLike("Project field"))

					_, err = NewReportFromCSVRow([]string{"checkout", ""})
					assert.Loosely(t, err, should.ErrLike("Project field"))
				})

				t.Run(`no ConfigFile`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow([]string{"checkout", "proj-foo"})
					assert.Loosely(t, err, should.ErrLike("ConfigFile field"))

					_, err = NewReportFromCSVRow([]string{"checkout", "proj-foo", ""})
					assert.Loosely(t, err, should.ErrLike("Tag field"))
				})

				t.Run(`no Tag`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow([]string{"checkout", "proj-foo", ""})
					assert.Loosely(t, err, should.ErrLike("Tag field"))

					_, err = NewReportFromCSVRow([]string{"checkout", "proj-foo", "", ""})
					assert.Loosely(t, err, should.ErrLike("Tag field"))
				})

				t.Run(`no Problem`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow([]string{"checkout", "proj-foo", "", "TAG"})
					assert.Loosely(t, err, should.ErrLike("Problem field"))

					_, err = NewReportFromCSVRow([]string{"checkout", "proj-foo", "", "TAG", ""})
					assert.Loosely(t, err, should.ErrLike("Actionable field"))
				})

				t.Run(`no Actionable`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow([]string{"checkout", "proj-foo", "", "TAG", ""})
					assert.Loosely(t, err, should.ErrLike("Actionable field"))

					_, err = NewReportFromCSVRow([]string{"checkout", "proj-foo", "", "TAG", "", "true"})
					assert.Loosely(t, err, should.BeNil)
				})

				t.Run(`bad metadata`, func(t *ftt.Test) {
					_, err := NewReportFromCSVRow([]string{"checkout", "proj-foo", "", "TAG", "", "true", "bad"})
					assert.Loosely(t, err, should.ErrLike("Malformed metadata"))

					_, err = NewReportFromCSVRow([]string{"checkout", "proj-foo", "", "TAG", "", "true", "ok:value"})
					assert.Loosely(t, err, should.BeNil)
				})
			})
		})
	})
}
