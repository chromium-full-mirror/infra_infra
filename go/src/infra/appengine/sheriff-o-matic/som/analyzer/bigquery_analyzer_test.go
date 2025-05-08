// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package analyzer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/appengine/gaetesting"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/analyzer/step"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model"
	"go.chromium.org/infra/monitoring/messages"
)

type mockResults struct {
	failures []failureRow
	err      error
	curr     int
}

func (m *mockResults) Next(dst any) error {
	if m.curr >= len(m.failures) {
		return iterator.Done
	}
	fdst := dst.(*failureRow)
	*fdst = m.failures[m.curr]
	m.curr++
	return m.err
}

func TestMockBQResults(t *testing.T) {
	ftt.Run("no results", t, func(t *ftt.Test) {
		mr := &mockResults{}
		r := &failureRow{}
		assert.Loosely(t, mr.Next(r), should.Equal(iterator.Done))
	})
	ftt.Run("copy op works", t, func(t *ftt.Test) {
		mr := &mockResults{
			failures: []failureRow{
				{
					StepName: "foo",
				},
			},
		}
		r := failureRow{}
		err := mr.Next(&r)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, r.StepName, should.Equal("foo"))
		assert.Loosely(t, mr.Next(&r), should.Equal(iterator.Done))
	})

}

func TestGenerateBuilderURL(t *testing.T) {
	ftt.Run("Test builder with no space", t, func(t *ftt.Test) {
		project := "chromium"
		bucket := "ci"
		builderName := "Win"
		url := generateBuilderURL(project, bucket, builderName)
		assert.Loosely(t, url, should.Equal("https://ci.chromium.org/p/chromium/builders/ci/Win"))
	})
	ftt.Run("Test builder with some spaces", t, func(t *ftt.Test) {
		project := "chromium"
		bucket := "ci"
		builderName := "Win 7 Test"
		url := generateBuilderURL(project, bucket, builderName)
		assert.Loosely(t, url, should.Equal("https://ci.chromium.org/p/chromium/builders/ci/Win%207%20Test"))
	})
	ftt.Run("Test builder with special characters", t, func(t *ftt.Test) {
		project := "chromium"
		bucket := "ci"
		builderName := "Mac 10.13 Tests (dbg)"
		url := generateBuilderURL(project, bucket, builderName)
		assert.Loosely(t, url, should.Equal("https://ci.chromium.org/p/chromium/builders/ci/Mac%2010.13%20Tests%20%28dbg%29"))
	})
}

func TestGenerateBuildURL(t *testing.T) {
	ftt.Run("Test build url with build ID", t, func(t *ftt.Test) {
		project := "chromium"
		bucket := "ci"
		builderName := "Win"
		buildID := bigquery.NullInt64{Int64: 8127364737474, Valid: true}
		url := generateBuildURL(project, bucket, builderName, buildID)
		assert.Loosely(t, url, should.Equal("https://ci.chromium.org/p/chromium/builders/ci/Win/b8127364737474"))
	})
	ftt.Run("Test build url with empty buildID", t, func(t *ftt.Test) {
		project := "chromium"
		bucket := "ci"
		builderName := "Win"
		buildID := bigquery.NullInt64{}
		url := generateBuildURL(project, bucket, builderName, buildID)
		assert.Loosely(t, url, should.BeEmpty)
	})
}

// Make SQL query uniform, for the purpose of testing
func formatQuery(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	query = regexp.MustCompile(`\s?\(\s?`).ReplaceAllString(query, "(")
	query = regexp.MustCompile(`\s?\)\s?`).ReplaceAllString(query, ")")
	return query
}

