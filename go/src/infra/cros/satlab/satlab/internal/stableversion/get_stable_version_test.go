// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"testing"
)

func TestValidateArgs(t *testing.T) {
	var tests = []struct {
		name string
		sv   getStableVersionRun
		err  bool
	}{
		{"All required params", getStableVersionRun{board: "zork", model: "gumboz"}, false},
		{"Lack of a board name", getStableVersionRun{model: "gumboz"}, true},
		{"Lack of a model name", getStableVersionRun{board: "zork"}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.sv.validateArgs()
			if tt.err && err == nil {
				t.Fatal("want an error for invalid args")
			}
			if !tt.err && err != nil {
				t.Fatalf("got an error (%s) for valid args", err.Error())
			}
		})
	}
}
