// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package analytics

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.chromium.org/chromiumos/config/go/test/api"
	dut_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"

	"infra/cros/cmd/ctpv2/data"
)

const dataset = "analytics"
const resultsTable = "CTPV2Metrics"
const taskResultsTable = "CTPV2TaskMetrics"

const saProject = "chromeos-test-platform-data"

const cacheTable = "DurationCache"
const minDuration = 1
const saFile = "/creds/service_accounts/service-account-chromeos.json"

const Start = "START"
const Success = "SUCCESS"
const Fail = "FAIL"
const Panic = "PANIC"

type BqData struct {
	SuiteName     string
	AnalyticsName string
	BBID          string
	Build         string
	Step          string
	Freeform      string
	Pool          string
	Status        string
	Duration      float32
	Date          civil.DateTime
}

type TaskData struct {
	SuiteName     string
	Date          civil.DateTime
	AnalyticsName string
	BBID          string
	Build         string
	Step          string
	Freeform      string
	Pool          string
	Status        string
	Duration      float32
	DisplayName   string
	TrTaskID      string
	SchedukeID    string
	Board         string
	Model         string
	Deps          []string
}

// CtpAnalyticsBQClient will build the client for the CTP BQ tables, using the default CTP SA
func CtpAnalyticsBQClient(ctx context.Context) *bigquery.Client {
	c, err := bigquery.NewClient(ctx, saProject,
		option.WithCredentialsFile(saFile))
	if err != nil {
		logging.Infof(ctx, "Unable to make BQ client :%s", err)
		return nil
	}
	return c
}

// InsertCTPMetrics will insert the CTP Analytics Data into the CTPv2Metrics Table.
func InsertCTPMetrics(c *bigquery.Client, data []*BqData) error {
	ctx := context.Background()
	inserter := c.Dataset(dataset).Table(resultsTable).Inserter()
	if err := inserter.Put(ctx, data); err != nil {
		return err
	}
	return nil
}

type resSchema struct {
	Normalized_test string
	Duration        float64
}

// ReadDurationMetrics will query the duration cache table, and return a map of the test results.
// If a board is provided and not "", it will limit the results to that board.
func ReadDurationMetrics(c *bigquery.Client, board string, harness string) (map[string]float64, error) {
	ctx := context.Background()
	durs := make(map[string]float64)
	table := fmt.Sprintf("%s.%s.%s", saProject, dataset, cacheTable)

	// Limit to a Duration > 1 second to reduce the size of the response; makes the query faster, cheaper, and we can treat all "no results" as 1 sec duration.
	cmd := fmt.Sprintf("SELECT * FROM %s WHERE Duration > %v and harness = \"%s\" ORDER BY dur DESC", table, minDuration, harness)
	if board != "" {
		cmd = fmt.Sprintf("SELECT * FROM %s WHERE board = \"%s\" and Duration > %v and harness = \"%s\" ORDER BY dur DESC", table, board, minDuration, harness)
	}

	bqQ := c.Query(cmd)

	// Execute the query.
	it, err := bqQ.Read(ctx)
	if err != nil {
		fmt.Printf("INFORMATIONAL: query error: %s", err)
	}

	for {
		var resp resSchema
		err := it.Next(&resp)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Println("found this err while parsing", err)
			return nil, err
		}
		durs[resp.Normalized_test] = resp.Duration
	}

	return durs, nil
}

// InsertCTPMetrics will insert the CTP Analytics Data into the CTPv2TaskMetrics Table.
func InsertCTPTaskMetrics(c *bigquery.Client, data []*TaskData) error {
	ctx := context.Background()
	inserter := c.Dataset(dataset).Table(taskResultsTable).Inserter()

	upload := []*TaskData{}
	for c, d := range data {
		if c%10 == 0 && c != 0 {
			if err := inserter.Put(ctx, upload); err != nil {
				return err
			}
			upload = []*TaskData{}
		}
		upload = append(upload, d)
	}

	// Any left over.
	if len(upload) != 0 {
		if err := inserter.Put(ctx, upload); err != nil {
			return err
		}
	}

	return nil
}

