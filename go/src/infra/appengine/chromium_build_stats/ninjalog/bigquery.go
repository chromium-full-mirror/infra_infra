// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ninjalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
	datatransfer "cloud.google.com/go/bigquery/datatransfer/apiv1"
	"cloud.google.com/go/bigquery/datatransfer/apiv1/datatransferpb"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/structpb"
)

// bqSchema is BigQuery schema used to store ninja log.
// This should be sync with avro_schema.yaml.
// TODO: generate from avro_schema.yaml?
var bqSchema = bigquery.Schema{
	{
		Name: "build_id",
		Type: bigquery.IntegerFieldType,
	},
	{
		Name: "invocation_id",
		Type: bigquery.StringFieldType,
	},
	{
		Name:     "targets",
		Type:     bigquery.StringFieldType,
		Repeated: true,
	},
	{
		Name: "step_name",
		Type: bigquery.StringFieldType,
	},
	{
		Name: "jobs",
		Type: bigquery.IntegerFieldType,
	},
	{
		Name: "os",
		Type: bigquery.StringFieldType,
	},
	{
		Name: "cpu_core",
		Type: bigquery.IntegerFieldType,
	},
	{
		Name: "build_configs",
		Type: bigquery.RecordFieldType,
		Schema: bigquery.Schema{
			{
				Name: "key",
				Type: bigquery.StringFieldType,
			},
			{
				Name: "value",
				Type: bigquery.StringFieldType,
			},
		},
		Repeated: true,
	},
	{
		Name: "log_entries",
		Type: bigquery.RecordFieldType,
		Schema: bigquery.Schema{
			{
				Name:     "outputs",
				Type:     bigquery.StringFieldType,
				Repeated: true,
			},
			{
				Name: "start_duration_sec",
				Type: bigquery.FloatFieldType,
			},
			{
				Name: "end_duration_sec",
				Type: bigquery.FloatFieldType,
			},
			{
				Name: "weighted_duration_sec",
				Type: bigquery.FloatFieldType,
			},
		},
		Repeated: true,
	},
	{
		Name: "created_at",
		Type: bigquery.TimestampFieldType,
	},
}

// CreateBQTable creates BigQuery table storing ninjalog.
func CreateBQTable(ctx context.Context, projectID, table string) error {
	return updateBQTable(ctx, projectID, table, true)
}

// UpdateBQTable updates BigQuery table storing ninjalog.
func UpdateBQTable(ctx context.Context, projectID, table string) error {
	return updateBQTable(ctx, projectID, table, false)
}

func updateBQTable(ctx context.Context, projectID, table string, initializeTable bool) (err error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return err
	}
	defer func() {
		cerr := client.Close()
		if err == nil {
			err = cerr
		}
	}()

	d := client.Dataset("ninjalog")
	t := d.Table(table)

	partitioning := &bigquery.TimePartitioning{
		Field:                  "created_at",
		RequirePartitionFilter: true,
		// Daily partitioning is necessary to hold 540 days data which is less than the number of partitioning limit 4000.
		// https://cloud.google.com/bigquery/quotas#partitioned_tables
		Type:       bigquery.DayPartitioningType,
		Expiration: 540 * 24 * time.Hour,
	}

	if initializeTable {
		if table == "test" {
			// Only test table can be deleted.
			_ = t.Delete(ctx) // ignore error
		} else {
			// Non test table should not exist when initialize.
			_, err := t.Metadata(ctx)
			var gerr *googleapi.Error
			if !errors.As(err, &gerr) {
				return fmt.Errorf("unexpected error: %w", err)
			}

			if gerr.Code != http.StatusNotFound {
				return fmt.Errorf("unexpected error: %w", gerr)
			}
		}

		err := t.Create(ctx, &bigquery.TableMetadata{
			Schema: bigquery.Schema{
				{
					Name: "created_at",
					Type: bigquery.TimestampFieldType,
				},
			},
			TimePartitioning: partitioning,
		})
		if err != nil {
			return err
		}
	}

	md, err := t.Metadata(ctx)
	if err != nil {
		return err
	}
	_, err = t.Update(ctx, bigquery.TableMetadataToUpdate{
		Schema:           bqSchema,
		ExpirationTime:   bigquery.NeverExpire,
		TimePartitioning: partitioning,
	}, md.ETag)
	return err
}

// CreateTransferConfig crates BigQuery transfer config that loads avro files
// from GCS to BigQuery table periodically.
func CreateTransferConfig(ctx context.Context, project, table string) (tc *datatransferpb.TransferConfig, err error) {
	client, err := datatransfer.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		cerr := client.Close()
		if err == nil {
			err = cerr
		}
	}()

	displayName := table + " ninjalog transfer config"

	// Delete transfer configs having the same display name.
	iter := client.ListTransferConfigs(ctx, &datatransferpb.ListTransferConfigsRequest{
		Parent:        "projects/" + project,
		DataSourceIds: []string{"google_cloud_storage"},
	})

	for {
		transferConfig, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		if transferConfig.DisplayName != displayName {
			continue
		}

		if err := client.DeleteTransferConfig(ctx, &datatransferpb.DeleteTransferConfigRequest{
			Name: transferConfig.Name,
		}); err != nil {
			return nil, err
		}
	}

	// ref: https://cloud.google.com/bigquery-transfer/docs/cloud-storage-transfer#bq
	params, err := structpb.NewStruct(map[string]interface{}{
		// Filter by runtime date prefix, otherwise it may exceeds the max number of files to transfer in a batch.
		// https://cloud.google.com/bigquery/docs/gcs-transfer-parameters#load_a_snapshot_of_all_data_into_an_ingestion-time_partitioned_table
		"data_path_template":              fmt.Sprintf(`gs://%s.appspot.com/ninjalog_%s_avro/{run_time|"%%Y_%%m_%%d"}*`, project, table),
		"destination_table_name_template": table,
		"file_format":                     "AVRO",
		"delete_source_files":             true,
		"use_avro_logical_types":          true,
	})
	if err != nil {
		return nil, err
	}
	return client.CreateTransferConfig(ctx, &datatransferpb.CreateTransferConfigRequest{
		Parent: "projects/" + project,
		TransferConfig: &datatransferpb.TransferConfig{
			Destination: &datatransferpb.TransferConfig_DestinationDatasetId{
				DestinationDatasetId: "ninjalog",
			},
			DisplayName:  displayName,
			Schedule:     "every 15 minutes",
			DataSourceId: "google_cloud_storage",
			Params:       params,
			EmailPreferences: &datatransferpb.EmailPreferences{
				// This only notifies failures to transfer creator.
				// https://cloud.google.com/bigquery-transfer/docs/transfer-run-notifications#email_notifications
				EnableFailureEmail: true,
			},
		},
		ServiceAccountName: project + "@appspot.gserviceaccount.com",
	})

}
