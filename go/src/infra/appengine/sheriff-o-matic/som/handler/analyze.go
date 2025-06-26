// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package handler

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/appengine"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	bisectionpb "go.chromium.org/luci/bisection/proto/v1"
	"go.chromium.org/luci/common/bq"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/info"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/analyzer"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/analyzer/step"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/client"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model"
	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model/gen"
	"go.chromium.org/infra/monitoring/messages"
)

const (
	bqDatasetID = "events"
	bqTableID   = "alerts"
)

var errStatus = func(c context.Context, w http.ResponseWriter, status int, msg string) {
	logging.Errorf(c, "Status %d msg %s", status, msg)
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

type bySeverity []*messages.Alert

func (a bySeverity) Len() int      { return len(a) }
func (a bySeverity) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a bySeverity) Less(i, j int) bool {
	return a[i].Severity < a[j].Severity
}

type ctxKeyType string

var analyzerCtxKey = ctxKeyType("analyzer")

// WithAnalyzer returns a context with a attached as a context value.
func WithAnalyzer(ctx context.Context, a *analyzer.Analyzer) context.Context {
	return context.WithValue(ctx, analyzerCtxKey, a)
}

// GetAnalyzeHandler enqueues a request to run an analysis on a particular tree.
// This is usually hit by appengine cron rather than manually.
func GetAnalyzeHandler(ctx *router.Context) {
	c, w, r, p := ctx.Request.Context(), ctx.Writer, ctx.Request, ctx.Params

	tree := p.ByName("tree")
	a, ok := c.Value(analyzerCtxKey).(*analyzer.Analyzer)
	if !ok {
		errStatus(c, w, http.StatusInternalServerError, "no analyzer set in Context")
		return
	}
	var alertsSummary *messages.AlertsSummary
	var err error
	c = appengine.WithContext(c, r)

	alertsSummary, err = generateBigQueryAlerts(c, a, tree)

	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	alertsSummary.Timestamp = messages.TimeToEpochTime(time.Now())
	if err := putAlertsBigQuery(c, tree, alertsSummary); err != nil {
		logging.Errorf(c, "error sending alerts to bigquery: %v", err)
		// Not fatal, just log and continue.
	}

	w.Write([]byte("ok"))
}

// GetBQQueryHandler queries BQ sheriffable_failures for a particular project, and caches the result.
// This is usually hit by appengine cron rather than manually.
func GetBQQueryHandler(ctx *router.Context) {
	c, w, r, p := ctx.Request.Context(), ctx.Writer, ctx.Request, ctx.Params
	c = appengine.WithContext(c, r)
	project := p.ByName("project")
	err := analyzer.QueryBQForProject(c, project)
	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte("ok"))
}

