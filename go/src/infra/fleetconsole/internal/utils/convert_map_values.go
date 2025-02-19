// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

func ConvertMapValues[K comparable, T any, G any](input map[K]T, f func(x T) G) map[K]G {
	out := make(map[K]G, len(input))

	for k, v := range input {
		out[k] = f(v)
	}
	return out
}
