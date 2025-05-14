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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	conf "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	commonutils "go.chromium.org/infra/cros/cmd/provision/common-utils"
)

var reBoard = regexp.MustCompile(`CHROMEOS_RELEASE_BOARD=(.*)`)

const DRONE_SERVICE_ACCOUNT_PATH = "/creds/service_accounts/skylab-drone.json"
const DRONE_SERVICE_ACCOUNT_GSUTIL_OPT = "Credentials:gs_service_key_file=/creds/service_accounts/skylab-drone.json"

// AshChromeService implements ServiceInterface
type AshChromeService struct {
	// |connection| connects to the DUT.
	connection commonutils.ServiceAdapterInterface

	artifactPath *conf.StoragePath
	buildDir     string

	tmpDir string

	// DUTServer is the gRPC connection to the DUT.
	DUTServer api.DutServiceClient
	dut       *labapi.Dut

	CacheServer url.URL
}

// NewAshChromeService initializes an AshChromeService.
func NewAshChromeService(ctx context.Context, dutServer api.DutServiceClient,
	cacheServer url.URL, req *api.InstallRequest, dut *labapi.Dut) (*AshChromeService, api.InstallResponse_Status, error) {
	metadata := new(api.AshChromeProvisionInstallMetadata)
	if req.GetMetadata().MessageIs(metadata) {
		if err := req.GetMetadata().UnmarshalTo(metadata); err != nil {
			return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.Wrap(err, "unmarshalling metadata")
		}
	} else {
		return nil, api.InstallResponse_STATUS_INVALID_REQUEST, errors.Errorf("AshChromeProvisionInstallMetadata is required, got %s", req.String())
	}
	detailedRequest := metadata.AshChromeConfig
	dutAdapter := commonutils.NewServiceAdapter(dutServer, false /*noReboot*/)

	service := AshChromeService{
		connection:  dutAdapter,
		DUTServer:   dutServer,
		CacheServer: cacheServer,
		dut:         dut,
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

// RestartDut restarts the DUT using one of the available mechanisms.
func (service *AshChromeService) RestartDut(ctx context.Context) error {
	if service.connection != nil {
		log.Printf("[AshChrome Provisioning: Restart DUT] restarting DUT over SSH.")
		service.connection.Restart(ctx)
		return service.connection.ForceReconnectWithBackoff(ctx)
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
	var err error
	if _, e := os.Stat(DRONE_SERVICE_ACCOUNT_PATH); errors.Is(e, os.ErrNotExist) {
		if _, _, err := service.runWithTimeout(ctx, 300, true, "gcloud", "auth", "list", "--format", "value(account)"); err != nil {
			return err
		}
		_, _, err = service.runWithTimeout(ctx, 1200, true, "gsutil", "cp", gsPath, dest)
	} else {
		_, _, err = service.runWithTimeout(ctx, 1200, true, "gsutil", "-o", DRONE_SERVICE_ACCOUNT_GSUTIL_OPT,
			"cp", gsPath, dest)
	}

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

func (service *AshChromeService) LogChromeVersion(ctx context.Context) error {
	if err := service.runRemoteCommandAndLog(ctx, "/opt/google/chrome/chrome", "--version"); err != nil {
		return err
	}
	return nil
}

func (service *AshChromeService) MakeRootfsWritable(ctx context.Context) error {
	out, err := service.connection.RunCmd(ctx, "cat", []string{"/proc/mounts"})
	if err != nil {
		return fmt.Errorf("cat /proc/mounts failed: %v", err)
	}
	log.Printf("cat /proc/mounts: \n%s", out)
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		split := strings.Split(line, " ")
		if split[0] != "/dev/root" {
			continue
		}
		if len(split) < 4 {
			return fmt.Errorf("unexpected result: %v", split)
		}
		opts := strings.Split(split[3], ",")
		if opts[0] != "rw" && opts[0] != "ro" {
			return fmt.Errorf("unexpected result: %v", split)
		}
		if opts[0] == "rw" {
			return nil
		}
	}

	err = service.runRemoteCommandAndLog(ctx, "/usr/share/vboot/bin/make_dev_ssd.sh", "--partitions \"2 4\"", "--remove_rootfs_verification", "--force")
	if err != nil {
		return err
	}

	err = service.RestartDut(ctx)
	if err != nil {
		return err
	}

	err = service.runRemoteCommandAndLog(ctx, "mount", "-o", "remount,rw", "/")
	if err != nil {
		return err
	}

	return nil
}

func (service *AshChromeService) ChromiteDeployChrome(ctx context.Context, localChromeDir string) error {
	if _, _, err := service.runWithTimeout(ctx, 60, true, "chmod", "-R", "755", localChromeDir); err != nil {
		return err
	}

	chromiteDir := filepath.Join(localChromeDir, "third_party/chromite")

	board, err := service.GetDUTBoard(ctx)
	if err != nil {
		return err
	}

	if _, _, err := service.runWithTimeout(ctx, 1200, true,
		"python3",
		filepath.Join(chromiteDir, "bin/deploy_chrome"), "--noremove-rootfs-verification", "--force", "--nostrip",
		"--build-dir", filepath.Join(localChromeDir, service.buildDir),
		"--process-timeout", "180", "--device", fmt.Sprintf("%s:%d", service.dut.GetChromeos().GetSsh().GetAddress(), service.dut.GetChromeos().GetSsh().GetPort()), "--board", board, "--mount"); err != nil {
		return err
	}

	return nil
}

func (service *AshChromeService) GetDUTBoard(ctx context.Context) (string, error) {
	out, err := service.connection.RunCmd(ctx, "cat", []string{"/etc/lsb-release"})
	if err != nil {
		return "", fmt.Errorf("cat /etc/lsb-release failed: %v", err)
	}
	log.Printf("cat /etc/lsb-release: \n%s", out)
	match := reBoard.FindStringSubmatch(out)
	if match == nil {
		return "", fmt.Errorf("No match found for %s", reBoard.String())
	}
	return match[1], nil
}

func (service *AshChromeService) GetArtifactPath() *conf.StoragePath {
	return service.artifactPath
}

func (service *AshChromeService) GetTmpDir() string {
	return service.tmpDir
}

func (service *AshChromeService) runRemoteCommandAndLog(ctx context.Context, cmd string, args ...string) error {
	out, err := service.connection.RunCmd(ctx, cmd, args)
	if err != nil {
		return fmt.Errorf("%s %v failed: %v", cmd, args, err)
	}
	log.Printf("%s %v: %s", cmd, args, out)
	return nil
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
