// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package validation

import (
	"reflect"

	errors "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
)

func Map[Tin, Tout any](sourceTable []Tin, f func(int, Tin) Tout) []Tout {
	mapped := make([]Tout, len(sourceTable))
	for i, v := range sourceTable {
		mapped[i] = f(i, v)
	}
	return mapped
}

func CheckFieldIsNotNil(value interface{}) error {
	structValue := reflect.ValueOf(value)
	if structValue.Kind() == reflect.Pointer {
		if structValue.IsNil() {
			return errors.NewErrorf("Unexpected nil.")
		}
	}
	return nil
}

func CheckFieldIsEmptyString(value interface{}) error {
	structValue := reflect.ValueOf(value)
	if structValue.Kind() == reflect.Pointer {
		if structValue.IsNil() {
			return errors.NewErrorf("Expected a non null string pointer instead of:%v", value)
		}
		structValue = structValue.Elem()
	}

	if structValue.Kind() == reflect.String {
		if structValue.String() == "" {
			return errors.NewErrorf("The string field is \"\"")
		}
	} else {
		return errors.NewErrorf("Expected a string or string pointer, instead of %d", structValue.Len())
	}
	return nil
}
