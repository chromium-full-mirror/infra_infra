// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/artifact"
)

const (
	// TesthausGCPProject Testhaus GCP project ID.
	TesthausGCPProject = "cros-test-analytics"

	// TestInfoDataset Test info BigQuery dataset name.
	TestInfoDataset = "testinfo"

	// EQCInfoTable EqC info BigQuery table name.
	EQCInfoTable = "eqc_info"

	// EQCSubjectKey 3D EqC subject key used to fetch EqC info from the
	// scheduling publish key map.
	EQCSubjectKey = "3D"
)

// EQCPublishService 3D EqC publish service to interact with BigQuery.
// Note that the caller should call the Close function to close the bqClient
// before the service is done.
type EQCPublishService struct {
	// bqClient the underlying BigQuery client.
	bqClient *bigquery.Client

	// eqcInfos the list of EqC infos related to the current test results.
	eqcInfos []*artifact.EqcInfo
}

// EQCRow represents a row in the EqC info BigQuery table.
type EQCRow struct {
	EQCHash               string
	EQCName               string
	EQCCategoryExpression map[string]string
	EQCDimensions         map[string]string
}

// NewEQCPublishService create a new EqC publish service to interact with
// BigQuery.
func NewEQCPublishService(ctx context.Context, req *api.PublishRequest) (*EQCPublishService, error) {
	if req == nil {
		return nil, fmt.Errorf("publish request is nil")
	}

	client, err := bigquery.NewClient(ctx, TesthausGCPProject)
	if err != nil {
		return nil, fmt.Errorf("creating the BQ client: %w", err)
	}

	eqcInfos, err := EqcInfos(req)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("extracting the EqC info from test results: %w", err)
	}

	return &EQCPublishService{
		bqClient: client,
		eqcInfos: eqcInfos,
	}, nil
}

// Close closes the EqC publish service.
func (eps *EQCPublishService) Close() error {
	err := eps.bqClient.Close()
	if err != nil {
		return fmt.Errorf("close BQ client: %w", err)
	}

	return nil
}

// ExportToBQ exports the EqC infos to BigQuery.
func (eps *EQCPublishService) ExportToBQ(ctx context.Context) error {
	eqcInfos := eps.eqcInfos
	log.Printf("Exporting the EqC infos: %#v to BQ", eqcInfos)

	if len(eqcInfos) == 0 {
		log.Printf("Skipping the BQ export because of the empty EqC info list")
		return nil
	}

	for _, eqcInfo := range eqcInfos {
		if eqcInfo.GetEqcHash() == "" {
			log.Printf("Skipping the BQ export because the EqC hash is empty: %#v", eqcInfo)
			continue
		}

		exist, err := eps.checkExists(ctx, eqcInfo)
		if err != nil {
			log.Printf("Failed to check if the EqC info already exists: %s", err)
			continue
		}

		if exist {
			log.Printf("Skipping the BQ export because the EqC info already exists: %#v", eqcInfo)
			continue
		}

		// Inserts the EqC info because it doesn't exist in the BQ table.
		eqcRow := &EQCRow{
			EQCHash:               eqcInfo.EqcHash,
			EQCName:               eqcInfo.EqcName,
			EQCCategoryExpression: eqcInfo.EqcCategoryExpression,
			EQCDimensions:         eqcInfo.EqcDimensions,
		}

		inserter := eps.bqClient.Dataset(TestInfoDataset).Table(EQCInfoTable).Inserter()
		if err := inserter.Put(ctx, eqcRow); err != nil {
			log.Printf("Failed to export the EqC info: %#v to BQ: %s", eqcInfo, err)
			continue
		}
	}

	log.Printf("Successfully exported the EqC infos: %#v to BQ", eqcInfos)
	return nil
}

