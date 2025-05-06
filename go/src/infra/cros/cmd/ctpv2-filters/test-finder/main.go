// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/test-finder/service"
)

var binName = "testFinder"

type TestFinderFilter struct {
	servertemplate.Filter
}

func NewTestFinderFilter() servertemplate.Filter {
	return &TestFinderFilter{}
}

func (*TestFinderFilter) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	ctx := context.Background()

	err := service.FindTests(ctx, req, log)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func main() {
	err := servertemplate.Server(NewTestFinderFilter, binName)
	if err != nil {
		os.Exit(2)
	}

	os.Exit(0)
}
