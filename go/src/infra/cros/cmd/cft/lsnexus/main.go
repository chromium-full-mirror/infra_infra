// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements main function to start CLI.
package main

import (
	"log"
	"os"

	"go.chromium.org/infra/cros/cmd/cft/lsnexus/internal/server"
)

const (
	Name        = "lsnexus"
	ArtifactDir = "/tmp/lsnexus"
)

func main() {
	err := server.StartServer(Name, ArtifactDir)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