func TestGenerateSQLQuery(t *testing.T) {
	c := gaetesting.TestingContext()

	ftt.Run("Test generate SQL query for project", t, func(t *ftt.Test) {
		expected := `
			SELECT
			  Project,
			  Bucket,
			  Builder,
			  BuilderGroup,
			  SheriffRotations,
			  Critical,
			  StepName,
			  TestNamesFingerprint,
			  TestNamesTrunc,
			  TestsTrunc,
			  NumTests,
			  BuildIdBegin,
			  BuildIdEnd,
			  BuildNumberBegin,
			  BuildNumberEnd,
			  CPRangeOutputBegin,
			  CPRangeOutputEnd,
			  CPRangeInputBegin,
			  CPRangeInputEnd,
			  CulpritIdRangeBegin,
			  CulpritIdRangeEnd,
			  StartTime,
			  BuildStatus
			FROM
				` + "`sheriff-o-matic.chrome.sheriffable_failures`"
		actual := generateQueryForProject("sheriff-o-matic", "chrome")
		assert.Loosely(t, formatQuery(actual), should.Equal(formatQuery(expected)))
	})

	ftt.Run("Test generate SQL query for chromeos", t, func(t *ftt.Test) {
		treeName := "chromeos"
		tree := &model.Tree{
			Name: treeName,
		}
		assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
		datastore.GetTestable(c).CatchupIndexes()
		expected := `
			SELECT
			  Project,
			  Bucket,
			  Builder,
			  BuilderGroup,
			  SheriffRotations,
			  Critical,
			  StepName,
			  TestNamesFingerprint,
			  TestNamesTrunc,
			  TestsTrunc,
			  NumTests,
			  BuildIdBegin,
			  BuildIdEnd,
			  BuildNumberBegin,
			  BuildNumberEnd,
			  CPRangeOutputBegin,
			  CPRangeOutputEnd,
			  CPRangeInputBegin,
			  CPRangeInputEnd,
			  CulpritIdRangeBegin,
			  CulpritIdRangeEnd,
			  StartTime,
			  BuildStatus
			FROM
				` + "`sheriff-o-matic.chromeos.sheriffable_failures`" + `
			WHERE Project = "chromeos"
				AND ((Bucket IN ("postsubmit"))
                                     OR (Bucket IN ("release")
					 AND Builder LIKE "%-release-main"))
				AND (Critical != "NO" OR Critical is NULL)
		`
		actual, err := generateSQLQuery(c, treeName, "sheriff-o-matic")
		assert.Loosely(t, formatQuery(actual), should.Equal(formatQuery(expected)))
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Test generate SQL query for fuchsia", t, func(t *ftt.Test) {
		treeName := "fuchsia"
		tree := &model.Tree{
			Name:                     treeName,
			BuildBucketProjectFilter: "fuchsia-test",
		}
		assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
		datastore.GetTestable(c).CatchupIndexes()
		expected := `
			SELECT
			  Project,
			  Bucket,
			  Builder,
			  BuilderGroup,
			  SheriffRotations,
			  Critical,
			  StepName,
			  TestNamesFingerprint,
			  TestNamesTrunc,
			  TestsTrunc,
			  NumTests,
			  BuildIdBegin,
			  BuildIdEnd,
			  BuildNumberBegin,
			  BuildNumberEnd,
			  CPRangeOutputBegin,
			  CPRangeOutputEnd,
			  CPRangeInputBegin,
			  CPRangeInputEnd,
			  CulpritIdRangeBegin,
			  CulpritIdRangeEnd,
			  StartTime,
			  BuildStatus
			FROM
				` + "`sheriff-o-matic.fuchsia.sheriffable_failures`" + `
			WHERE
				Project = "fuchsia-test"
				AND Bucket = "global.ci"
			LIMIT
				1000
		`
		actual, err := generateSQLQuery(c, treeName, "sheriff-o-matic")
		assert.Loosely(t, formatQuery(actual), should.Equal(formatQuery(expected)))
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Test generate SQL query for angle", t, func(t *ftt.Test) {
		treeName := "angle"
		tree := &model.Tree{
			Name:                     treeName,
			BuildBucketProjectFilter: "angle-test",
		}
		assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
		datastore.GetTestable(c).CatchupIndexes()
		expected := `
			SELECT
			  Project,
			  Bucket,
			  Builder,
			  BuilderGroup,
			  SheriffRotations,
			  Critical,
			  StepName,
			  TestNamesFingerprint,
			  TestNamesTrunc,
			  TestsTrunc,
			  NumTests,
			  BuildIdBegin,
			  BuildIdEnd,
			  BuildNumberBegin,
			  BuildNumberEnd,
			  CPRangeOutputBegin,
			  CPRangeOutputEnd,
			  CPRangeInputBegin,
			  CPRangeInputEnd,
			  CulpritIdRangeBegin,
			  CulpritIdRangeEnd,
			  StartTime,
			  BuildStatus
			FROM
				` + "`sheriff-o-matic.angle.sheriffable_failures`" + `
			WHERE
				"angle" in UNNEST(SheriffRotations)
		`
		actual, err := generateSQLQuery(c, treeName, "sheriff-o-matic")
		assert.Loosely(t, formatQuery(actual), should.Equal(formatQuery(expected)))
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Test generate SQL query for devtools frontend", t, func(t *ftt.Test) {
		treeName := "devtools_frontend"
		tree := &model.Tree{
			Name: treeName,
		}
		assert.Loosely(t, datastore.Put(c, tree), should.BeNil)
		datastore.GetTestable(c).CatchupIndexes()
		expected := `
			SELECT
			  Project,
			  Bucket,
			  Builder,
			  BuilderGroup,
			  SheriffRotations,
			  Critical,
			  StepName,
			  TestNamesFingerprint,
			  TestNamesTrunc,
			  TestsTrunc,
			  NumTests,
			  BuildIdBegin,
			  BuildIdEnd,
			  BuildNumberBegin,
			  BuildNumberEnd,
			  CPRangeOutputBegin,
			  CPRangeOutputEnd,
			  CPRangeInputBegin,
			  CPRangeInputEnd,
			  CulpritIdRangeBegin,
			  CulpritIdRangeEnd,
			  StartTime,
			  BuildStatus
			FROM
				` + "`sheriff-o-matic.devtools_frontend.sheriffable_failures`" + `
			WHERE
				"devtools_frontend" in UNNEST(SheriffRotations)
		`
		actual, err := generateSQLQuery(c, treeName, "sheriff-o-matic")
		assert.Loosely(t, formatQuery(actual), should.Equal(formatQuery(expected)))
		assert.Loosely(t, err, should.BeNil)
	})

	ftt.Run("Test generate SQL query for invalid tree", t, func(t *ftt.Test) {
		_, err := generateSQLQuery(c, "abc", "sheriff-o-matic")
		assert.Loosely(t, err, should.NotBeNil)
	})
}

