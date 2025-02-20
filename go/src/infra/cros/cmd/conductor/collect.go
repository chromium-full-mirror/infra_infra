// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "go.chromium.org/chromiumos/infra/proto/go/chromiumos"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/try/try"
	"go.chromium.org/infra/cros/internal/shared"
)

// retryBuild retries a build using `cros try retry`. It returns the BBID of the
// new build.
func (c *collectRun) retryBuild(build *bbpb.Build) (string, error) {
	opts := &try.RetryRunOpts{
		StdoutLog: c.stdoutLog,
		StderrLog: c.stderrLog,
		CmdRunner: c.cmdRunner,

		BBID:   fmt.Sprintf("%d", build.GetId()),
		Dryrun: c.dryrun,
	}
	return c.tryClient.DoRetry(opts)
}

// filterReturnSet includes only the BBIDs that have a value of "true" in the
// given map.
func filterReturnSet(returnSet map[string]bool) []string {
	returnBBIDs := []string{}
	for bbid, include := range returnSet {
		if include {
			returnBBIDs = append(returnBBIDs, bbid)
		}
	}
	return returnBBIDs
}

// getOriginalBBID traverses backwards through the previousBBID to find the
// original BBID for a retry build.
func getOriginalBBID(previousBBID map[string]string, BBID string) string {
	prevBBID, ok := previousBBID[BBID]
	if !ok {
		return BBID
	}
	return getOriginalBBID(previousBBID, prevBBID)
}

// Collect collects on the specified BBIDs, retrying as configured.
// It returns the final set of BBIDs (the last retry for each build) and any
// errors.
func (c *collectRun) Collect(ctx context.Context, config *pb.CollectConfig, initialRetry bool) (*CollectOutput, error) {
	state := initCollectState(&collectStateOpts{
		config:            config,
		initialBuildCount: len(c.bbids),
		stdoutLog:         c.stdoutLog,
		stderrLog:         c.stderrLog})
	report := &CollectReport{}
	watchSet := c.bbids

	pollingDelay := time.Duration(c.pollingIntervalSeconds) * time.Second
	// Retry `bb get` as needed, waiting multiples of the polling interval.
	// Want to include a little backoff in case there's a legimiate outage.
	bbRetryOpts := shared.Options{
		BaseDelay:   pollingDelay,
		BackoffBase: 1.5,
		Retries:     5,
	}

	errs := []error{}
	previousBuild := map[string]string{}
	// Will only keep the most recent retry.
	returnSet := map[string]bool{}

	// We don't want to log "Waiting..."/"Sleeping..." messages over and over,
	// so we'll go quiet after the first one and only continue logging if
	// we've logged something else in the meantime.
	quiet := false
	justWentQuiet := false

	for len(watchSet) > 0 {
		sort.Strings(watchSet)
		if justWentQuiet {
			c.LogOut("(Omitting identical log messages)")
		}
		justWentQuiet = false
		if !quiet {
			c.LogOut("Waiting for %s.", strings.Join(watchSet, ","))
			c.LogOut("Sleeping for %d seconds", c.pollingIntervalSeconds)
			quiet = true
			justWentQuiet = true
		}
		time.Sleep(pollingDelay)

		ch := make(chan []*bbpb.Build, 1)
		err := shared.DoWithRetry(ctx, bbRetryOpts, func() error {
			builds, err := c.bbClient.GetBuilds(context.Background(), watchSet)
			if err != nil {
				return err
			}
			ch <- builds
			return nil
		})
		if err != nil {
			return &CollectOutput{
				BBIDs:  append(filterReturnSet(returnSet), watchSet...),
				Report: report,
			}, err
		}
		builds := <-ch

		watchSetMap := map[string]bool{}
		for _, bbid := range watchSet {
			watchSetMap[bbid] = true
		}

		newWatchSet := []string{}
		for _, build := range builds {
			bbid := fmt.Sprintf("%d", build.GetId())
			if _, ok := watchSetMap[bbid]; !ok {
				continue
			}
			if (int(build.GetStatus()) & int(bbpb.Status_ENDED_MASK)) == 0 {
				newWatchSet = append(newWatchSet, bbid)
			} else {
				quiet = false
				c.LogOut("Build %d finished with status %s", build.GetId(), build.GetStatus())
				returnSet[bbid] = true
				previousBBID, isRetry := previousBuild[bbid]
				if isRetry {
					returnSet[previousBBID] = false
				}
				originalBBID := getOriginalBBID(previousBuild, bbid)
				report.recordBuild(build, originalBBID, isRetry)
				if build.GetStatus() != bbpb.Status_SUCCESS {
					if initialRetry || state.canRetry(build, originalBBID) {
						if c.dryrun {
							c.LogOut("(Dryrun) Would have retried %d", build.GetId())
						} else {
							if newBBID, err := c.retryBuild(build); err != nil {
								errs = append(errs, err)
								c.LogErr("Failed to retry %d: %v", build.GetId(), err)
								c.LogErr("Continuing with best effort collection")
							} else {
								c.LogOut("Retrying %s with build %s.", bbid, newBBID)
								newWatchSet = append(newWatchSet, newBBID)
								previousBuild[newBBID] = bbid
								state.recordRetry(build, originalBBID)
							}
						}
					}
				}
			}
		}
		watchSet = newWatchSet
		initialRetry = false
	}
	output := &CollectOutput{
		BBIDs:  filterReturnSet(returnSet),
		Report: report,
	}
	if len(errs) > 0 {
		return output, errors.NewMultiError(errs...)
	}
	return output, nil
}
