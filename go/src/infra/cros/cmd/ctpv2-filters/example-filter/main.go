// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/common/servertemplate"
)

type ExampleFilter struct {
	servertemplate.FilterBase

	ExampleParam string
}

func (pru *ExampleFilter) Init(fs *flag.FlagSet, args []string) error {
	/*
		Initialize arguments here.
		If no parameters need to be set, remove this function.
	*/
	fs.StringVar(&pru.ExampleParam, "example", "", "Example param description")

	return fs.Parse(args)
}

func (pru *ExampleFilter) Executor(req *api.InternalTestplan, log *log.Logger, commonParams *common.CommonFilterParams) (*api.InternalTestplan, error) {
	/*
		Main logic of the filter goes here.
	*/

	return req, nil
}

func GenerateFilterExecutor() servertemplate.Filter {
	return &ExampleFilter{}
}

func main() {
	err := servertemplate.Server(GenerateFilterExecutor, "example-filter")
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
