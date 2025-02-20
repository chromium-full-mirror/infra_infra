// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bqsuites provides a common interface for publishing
// Suites and SuiteSets to BigQuery.
package bqsuites

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"

	"go.chromium.org/infra/cros/cmd/suite_publisher/internal/suite"
)

// BuildInfo holds the build and version info that is associated with
// published Suites/SuiteSets
type BuildInfo struct {
	BuildTarget   string
	CrosMilestone string
	CrosVersion   string
}

// PublishInfo is the information needed to publish a Suite or SuiteSet
// to BigQuery.
type PublishInfo struct {
	Suite suite.CentralizedSuite
	Build BuildInfo
}

type ClosurePublishInfo struct {
	Closure *suite.SuiteClosure
	Build   BuildInfo
}

// PublishSuite publishes a suite to BigQuery using the provided inserter.
func PublishSuite(ctx context.Context, inserter *bigquery.Inserter, suite *PublishInfo) error {
	return inserter.Put(ctx, suite)
}

// Save implements the ValueSaver interface so that write operations can write
// from a PublishInfo struct to the database.
func (p *PublishInfo) Save() (map[string]bigquery.Value, string, error) {
	ret := map[string]bigquery.Value{
		"id": p.Suite.ID(),
	}
	if err := saveBuildInfo(&p.Build, ret); err != nil {
		return nil, "", err
	}
	if err := saveMetadata(p.Suite.Metadata(), ret); err != nil {
		return nil, "", err
	}
	ret["test_ids"] = p.Suite.Tests()
	ret["suites"] = p.Suite.Suites()
	ret["suite_sets"] = p.Suite.SuiteSets()
	dedupeID := fmt.Sprintf("%s.%s.%s", ret["id"], ret["build_target"], ret["cros_version"])
	return ret, dedupeID, nil
}

// PublishSuiteClosures publishes a list of SuiteClosures to BigQuery
// using the provided inserter, these are used for quicker lookups of relation
// info between suites and suite sets.
func PublishSuiteClosures(ctx context.Context, inserter *bigquery.Inserter, closures []*ClosurePublishInfo) error {
	return inserter.Put(ctx, closures)
}

// Save implements the ValueSaver interface so that write operations can write
// from a ClosurePublishInfo struct to the database.
func (p *ClosurePublishInfo) Save() (map[string]bigquery.Value, string, error) {
	ret := map[string]bigquery.Value{
		"id":    p.Closure.ID,
		"child": p.Closure.Child,
		"depth": p.Closure.Depth,
		"path":  p.Closure.Path,
	}
	if err := saveBuildInfo(&p.Build, ret); err != nil {
		return nil, "", err
	}
	return ret, "", nil
}

func saveMetadata(metadata *suite.Metadata, v map[string]bigquery.Value) error {
	if metadata == nil {
		return fmt.Errorf("expected non-nil metadata")
	}
	v["owners"] = metadata.Owners
	v["criteria"] = metadata.Criteria
	v["bug_component"] = metadata.BugComponent
	return nil
}

func saveBuildInfo(build *BuildInfo, v map[string]bigquery.Value) error {
	if build == nil {
		return fmt.Errorf("expected non-nil build")
	}
	v["cros_milestone"] = build.CrosMilestone
	v["cros_version"] = build.CrosVersion
	v["build_target"] = build.BuildTarget
	return nil
}
