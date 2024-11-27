// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package misc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"infra/cros/recovery/models"
)

// TestWriteLocalStableVersion tests stable version file creation
func TestWriteLocalStableVersion(t *testing.T) {
	t.Parallel()

	rv := &models.RecoveryVersion{
		Board:     "zork",
		Model:     "gumboz",
		OsImage:   "R115-15474.70.0",
		FwVersion: "Google_Berknip.13434.356.0",
		FwImage:   "zork-firmware/R87-13434.819.0",
	}

	// Perform our test in a temporary file managed by the test framework.
	path := filepath.Join(t.TempDir(), "tmp", "recovery_versions")

	if err := os.RemoveAll(path); err != nil {
		t.Errorf("Unexpected err: %v", err)
	}
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Errorf("Unexpected err: %v", err)
	}
	if err := WriteLocalStableVersion(rv, path); err != nil {
		t.Errorf("Unexpected err: %v", err)
	}
	file := fmt.Sprintf("%s%s-%s.json", path, rv.Board, rv.Model)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		t.Errorf("Unexpected err: %v", err)
	}

	savedRecoveryVersion, err := os.ReadFile(file)
	if err != nil {
		t.Errorf("Unexpected err: %v", err)
	}

	rv2 := &models.RecoveryVersion{}
	_ = json.Unmarshal([]byte(savedRecoveryVersion), rv2)

	if !reflect.DeepEqual(rv, rv2) {
		t.Errorf("Recovery version saved incorrectly")
	}
}
