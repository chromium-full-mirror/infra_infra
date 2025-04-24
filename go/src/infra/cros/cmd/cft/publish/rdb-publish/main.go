// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"

	"go.chromium.org/infra/cros/cmd/cft/publish/rdb-publish/service"
	"go.chromium.org/infra/cros/cmd/cft/publish/servertemplate"
)

const (
	Name        = "rdb-publish"
	ArtifactDir = "/tmp/rdb-publish"
)

type RdbPublishService struct{}

func (s *RdbPublishService) Publish(ctx context.Context, log *log.Logger, req *api.PublishRequest) (*api.PublishResponse, error) {
	out := &api.PublishResponse{
		Status: api.PublishResponse_STATUS_SUCCESS,
	}

	// Exports the EqC info to BQ if it's a 3D test run.
	if err := service.PublishEQC(ctx, req); err != nil {
		// Only logs the error message without exiting early.
		log.Printf("failed to publish the EqC info to BQ: %s", err)
	}

	gps, err := service.NewRdbPublishService(req)
	if err != nil {
		log.Printf("failed to create new rdb publish service: %s", err)
		out.Status = api.PublishResponse_STATUS_INVALID_REQUEST
		out.Message = fmt.Sprintf("failed to create new rdb publish service: %s", err.Error())
		return out, fmt.Errorf("failed to create new rdb publish service: %s", err)
	}

	if err := gps.UploadToRdb(context.Background()); err != nil {
		log.Printf("upload to rdb failed: %s", err)
		out.Status = api.PublishResponse_STATUS_FAILURE
		out.Message = fmt.Sprintf("failed upload to rdb: %s", err.Error())
		return out, fmt.Errorf("failed upload to rdb: %s", err)
	}

	log.Println("Finished Successfuly!")

	return out, nil
}

func main() {
	service := &RdbPublishService{}
	servertemplate.Server(Name, ArtifactDir, service)
}
