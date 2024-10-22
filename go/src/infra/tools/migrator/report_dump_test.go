// Copyright 2020 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package migrator

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func mkReport(t *ftt.Test, row ...string) *Report {
	ret, err := NewReportFromCSVRow(row)
	assert.Loosely(t, err, should.BeNil)
	return ret
}

func TestReportDump(t *testing.T) {
	t.Parallel()

	ftt.Run(`ReportDump`, t, func(t *ftt.Test) {
		rd := &ReportDump{}

		assert.Loosely(t, rd.Empty(), should.BeTrue)

		t.Run(`Add`, func(t *ftt.Test) {
			rd.Add(
				mkReport(t, "checkout", "proj-foo", "some.file", "TAG_THIRD", "another problem", "true"),
				mkReport(t, "checkout", "proj-foo", "", "TAG", "problem", "true"),
				mkReport(t, "checkout", "proj-foo", "", "TAG_OTHER", "problem", "true"),
				mkReport(t, "checkout", "other-proj", "", "TAG", "problem", "true"),
			)

			assert.Loosely(t, rd.Empty(), should.BeFalse)
			assert.Loosely(t, rd.data, should.HaveLength(3))
			assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}], should.HaveLength(2))
			assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}][0].Tag, should.Match("TAG"))
			assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}][1].Tag, should.Match("TAG_OTHER"))
			assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", "some.file"}][0].Tag, should.Match("TAG_THIRD"))
			assert.Loosely(t, rd.data[ReportID{"checkout", "other-proj", ""}], should.HaveLength(1))
			assert.Loosely(t, rd.data[ReportID{"checkout", "other-proj", ""}][0].Tag, should.Match("TAG"))

			t.Run(`Iterate`, func(t *ftt.Test) {
				reports := []*Report{}
				rd.Iterate(func(rid ReportID, reps []*Report) bool {
					for _, r := range reps {
						assert.Loosely(t, rid, should.Resemble(r.ReportID))
					}
					reports = append(reports, reps...)
					return true
				})

				assert.Loosely(t, reports, should.HaveLength(4))
				assert.Loosely(t, reports[0].ReportID, should.Resemble(ReportID{"checkout", "other-proj", ""}))
				assert.Loosely(t, reports[1].ReportID, should.Resemble(ReportID{"checkout", "proj-foo", ""}))
				assert.Loosely(t, reports[2].ReportID, should.Resemble(ReportID{"checkout", "proj-foo", ""}))
				assert.Loosely(t, reports[3].ReportID, should.Resemble(ReportID{"checkout", "proj-foo", "some.file"}))
			})

			t.Run(`UpdateFrom`, func(t *ftt.Test) {
				assert.Loosely(t, rd.UpdateFrom(rd.Clone()), should.Equal(4))
				assert.Loosely(t, rd.data, should.HaveLength(3))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}], should.HaveLength(4))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}][0].Tag, should.Match("TAG"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}][1].Tag, should.Match("TAG_OTHER"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}][2].Tag, should.Match("TAG"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", ""}][3].Tag, should.Match("TAG_OTHER"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", "some.file"}][0].Tag, should.Match("TAG_THIRD"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "proj-foo", "some.file"}][1].Tag, should.Match("TAG_THIRD"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "other-proj", ""}], should.HaveLength(2))
				assert.Loosely(t, rd.data[ReportID{"checkout", "other-proj", ""}][0].Tag, should.Match("TAG"))
				assert.Loosely(t, rd.data[ReportID{"checkout", "other-proj", ""}][1].Tag, should.Match("TAG"))
			})

			t.Run(`Clone`, func(t *ftt.Test) {
				c := rd.Clone()
				assert.Loosely(t, reflect.ValueOf(c.data).Pointer(), should.NotEqual(reflect.ValueOf(rd.data).Pointer()))
				assert.Loosely(t, c.data, should.Resemble(rd.data))
			})
		})
	})
}

