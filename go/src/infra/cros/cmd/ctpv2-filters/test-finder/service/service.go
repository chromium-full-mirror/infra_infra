// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package service implements the logic of test-finder.
package service

import (
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/finders"
)

func FindTests(ctx context.Context, req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	finders := getFindersFromRequest(ctx, req, log)
	allResults := []*api.InternalTestplan{}
	for _, finder := range finders {
		rspn, err := finder.FindTestsAB()
		if err != nil {
			// TBD on if we want 1 finder to stop all, or isolate them.
			return nil, err
		}
		// TODO, determine if we want a no-find to be an empty list or nil
		if rspn != nil {
			allResults = append(allResults, rspn)
		}
	}
	return flattenResults(allResults), nil
}

func getFindersFromRequest(ctx context.Context, req *api.InternalTestplan, log *log.Logger) []common.FinderInterface {
	g3MoblyFinder := finders.NewG3MoblyFinder(ctx, req, log)
	// For now, we will use all. To be adjusted.
	// TODO implement; currently just building up the logical flow + signatures.
	finders := []common.FinderInterface{g3MoblyFinder}
	return finders
}

// TODO implement; currently just building up the logical flow + signatures.
func flattenResults(allResults []*api.InternalTestplan) *api.InternalTestplan {
	return nil
}
