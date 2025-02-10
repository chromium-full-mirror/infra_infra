// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils is a set of utilities for the console server.
package utils

func FlatMap[T any, G any](slice []T, f func(x T) []G) []G {
	var out []G

	for _, el := range slice {
		out = append(out, f(el)...)
	}
	return out
}
