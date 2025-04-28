// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestErrLinks(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		var err error
		err = attachLinks(err, "name", "url")
		if err != nil {
			t.Fatal("got non-nil error from attaching links to a nil error")
		}
		links := extractLinks(err)
		if len(links) != 0 {
			t.Fatal("found links on nil error")
		}
	})
	t.Run("bad attach", func(t *testing.T) {
		defer func() {
			if e := recover(); e == nil {
				t.Fatal("expected panic")
			}
		}()
		attachLinks(nil, "name") //nolint:staticcheck // SA5012 intentional test that odd links panics
	})
	t.Run("tree", func(t *testing.T) {
		want := []link{
			{name: "name0", url: "url0"},
			{name: "name1", url: "url1"},
			{name: "name2", url: "url2"},
			{name: "name3", url: "url3"},
			{name: "name4", url: "url4"},
			{name: "name5", url: "url5"},
		}
		// Test simple attachment.
		err := fmt.Errorf("my error")
		err = attachLinks(err, want[0].name, want[0].url)
		if err == nil {
			t.Fatal("got nil error from attaching links to a non-nil error")
		}

		// Test simple wrapping and unwrapping.
		err = fmt.Errorf("wrapped: %w", err)
		compareLinks(t, extractLinks(err), want[0:1])
		err = attachLinks(err, want[1].name, want[1].url)
		compareLinks(t, extractLinks(err), want[0:2])
		err = attachLinks(err)
		compareLinks(t, extractLinks(err), want[0:2])

		// Test multiple attachments.
		err2 := fmt.Errorf("my error 2")
		err2 = attachLinks(err2, want[2].name, want[2].url, want[3].name, want[3].url)
		compareLinks(t, extractLinks(err2), want[2:4])
		err2 = attachLinks(err2, want[4].name, want[4].url)
		compareLinks(t, extractLinks(err2), want[2:5])

		// Test joined errors.
		err3 := fmt.Errorf("my error 3")
		err = errors.Join(err2, err, err3)
		compareLinks(t, extractLinks(err), want[0:5])

		// Test a chain on top of the joined errors.
		err = fmt.Errorf("wrapped 2: %w", err)
		err = attachLinks(err, want[5].name, want[5].url)
		compareLinks(t, extractLinks(err), want[0:6])
	})
}

func compareLinks(t *testing.T, got, want []link) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("invalid links on error: got %#v, want %#v", got, want)
	}
	// Sort by name for comparability.
	slices.SortFunc(got, func(a, b link) int {
		return cmp.Compare(a.name, b.name)
	})
	slices.SortFunc(want, func(a, b link) int {
		return cmp.Compare(a.name, b.name)
	})
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("invalid links on error: got %#v, want %#v", got, want)
		}
	}
}

func TestErrTestsFailedMarker(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		var err error
		err = attachTestsFailed(err)
		if err != nil {
			t.Fatal("got non-nil error from attaching links to a nil error")
		}
		if errorTestsFailed(err) {
			t.Fatal("found tests failed marker on nil error")
		}
	})
	t.Run("tree", func(t *testing.T) {
		// Test simple attachment.
		err := fmt.Errorf("my error")
		err = attachTestsFailed(err)
		if err == nil {
			t.Fatal("got nil error from attaching tests failed marker to a non-nil error")
		}

		// Test simple wrapping.
		err = fmt.Errorf("wrapped: %w", err)
		if !errorTestsFailed(err) {
			t.Fatalf("expected marker, but marker not found on wrapped error")
		}

		// Test no attachments.
		err2 := fmt.Errorf("my error 2")
		if errorTestsFailed(err2) {
			t.Fatal("found tests failed marker on error without explicit marker")
		}

		// Test joined errors.
		err3 := fmt.Errorf("my error 3")
		err = errors.Join(err2, err, err3)
		if !errorTestsFailed(err) {
			t.Fatalf("expected marker, but marker not found on joined error")
		}

		// Test a chain on top of the joined errors.
		err = fmt.Errorf("wrapped 2: %w", err)
		if !errorTestsFailed(err) {
			t.Fatalf("expected marker, but marker not found on chained, then joined error")
		}
	})
}
