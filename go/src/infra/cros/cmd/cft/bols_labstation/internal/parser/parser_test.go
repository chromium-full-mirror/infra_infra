// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package parser

import (
	"testing"
)

func TestParseRunMode(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		wantMode runMode
		wantErr  bool
	}{
		{
			name:     "server mode",
			args:     []string{"bols_labstation", "server"},
			wantMode: runServer,
			wantErr:  false,
		},
		{
			name:     "version mode",
			args:     []string{"bols_labstation", "version"},
			wantMode: runVersion,
			wantErr:  false,
		},
		{
			name:     "help mode",
			args:     []string{"bols_labstation", "help"},
			wantMode: runHelp,
			wantErr:  false,
		},
		{
			name:     "default help mode",
			args:     []string{"bols_labstation"},
			wantMode: runHelp,
			wantErr:  false,
		},
		{
			name:     "unknown mode",
			args:     []string{"bols_labstation", "unknown"},
			wantMode: runHelp,
			wantErr:  false,
		},
		{
			name:     "version mode with flag",
			args:     []string{"bols_labstation", "-version"},
			wantMode: runVersion,
			wantErr:  false,
		},
		{
			name:     "help mode with flag",
			args:     []string{"bols_labstation", "-help"},
			wantMode: runVersion,
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := parseRunMode(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseRunMode() error = %v, wantErr %v", err, tc.wantErr)
			}
			if mode != tc.wantMode {
				t.Errorf("parseRunMode() mode = %v, want %v", mode, tc.wantMode)
			}
		})
	}
}
