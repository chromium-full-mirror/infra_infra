// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"reflect"
	"testing"
)

func TestParseValue(t *testing.T) {
	tests := []struct {
		name          string
		originalValue interface{}
		newValueStr   string
		expectedValue interface{}
		expectedError bool
		errorMsg      string
	}{
		{
			name:          "String to String",
			originalValue: "old string",
			newValueStr:   "new string",
			expectedValue: "new string",
			expectedError: false,
		},
		{
			name:          "String to Bool (true)",
			originalValue: false,
			newValueStr:   "true",
			expectedValue: true,
			expectedError: false,
		},
		{
			name:          "String to Bool (false)",
			originalValue: true,
			newValueStr:   "false",
			expectedValue: false,
			expectedError: false,
		},
		{
			name:          "String to Invalid Bool",
			originalValue: true,
			newValueStr:   "not-a-bool",
			expectedValue: false,
			expectedError: true,
			errorMsg:      `strconv.ParseBool: parsing "not-a-bool": invalid syntax`,
		},
		{
			name:          "String to Int",
			originalValue: 10,
			newValueStr:   "20",
			expectedValue: 20,
			expectedError: false,
		},
		{
			name:          "String to Invalid Int",
			originalValue: 10,
			newValueStr:   "abc",
			expectedValue: 0,
			expectedError: true,
			errorMsg:      `strconv.Atoi: parsing "abc": invalid syntax`,
		},
		{
			name:          "String to Float64",
			originalValue: 10.5,
			newValueStr:   "20.7",
			expectedValue: 20.7,
			expectedError: false,
		},
		{
			name:          "String to Invalid Float64",
			originalValue: 10.5,
			newValueStr:   "xyz",
			expectedValue: 0.0,
			expectedError: true,
			errorMsg:      `strconv.ParseFloat: parsing "xyz": invalid syntax`,
		},
		{
			name:          "Unsupported Type (map)",
			originalValue: map[string]int{"a": 1},
			newValueStr:   "some value",
			expectedValue: nil,
			expectedError: true,
			errorMsg:      "unsupported type for key 'map[a:1]'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualValue, err := parseValue(tt.originalValue, tt.newValueStr)

			if (err != nil) != tt.expectedError {
				t.Errorf("parseValue() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if err != nil && err.Error() != tt.errorMsg {
				t.Errorf("parseValue() error message = %q, want %q", err.Error(), tt.errorMsg)
			}

			if !reflect.DeepEqual(actualValue, tt.expectedValue) {
				t.Errorf("parseValue() value = %v (type %T), want %v (type %T)", actualValue, actualValue, tt.expectedValue, tt.expectedValue)
			}
		})
	}
}
