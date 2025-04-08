// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements main function to start CLI.
package main

import (
	"context"
	"log"
	"os"

	"go.chromium.org/infra/cros/cmd/cft/bols_satlab/internal/parser"
)

func main() {
	ctx := context.Background()
	r, err := parser.ParseArgs(ctx, os.Args)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
		return
	}
	if r == nil {
		os.Exit(0)
		return
	}
	if err := r.Run(ctx); err != nil {
		log.Println(err.Error())
		os.Exit(1)
		return
	}
	os.Exit(0)
}
