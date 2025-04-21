// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ashchromeservice

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
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

	tmpDir string

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

	tmpDir, err := ioutil.TempDir("/tmp", "ash-chrome-provision-*")
	if err != nil {
		return nil, api.InstallResponse_STATUS_PRE_PROVISION_SETUP_FAILED, err
	}
	service.tmpDir = tmpDir

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

func (service *AshChromeService) runWithTimeout(ctx context.Context, timeoutSecs int, printLog bool, command ...string) (stdout string, stderr string, err error) {
	log.Printf("Run command: %v", command)
	ctx, cancel := context.WithTimeout(ctx, 1200*time.Second)
	var se, so bytes.Buffer
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stdout = &so
	cmd.Stderr = &se
	defer func() {
		cancel()
		stdout = so.String()
		stderr = se.String()
		if printLog {
			if stdout != "" {
				log.Printf("stdout: %s", stdout)
			}
			if stderr != "" {
				log.Printf("stderr: %s", stderr)
			}
		}
	}()

	err = cmd.Run()
	if err != nil {
		log.Printf("error found with cmd: %s", err)
	}
	return
}

// Download Chrome Artifacts
func (service *AshChromeService) DownloadChromeArtifactsFromGS(ctx context.Context, gsPath string, dest string) error {
	log.Printf("Downloading Chrome build artifacts from %s.", gsPath)
	_, _, err := service.runWithTimeout(ctx, 1200, true, "gsutil", "-o", "Credentials:gs_service_key_file=/creds/service_accounts/skylab-drone.json",
		"cp", gsPath, dest)

	if err != nil {
		log.Printf("Download Chrome artifact failed: %v", err)
		return err
	}
	return nil
}

// Extract Chrome Artifacts tarball to directory
func (service *AshChromeService) ExtractChromeArtifacts(ctx context.Context, tarball string, out string) error {
	log.Printf("Extracting %v to %v", tarball, out)
	_, _, err := service.runWithTimeout(ctx, 1200, true, "tar", "-I", "zstd", "-xf", tarball, "-C", out)

	if err != nil {
		log.Printf("Error extracting tarball: %v", err)
		return err
	}
	return nil
}

func (service *AshChromeService) GetArtifactPath() *conf.StoragePath {
	return service.artifactPath
}

func (service *AshChromeService) GetTmpDir() string {
	return service.tmpDir
}

// DeleteArchiveDirectories deletes files on the servo host or DUT.
func (service *AshChromeService) DeleteArchiveDirectories() error {
	if service.tmpDir != "" {
		os.RemoveAll(service.tmpDir)
	}
	/*
		// To be adapted for ash-chrome-provision.
		var allErrors []string
		for _, imgMetadata := range service.imagesMetadata {
			err := connection.DeleteDirectory(context.Background(), imgMetadata.ArchiveDir)
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
