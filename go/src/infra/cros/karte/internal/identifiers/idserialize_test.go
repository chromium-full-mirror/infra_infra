// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package identifiers

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// TestVersionlessBytes tests that lowering the version-free portion of an IDInfo works.
func TestVersionlessBytes(t *testing.T) {
	t.Parallel()
	input := &idInfo{
		Version:        "zzzz",
		CoarseTime:     0xF1F2F3F4F5F6F7F8,
		FineTime:       0xF1F2F3F4,
		Disambiguation: 0xF1F2F3F4,
	}
	expected := hex.EncodeToString([]byte("\xF1\xF2\xF3\xF4\xF5\xF6\xF7\xF8\xF1\xF2\xF3\xF4\xF1\xF2\xF3\xF4"))
	bytes, err := input.VersionlessBytes()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	actual := hex.EncodeToString(bytes)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected diff (-want +got): %s", diff)
	}
}

// TestEncodedResultContainsVersion tests that the encoded result begins with a version prefix.
func TestEncodedResultContainsVersion(t *testing.T) {
	t.Parallel()
	input := &idInfo{
		Version:        "zzzz",
		CoarseTime:     2,
		FineTime:       3,
		Disambiguation: 4,
	}
	str, err := input.Encoded()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if strings.HasPrefix(str, "zzzz") {
		// Do nothing. Output string has correct prefix.
	} else {
		t.Errorf("str %q (hex) unexpectedly lacks prefix", hex.EncodeToString([]byte(str)))
	}
}

// TestEncodedReturnsValidUTF8 tests that the encoding strategy returns valid UTF8.
func TestEncodedReturnsValidUTF8(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   idInfo
	}{
		{
			name: "empty",
			in: idInfo{
				Version: "zzzz",
			},
		},
		{
			name: "ones",
			in: idInfo{
				Version:        "zzzz",
				CoarseTime:     1,
				FineTime:       1,
				Disambiguation: 1,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bytes, _ := tt.in.VersionlessBytes()
			str, err := tt.in.Encoded()
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}
			if utf8.ValidString(str) {
				// Do nothing. Test successful.
			} else {
				t.Errorf("idinfo does not serialize to a utf-8 string: %q --> %q", hex.EncodeToString(bytes), hex.EncodeToString([]byte(str)))
			}
		})
	}
}

// TestEndToEnd tests the exact encoding of an IDInfo.
// We are upstreaming lex64 from Karte to LUCI, so we need to make sure that IDs
// have the same interpretation before and after this change.
func TestEndToEnd(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		idInfo idInfo
		out    string
		ok     bool
	}{
		{
			name: "karte record at the beginning of time",
			idInfo: idInfo{
				Version:        "zzzz",
				CoarseTime:     0,
				FineTime:       0,
				Disambiguation: 0,
			},
			out: "zzzz0000000000000000000000",
			ok:  true,
		},
		{
			name: "karte record at the beginning of 2022",
			idInfo: idInfo{
				Version: "zzzz",
				// Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location)
				CoarseTime:     uint64(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC).Unix()),
				FineTime:       0,
				Disambiguation: 0,
			},
			out: "zzzz0000067EaN000000000000",
			ok:  true,
		},
		{
			name: "karte record with many non-default values",
			idInfo: idInfo{
				Version: "zzzz",
				// Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location)
				CoarseTime:     uint64(time.Date(2022, 1, 1, 7, 3, 1, 5, time.UTC).Unix()),
				FineTime:       2,
				Disambiguation: 9,
			},
			out: "zzzz0000067Ez=J0000200002F",
			ok:  true,
		},
		{
			name: "karte record with many non-default values",
			idInfo: idInfo{
				Version: "zzzz",
				// Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location)
				CoarseTime:     uint64(time.Date(2022, 9, 7, 7, 1, 0, 5, time.UTC).Unix()),
				FineTime:       274,
				Disambiguation: 633,
			},
			out: "zzzz000006BNFPk0004H0002TF",
			ok:  true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := tt.idInfo.Encoded()
			switch {
			case tt.ok && err != nil:
				t.Errorf("unexpected error %s", err)
			case !tt.ok && err == nil:
				t.Error("error is unexpectedly nil")
			}
			if diff := cmp.Diff(tt.out, actual); diff != "" {
				t.Errorf("unexpcted diff (-want +got): %s", diff)
			}
		})
	}
}