type mockBuildersClient struct{}

func (mbc mockBuildersClient) ListBuilders(c context.Context, req *buildbucketpb.ListBuildersRequest, opts ...grpc.CallOption) (*buildbucketpb.ListBuildersResponse, error) {
	if req.Bucket == "ci" {
		return &buildbucketpb.ListBuildersResponse{
			Builders: []*buildbucketpb.BuilderItem{
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "ci",
						Builder: "ci_1",
					},
				},
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "ci",
						Builder: "ci_2",
					},
				},
			},
		}, nil
	}
	if req.Bucket == "try" {
		return &buildbucketpb.ListBuildersResponse{
			Builders: []*buildbucketpb.BuilderItem{
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "try",
						Builder: "try_1",
					},
				},
				{
					Id: &buildbucketpb.BuilderID{
						Project: "chromium",
						Bucket:  "try",
						Builder: "try_2",
					},
				},
			},
		}, nil
	}
	if req.Bucket == "err" {
		return nil, fmt.Errorf("some infra error")
	}
	if req.Bucket == "notfound" {
		return nil, status.Error(codes.NotFound, "Not found")
	}

	return nil, nil
}

func TestFilterDeletedBuilders(t *testing.T) {
	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)
	cl := mockBuildersClient{}

	ftt.Run("no builder", t, func(t *ftt.Test) {
		failureRows := []failureRow{}
		filtered, err := filterDeletedBuildersWithClient(ctx, cl, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.BeEmpty)
	})

	ftt.Run("builders belong to one bucket", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_3",
			},
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_2",
			},
		}
		filtered, err := filterDeletedBuildersWithClient(ctx, cl, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.Resemble([]failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_2",
			},
		}))
	})

	ftt.Run("builders belong to more than one buckets", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_3",
			},
			{
				Project: "chromium",
				Bucket:  "try",
				Builder: "try_3",
			},
			{
				Project: "chromium",
				Bucket:  "try",
				Builder: "try_1",
			},
		}
		filtered, err := filterDeletedBuildersWithClient(ctx, cl, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.Resemble([]failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
			{
				Project: "chromium",
				Bucket:  "try",
				Builder: "try_1",
			},
		}))
	})

	ftt.Run("rpc returns errors", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
			{
				Project: "chromium",
				Bucket:  "err",
				Builder: "err_1",
			},
		}
		_, err := filterDeletedBuildersWithClient(ctx, cl, failureRows)
		assert.Loosely(t, err, should.NotBeNil)
	})

	ftt.Run("rpc returns NotFound", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
			{
				Project: "chromium",
				Bucket:  "notfound",
				Builder: "notfound_1",
			},
		}
		filtered, err := filterDeletedBuildersWithClient(ctx, cl, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, filtered, should.Resemble([]failureRow{
			{
				Project: "chromium",
				Bucket:  "ci",
				Builder: "ci_1",
			},
		}))
	})
}

