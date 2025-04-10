// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package helpers

import (
	"fmt"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/dynamic_updates/common"
)

// LookupKey is a string wrapper that helps
// with defining the keys that are used within
// the dynamic lookup tables.
type LookupKey string

// WithIndex is intended for multi-dut use, where
// multiple duts may use the same base lookup key
// but should be marked with an index to indicate
// being the 2nd or 3rd o this key.
func (L LookupKey) WithIndex(idx int) LookupKey {
	// Indexes of 0 are primary, and need no indexing.
	if idx == 0 {
		return L
	}
	// Readability.
	// Increment by 1 to signify being the 2nd lookup key and onward.
	return LookupKey(fmt.Sprintf("%s_%d", L, idx+1))
}

// AsPlaceholder is intended when calling upon the lookup
// key for defining a dynamic update.
func (L LookupKey) AsPlaceholder() string {
	return common.SetPlaceholder(string(L))
}

// AsKey is intended for when adding a value
// to the lookup table.
func (L LookupKey) AsKey() string {
	return string(L)
}