func generateBigQueryAlerts(c context.Context, a *analyzer.Analyzer, tree string) (*messages.AlertsSummary, error) {
	configRules, err := analyzer.GetConfigRules(c)
	if err != nil {
		logging.Errorf(c, "error getting config rules: %v", err)
		return nil, err
	}

	builderAlerts, err := analyzer.GetBigQueryAlerts(c, tree)
	if err != nil {
		return nil, err
	}

	// Filter out ignored builders/steps.
	filteredBuilderAlerts := []*messages.BuildFailure{}
	for _, ba := range builderAlerts {
		builders := []*messages.AlertedBuilder{}
		for _, b := range ba.Builders {
			// The chromium.clang tree specifically wants all of the failures.
			// Some other trees, who also reference chromium.clang builders do *not* want all of them.
			// This extra tree == "chromium.clang" condition works around this shortcoming of the gatekeeper
			// tree config format.
			if tree == "chromium.clang" || !configRules.ExcludeFailure(c, b.BuilderGroup, b.Name, ba.StepAtFault.Step.Name) {
				builders = append(builders, b)
			}
		}
		if len(builders) > 0 {
			ba.Builders = builders
			filteredBuilderAlerts = append(filteredBuilderAlerts, ba)
		}
	}
	logging.Infof(c, "filtered alerts, before: %d after: %d", len(builderAlerts), len(filteredBuilderAlerts))
	err = attachLuciBisectionResults(c, filteredBuilderAlerts, a.Bisection)
	if err != nil {
		// It is not critical, so log and continue
		logging.Errorf(c, "Failure getting LUCI Bisection results %v", err)
	}

	alerts := []*messages.Alert{}
	for _, ba := range filteredBuilderAlerts {
		title := fmt.Sprintf("Step %q failing on %d builder(s)", ba.StepAtFault.Step.Name, len(ba.Builders))
		// TODO(crbug.com/1043371): Remove the if condition after we disable automatic grouping.
		if len(ba.Builders) == 1 {
			title = fmt.Sprintf("Step %q failing on builder %q", ba.StepAtFault.Step.Name, getBuilderId(ba.Builders[0]))
		}
		startTime := messages.TimeToEpochTime(time.Now())
		severity := messages.NewFailure
		for _, b := range ba.Builders {
			if b.StartTime > 0 && b.StartTime < startTime {
				startTime = b.StartTime
			}
			if b.LatestFailure-b.FirstFailure != 0 {
				severity = messages.ReliableFailure
			}
		}

		alert := &messages.Alert{
			Key:       getKeyForAlert(c, ba, tree),
			Title:     title,
			Extension: ba,
			StartTime: startTime,
			Severity:  severity,
		}

		switch ba.Reason.Kind() {
		case "test":
			alert.Type = messages.AlertTestFailure
		default:
			alert.Type = messages.AlertBuildFailure
		}

		alerts = append(alerts, alert)
	}

	logging.Infof(c, "%d alerts generated for tree %q", len(alerts), tree)

	alertsSummary := &messages.AlertsSummary{
		Timestamp:         messages.TimeToEpochTime(time.Now()),
		RevisionSummaries: map[string]*messages.RevisionSummary{},
		Alerts:            alerts,
	}

	if err := storeAlertsSummary(c, a, tree, alertsSummary); err != nil {
		logging.Errorf(c, "error storing alerts: %v", err)
		return nil, err
	}

	return alertsSummary, nil
}

func getBuilderId(builder *messages.AlertedBuilder) string {
	return fmt.Sprintf("%s/%s/%s", builder.Project, builder.Bucket, builder.Name)
}

func getKeyForAlert(ctx context.Context, bf *messages.BuildFailure, tree string) string {
	step := bf.StepAtFault.Step.Name
	project := bf.Builders[0].Project
	bucket := bf.Builders[0].Bucket
	builder := bf.Builders[0].Name
	firstFailure := bf.Builders[0].FirstFailure
	strs := []string{tree, project, bucket, builder, step, strconv.FormatInt(firstFailure, 10)}
	return strings.Join(strs, model.AlertKeySeparator)
}

func attachLUCIBisectionTestAnalysesResults(c context.Context, failures []*messages.BuildFailure, bisectionClient client.Bisection) error {
	testfailuresForProjects := map[string][]*step.TestWithResult{}
	for _, f := range failures {
		if f.Reason == nil || f.Reason.Kind() != "test" {
			continue
		}
		project := f.Builders[0].Project
		// Bisection only support chromium and chrome at this moment.
		if project != "chromium" && project != "chrome" {
			continue
		}
		if _, ok := testfailuresForProjects[project]; !ok {
			testfailuresForProjects[project] = []*step.TestWithResult{}
		}
		tests := f.Reason.Raw.(*analyzer.BqFailure).Tests
		for i := range tests {
			// Skip test with no ref hash.
			if tests[i].RefHash == "" {
				continue
			}
			// Skip test with no changepoint analysis and non-deterministically failing tests.
			// Because there won't be a bisection for test that is not failing deterministically.
			// This is an optimization to reduce the call volume to Bisection.
			if tests[i].CurCounts.TotalResults == 0 || tests[i].CurCounts.UnexpectedResults != tests[i].CurCounts.TotalResults {
				continue
			}
			// Append pointer to the test. Bisection result will be attached using this pointer.
			testfailuresForProjects[project] = append(testfailuresForProjects[project], &tests[i])
		}
	}
	for p, tests := range testfailuresForProjects {
		if len(tests) == 0 {
			continue
		}
		mask := &fieldmaskpb.FieldMask{Paths: []string{"analysis_id", "status", "culprit"}}
		testAnalyses, err := batchGetLUCIBisectionTestAnalyses(c, p, tests, mask, bisectionClient)
		if err != nil {
			return errors.Fmt("batch get test analyses: %w", err)
		}
		if len(testAnalyses) != len(tests) {
			return errors.Fmt("number of test analyses(%d) in response doesn't equal number of tests(%d) for project %s", len(testAnalyses), len(tests), p)
		}
		// Attach test analyses to tests.
		for i, analysis := range testAnalyses {
			// AnalysisID must not be zero, otherwise it suggests no bisection found for this test failure.
			if analysis == nil || analysis.AnalysisId == 0 {
				continue
			}
			tests[i].LUCIBisectionResult = &step.LUCIBisectionTestAnalysis{
				AnalysisID: fmt.Sprint(analysis.AnalysisId),
				Status:     analysis.Status.String(),
				Culprit:    analysis.Culprit,
			}
		}
	}
	return nil
}