func TestProcessBQResults(t *testing.T) {
	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)

	ftt.Run("smoke", t, func(t *ftt.Test) {
		failureRows := []failureRow{}
		got, err := processBQResults(ctx, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, got, should.BeEmpty)
	})

	ftt.Run("single result, only start/end build numbers", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "some builder",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(1))
	})

	ftt.Run("single result, only end build number", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "some builder",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(1))
	})

	ftt.Run("single result, start/end build numbers, single test name", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "some builder",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("1"),
				NumTests: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(1))
		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 1,
			Tests:           makeTestWithResults("1"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
	})

	ftt.Run("multiple results, start/end build numbers, same step, same test name", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 1",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("1"),
				NumTests: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
			},
			{
				StepName: "some step",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 2",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestNamesTrunc: bigquery.NullString{
					StringVal: "some/test/name",
					Valid:     true,
				},
				TestsTrunc: makeTestFailures("1"),
				NumTests: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(2))
		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 1,
			Tests:           makeTestWithResults("1"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
		assert.Loosely(t, len(got[1].Builders), should.Equal(1))
	})

	ftt.Run("multiple results, start/end build numbers, different steps, different sets of test names", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step 1",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 1",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("1", "2"),
				NumTests: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
			},
			{
				StepName: "some step 2",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 2",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("3"),
				NumTests: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		sort.Sort(byStepName(got))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(2))

		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step 1",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 2,
			Tests:           makeTestWithResults("1", "2"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))

		reason = got[1].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step 2",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 1,
			Tests:           makeTestWithResults("3"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
	})

	ftt.Run("multiple results, start/end build numbers, same step, different sets of test names", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step 1",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 1",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("1", "2"),
				NumTests: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
			},
			{
				StepName: "some step 1",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 2",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("3"),
				NumTests: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		sort.Sort(byTests(got))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(2))

		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step 1",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 2,
			Tests:           makeTestWithResults("1", "2"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
		assert.Loosely(t, got[0].Builders[0].Name, should.Equal("builder 1"))

		reason = got[1].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step 1",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 1,
			Tests:           makeTestWithResults("3"),
		}))
		assert.Loosely(t, len(got[1].Builders), should.Equal(1))
		assert.Loosely(t, got[1].Builders[0].Name, should.Equal("builder 2"))
	})

	ftt.Run("chromium.perf case: multiple results, different start build numbers, same end build number, same step, different sets of test names", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "performance_test_suite",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "win-10-perf",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 110,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("A1", "A2", "A3"),
				NumTests: bigquery.NullInt64{
					Int64: 3,
					Valid: true,
				},
			},
			{
				StepName: "performance_test_suite",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "win-10-perf",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 102,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 110,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("B1", "B2", "B3"),
				NumTests: bigquery.NullInt64{
					Int64: 3,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		sort.Sort(byTests(got))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(2))

		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "performance_test_suite",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 3,
			Tests:           makeTestWithResults("A1", "A2", "A3"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
		assert.Loosely(t, got[0].Builders[0].Name, should.Equal("win-10-perf"))
		assert.Loosely(t, got[0].Builders[0].FirstFailure, should.Equal(100))
		assert.Loosely(t, got[0].Builders[0].LatestFailure, should.Equal(110))

		reason = got[1].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "performance_test_suite",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 3,
			Tests:           makeTestWithResults("B1", "B2", "B3"),
		}))
		assert.Loosely(t, len(got[1].Builders), should.Equal(1))
		assert.Loosely(t, got[1].Builders[0].Name, should.Equal("win-10-perf"))
		assert.Loosely(t, got[1].Builders[0].FirstFailure, should.Equal(102))
		assert.Loosely(t, got[1].Builders[0].LatestFailure, should.Equal(110))
	})

	ftt.Run("chromium.perf case: multiple results, same step, same truncated list of test names, different test name fingerprints", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "performance_test_suite",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "win-10-perf",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 110,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("A1", "A2", "A3"),
				NumTests: bigquery.NullInt64{
					Int64: 3,
					Valid: true,
				},
			},
			{
				StepName: "performance_test_suite",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "win-10-perf",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 102,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 110,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("A1", "A2", "A3"),
				NumTests: bigquery.NullInt64{
					Int64: 3,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		sort.Sort(byFirstFailure(got))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(2))

		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "performance_test_suite",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 3,
			Tests:           makeTestWithResults("A1", "A2", "A3"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))

		assert.Loosely(t, got[0].Builders[0].Name, should.Equal("win-10-perf"))
		assert.Loosely(t, got[0].Builders[0].FirstFailure, should.Equal(100))
		assert.Loosely(t, got[0].Builders[0].LatestFailure, should.Equal(110))

		reason = got[1].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "performance_test_suite",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 3,
			Tests:           makeTestWithResults("A1", "A2", "A3"),
		}))
		assert.Loosely(t, len(got[1].Builders), should.Equal(1))
		assert.Loosely(t, got[1].Builders[0].Name, should.Equal("win-10-perf"))
		assert.Loosely(t, got[1].Builders[0].FirstFailure, should.Equal(102))
		assert.Loosely(t, got[1].Builders[0].LatestFailure, should.Equal(110))
	})

	ftt.Run("multiple results, start/end build numbers, different steps, same set of test names", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step 1",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 1",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("1", "2"),
				NumTests: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
			},
			{
				StepName: "some step 2",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder: "builder 2",
				Project: "some project",
				Bucket:  "some bucket",
				BuildIDBegin: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				BuildIDEnd: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestsTrunc: makeTestFailures("1", "2"),
				NumTests: bigquery.NullInt64{
					Int64: 2,
					Valid: true,
				},
			},
		}
		got, err := processBQResults(ctx, failureRows)
		sort.Sort(byStepName(got))
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, len(got), should.Equal(2))
		reason := got[0].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step 1",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 2,
			Tests:           makeTestWithResults("1", "2"),
		}))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))

		reason = got[1].Reason
		assert.Loosely(t, reason, should.NotBeNil)
		assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
			Name:            "some step 2",
			kind:            "test",
			severity:        messages.ReliableFailure,
			NumFailingTests: 2,
			Tests:           makeTestWithResults("1", "2"),
		}))
		assert.Loosely(t, len(got[1].Builders), should.Equal(1))
	})

	ftt.Run("process changepoint result", t, func(t *ftt.Test) {
		failureRows := []failureRow{
			{
				StepName: "some step",
				BuilderGroup: bigquery.NullString{
					StringVal: "some builder group",
					Valid:     true,
				},
				Builder:    "some builder",
				Project:    "some project",
				Bucket:     "some bucket",
				TestsTrunc: makeTestFailures("1"),
				NumTests: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
				TestNamesFingerprint: bigquery.NullInt64{
					Int64: 1,
					Valid: true,
				},
			},
		}
		t.Run("one segment", func(t *ftt.Test) {
			failureRows[0].TestsTrunc[0].Segments = []*segment{{
				StartHour: bigquery.NullTimestamp{
					Timestamp: time.Unix(3600*11, 0),
					Valid:     true,
				},
				CountUnexpectedResults: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
				CountTotalResults: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
			}}
			result := step.TestWithResult{
				TestName:                "some/test/1",
				TestID:                  "ninja://some/test/1",
				VariantHash:             "12341",
				ClusterName:             "reason-v3/1",
				Realm:                   "chromium:1",
				RefHash:                 "ref-hash/1",
				RegressionStartPosition: 0,
				RegressionEndPosition:   0,
				CurStartHour:            time.Unix(3600*11, 0),
				PrevEndHour:             time.Time{},
				CurCounts: step.Counts{
					UnexpectedResults: 10,
					TotalResults:      100,
				},
				PrevCounts: step.Counts{},
			}
			got, err := processBQResults(ctx, failureRows)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(got), should.Equal(1))
			reason := got[0].Reason
			assert.Loosely(t, reason, should.NotBeNil)
			assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
				Name:            "some step",
				kind:            "test",
				severity:        messages.ReliableFailure,
				NumFailingTests: 1,
				Tests:           []step.TestWithResult{result},
			}))
			assert.Loosely(t, len(got[0].Builders), should.Equal(1))
		})
		t.Run("two segments, deterministic failure", func(t *ftt.Test) {
			failureRows[0].TestsTrunc[0].Segments = []*segment{{
				StartHour: bigquery.NullTimestamp{
					Timestamp: time.Unix(3600*11, 0),
					Valid:     true,
				},
				CountUnexpectedResults: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
				CountTotalResults: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
				StartPosition: bigquery.NullInt64{
					Int64: 6,
					Valid: true,
				},
				StartPositionLowerBound99Th: bigquery.NullInt64{
					Int64: 5,
					Valid: true,
				},
				StartPositionUpperBound99Th: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
			},
				{
					EndHour: bigquery.NullTimestamp{
						Timestamp: time.Unix(3600*10, 0),
						Valid:     true,
					},
					EndPosition: bigquery.NullInt64{
						Int64: 4,
						Valid: true,
					},
					CountUnexpectedResults: bigquery.NullInt64{
						Int64: 0,
						Valid: true,
					},
					CountTotalResults: bigquery.NullInt64{
						Int64: 100,
						Valid: true,
					},
				}}
			result := step.TestWithResult{
				TestName:                "some/test/1",
				TestID:                  "ninja://some/test/1",
				VariantHash:             "12341",
				ClusterName:             "reason-v3/1",
				Realm:                   "chromium:1",
				RefHash:                 "ref-hash/1",
				RegressionStartPosition: 4,
				RegressionEndPosition:   6,
				CurStartHour:            time.Unix(3600*11, 0),
				PrevEndHour:             time.Unix(3600*10, 0),
				CurCounts: step.Counts{
					UnexpectedResults: 100,
					TotalResults:      100,
				},
				PrevCounts: step.Counts{
					UnexpectedResults: 0,
					TotalResults:      100,
				},
			}
			got, err := processBQResults(ctx, failureRows)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(got), should.Equal(1))
			reason := got[0].Reason
			assert.Loosely(t, reason, should.NotBeNil)
			assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
				Name:            "some step",
				kind:            "test",
				severity:        messages.ReliableFailure,
				NumFailingTests: 1,
				Tests:           []step.TestWithResult{result},
			}))
			assert.Loosely(t, len(got[0].Builders), should.Equal(1))
		})
		t.Run("two segments, non-deterministic failure", func(t *ftt.Test) {
			failureRows[0].TestsTrunc[0].Segments = []*segment{{
				StartHour: bigquery.NullTimestamp{
					Timestamp: time.Unix(3600*11, 0),
					Valid:     true,
				},
				CountUnexpectedResults: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
				CountTotalResults: bigquery.NullInt64{
					Int64: 100,
					Valid: true,
				},
				StartPosition: bigquery.NullInt64{
					Int64: 6,
					Valid: true,
				},
				StartPositionLowerBound99Th: bigquery.NullInt64{
					Int64: 5,
					Valid: true,
				},
				StartPositionUpperBound99Th: bigquery.NullInt64{
					Int64: 10,
					Valid: true,
				},
			},
				{
					EndHour: bigquery.NullTimestamp{
						Timestamp: time.Unix(3600*10, 0),
						Valid:     true,
					},
					EndPosition: bigquery.NullInt64{
						Int64: 4,
						Valid: true,
					},
					CountUnexpectedResults: bigquery.NullInt64{
						Int64: 1,
						Valid: true,
					},
					CountTotalResults: bigquery.NullInt64{
						Int64: 100,
						Valid: true,
					},
				}}
			result := step.TestWithResult{
				TestName:                "some/test/1",
				TestID:                  "ninja://some/test/1",
				VariantHash:             "12341",
				ClusterName:             "reason-v3/1",
				Realm:                   "chromium:1",
				RefHash:                 "ref-hash/1",
				RegressionStartPosition: 4,
				RegressionEndPosition:   10,
				CurStartHour:            time.Unix(3600*11, 0),
				PrevEndHour:             time.Unix(3600*10, 0),
				CurCounts: step.Counts{
					UnexpectedResults: 100,
					TotalResults:      100,
				},
				PrevCounts: step.Counts{
					UnexpectedResults: 1,
					TotalResults:      100,
				},
			}
			got, err := processBQResults(ctx, failureRows)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, len(got), should.Equal(1))
			reason := got[0].Reason
			assert.Loosely(t, reason, should.NotBeNil)
			assert.Loosely(t, reason.Raw, should.Resemble(&BqFailure{
				Name:            "some step",
				kind:            "test",
				severity:        messages.ReliableFailure,
				NumFailingTests: 1,
				Tests:           []step.TestWithResult{result},
			}))
			assert.Loosely(t, len(got[0].Builders), should.Equal(1))
		})
	})
}

