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

type ExamplePublishService struct{}

func (service *ExamplePublishService) Publish(ctx context.Context, log *log.Logger, req *api.PublishRequest) (*api.PublishResponse, error) {
	return &api.PublishResponse{}, nil
}

func main() {
	service := &ExamplePublishService{}
	flagSet := flag.NewFlagSet(fmt.Sprintf("%s flagset", Name), flag.ExitOnError)
	ServerWithFlagSet(Name, ArtifactDir, flagSet, service)
}