// BatchGetLUCIBisectionTestAnalyses calls LUCI bisection BatchGetTestAnalyses RPC to get test bisection result for each test.
// The returned test analyses for each test are in the order of tests from the input.
func batchGetLUCIBisectionTestAnalyses(ctx context.Context, project string, tests []*step.TestWithResult, mask *fieldmaskpb.FieldMask, client client.Bisection) ([]*bisectionpb.TestAnalysis, error) {
	// Split tests into multiple slices, each has at most 100 tests.
	// Because BatchGetTestAnalyses enforce at most 100 test failures in one request.
	batchSize := 100
	batchedTests := [][]*step.TestWithResult{}
	for i := 0; i < len(tests); i += batchSize {
		batchedTests = append(batchedTests, tests[i:int(math.Min(float64(len(tests)), float64(i+batchSize)))])
	}
	// Call BatchGetTestAnalyses for each batch.
	results := make([]*bisectionpb.TestAnalysis, 0, len(tests))
	for _, batch := range batchedTests {
		tf := []*bisectionpb.BatchGetTestAnalysesRequest_TestFailureIdentifier{}
		for _, t := range batch {
			tf = append(tf, &bisectionpb.BatchGetTestAnalysesRequest_TestFailureIdentifier{
				TestId:      t.TestID,
				VariantHash: t.VariantHash,
				RefHash:     t.RefHash,
			})
		}
		resp, err := client.BatchGetTestAnalyses(ctx, &bisectionpb.BatchGetTestAnalysesRequest{
			Project:      project,
			TestFailures: tf,
			Fields:       mask,
		})
		if err != nil {
			return nil, errors.Fmt("batch get test analyses for project %s: %w", project, err)
		}
		results = append(results, resp.GetTestAnalyses()...)
	}
	return results, nil
}

func attachLUCIBisectionCompileFailureAnalyses(c context.Context, failures []*messages.BuildFailure, bisectionClient client.Bisection) error {
	// TODO (nqmtuan): supports queries for a list of bbids.
	var errs []error
	for _, bf := range failures {
		stepName := bf.StepAtFault.Step.Name
		if stepName != "compile" {
			continue
		}
		// bf.Builders exists for historical reasons, since SoM used to do autogrouping of failures
		// of the same step name in the past.
		// Now, it can only contain 1 builder.
		if len(bf.Builders) == 0 {
			continue
		}

		builder := bf.Builders[0]

		// Currently LUCI Bisection only supports "chromium"/"ci" failures
		// Check here as we don't want to waste an RPC call.
		if builder.Project != "chromium" || builder.Bucket != "ci" {
			bf.LuciBisectionResult = &messages.LuciBisectionResult{
				IsSupported: false,
			}
			continue
		}

		// We query by LastestFailure instead of FirstFailure here SoM does not
		// know what reason a builder is failing for. If the first build failed because of
		// a non-compile failure, querying by the first failure will give us no result.
		// When we query by the latest build failure, LUCI Bisection will associate it
		// with the corresponding compile failure analysis (if existed) and return the result.
		bbid := builder.LatestFailure
		res, err := bisectionClient.QueryBisectionResults(c, bbid, stepName)
		if err != nil {
			errs = append(errs, errors.Fmt("failed getting LUCI Bisection results for build %d: %w", bbid, err))
			continue
		}

		bf.LuciBisectionResult = &messages.LuciBisectionResult{
			IsSupported: true,
		}

		if len(res.Analyses) > 0 {
			bf.LuciBisectionResult.Analysis = res.Analyses[0]
			bf.LuciBisectionResult.FailedBBID = strconv.FormatInt(res.Analyses[0].FirstFailedBbid, 10)
			if len(res.Analyses[0].Culprits) > 0 {
				logging.Infof(c, "Found LUCI Bisection culprit for build %d", bbid)
			}
			if res.Analyses[0].HeuristicResult != nil && res.Analyses[0].HeuristicResult.Status == bisectionpb.AnalysisStatus_FOUND {
				logging.Infof(c, "Found LUCI Bisection heuristic result for build %d", bbid)
			}
		}
	}
	if len(errs) > 0 {
		return errors.NewMultiError(errs...)
	}
	return nil
}

