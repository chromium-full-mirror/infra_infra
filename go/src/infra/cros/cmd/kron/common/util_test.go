// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"reflect"
	"testing"
)

func TestFindFactors15(t *testing.T) {
	expected := []int{
		15, 5, 3, 1,
	}
	factors := FindFactors(15)

	if !reflect.DeepEqual(factors, expected) {
		t.Errorf("Expected %+v got %+v", expected, factors)
	}
}

func TestFindFactors73(t *testing.T) {
	expected := []int{
		73, 1,
	}
	factors := FindFactors(73)

	if !reflect.DeepEqual(factors, expected) {
		t.Errorf("Expected %+v got %+v", expected, factors)
	}
}

func TestFindFactors42(t *testing.T) {
	expected := []int{
		42, 21, 14, 7, 6, 3, 2, 1,
	}
	factors := FindFactors(42)

	if !reflect.DeepEqual(factors, expected) {
		t.Errorf("Expected %+v got %+v", expected, factors)
	}
}
