// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package entities

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"

	"go.chromium.org/infra/appengine/chrome-test-health/datastorage"
)

type SummaryCoverageData struct {
	Key                     *datastore.Key
	DataType                string `datastore:"data_type"`
	Path                    string `datastore:"path"`
	Bucket                  string `datastore:"bucket"`
	Builder                 string `datastore:"builder"`
	ModifierId              int64  `datastore:"modifier_id"`
	Data                    []byte `datastore:"data"`
	GitilesCommitProject    string `datastore:"gitiles_commit.project"`
	GitilesCommitRef        string `datastore:"gitiles_commit.ref"`
	GitilesCommitRevision   string `datastore:"gitiles_commit.revision"`
	GitilesCommitServerHost string `datastore:"gitiles_commit.server_host"`
}

// Get function fetches the SummaryCoverageData entity by creating key from the given args.
// See here for more details about SummaryCoverageData's format:
// https://source.chromium.org/chromium/infra/infra/+/main:appengine/findit/model/code_coverage.py;l=439
func (s *SummaryCoverageData) Get(ctx context.Context, client datastorage.IDataClient, host string, project string, ref string, revision string, dataType string, path string, bucket string, builder string) error {
	keyStr := fmt.Sprintf("%s$%s$%s$%s$%s$%s$%s$%s$0", host, project, ref, revision, dataType, path, bucket, builder)
	err := client.Get(ctx, s, "SummaryCoverageData", keyStr)
	if err != nil {
		return fmt.Errorf("SummaryCoverageData: %w", err)
	}
	return nil
}
