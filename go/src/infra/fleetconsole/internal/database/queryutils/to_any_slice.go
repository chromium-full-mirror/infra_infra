// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package queryutils

// ToAnySlice converts a slice of any type to a slice of any.
func ToAnySlice[T any](s []T) []any {
	if s == nil {
		return nil
	}
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
