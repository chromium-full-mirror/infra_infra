// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReadVersionFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{input: "12345", want: "12345"},
		{input: "00000", want: "00000"},
		{input: "12", want: "12"},
		{input: "12212121", want: "12212121"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()
			tmpdir, err := os.MkdirTemp("", c.input)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(tmpdir) })
			testFile := filepath.Join(tmpdir, "version_file")
			if err := os.WriteFile(testFile, []byte(c.input), 0600); err != nil {
				t.Fatal(err)
			}
			got := readVersionFile(testFile)
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("No Path", func(t *testing.T) {
		t.Parallel()
		got := readVersionFile("")
		if got != "unknown" {
			t.Errorf("Got: %v; want unknown", got)
		}
	})
	t.Run("Wrong path", func(t *testing.T) {
		t.Parallel()
		got := readVersionFile("does_not_exist/version_file")
		if got != "unknown" {
			t.Errorf("Got: %v; want unknown", got)
		}
	})
	t.Run("Non numeric version", func(t *testing.T) {
		t.Parallel()
		tmpdir, err := os.MkdirTemp("", "nonNumeric-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(tmpdir) })
		testFile := filepath.Join(tmpdir, "version_file")
		if err := os.WriteFile(testFile, []byte("aa123"), 0600); err != nil {
			t.Fatal(err)
		}
		got := readVersionFile(testFile)
		if got != "unknown" {
			t.Errorf("Got: %v; want unknown", got)
		}
	})
}
