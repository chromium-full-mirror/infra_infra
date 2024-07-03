// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package site

// ReservedCostIndicatorNames names maps names that are not present in any cost model to
// the corresponding names that should be used instead.
var ReservedCostIndicatorNames = map[string]string{
	"connectivity-rack-setup":         "rack-setup",
	"connectivity-phase-deployments":  "phase-deployments",
	"connectivity-annual-maintenance": "annual-maintenance",
	"quota-faft-rack-setup":           "rack-setup",
	"quota-faft-phase-deployments":    "phase-deployments",
	"quota-faft-annual-maintenance":   "annual-maintenance",
}