type byFirstFailure []*messages.BuildFailure

func (f byFirstFailure) Len() int      { return len(f) }
func (f byFirstFailure) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f byFirstFailure) Less(i, j int) bool {
	return f[i].Builders[0].FirstFailure < f[j].Builders[0].FirstFailure
}

type byTests []*messages.BuildFailure

func (f byTests) Len() int      { return len(f) }
func (f byTests) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f byTests) Less(i, j int) bool {
	iTests, jTests := []string{}, []string{}
	for _, t := range f[i].Reason.Raw.(*BqFailure).Tests {
		iTests = append(iTests, t.TestName)
	}
	for _, t := range f[j].Reason.Raw.(*BqFailure).Tests {
		jTests = append(jTests, t.TestName)
	}

	return strings.Join(iTests, "\n") < strings.Join(jTests, "\n")
}

func TestFilterHierarchicalSteps(t *testing.T) {
	ftt.Run("smoke", t, func(t *ftt.Test) {
		failures := []*messages.BuildFailure{}
		got := filterHierarchicalSteps(failures)
		assert.Loosely(t, len(got), should.BeZero)
	})

	ftt.Run("single step, single builder", t, func(t *ftt.Test) {
		failures := []*messages.BuildFailure{
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results",
					},
				},
			},
		}

		got := filterHierarchicalSteps(failures)
		assert.Loosely(t, len(got), should.Equal(1))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
	})

	ftt.Run("nested step, single builder", t, func(t *ftt.Test) {
		failures := []*messages.BuildFailure{
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results|build results",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results|build results|chromeos.postsubmit.beaglebone_servo-postsubmit",
					},
				},
			},
		}

		got := filterHierarchicalSteps(failures)
		assert.Loosely(t, len(got), should.Equal(1))
		assert.Loosely(t, len(got[0].Builders), should.Equal(1))
	})

	ftt.Run("single step, multiple builders", t, func(t *ftt.Test) {
		failures := []*messages.BuildFailure{
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results",
					},
				},
			},
		}

		got := filterHierarchicalSteps(failures)
		assert.Loosely(t, len(got), should.Equal(1))
		assert.Loosely(t, len(got[0].Builders), should.Equal(2))
	})

	ftt.Run("nested step, multiple builder", t, func(t *ftt.Test) {
		failures := []*messages.BuildFailure{
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results|build results",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results|build results|chromeos.postsubmit.beaglebone_servo-postsubmit",
					},
				},
			},
		}

		got := filterHierarchicalSteps(failures)
		assert.Loosely(t, len(got), should.Equal(1))
		assert.Loosely(t, len(got[0].Builders), should.Equal(2))
		assert.Loosely(t, got[0].StepAtFault.Step.Name, should.Equal("check build results|build results|chromeos.postsubmit.beaglebone_servo-postsubmit"))
	})

	ftt.Run("mixed nested steps, multiple builder", t, func(t *ftt.Test) {
		failures := []*messages.BuildFailure{
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "test foo",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "test bar",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "test baz",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results|build results",
					},
				},
			},
			{
				Builders: []*messages.AlertedBuilder{
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name A",
					},
					{
						Project: "project",
						Bucket:  "bucket",
						Name:    "builder name B",
					},
				},
				StepAtFault: &messages.BuildStep{
					Step: &messages.Step{
						Name: "check build results|build results|chromeos.postsubmit.beaglebone_servo-postsubmit",
					},
				},
			},
		}

		got := filterHierarchicalSteps(failures)
		assert.Loosely(t, len(got), should.Equal(4))
		assert.Loosely(t, len(got[0].Builders), should.Equal(2))
		assert.Loosely(t, got[0].StepAtFault.Step.Name, should.Equal("test foo"))
		assert.Loosely(t, len(got[1].Builders), should.Equal(1))
		assert.Loosely(t, got[1].StepAtFault.Step.Name, should.Equal("test bar"))
		assert.Loosely(t, len(got[2].Builders), should.Equal(1))
		assert.Loosely(t, got[2].StepAtFault.Step.Name, should.Equal("test baz"))
		assert.Loosely(t, len(got[3].Builders), should.Equal(2))
		assert.Loosely(t, got[3].StepAtFault.Step.Name, should.Equal("check build results|build results|chromeos.postsubmit.beaglebone_servo-postsubmit"))
	})
}

