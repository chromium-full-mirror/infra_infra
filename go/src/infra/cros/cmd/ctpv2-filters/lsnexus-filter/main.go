// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

type LSNexusFilter struct {
	servertemplate.FilterBase
}

func NewLSNexusFilter() servertemplate.Filter {
	return &LSNexusFilter{}
}

func (*LSNexusFilter) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	// Create Dynamic Updates and lookup information.
	if err := GenerateDynamicInfo(req, log, commonParams); err != nil {
		return req, err
	}

	return req, nil
}

func main() {
	err := servertemplate.Server(NewLSNexusFilter, "lsnexus_filter")
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
