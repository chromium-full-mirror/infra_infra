// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/publish/gcs-publish/service"
	"go.chromium.org/infra/cros/cmd/cft/publish/servertemplate"
)

const (
	Name        = "gcs-publish"
	ArtifactDir = "/tmp/gcs-publish"
)

type GcsPublishService struct{}

func (s *GcsPublishService) Publish(ctx context.Context, log *log.Logger, req *api.PublishRequest) (*api.PublishResponse, error) {
	out := &api.PublishResponse{
		Status: api.PublishResponse_STATUS_SUCCESS,
	}

	gps, err := service.NewGcsPublishService(ctx, req)
	if err != nil {
		log.Printf("failed to create new gcs publish service: %s", err)
		out.Status = api.PublishResponse_STATUS_INVALID_REQUEST
		out.Message = fmt.Sprintf("failed to create new gcs publish service: %s", err.Error())
		return out, fmt.Errorf("failed to create new gcs publish service: %s", err)
	}

	if err := gps.UploadToGS(ctx); err != nil {
		log.Printf("upload to gs failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload to gs: %s", err.Error())
		return out, fmt.Errorf("failed upload to gs: %s", err)
	}

	if gps.EnableXTSArchiver {
		if err := gps.ArchiveXTSResults(ctx); err != nil {
			log.Printf("XTS archiver failed: %s", err)
			out.Status = api.PublishResponse_STATUS_FAILURE
			out.Message = fmt.Sprintf("failed to archive XTS results: %s", err.Error())
			return out, fmt.Errorf("failed to archive XTS results: %s", err)
		}
	}

	return out, nil
}

func main() {
	service := &GcsPublishService{}
	servertemplate.Server(Name, ArtifactDir, service)
}