// SoftInsertStepWCtp2Req insert a step info to BQl built from the CTPv2Request req. Do not fail on errors.
func SoftInsertStepWCtp2Req(ctx context.Context, BQClient *bigquery.Client, data *BqData, ctpv2Req *api.CTPv2Request, build *build.State, v1Req *api.CTPRequest) {
	if ctpv2Req != nil && len(ctpv2Req.GetRequests()) > 0 {
		data = buildDataFromCTPReq(data, ctpv2Req)
	} else if v1Req != nil {
		data = buildDataFromCTPReq(data, &api.CTPv2Request{Requests: []*api.CTPRequest{v1Req}})
	}

	if BQClient != nil {
		err := InsertCTPMetrics(BQClient, []*BqData{data})
		if err != nil {
			logging.Infof(ctx, "Error during BQ write: %s", err)
		}
		logging.Infof(ctx, "Successful write")
	} else {
		logging.Infof(ctx, "Skipped BQ write as no client provided.")
	}
}

func CreateTrReqData(ctx context.Context, BQClient *bigquery.Client, data *TaskData, req *data.TrRequest, suiteInfo *api.SuiteInfo, build *build.State) *TaskData {
	data = addBBIDToTaskData(data, build)
	data.SuiteName = suiteInfo.GetSuiteRequest().GetTestSuite().GetName()
	data.AnalyticsName = suiteInfo.GetSuiteRequest().GetAnalyticsName()
	data.Pool = suiteInfo.GetSuiteMetadata().GetPool()
	data.Date = civil.DateTimeOf(time.Now())

	if req.NewReq != nil {
		if len(req.NewReq.GetSchedulingUnits()) > 0 {
			data.Board = boardFromSU(req.NewReq.GetSchedulingUnits()[0])
			data.Model = modelFromSU(req.NewReq.GetSchedulingUnits()[0])
			data.Build = buildFromGcs(req.NewReq.GetSchedulingUnits()[0].GetPrimaryTarget().GetSwReq().GetGcsPath())
			data.Deps = deps(req.NewReq.GetSchedulingUnits()[0])
		}
	}
	data.SuiteName = suiteInfo.GetSuiteRequest().GetTestSuite().GetName()

	return data
}

// SoftInsertStepWTrReq insert a step info to BQ built from the Trreq. Do not fail on errors.
func SoftInsertStepWTrReqBulk(ctx context.Context, BQClient *bigquery.Client, td []*TaskData) {
	if len(td) == 0 {
		return
	}

	if BQClient != nil {
		err := InsertCTPTaskMetrics(BQClient, td)
		if err != nil {
			logging.Infof(ctx, "Error During BQ write: %s", err)
		}
		logging.Infof(ctx, "Successful write")
	}
}

// SoftInsertStepWTrReq insert a step info to BQ built from the Trreq. Do not fail on errors.
func SoftInsertStepWTrReq(ctx context.Context, BQClient *bigquery.Client, data *TaskData, req *data.TrRequest, suiteInfo *api.SuiteInfo, build *build.State) {
	if req == nil {
		return
	}
	data = CreateTrReqData(ctx, BQClient, data, req, suiteInfo, build)
	if BQClient != nil {
		err := InsertCTPTaskMetrics(BQClient, []*TaskData{data})
		if err != nil {
			logging.Infof(ctx, "Error During BQ write: %s", err)
		}
		logging.Infof(ctx, "Successful write")
	}
}

// SoftInsertStepWInternalPlan insert a step info to BQl built from the InternalTestPlan req. Do not fail on errors.
func SoftInsertStepWInternalPlan(ctx context.Context, BQClient *bigquery.Client, data *BqData, req *api.InternalTestplan, build *build.State) {
	if req != nil {
		data = buildDataFromInternalTP(data, req)
	}
	data = addBBID(data, build)
	data.Date = civil.DateTimeOf(time.Now())

	if BQClient != nil {
		err := InsertCTPMetrics(BQClient, []*BqData{data})
		if err != nil {
			logging.Infof(ctx, "Error During BQ write: %s", err)
		}
		logging.Infof(ctx, "Successful write")
	}
}

// gs://chromeos-image-archive/dedede-release/R124-15815.0.0 --> R124-15815.0.0
func buildFromGcs(build string) string {
	f := strings.Split(build, "/")
	return f[len(f)-1]
}

