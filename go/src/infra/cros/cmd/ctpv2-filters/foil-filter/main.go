// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"
)

type FoilRequestUpdater struct {
	ProvisionPath   string
	ProvisionBinary string
	TestPath        string
}

func (ru *FoilRequestUpdater) executor(req *api.InternalTestplan, log *log.Logger) (*api.InternalTestplan, error) {
	log.Println("Executing request-updater filter.")

	if err := GenerateDynamicUpdates(req, ru, log); err != nil {
		log.Printf("Error while generating dynamic updates, %s", err)
		return req, err
	}
	log.Println("Finished generating dyanmic updates.")

	return req, nil
}

func main() {
	requestUpdater := &FoilRequestUpdater{}
	fs := flag.NewFlagSet("Run foil request-updater", flag.ExitOnError)
	fs.StringVar(&requestUpdater.ProvisionPath, "prov-path", "", "SHA256 value for provision container")
	fs.StringVar(&requestUpdater.ProvisionBinary, "prov-bin", "", "Binary called within provision container")
	fs.StringVar(&requestUpdater.TestPath, "test-path", "", "SHA256 value for test container")

	err := server.ServerWithFlagSet(fs, requestUpdater.executor, "request-updater")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
