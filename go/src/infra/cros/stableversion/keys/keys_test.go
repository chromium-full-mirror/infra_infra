// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package keys

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewBuilder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     []string
		output    string
		expectErr bool
	}{
		{"nil", nil, "", false},
		{"empty", []string{}, "", false},
		{"empty key", []string{"", ""}, "", true},
		{"single: no value", []string{"one", ""}, "one=", false},
		{"single: with value", []string{"one", " val "}, "one=val", false},
		{"mix 1", []string{"one", " ", "two", "ha"}, "one=;two=ha", false},
		{"mix 2", []string{" one ", "   1 ", "two", " 2 "}, "one=1;two=2", false},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kb := NewBuilder()
			i := 0
			for len(tt.input) > i {
				n := tt.input[i]
				var v string
				i++
				if len(tt.input) > i {
					v = tt.input[i]
				}
				i++
				err := kb.Add(n, v)
				if err != nil && !tt.expectErr {
					t.Errorf("TestNewBuilder %q: got error when not expected: %s", tt.name, err)
				} else if err == nil && tt.expectErr {
					t.Errorf("TestNewBuilder %q: did not get error when expected", tt.name)
				}
			}
			out := kb.String()
			if diff := cmp.Diff(tt.output, out); diff != "" {
				t.Errorf("TestNewBuilder %q: -want +got: %s", tt.name, diff)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		board  string
		model  string
		pool   string
		output string
	}{
		{"empty", "", "", "", "board=;model=;pool="},
		{"just board", "b1", "", "", "board=b1;model=;pool="},
		{"just model", "", "m1", "", "board=;model=m1;pool="},
		{"just pool", "", "", "p1", "board=;model=;pool=p1"},
		{"just board+model", "b2", "m2", "", "board=b2;model=m2;pool="},
		{"just model+pool", "", "m3", "p3", "board=;model=m3;pool=p3"},
		{"all 1", "b4", "m4", "p4", "board=b4;model=m4;pool=p4"},
		{"all 2", "b5", "m5", "p5", "board=b5;model=m5;pool=p5"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kb := New(tt.board, tt.model, tt.pool)
			out := kb.String()
			if diff := cmp.Diff(tt.output, out); diff != "" {
				t.Errorf("TestNewKey %q: -want +got: %s", tt.name, diff)
			}
		})
	}
}
