// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"testing"
)

var parseContentTestCases = []struct {
	testName string
	got      string
	exp      []string
}{
	{
		"empty",
		"",
		nil,
	},
	{
		"empty-2",
		"''",
		nil,
	},
	{
		"empty-3",
		"\"\"",
		nil,
	},
	{
		"simple",
		"'Hello'",
		[]string{"Hello"},
	},
	{
		"simple-2",
		"\"Hello\"",
		[]string{"Hello"},
	},
	{
		"complex",
		"\"\\r\\n23-02-08 15:10:48.288 > \\r\\n23-02-08 15:10:48.305 > ccdstate\\r\\n23-02-08 15:10:48.330 DS Dis:  off\\r\\n23-02-08 15:10:48.330 AP:      on (F)\\r\\n23-02-08 15:10:48.330 AP UART: on\\r\\n23-02-08 15:10:48.330 EC:      on\\r\\n23-02-08 15:10:48.330 Servo:   undetectable\\r\\n23-02-08 15:10:48.330 Rdd:       connected\\r\\n\"",
		[]string{
			"",
			"23-02-08 15:10:48.288 > ",
			"23-02-08 15:10:48.305 > ccdstate",
			"23-02-08 15:10:48.330 DS Dis:  off",
			"23-02-08 15:10:48.330 AP:      on (F)",
			"23-02-08 15:10:48.330 AP UART: on",
			"23-02-08 15:10:48.330 EC:      on",
			"23-02-08 15:10:48.330 Servo:   undetectable",
			"23-02-08 15:10:48.330 Rdd:       connected",
			"",
		},
	},
	{
		"complex-2",
		"'\\r\\n23-02-08 15:10:48.288 > \\r\\n23-02-08 15:10:48.305 > ccdstate\\r\\n23-02-08 15:10:48.330 DS Dis:  off\\r\\n23-02-08 15:10:48.330 AP:      on (F)\\r\\n23-02-08 15:10:48.330 AP UART: on\\r\\n23-02-08 15:10:48.330 EC:      on\\r\\n23-02-08 15:10:48.330 Servo:   undetectable\\r\\n23-02-08 15:10:48.330 Rdd:       connected\\r\\n'",
		[]string{
			"",
			"23-02-08 15:10:48.288 > ",
			"23-02-08 15:10:48.305 > ccdstate",
			"23-02-08 15:10:48.330 DS Dis:  off",
			"23-02-08 15:10:48.330 AP:      on (F)",
			"23-02-08 15:10:48.330 AP UART: on",
			"23-02-08 15:10:48.330 EC:      on",
			"23-02-08 15:10:48.330 Servo:   undetectable",
			"23-02-08 15:10:48.330 Rdd:       connected",
			"",
		},
	},
}

func TestParseUartStreamContent(t *testing.T) {
	t.Parallel()
	for _, tt := range parseContentTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			r := parseUartStreamContent(tt.got)
			if len(r) != len(tt.exp) {
				t.Errorf("%s: length  does not mathces got:%d, expected:%d", tt.testName, len(r), len(tt.exp))
			} else if len(r) > 0 {
				for i, v := range r {
					if tt.exp[i] != v {
						t.Errorf("%s: expected to get %v, but got %v", tt.testName, tt.exp[i], v)
					}
				}
			}
		})
	}
}
