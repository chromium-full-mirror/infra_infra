// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testingtools

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func getParentParentStackTrace() string {
	buffer := make([]byte, 2048)
	nbChars := runtime.Stack(buffer, false)
	trace := string(buffer[0:nbChars])
	res := strings.Split(trace, "\n")
	return strings.Join(res[5:], "\n")
}

// Equal test if the computed and expexted values are identical.
func Equal(t testing.TB, computed interface{}, expected interface{}) {
	switch v := expected.(type) {
	case int:
		computedTyped, ok := computed.(int)
		if !ok {
			t.Errorf("computed value is %v(%t) instead of the expected  value(int)  %v.\n    @%v",
				computed,
				computed,
				expected,
				getParentParentStackTrace())
			return
		}
		if v != computedTyped {
			t.Errorf("Computed value is %v instead of the expected value %v\n     @%v",
				computed,
				expected,
				getParentParentStackTrace())
		}
	case uint64:
		computedTyped, ok := computed.(uint64)
		if !ok {
			t.Errorf("computed value is %v(%t) instead of the expected value(uint64) %v.\n     @%v",
				computed,
				computed,
				expected,
				getParentParentStackTrace())
			return
		}
		if v != computedTyped {
			t.Errorf("Computed value is %v instead of the expected value %v\n     @%v",
				computed,
				expected,
				getParentParentStackTrace())
		}
	case int64:
		computedTyped, ok := computed.(int64)
		if !ok {
			t.Errorf("computed value is %v(%t) instead of the expected value(int64) %v.\n     @%v",
				computed,
				computed,
				expected,
				getParentParentStackTrace())
			return
		}
		if v != computedTyped {
			t.Errorf("Computed value is %v instead of the expected value %v\n     @%v",
				computed,
				expected,
				getParentParentStackTrace())
		}

	case string:
		computedTyped, ok := computed.(string)
		if !ok {
			t.Errorf("computed value is \"%v\"(%t) instead of the expected value %v(string).\n     @%v",
				computed,
				computed,
				expected,
				getParentParentStackTrace())
			return
		}
		if v != computedTyped {
			if len(v) != len(computedTyped) {
				t.Log(
					"The computed string value if of length ",
					len(v),
					" while the expected string value is of length ",
					len(computedTyped))
			} else {
				for i := range len(v) {
					if v[i] != computedTyped[i] {
						t.Logf("The first different character is %c at position %d the expected character is %c", computedTyped[i], i, v[i])
					}
				}
			}
			t.Errorf("Computed value is \"%v\" instead of the expected value \"%v\"\n     @%v",
				computed,
				expected,
				getParentParentStackTrace())
		}
	default:
		t.Errorf("equality assertions is not yet implemented for type %v\n     @%v",
			reflect.TypeOf(expected),
			getParentParentStackTrace())
	}
}

// IsNil checks that the value is invalid or a pointer with a nil value.
func IsNilOrInvalid(t testing.TB, value interface{}) {
	metaValue := reflect.ValueOf(value)
	if !metaValue.IsValid() || (metaValue.Kind() == reflect.Ptr && metaValue.IsNil()) {
		return
	}
	t.Errorf("The value was expected to be nil instead of %v \n   @%v",
		metaValue,
		getParentParentStackTrace())
}

// IsNotNil checks that the value is not a pointer with a nil value
func IsNotNil(t testing.TB, value interface{}) {
	metaValue := reflect.ValueOf(value)
	if value == nil || (metaValue.Kind() == reflect.Ptr && metaValue.IsNil()) {
		t.Errorf("The value was expected to not be nil \n   @%v",
			getParentParentStackTrace())
	}
}

// Equivalent assertion, with the following properties:
// - If the expected value is equivalent to the computed value, the computed value is also equivalent to the expected value.
// - Two pointers are Equivalent when they are same type and point to equivalent values.
// - A nil pointer is Equivalent to the nil value
// - Two basic values are Equivalent when they are of the same type and value
// - Two structures are Equivalent when they are of the same type and all the fields hold values that are Equivalent between the to structures.
func Equivalent(t testing.TB, computed interface{}, expected interface{}) {
	expectedValue := reflect.ValueOf(expected)
	computedValue := reflect.ValueOf(computed)
	if expectedValue.Kind() == reflect.Ptr {
		if computedValue.Kind() != reflect.Ptr {
			t.Errorf("The expected value is a pointer and the computed value is not. \n   @%v",
				getParentParentStackTrace())
			return
		}
		if expectedValue.IsNil() {
			if !computedValue.IsNil() {
				t.Errorf("The expected value is nil and the computed value is not. \n   @%v",
					getParentParentStackTrace())
				return
			}
		} else {
			if computedValue.IsNil() {
				t.Errorf("The expected value is not nil and the computed value is. \n   @%v",
					getParentParentStackTrace())
				return
			}
		}
		if !reflect.DeepEqual(expectedValue.Elem(), computedValue.Elem()) {
			t.Errorf("The expected value and computed value are both pointers but do not point to the same object. \n   @%v",
				getParentParentStackTrace())
			return
		}
	}
	expectedType := expectedValue.Kind()
	computedType := computedValue.Kind()
	if expectedType != computedType {
		t.Errorf("The expected value is of kind different type than the computed value.\n   Expected type:%s\n   Computed type:%s \n   @%v",
			expectedType,
			computedType,
			getParentParentStackTrace())
		return
	}
	if !reflect.DeepEqual(expected, computed) {
		t.Errorf("The expected value and the computed value are not equivalent.\n   Expected type:%s\n   Computed type:%s \n   @%v",
			expectedValue,
			computedValue,
			getParentParentStackTrace())
		return
	}
}