// checkExists checks if the given EqC info already exists in the BQ table.
func (eps *EQCPublishService) checkExists(ctx context.Context, eqcInfo *artifact.EqcInfo) (bool, error) {
	// Constructs the query to check if the EqC info already exists.
	q := eps.bqClient.Query(fmt.Sprintf("SELECT * FROM %s.%s.%s WHERE eqc_hash = @eqc_hash", TesthausGCPProject, TestInfoDataset, EQCInfoTable))
	q.Parameters = []bigquery.QueryParameter{
		{
			Name:  "eqc_hash",
			Value: eqcInfo.EqcHash,
		},
	}

	// Runs the query.
	job, err := q.Run(ctx)
	if err != nil {
		return false, fmt.Errorf("running the BQ query: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return false, fmt.Errorf("waiting for the BQ job: %w", err)
	}
	if err := status.Err(); err != nil {
		return false, fmt.Errorf("checking the BQ job status: %w", err)
	}

	// Checks if any rows were returned.
	it, err := job.Read(ctx)
	if err != nil {
		return false, fmt.Errorf("reading the BQ job results: %w", err)
	}

	err = it.Next(&EQCRow{})
	if err == iterator.Done {
		log.Printf("EqC info doesn't exist in the BQ: %#v", eqcInfo)
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("checking if the EqC info already exists: %w", err)
	} else {
		log.Printf("EqC info already exists in the BQ: %#v", eqcInfo)
		return true, nil
	}
}

// Save implements the ValueSaver interface for the EQCRow struct.
func (e *EQCRow) Save() (map[string]bigquery.Value, string, error) {
	row := make(map[string]bigquery.Value)
	if e.EQCHash != "" {
		row["eqc_hash"] = e.EQCHash
	}

	if e.EQCName != "" {
		row["eqc_name"] = e.EQCName
	}

	if e.EQCCategoryExpression != nil {
		categoryExpression, err := json.Marshal(e.EQCCategoryExpression)
		if err != nil {
			return nil, "", fmt.Errorf("marshalling the EqC category expression: %w", err)
		}

		row["eqc_category_expression"] = string(categoryExpression)
	}

	if e.EQCDimensions != nil {
		dimensions, err := json.Marshal(e.EQCDimensions)
		if err != nil {
			return nil, "", fmt.Errorf("marshalling the EqC dimensions: %w", err)
		}

		row["eqc_dimensions"] = string(dimensions)
	}

	return row, e.EQCHash, nil
}

// EqcInfos extracts the EqC infos from the rdb metadata in the publish
// request.
func EqcInfos(req *api.PublishRequest) ([]*artifact.EqcInfo, error) {
	log.Printf("Started to extract the EqC infos from the request: %#v", req)
	metadata, err := UnpackMetadata(req)
	if err != nil {
		return nil, fmt.Errorf("unpacking the metadata in the publish request: %s", err)
	}

	eqcInfos := make([]*artifact.EqcInfo, 0, len(metadata.GetPublishKeys()))
	for _, items := range metadata.GetPublishKeys() {
		if items.GetSubject() == EQCSubjectKey {
			eqcInfo, err := EqcInfo(items.GetKeyValues())
			if err != nil {
				log.Printf("Failed to extract the EqC info from the map: %#v due to error: %s", eqcInfo, err)
				continue
			}
			eqcInfos = append(eqcInfos, eqcInfo)
		}
	}

	if len(eqcInfos) == 0 {
		log.Printf("No EqC infos found in the request")
		return []*artifact.EqcInfo{}, nil
	}

	log.Printf("Successfully extracted the EqC infos: %#v", eqcInfos)
	return eqcInfos, nil
}

// EqcInfo extracts the EqC info from the EqC map.
func EqcInfo(eqcInfoMap map[string]string) (*artifact.EqcInfo, error) {
	eqcInfo := &artifact.EqcInfo{
		EqcHash: eqcInfoMap["eqcHash"],
		EqcName: eqcInfoMap["eqcName"],
	}

	// The content of the "eqcCategoryExpression" field is aligned with the
	// CategoryExpression proto in "/ttcp/protos/ttcp/syntax/syntax.proto".
	if categoryExpressionJSON, ok := eqcInfoMap["eqcCategoryExpression"]; ok {
		categoryExpressionMap := make(map[string]interface{})
		if err := json.Unmarshal([]byte(categoryExpressionJSON), &categoryExpressionMap); err != nil {
			return nil, fmt.Errorf("unmarshalling the EqC category expression: %w", err)
		}

		eqcInfo.EqcCategoryExpression = make(map[string]string)
		for k, v := range categoryExpressionMap {
			switch value := v.(type) {
			case string:
				// Corresponds to the name field which indicates the name of the
				// predefined category. Most of the time (> 99.99%) it will be
				// mapped to this string type.
				eqcInfo.EqcCategoryExpression[k] = fmt.Sprintf("%v", value)
			default:
				// Corresponds to the category proto field which explicitly
				// specifies the category contents.
				jsonString, err := json.Marshal(value)
				if err != nil {
					log.Printf("Failed to marshal the category expression key: %s", k)
					continue
				}
				eqcInfo.EqcCategoryExpression[k] = string(jsonString)
			}

		}
	}

	if dimensionsJSON, ok := eqcInfoMap["eqcDimensions"]; ok {
		eqcInfo.EqcDimensions = make(map[string]string)
		if err := json.Unmarshal([]byte(dimensionsJSON), &eqcInfo.EqcDimensions); err != nil {
			return nil, fmt.Errorf("unmarshalling the EqC dimensions: %w", err)
		}
	}

	log.Printf("Successfully extracted the EqC info: %#v", eqcInfo)
	return eqcInfo, nil
}

// PublishEQC exports the EqC info to BigQuery.
func PublishEQC(ctx context.Context, req *api.PublishRequest) error {
	if !req.Is_3DRun {
		return nil
	}

	eps, err := NewEQCPublishService(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create new EqC publish service: %s", err.Error())
	}

	defer eps.Close()

	if err := eps.ExportToBQ(ctx); err != nil {
		return fmt.Errorf("failed to export the EqC info to BQ: %s", err.Error())
	}
	return nil
}
