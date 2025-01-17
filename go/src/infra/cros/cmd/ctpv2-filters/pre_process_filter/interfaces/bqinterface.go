// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package interfaces

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"

	"infra/cros/cmd/ctpv2-filters/pre_process_filter/structs"

	"go.chromium.org/chromiumos/config/go/test/api"
)

const dataset = "analytics"
const resultsTable = "FlakeReportTestv2"
const saProject = "chromeos-bot"
const tableProject = "chromeos-test-platform-data"
const saFile = "/creds/service_accounts/service-account-chromeos.json"

// QueryForResults will query the Flake  tables to find the history for tests.
func QueryForResults(variant string, milestone string, log *log.Logger) (*bigquery.RowIterator, error) {
	ctx := context.Background()
	c, err := bigquery.NewClient(ctx, saProject,
		option.WithCredentialsFile(saFile))
	if err != nil {
		return nil, fmt.Errorf("unable to make bq client %s", err)
	}
	defer c.Close()

	cmd := fmt.Sprintf("SELECT * FROM chromeos-test-platform-data.analytics.FlakeCacheTest WHERE total_runs > 0 AND board = \"%s\"  AND (REGEXP_CONTAINS(build, \"%s\")) ORDER BY success_permille", variant, milestone)
	log.Println(cmd)
	bqQ := c.Query(cmd)
	// Execute the query.
	it, err := bqQ.Read(ctx)
	if err != nil {
		log.Printf("INFORMATIONAL: query error: %s", err)
	}

	return it, nil
}

// insertRows demonstrates inserting data into a table using the streaming insert mechanism.
func insertRows(filteredTests []*Entry, log *log.Logger) error {
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, tableProject,
		option.WithCredentialsFile(saFile))
	if err != nil {
		return fmt.Errorf("bigquery.NewClient: %w", err)
	}
	defer client.Close()
	inserter := client.Dataset(dataset).Table(resultsTable).Inserter()

	if err := inserter.Put(ctx, filteredTests); err != nil {
		return err
	}
	log.Println("Upload success")
	return nil
}

// Entry represents a row item.
type Entry struct {
	date                         string
	bbid                         string
	board                        string
	test                         string
	passrate14day                float32
	passrate14dayrequired        float32
	passrate14daysamples         int
	passrate14daysamplesrequired int
	passrate3day                 float32
	passrate3dayrequired         float32
	passrate3daysamples          int
	passrate3daysamplesrequired  int
	dryrun                       string
	milestone                    int
	build                        string
}

// Save implements the ValueSaver interface.
// This example disables best-effort de-duplication, which allows for higher throughput.
func (i *Entry) Save() (map[string]bigquery.Value, string, error) {
	return map[string]bigquery.Value{
		"date":                         i.date,
		"bbid":                         i.bbid,
		"board":                        i.board,
		"test":                         i.test,
		"passrate14day":                i.passrate14day,
		"passrate14dayrequired":        i.passrate14dayrequired,
		"passrate14daysamples":         i.passrate14daysamples,
		"passrate14daysamplesrequired": i.passrate14daysamplesrequired,
		"passrate3day":                 i.passrate3day,
		"passrate3dayrequired":         i.passrate3dayrequired,
		"passrate3daysamples":          i.passrate3daysamples,
		"passrate3daysamplesrequired":  i.passrate3daysamplesrequired,
		"dryrun":                       i.dryrun,
		"milestone":                    i.milestone,
		"build":                        i.build,
	}, bigquery.NoDedupeID, nil
}

func policyData(req *api.FilterFlakyRequest) (int, int, int, int, error) {
	var requiredPassRate14Day int
	var requiredPassRate3Day int
	var requiredSamples14Day int
	var requiredSamples3Day int

	switch op := req.Policy.(type) {
	case *api.FilterFlakyRequest_PassRatePolicy:
		fmt.Println(op)
		requiredPassRate14Day = int(op.PassRatePolicy.PassRate)
		requiredSamples14Day = int(op.PassRatePolicy.MinRuns)

		// Not yet set in the prod policy.

		requiredSamples3Day = int(op.PassRatePolicy.PassRateRecent)
		requiredPassRate3Day = int(op.PassRatePolicy.MinRunsRecent)
	case *api.FilterFlakyRequest_StabilitySensorPolicy:
		return 0, 0, 0, 0, fmt.Errorf("stabilitySensor policy cannot upload results")
	}
	return requiredPassRate14Day, requiredSamples14Day, requiredSamples3Day, requiredPassRate3Day, nil
}

func boardData(req *api.FilterFlakyRequest) (string, error) {
	var board string
	switch variantOp := req.Variant.(type) {
	case *api.FilterFlakyRequest_Board:
		return variantOp.Board, nil
	}
	if board == "" {
		return "", fmt.Errorf("no variant provided, upload filter results")
	}
	return "", fmt.Errorf("unknown case from upload boardData")
}

func uploadData(filteredTests []string, req *api.FilterFlakyRequest, data map[string]structs.SignalFormat, log *log.Logger) (err error) {
	timeFmt := time.Now().Format("2006-01-02_15:04:05")
	requiredPassRate14Day, requiredSamples14Day, requiredSamples3Day, requiredPassRate3Day, err := policyData(req)
	if err != nil {
		return err
	}
	mileStone, _ := strconv.Atoi(req.Milestone)

	board, err := boardData(req)
	if err != nil {
		return err
	}
	var upload []*Entry

	for i, tc := range filteredTests {
		// Initialize the Entry with default values.
		entry := &Entry{
			test:                         tc,
			bbid:                         req.Bbid,
			board:                        board,
			date:                         timeFmt,
			passrate14day:                float32(-1),
			passrate14dayrequired:        float32(requiredPassRate14Day),
			passrate14daysamples:         -1,
			passrate14daysamplesrequired: requiredSamples14Day,
			passrate3day:                 float32(-1),
			passrate3dayrequired:         float32(requiredPassRate3Day),
			passrate3daysamples:          -1,
			passrate3daysamplesrequired:  requiredSamples3Day,
			dryrun:                       strconv.FormatBool(req.GetPassRatePolicy().Dryrun),
			milestone:                    mileStone,
			build:                        "NOT_PROVIDED",
		}

		_, ok := data[tc]
		// If the test is found, get add its data.
		if ok {
			fmt.Println(data[tc].Passrate)
			fmt.Println(data[tc].Runs)
			entry.passrate14daysamples = data[tc].Runs
			entry.passrate14day = float32(data[tc].Passrate)
			entry.passrate3daysamples = data[tc].RunsRecent
			entry.passrate3day = float32(data[tc].PassrateRecent)
		}
		upload = append(upload, entry)

		// Bulk upload by 10s. Full size causes table upload issues.
		if i%10 == 0 {
			err := insertRows(upload, log)
			if err != nil {
				return err
			}
			upload = make([]*Entry, 0)

		}
	}
	if len(upload) != 0 {
		insertRows(upload, log)
	}

	return err
}

// WriteResults will publish the filtering results into Bq.
func WriteResults(filteredTests []string, req *api.FilterFlakyRequest, d map[string]structs.SignalFormat, log *log.Logger) error {
	err := uploadData(filteredTests, req, d, log)
	if err != nil {
		return err
	}
	return nil
}