func attachLuciBisectionResults(c context.Context, failures []*messages.BuildFailure, bisectionClient client.Bisection) error {
	if bisectionClient == nil {
		return fmt.Errorf("bisectionClient is nil")
	}
	var errs []error
	if err := attachLUCIBisectionCompileFailureAnalyses(c, failures, bisectionClient); err != nil {
		errs = append(errs, err)
	}
	if err := attachLUCIBisectionTestAnalysesResults(c, failures, bisectionClient); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Flatten(errors.NewMultiError(errs...))
	}
	return nil
}

func storeAlertsSummary(c context.Context, a *analyzer.Analyzer, tree string, alertsSummary *messages.AlertsSummary) error {
	sort.Sort(messages.Alerts(alertsSummary.Alerts))
	sort.Stable(bySeverity(alertsSummary.Alerts))

	// Make sure we have summaries for each revision implicated in a builder failure.
	for _, alert := range alertsSummary.Alerts {
		if bf, ok := alert.Extension.(messages.BuildFailure); ok {
			for _, r := range bf.RegressionRanges {
				revs, err := a.GetRevisionSummaries(r.Revisions)
				if err != nil {
					return err
				}
				for _, rev := range revs {
					alertsSummary.RevisionSummaries[rev.GitHash] = rev
				}
			}
		}
	}
	alertsSummary.Timestamp = messages.TimeToEpochTime(time.Now())

	return putAlertsDatastore(c, tree, alertsSummary, true)
}

func putAlertsBigQuery(c context.Context, tree string, alertsSummary *messages.AlertsSummary) error {
	client, err := bigquery.NewClient(c, info.AppID(c))
	if err != nil {
		return err
	}
	up := bq.NewUploader(c, client, bqDatasetID, bqTableID)
	up.SkipInvalidRows = true
	up.IgnoreUnknownValues = true

	ts := timestamppb.New(alertsSummary.Timestamp.Time())
	if err := ts.CheckValid(); err != nil {
		return err
	}

	row := &gen.SOMAlertsEvent{
		Timestamp: ts,
		Tree:      tree,
		RequestId: appengine.RequestID(c),
	}

	for _, a := range alertsSummary.Alerts {
		alertEvt := &gen.SOMAlertsEvent_Alert{
			Key:   a.Key,
			Title: a.Title,
			Body:  a.Body,
			Type:  alertEventType(a.Type),
		}

		if bf, ok := a.Extension.(messages.BuildFailure); ok {
			for _, builder := range bf.Builders {
				newBF := &gen.SOMAlertsEvent_Alert_BuildbotFailure{
					BuilderGroup:  builder.BuilderGroup,
					Builder:       builder.Name,
					Step:          bf.StepAtFault.Step.Name,
					FirstFailure:  builder.FirstFailure,
					LatestFailure: builder.LatestFailure,
					LatestPassing: builder.LatestPassing,
				}
				alertEvt.BuildbotFailures = append(alertEvt.BuildbotFailures, newBF)
			}
		}

		row.Alerts = append(row.Alerts, alertEvt)
	}

	return up.Put(c, row)
}

var (
	alertToEventType = map[messages.AlertType]gen.SOMAlertsEvent_Alert_AlertType{
		messages.AlertHungBuilder:    gen.SOMAlertsEvent_Alert_HUNG_BUILDER,
		messages.AlertOfflineBuilder: gen.SOMAlertsEvent_Alert_OFFLINE_BUILDER,
		messages.AlertIdleBuilder:    gen.SOMAlertsEvent_Alert_IDLE_BUILDER,
		messages.AlertInfraFailure:   gen.SOMAlertsEvent_Alert_INFRA_FAILURE,
		messages.AlertBuildFailure:   gen.SOMAlertsEvent_Alert_BUILD_FAILURE,
		messages.AlertTestFailure:    gen.SOMAlertsEvent_Alert_TEST_FAILURE,
	}
)

func alertEventType(t messages.AlertType) gen.SOMAlertsEvent_Alert_AlertType {
	if val, ok := alertToEventType[t]; ok {
		return val
	}
	panic("unknown alert type: " + string(t))
}
