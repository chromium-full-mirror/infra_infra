// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package version provides runner for version of CLI.
package version

import (
	"context"
	"log"
)

const (
	Version = "undefined!"
)

type versioner struct {
}

func New() *versioner {
	return &versioner{}
}

func (v *versioner) Run(ctx context.Context) error {
	log.Printf("Version: %q", Version)
	return nil
}
