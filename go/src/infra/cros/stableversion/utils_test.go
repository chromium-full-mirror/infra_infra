// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stableversion

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFindMatchMap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		regexp string
		input  string
		output map[string]string
		ok     bool
	}{
		{
			regexp: `\A(?P<a>a+)(?P<b>b+)\z`,
			input:  "aaaabbb",
			output: map[string]string{
				"a": "aaaa",
				"b": "bbb",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			pattern := regexp.MustCompile(tt.regexp)
			out, err := findMatchMap(pattern, tt.input)
			if diff := cmp.Diff(tt.output, out); diff != "" {
				t.Errorf("-want +got: %s", diff)
			}
			if diff := cmp.Diff(tt.ok, err != nil); diff != "" {
				t.Errorf("-want +got: %s", diff)
			}
		})
	}
}
