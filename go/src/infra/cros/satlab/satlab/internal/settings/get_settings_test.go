// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrintValues(t *testing.T) {
	tests := []struct {
		name        string
		settings    Settings
		expectedOut map[string]string
	}{
		{
			name:        "Empty Settings",
			settings:    Settings{},
			expectedOut: map[string]string{},
		},
		{
			name: "Multiple Settings",
			settings: Settings{
				"key1": "value1",
				"key2": 123,
				"key3": true,
			},
			expectedOut: map[string]string{
				"key1: value1\n": "",
				"key2: 123\n":    "",
				"key3: true\n":   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := tt.settings.PrintValues()

			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)

			outputLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			outputMap := make(map[string]string)
			for _, line := range outputLines {
				if line != "" {
					outputMap[line+"\n"] = ""
				}
			}

			if diff := cmp.Diff(tt.expectedOut, outputMap); diff != "" {
				t.Errorf("PrintValues() output mismatch (-want +got):\n%s", diff)
			}

			if err != nil {
				t.Errorf("PrintValues() returned an unexpected error: %v", err)
			}
		})
	}
}

func TestPrintValue(t *testing.T) {
	tests := []struct {
		name          string
		settings      Settings
		key           string
		expectedOut   string
		expectedError bool
		errorMsg      string
	}{
		{
			name: "Key Exists",
			settings: Settings{
				"existingKey": "value1",
				"anotherKey":  123,
			},
			key:           "existingKey",
			expectedOut:   "existingKey: value1\n",
			expectedError: false,
		},
		{
			name: "Key Does Not Exist",
			settings: Settings{
				"existingKey": "value1",
			},
			key:           "nonExistingKey",
			expectedOut:   "",
			expectedError: true,
			errorMsg:      "key 'nonExistingKey' does not exist in settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := tt.settings.PrintValue(tt.key)

			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)

			if buf.String() != tt.expectedOut {
				t.Errorf("PrintValue() output = %q, want %q", buf.String(), tt.expectedOut)
			}

			if (err != nil) != tt.expectedError {
				t.Errorf("PrintValue() error = %v, expectedError %v", err, tt.expectedError)
			} else if err != nil && err.Error() != tt.errorMsg {
				t.Errorf("PrintValue() error message = %q, want %q", err.Error(), tt.errorMsg)
			}
		})
	}
}
