// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

func Filter[T any](list []T, f func(T) bool) []T {
	var out []T
	for _, e := range list {
		if f(e) {
			out = append(out, e)
		}
	}
	return out
}
