// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package client

import (
	"context"

	bisectionpb "go.chromium.org/luci/bisection/proto/v1"

	"go.chromium.org/infra/monitoring/messages"
)

// CrBug returns bug information.
type CrBug interface {
	// CrBugItems returns issue matching label.
	CrbugItems(ctx context.Context, label string) ([]messages.CrbugItem, error)
}

// Bisection returns information about failures that LUCI Bisection analyzes
type Bisection interface {
	QueryBisectionResults(c context.Context, bbid int64, stepName string) (*bisectionpb.QueryAnalysisResponse, error)
	BatchGetTestAnalyses(c context.Context, req *bisectionpb.BatchGetTestAnalysesRequest) (*bisectionpb.BatchGetTestAnalysesResponse, error)
}

// CrRev returns redirects for commit positions.
type CrRev interface {
	// GetRedirect gets the redirect for a commit position.
	GetRedirect(c context.Context, pos string) (map[string]string, error)
}
