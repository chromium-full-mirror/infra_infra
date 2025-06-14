// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testweights contains test weights for the test sharding strategy
// for golangbuild.
package testweights

// For returns a weight for the test by name.
func For(builderName, testName string) float64 {
	if builderWeights, ok := allWeights[builderName]; ok {
		if weight, ok := builderWeights[testName]; ok {
			return weight
		}
	}
	return 0 // No information, assume short.
}

// SupportedBuilders returns the complete set of builders that have weights
// available for them.
func SupportedBuilders() []string {
	var builders []string
	for builderName := range allWeights {
		builders = append(builders, builderName)
	}
	return builders
}
