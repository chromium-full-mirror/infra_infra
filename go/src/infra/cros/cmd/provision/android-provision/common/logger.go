// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// SetUpLog sets up the logging.
func SetUpLog(dir string) (*log.Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %v: %w", dir, err)
	}
	lfp := filepath.Join(dir, "log.txt")
	lf, err := os.Create(lfp)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %v: %w", lfp, err)
	}
	return log.New(io.MultiWriter(lf, os.Stderr), "", log.LstdFlags|log.LUTC), nil
}
