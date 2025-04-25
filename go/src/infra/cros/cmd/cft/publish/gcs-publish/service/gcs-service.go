// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/clock"

	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils"
	"go.chromium.org/infra/cros/cmd/cft/publish/commonutils/storage"
)

type GcsPublishService struct {
	Client                      *storage.GSClient
	LocalArtifactPath           string
	GcsPath                     string
	ServiceAccountCredsFilePath string
	RetryCount                  int
	XtsArchiverMetadata         *api.XtsArchiverMetadata
	EnableXTSArchiver           bool
}

func NewGcsPublishService(ctx context.Context, req *api.PublishRequest) (*GcsPublishService, error) {
	m, err := unpackMetadata(req)
	if err != nil {
		return nil, err
	}

	if err = commonutils.ValidateGCSPublishRequest(req, m); err != nil {
		return nil, err
	}

	serviceAccountPath := m.GetServiceAccountCredsFilePath().GetPath()
	gsClient, err := storage.NewGSClient(ctx, serviceAccountPath)
	if err != nil {
		log.Printf("error while creating new gs client: %s", err)
		return nil, fmt.Errorf("error while creating new gs client: %w", err)
	}

	retryCount := 0
	if req.GetRetryCount() > 0 {
		retryCount = int(req.GetRetryCount())
	}

	return &GcsPublishService{
		Client:                      gsClient,
		LocalArtifactPath:           req.GetArtifactDirPath().GetPath(),
		GcsPath:                     m.GetGcsPath().GetPath(),
		RetryCount:                  retryCount,
		ServiceAccountCredsFilePath: serviceAccountPath,
		XtsArchiverMetadata:         m.GetXtsArchiverMetadata(),
		EnableXTSArchiver:           m.GetEnableXtsArchiver(),
	}, nil
}

func (gs *GcsPublishService) UploadToGS(ctx context.Context) error {
	if err := gs.Client.Upload(ctx, gs.LocalArtifactPath, gs.GcsPath); err != nil {
		log.Printf("error in gcs upload: %s", err)
		return fmt.Errorf("error in gcs upload: %w", err)
	}
	return nil
}

// ArchiveXTSResults uploads all XTS results in the local artifact path to the specific GCS buckets in the metadata.
// TODO: b/379711782 - Propagate errors after process has been stabilized.
func (gs *GcsPublishService) ArchiveXTSResults(ctx context.Context) error {
	log.Printf("Archiving XTS results for metadata: %+v", gs.XtsArchiverMetadata)

	if err := validateXTSArchiverMetadata(gs.XtsArchiverMetadata); err != nil {
		log.Printf("XTS metadata validation failed: %s", err)
		return nil
	}

	// Find the root result dir where XTS results are located.
	commonPattern := filepath.Join(gs.LocalArtifactPath, "**", "cros-test", "results")
	var resultDir string
	if gs.XtsArchiverMetadata.GetAlRun() {
		// TODO: b/379711782 - Verify and propagate the AL flow logic.
		resultDir = filepath.Join(commonPattern, "tradefed")
	} else {
		resultDir = filepath.Join(commonPattern, "tauto")
	}

	dirs, err := filepath.Glob(resultDir)
	if err != nil {
		fmt.Printf("Error finding test result dir: %s: %s", resultDir, err)
		return nil
	}

	var infos []ArchiveInfo
	now := clock.Now(ctx).UTC()
	for _, dir := range dirs {
		archiveInfos, err := archiveInfo(dir, gs.XtsArchiverMetadata, now)
		if err != nil {
			fmt.Printf("Error in archive info for dir %s: %s", dir, err)
			continue
		}
		infos = append(infos, archiveInfos...)
	}

	if len(infos) == 0 {
		log.Printf("Skipped XTS Archive process: no XTS results found.")
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(len(infos))
	for _, info := range infos {
		go func(info ArchiveInfo) {
			defer wg.Done()
			log.Printf("Archiving XTS result: %s from local file: %s to GCS bucket: %s", info.Name, info.Source, info.Destination)
			if err := gs.Client.Upload(ctx, info.Source, info.Destination); err != nil {
				log.Printf("Error in gcs upload for XTS result: %s from local file: %s to GCS bucket: %s: %s", info.Name, info.Source, info.Destination, err)
			}
		}(info)
	}

	wg.Wait()
	return nil
}

// unpackMetadata unpacks the Any metadata field into PublishGcsMetadata
func unpackMetadata(req *api.PublishRequest) (*api.PublishGcsMetadata, error) {
	var m api.PublishGcsMetadata
	if err := req.Metadata.UnmarshalTo(&m); err != nil {
		return &m, fmt.Errorf("improperly formatted input proto metadata, %w", err)
	}
	return &m, nil
}
