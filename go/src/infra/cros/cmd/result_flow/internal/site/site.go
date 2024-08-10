// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package site contains site local constants for the Result Flow.
package site

import (
	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/hardcoded/chromeinfra"
)

// DefaultDeadlineSeconds is the default command deadline in seconds.
const DefaultDeadlineSeconds = 1800

const (
	// CTPBatchSize is the size of one single Buildbucket batch request to fetch
	// CTP builds. Increase the batch size below with caution. It may cause
	// Buildbucket to return 500 errors, due to the response size.
	CTPBatchSize int32 = 3
	// TestRunnerBatchSize is the size of one single Buildbucket batch request
	// to fetch test runner builds.
	TestRunnerBatchSize int32 = 50
)

const (
	authScopeBigquery = "https://www.googleapis.com/auth/bigquery"
	authScopePubsub   = "https://www.googleapis.com/auth/pubsub"
)

// DefaultAuthOptions is an auth.Options struct prefilled with chrome-infra
// defaults. The credentials here are used for local test.
var DefaultAuthOptions auth.Options

func init() {
	DefaultAuthOptions = chromeinfra.DefaultAuthOptions()
	DefaultAuthOptions.Scopes = []string{auth.OAuthScopeEmail, authScopeBigquery, authScopePubsub}
}