func TestSliceContains(t *testing.T) {
	ftt.Run("slice contains", t, func(t *ftt.Test) {
		haystack := []string{"a", "b", "c"}
		assert.Loosely(t, sliceContains(haystack, "a"), should.BeTrue)
		assert.Loosely(t, sliceContains(haystack, "b"), should.BeTrue)
		assert.Loosely(t, sliceContains(haystack, "c"), should.BeTrue)
		assert.Loosely(t, sliceContains(haystack, "d"), should.BeFalse)
	})
}

func TestZipUnzipData(t *testing.T) {
	ftt.Run("zip and unzip data", t, func(t *ftt.Test) {
		data := []byte("abcdef")
		zippedData, err := zipData(data)
		assert.Loosely(t, err, should.BeNil)
		unzippedData, err := unzipData(zippedData)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, unzippedData, should.Resemble(data))
	})
}

func TestGetFilterFuncForTree(t *testing.T) {
	ftt.Run("get filter func for tree", t, func(t *ftt.Test) {
		_, err := getFilterFuncForTree("android")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("chromium")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("chromium.gpu")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("chromium.perf")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("ios")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("chrome_browser_release")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("chromium.clang")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("dawn")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("chromiumos")
		assert.Loosely(t, err, should.BeNil)
		_, err = getFilterFuncForTree("another")
		assert.Loosely(t, err, should.NotBeNil)
	})
}

