// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ashchromeservice

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/pkg/errors"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"

	common_utils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

// AshChromeService implements ServiceInterface
type AshChromeService struct {
	// |connection| connects to the DUT.
	connection common_utils.ServiceAdapterInterface

	artifactPath *conf.StoragePath
	buildDir     string

	// DUTServer is the gRPC connection to the DUT.
	DUTServer api.DutServiceClient

	CacheServer url.URL
}

// NewAshChromeService initializes an AshChromeService.
func NewAshChromeService(ctx context.Context, dutServer api.DutServiceClient,
	cacheServer url.URL, req *api.InstallRequest) (*AshChromeService, api.InstallResponse_Status, error) {
	metadata := new(api.AshChromeProvisionInstallMetadata)
	if req.GetMetadata().MessageIs(metadata) {
		if err := req.GetMetadata().UnmarshalTo(metadata); err != nil {
			return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.Wrap(err, "unmarshalling metadata")
		}
	} else {
		return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.Errorf("AshChromeProvisionInstallMetadata is required, got %s", req.String())
	}
	detailedRequest := metadata.AshChromeConfig
	dutAdapter := common_utils.NewServiceAdapter(dutServer, false /*noReboot*/)

	service := AshChromeService{
		connection:  dutAdapter,
		DUTServer:   dutServer,
		CacheServer: cacheServer,
	}

	service.artifactPath = detailedRequest.GetChromeBuilderArtifactPath()
	service.buildDir = detailedRequest.GetBuildOutputDir()

	service.PrintRequestInfo()

	if service.buildDir == "" {
		return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.New("no build_output_dir specified")
	}
	if service.artifactPath.HostType != conf.StoragePath_LOCAL && service.artifactPath.HostType != conf.StoragePath_GS {
		return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.Errorf("want LOCAL or GS host_type, but got %s", service.artifactPath.HostType)
	}
	if service.artifactPath.Path == "" {
		return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.New("no chrome_builder_artifact_path.path specified")
	}

	return &service, api.InstallResponse_STATUS_SUCCESS, nil
}

// PrintRequestInfo logs details of the provisioning operation.
func (service *AshChromeService) PrintRequestInfo() {
	informationString := fmt.Sprintf("provisioning %v (%s)", service.artifactPath, service.buildDir)

	log.Println("[AshChrome Provisioning]", informationString)
}

// WaitForReconnect attempts to ssh to the DUT until it connects
func (service *AshChromeService) WaitForReconnect(ctx context.Context) error {
	// Attempts to run `true` on the DUT over SSH until |reconnectRetries| attempts.
	const reconnectRetries = 10
	const reconnectAttemptWait = 10 * time.Second
	const reconnectFailPause = 10 * time.Second
	var connectErr error
	for range reconnectRetries {
		reconnectCtx, reconnCancel := context.WithTimeout(ctx, reconnectAttemptWait)
		defer reconnCancel()
		_, connectErr = service.connection.RunCmd(reconnectCtx, "true", nil)
		if connectErr == nil {
			return nil
		}
		time.Sleep(reconnectFailPause)
	}
	log.Printf("Timed out waiting for DUT to connect: %v", connectErr)
	return connectErr
}

// RestartDut restarts the DUT using one of the available mechanisms.
func (service *AshChromeService) RestartDut(ctx context.Context) error {
	if service.connection != nil {
		log.Printf("[AshChrome Provisioning: Restart DUT] restarting DUT over SSH.")
		service.connection.Restart(ctx)
		return service.WaitForReconnect(ctx)
	}
	return errors.New("failed to restart: no SSH connection to the DUT")
}

// DeleteArchiveDirectories deletes files on the servo host or DUT.
func (service *AshChromeService) DeleteArchiveDirectories() error {
	/*
		var cleanedDevice common_utils.ServiceAdapterInterface
		if service.useServo {
			// If servo is used, the files will be located on the ServoHost.
			cleanedDevice = service.servoConnection
		} else {
			// If SSH is used, the files will be located on the DUT in /tmp/
			// It's not strictly necessary to delete them before reboot.
			cleanedDevice = service.connection
		}

		var allErrors []string
		for _, imgMetadata := range service.imagesMetadata {
			err := cleanedDevice.DeleteDirectory(context.Background(), imgMetadata.ArchiveDir)
			if err != nil {
				allErrors = append(allErrors, fmt.Sprintf("failed to delete %v: %v", imgMetadata.ArchiveDir, err))
			}
		}

		if len(allErrors) > 0 {
			return errors.New(strings.Join(allErrors, ". "))
		}
	*/

	return nil
}
