// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/publish/ants-publish/cli"
	"go.chromium.org/infra/cros/cmd/cft/publish/ants-publish/service"
	"go.chromium.org/infra/cros/cmd/cft/publish/servertemplate"
)

const (
	Name        = "ants-publish"
	ArtifactDir = "/tmp/ants-publish"
)

type AntsPublishService struct{}

func (s *AntsPublishService) Publish(ctx context.Context, log *log.Logger, req *api.PublishRequest) (*api.PublishResponse, error) {
	out := &api.PublishResponse{
		Status: api.PublishResponse_STATUS_SUCCESS,
	}

	aps, err := service.NewAntsPublishService(ctx, req)
	if err != nil {
		var invErr service.InvocationSealedError
		if errors.Is(err, &invErr) {
			log.Printf("Skipping publishing to Ants due to: %s", err)
			return out, nil
		}
		log.Printf("failed to create new ants publish service: %s", err)
		out.Status = api.PublishResponse_STATUS_INVALID_REQUEST
		out.Message = fmt.Sprintf("failed to create new ants publish service: %s", err.Error())
		return out, fmt.Errorf("failed to create new ants publish service: %w", err)
	}

	if err := aps.UploadToAnts(ctx); err != nil {
		log.Printf("upload to ants failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload to ants: %s", err.Error())
		return out, fmt.Errorf("failed upload to ants: %w", err)
	}

	if err := aps.UploadArtifacts(ctx); err != nil {
		log.Printf("upload artifacts to ants failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload artifacts to ants: %s", err.Error())
		return out, fmt.Errorf("failed artifacts upload to ants: %w", err)
	}

	log.Println("Finished Successfuly!")

	return out, nil
}

func main() {
	switch os.Args[1] {
	case "test":
		testCommand := cli.NewTestCommand()
		testCommand.Run()
	// Set default to server.
	default:
		service := &AntsPublishService{}
		servertemplate.Server(Name, ArtifactDir, service)
	}
}