func makeTestFailures(uniquifiers ...string) []*TestFailure {
	failures := make([]*TestFailure, 0, len(uniquifiers))
	for _, u := range uniquifiers {
		failures = append(failures, makeTestFailure(u))
	}
	return failures
}

func makeTestFailure(uniquifier string) *TestFailure {
	return &TestFailure{
		TestName:    bigquery.NullString{StringVal: fmt.Sprintf("some/test/%s", uniquifier), Valid: true},
		TestID:      bigquery.NullString{StringVal: fmt.Sprintf("ninja://some/test/%s", uniquifier), Valid: true},
		Realm:       bigquery.NullString{StringVal: fmt.Sprintf("chromium:%s", uniquifier), Valid: true},
		VariantHash: bigquery.NullString{StringVal: fmt.Sprintf("1234%s", uniquifier), Valid: true},
		ClusterName: bigquery.NullString{StringVal: fmt.Sprintf("reason-v3/%s", uniquifier), Valid: true},
		RefHash:     bigquery.NullString{StringVal: fmt.Sprintf("ref-hash/%s", uniquifier), Valid: true},
	}
}

func makeTestWithResults(uniquifiers ...string) []step.TestWithResult {
	failures := make([]step.TestWithResult, 0, len(uniquifiers))
	for _, u := range uniquifiers {
		failures = append(failures, makeTestWithResult(u))
	}
	return failures
}

func makeTestWithResult(uniquifier string) step.TestWithResult {
	return step.TestWithResult{
		TestName:    fmt.Sprintf("some/test/%s", uniquifier),
		TestID:      fmt.Sprintf("ninja://some/test/%s", uniquifier),
		VariantHash: fmt.Sprintf("1234%s", uniquifier),
		ClusterName: fmt.Sprintf("reason-v3/%s", uniquifier),
		Realm:       fmt.Sprintf("chromium:%s", uniquifier),
		RefHash:     fmt.Sprintf("ref-hash/%s", uniquifier),
	}
}
