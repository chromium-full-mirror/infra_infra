// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servertemplate

import (
	"context"
	"flag"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
)

const (
	Name        = "template"
	ArtifactDir = "/tmp/example-artifacts"
)

type ExampleGenericService struct {
	// Shared information between start, run, stop.
}

func (service *ExampleGenericService) Start(ctx context.Context, log *log.Logger, start *api.GenericStartRequest) (*api.GenericStartResponse, error) {
	return &api.GenericStartResponse{}, nil
}

func (service *ExampleGenericService) Run(ctx context.Context, log *log.Logger, start *api.GenericRunRequest) (*api.GenericRunResponse, error) {
	return &api.GenericRunResponse{}, nil
}

func (service *ExampleGenericService) Stop(ctx context.Context, log *log.Logger, start *api.GenericStopRequest) (*api.GenericStopResponse, error) {
	return &api.GenericStopResponse{}, nil
}

func main() {
	service := &ExampleGenericService{}
	flagSet := flag.NewFlagSet(fmt.Sprintf("%s flagset", Name), flag.ExitOnError)
	ServerWithFlagSet(Name, ArtifactDir, flagSet, service)
}
