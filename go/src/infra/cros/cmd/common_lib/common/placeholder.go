// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"regexp"
	"strings"
)

// PlaceholderRegex provides the format for how to find placeholders
// within the dynamic updates.
// Placeholders take the form of `${<placeholder>}`
// in which the named placeholder is wrapped by `${}`
//
// The only valid characters for the placeholder will be
// a combination of any letter, any number, and the `-` and `_`
// special characters.
const PlaceholderRegex = `\${[\w\d\-_.]+}`

// PlaceholderLookup defines an interface for getting
// a value that resolves the placeholder key.
type PlaceholderLookup interface {
	Get(key string) (val string, ok bool)
}

// ResolvePlaceholders searches the provided string for
// any placeholders and replaces them with the values
// corresponding in the lookup table.
//
// Placeholders should be a combination of letters, digits,
// hyphens, and underscores.
func ResolvePlaceholders(str string, lookup PlaceholderLookup) string {
	placeholders := regexp.MustCompile(PlaceholderRegex)
	return placeholders.ReplaceAllStringFunc(str, func(placeholder string) string {
		trimmed := strings.TrimLeft(placeholder, "${")
		lookupKey := strings.TrimRight(trimmed, "}")
		if value, ok := lookup.Get(lookupKey); ok {
			return value
		}
		return placeholder
	})
}
