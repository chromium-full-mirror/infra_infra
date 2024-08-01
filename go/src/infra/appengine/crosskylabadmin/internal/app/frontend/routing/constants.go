// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package routing

import "infra/libs/skylab/common/heuristics"

const (
	// ProdTaskType represents a decision to use the paris stack for this request.
	Paris = heuristics.ProdTaskType

	// LatestTaskType represents a decision to use the paris stack on the latest channel for this request.
	ParisLatest = heuristics.LatestTaskType
)

// Reason is a rationale for why we made the decision that we made.
type Reason int

const (
	ParisNotEnabled Reason = iota
	AllDevicesAreOptedIn
	NoPools
	WrongPool
	ScoreBelowThreshold
	ScoreTooHigh
	ThresholdZero
	MalformedPolicy
	NilArgument
	NotALabstation
	ErrorExtractingPermilleInfo
	InvalidRangeArgument
	RepairOnlyField
)

// ReasonMessageMap maps each reason to a readable description.
var ReasonMessageMap = map[Reason]string{
	ParisNotEnabled:             "PARIS is not enabled",
	AllDevicesAreOptedIn:        "All devices are opted in",
	NoPools:                     "Device has no pools, possibly due to error calling UFS",
	WrongPool:                   "Device has a pool not matching opted-in pools",
	ScoreBelowThreshold:         "Random score associated with is below threshold, authorizing new flow",
	ScoreTooHigh:                "Random score associated with task is too high",
	ThresholdZero:               "Route labstation repair task: a threshold of zero implies that optinAllLabstations should be set, but optinAllLabstations is not set",
	MalformedPolicy:             "Unrecognized policy",
	NilArgument:                 "A required argument was unexpectedly nil",
	NotALabstation:              "Paris not enabled yet for non-labstations",
	ErrorExtractingPermilleInfo: "Error extracting permille info",
}
