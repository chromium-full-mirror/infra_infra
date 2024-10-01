// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package version provides runner for version of CLI.
package version

import (
	"context"
	"log"
)

const (
	// Version is version of the server.
	Version = "0.1"
)

type versioner struct {
}

func New() *versioner {
	return &versioner{}
}

func (v *versioner) Run(ctx context.Context) error {
	log.Printf("Version: %s\n", Version)
	return nil
}
