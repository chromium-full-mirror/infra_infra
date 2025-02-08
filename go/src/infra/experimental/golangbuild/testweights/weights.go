// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testweights contains test weights for the test sharding strategy
// for golangbuild.
package testweights

// GoDistTest returns a weight for the test by name.
func GoDistTest(name string) int {
	if weight, ok := goDistTestWeights[name]; ok {
		return weight
	}
	return 1
}
