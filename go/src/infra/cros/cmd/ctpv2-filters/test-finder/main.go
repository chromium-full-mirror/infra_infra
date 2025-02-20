// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"
	server "go.chromium.org/chromiumos/test/ctpv2/common/server_template"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/service"
)

var binName = "testFinder"

func executor(req *api.InternalTestplan, log *log.Logger, commonParams *server.CommonFilterParams) (*api.InternalTestplan, error) {
	ctx := context.Background()

	_, err := service.FindTests(ctx, req, log)
	if err != nil {
		return nil, err
	}
	return req, nil
}
func main() {
	err := server.Server(executor, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