func TestReportCSV(t *testing.T) {
	t.Parallel()

	ftt.Run(`Report CSV`, t, func(t *ftt.Test) {
		t.Run(`Write`, func(t *ftt.Test) {
			rd := &ReportDump{}
			rd.Add(
				mkReport(t, "checkout", "proj-foo", "some.file", "TAG_THIRD", "another problem", "true"),
				mkReport(t, "checkout", "proj-foo", "", "TAG", "problem", "true", "meta:data", "a:value"),
				mkReport(t, "checkout", "proj-foo", "", "TAG_OTHER", "problem", "true"),
				mkReport(t, "checkout", "other-proj", "", "TAG", "problem", "true"),
				mkReport(t, "checkout", "a-third-prog", "", "TAG", "problem", "true"),
			)

			buf := &bytes.Buffer{}
			assert.Loosely(t, rd.WriteToCSV(buf), should.BeNil)

			csvRead := csv.NewReader(buf)
			csvRead.FieldsPerRecord = -1 // variable
			lines, err := csvRead.ReadAll()
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, lines, should.Resemble([][]string{
				csvHeader,
				{"checkout", "a-third-prog", "", "TAG", "problem", "true"},
				{"checkout", "other-proj", "", "TAG", "problem", "true"},
				{"checkout", "proj-foo", "", "TAG", "problem", "true", "a:value", "meta:data"},
				{"checkout", "proj-foo", "", "TAG_OTHER", "problem", "true"},
				{"checkout", "proj-foo", "some.file", "TAG_THIRD", "another problem", "true"},
			}))
		})

		t.Run(`Read`, func(t *ftt.Test) {
			t.Run(`OK`, func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				csvWrite := csv.NewWriter(buf)
				csvWrite.WriteAll([][]string{
					csvHeader,
					{"checkout", "a-third-prog", "", "TAG", "problem", "true"},
					{"checkout", "other-proj", "", "TAG", "problem", "true"},
					{"checkout", "proj-foo", "", "TAG", "problem", "true", "a:value", "meta:data"},
					{"checkout", "proj-foo", "", "TAG_OTHER", "problem", "true"},
					{"checkout", "proj-foo", "some.file", "TAG_THIRD", "another problem", "true"},
				})
				csvWrite.Flush()

				rd, err := NewReportDumpFromCSV(buf)
				assert.Loosely(t, err, should.BeNil)
				assert.Loosely(t, rd.data, should.Resemble(map[ReportID][]*Report{
					{"checkout", "proj-foo", ""}: {
						mkReport(t, "checkout", "proj-foo", "", "TAG", "problem", "true", "meta:data", "a:value"),
						mkReport(t, "checkout", "proj-foo", "", "TAG_OTHER", "problem", "true"),
					},
					{"checkout", "proj-foo", "some.file"}: {
						mkReport(t, "checkout", "proj-foo", "some.file", "TAG_THIRD", "another problem", "true"),
					},
					{"checkout", "other-proj", ""}: {
						mkReport(t, "checkout", "other-proj", "", "TAG", "problem", "true"),
					},
					{"checkout", "a-third-prog", ""}: {
						mkReport(t, "checkout", "a-third-prog", "", "TAG", "problem", "true"),
					},
				}))
			})

			t.Run(`Empty Header`, func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				buf.WriteString("\n")

				_, err := NewReportDumpFromCSV(buf)
				assert.Loosely(t, err, should.ErrLike("header was missing"))
			})

			t.Run(`Bad Schema`, func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				csvWrite := csv.NewWriter(buf)
				header := append([]string(nil), csvHeader[:len(csvHeader)-1]...)
				header = append(header, "{schema=v4}")
				csvWrite.Write(header)
				csvWrite.Flush()

				_, err := NewReportDumpFromCSV(buf)
				assert.Loosely(t, err, should.ErrLike("unexpected version: \"v4\", expected \"v3\""))
			})

			t.Run(`Bad Header length`, func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				csvWrite := csv.NewWriter(buf)
				csvWrite.WriteAll([][]string{
					{"some", "stuff"},
				})
				csvWrite.Flush()

				_, err := NewReportDumpFromCSV(buf)
				assert.Loosely(t, err, should.ErrLike("unexpected header:"))
			})

			t.Run(`Bad Header content`, func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				csvWrite := csv.NewWriter(buf)
				csvWrite.WriteAll([][]string{
					{"1", "2", "3", "4", "5", "6"},
				})
				csvWrite.Flush()

				_, err := NewReportDumpFromCSV(buf)
				assert.Loosely(t, err, should.ErrLike("unexpected header:"))
			})

			t.Run(`Bad Row`, func(t *ftt.Test) {
				buf := &bytes.Buffer{}
				csvWrite := csv.NewWriter(buf)
				csvWrite.WriteAll([][]string{
					csvHeader,
					{"wat"},
				})
				csvWrite.Flush()

				_, err := NewReportDumpFromCSV(buf)
				assert.Loosely(t, err, should.ErrLike("reading row 2"))
			})

		})
	})
}