func addBBIDToTaskData(data *TaskData, build *build.State) *TaskData {
	bbid := getBBID(build)
	if bbid != "" {
		data.BBID = bbid
	}
	return data
}

func addBBID(data *BqData, build *build.State) *BqData {
	bbid := getBBID(build)
	if bbid != "" {
		data.BBID = bbid
	}
	return data
}

// getBBID will return BBID from the build.state
func getBBID(build *build.State) string {
	if build == nil {
		return ""
	}
	bbid := strconv.FormatInt(build.Build().GetId(), 10)
	if bbid != "" {
		return bbid
	}
	return ""
}

// buildDataFromInternal will append the internal test plan details to the to be inserted BQ row.
func buildDataFromInternalTP(data *BqData, req *api.InternalTestplan) *BqData {
	aName := req.GetSuiteInfo().GetSuiteRequest().GetAnalyticsName()
	poolName := req.GetSuiteInfo().GetSuiteMetadata().GetPool()
	sName := req.GetSuiteInfo().GetSuiteRequest().GetTestSuite().GetName()
	if len(req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements()) > 0 {
		b := req.GetSuiteInfo().GetSuiteMetadata().GetTargetRequirements()[0].GetSwRequirement().GetGcsPath()
		if b != "" {
			data.Build = buildFromGcs(b)
		}
	} else if len(req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()) > 0 {
		b := req.GetSuiteInfo().GetSuiteMetadata().GetSchedulingUnits()[0].GetPrimaryTarget().GetSwReq().GetGcsPath()
		if b != "" {
			data.Build = buildFromGcs(b)
		}
	}
	if aName != "" {
		data.AnalyticsName = aName
	}
	if sName != "" {
		data.SuiteName = sName
	}
	if poolName != "" {
		data.Pool = poolName
	}
	return data
}

func deps(target *api.SchedulingUnit) []string {
	return target.GetPrimaryTarget().GetSwarmingDef().GetSwarmingLabels()
}

func boardFromSU(target *api.SchedulingUnit) string {
	return dutModelFromSwarmingDef(target.GetPrimaryTarget().GetSwarmingDef()).GetBuildTarget()
}
func modelFromSU(target *api.SchedulingUnit) string {
	return dutModelFromSwarmingDef(target.GetPrimaryTarget().GetSwarmingDef()).GetModelName()
}

func dutModelFromSwarmingDef(def *api.SwarmingDefinition) *dut_api.DutModel {
	switch hw := def.GetDutInfo().GetDutType().(type) {
	case *dut_api.Dut_Chromeos:
		return hw.Chromeos.GetDutModel()
	case *dut_api.Dut_Android_:
		return hw.Android.GetDutModel()
	case *dut_api.Dut_Devboard_:
		return hw.Devboard.GetDutModel()
	}
	return nil
}

// buildDataFromInternal will append the CTP Request details to the to be inserted BQ row.
func buildDataFromCTPReq(data *BqData, req *api.CTPv2Request) *BqData {
	// This should always only be len(1); but good to check anyways.
	if len(req.GetRequests()) < 1 {
		return data
	}
	poolName := req.GetRequests()[0].GetPool()
	aName := req.GetRequests()[0].GetSuiteRequest().GetAnalyticsName()
	sName := req.GetRequests()[0].GetSuiteRequest().GetTestSuite().GetName()

	// This will not actually always be len(1); but as this is used just to get the build which
	// will be identical across all; [0] index is OK
	if len(req.GetRequests()[0].GetScheduleTargets()) > 0 {
		if len(req.GetRequests()[0].GetScheduleTargets()[0].GetTargets()) > 0 {
			b := req.GetRequests()[0].GetScheduleTargets()[0].GetTargets()[0].GetSwTarget().GetLegacySw().GetGcsPath()
			if b != "" {
				data.Build = buildFromGcs(b)
			}
		}
	}

	if aName != "" {
		data.AnalyticsName = aName
	}
	if sName != "" {
		data.SuiteName = sName
	}
	if poolName != "" {
		data.Pool = poolName
	}
	data.Date = civil.DateTimeOf(time.Now())

	return data
}
