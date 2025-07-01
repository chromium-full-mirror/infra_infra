// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command labservice implements the Chrome OS Lab Service.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"

	"go.chromium.org/infra/cros/cmd/labservice/server"
)

func main() {
	// Configure the default Go logger only for handling fatal
	// errors in main and any libraries that are using it.
	// Otherwise, labservice code should use the labservice
	// internal log package.
	log.SetPrefix("labservice: ")
	if err := innerMain(); err != nil {
		log.Fatalf("Fatal error: %s", err)
	}
}

func innerMain() error {
	var (
		addr               = flag.String("addr", "0.0.0.0:1485", "Address to run tis service!")
		serviceAccountPath = flag.String("service-account-json", "", "Path to service account JSON file")
		ufsService         = flag.String("ufs-service", "ufs.api.cr.dev", "UFS service host")
	)
	var preferredCachingServices strSlice
	flag.Var(&preferredCachingServices, "preferred-caching-services", "Comma separated preferred caching services (each in format: [http://]server[:port]) which superseded the ones fetche from UFS")

	flag.Parse()
	s, err := server.New(*addr, &server.Config{
		PreferredCachingServices: preferredCachingServices,
		ServiceAccountPath:       *serviceAccountPath,
		UFSService:               *ufsService,
	})
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, handledSignals...)
	ctx := context.Background()
	// This goroutine exits when the program exits.
	go func() {
		for sig := range c {
			// Handle asynchronously so we can handle
			// cases like getting a SIGINT (graceful stop)
			// followed by a SIGTERM (immediate stop).
			go handleSignal(ctx, s, sig)
		}
	}()
	return s.Start()
}

// strSlice implements flag.Value interface for specify multiple value.
type strSlice []string

func (s *strSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *strSlice) Set(value string) error {
	if value == "" {
		return nil
	}
	*s = strings.Split(value, ",")
	return nil
}
