// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// cel2json is a simple utility for taking cel-encoded files
// and dumping them as json.
//
// This is useful for exploring the CEL syntax and for writing quick
// tools that use it.
package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestCel2JSON(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("testdata/*.cel")
	if err != nil {
		t.Fatal(err)
	}

	if err := doMain(files); err != nil {
		t.Errorf("failed to parse files; %s", err)
	}
}

// TestCel2JSON_Bad tests that each badcel file is individually bad due to parse issues (and not some other kind of error).
//
// All happy families are alike; each unhappy family is unhappy in its own way.
func TestCel2JSON_Bad(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("testdata/*.badcel")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		err := doMain([]string{file})
		if err == nil {
			t.Errorf("file %q should not have parsed", file)
		}
		var e *celParseError
		if ok := errors.As(err, &e); !ok {
			t.Errorf("file %q failed with an unexpected kind of error: %s", file, err)
		}
	}
}
