// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"testing"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/testingtools"
)

func TestAvlReferenceParser1(t *testing.T) {
	reference := "wireless_3646_10975"
	name, compType, approuval_id, index := parseComponentReference(reference)
	testingtools.Equal(t, name, "avl:3646")
	testingtools.Equal(t, compType, "wireless")
	testingtools.Equal(t, approuval_id, "10975")
	testingtools.Equal(t, index, "")
}

func TestAvlReferenceParser2(t *testing.T) {
	reference := "wireless_3645_7066#1"
	name, compType, approuval_id, index := parseComponentReference(reference)
	testingtools.Equal(t, name, "avl:3645")
	testingtools.Equal(t, compType, "wireless")
	testingtools.Equal(t, approuval_id, "7066")
	testingtools.Equal(t, index, "1")
}

func TestAvlReferenceParser3(t *testing.T) {
	reference := "Stonepeak_rev_0xb9"
	name, compType, approuval_id, index := parseComponentReference(reference)
	testingtools.Equal(t, name, "Stonepeak_rev_0xb9")
	testingtools.Equal(t, compType, "")
	testingtools.Equal(t, approuval_id, "")
	testingtools.Equal(t, index, "")
}
