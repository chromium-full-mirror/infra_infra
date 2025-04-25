// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/publish/cpcon-publish/service"
	"go.chromium.org/infra/cros/cmd/cft/publish/servertemplate"
)

const (
	Name        = "cpcon-publish"
	ArtifactDir = "/tmp/cpcon-publish"
)

type CpconPublishService struct{}

func (s *CpconPublishService) Publish(ctx context.Context, log *log.Logger, req *api.PublishRequest) (*api.PublishResponse, error) {
	out := &api.PublishResponse{
		Status: api.PublishResponse_STATUS_SUCCESS,
	}

	gps, err := service.NewCpconPublishService(req)
	if err != nil {
		log.Printf("failed to create new cpcon publish service: %s", err)
		out.Status = api.PublishResponse_STATUS_INVALID_REQUEST
		out.Message = fmt.Sprintf("failed to create new cpcon publish service: %s", err.Error())
		return out, fmt.Errorf("failed to create new cpcon publish service: %w", err)
	}

	if err := gps.UploadToCpcon(context.Background()); err != nil {
		log.Printf("upload to cpcon failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload to cpcon: %s", err.Error())
		return out, fmt.Errorf("failed upload to cpcon: %w", err)
	}

	log.Println("Finished Successfuly!")

	return out, nil
}

func main() {
	service := &CpconPublishService{}
	servertemplate.Server(Name, ArtifactDir, service)
}
