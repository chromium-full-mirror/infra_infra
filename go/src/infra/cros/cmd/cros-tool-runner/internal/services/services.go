// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package services

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	build_api "go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/chromiumos/config/go/test/api"
	lab_api "go.chromium.org/chromiumos/config/go/test/lab/api"
	"go.chromium.org/luci/common/errors"

	"infra/cros/cmd/cros-tool-runner/internal/common"
	"infra/cros/cmd/cros-tool-runner/internal/docker"
)

const (
	// Dut service container name template for .
	crosDutContainerNameTemplate = "cros-dut-%s_%d_%d"

	// Provision service running port, docker info.
	crosProvisionContainerNameTemplate = "cros-provision-%s_%d_%d"

	// Cros Test container name template.
	crosTestContainerNameTemplate = "cros-test-%d_%d"

	// Pre-process container name template.
	preProcessContainerNameTemplate = "pre-process-%d_%d"

	// File names used to interact with cros-provision CLI.
	InputFileName  = "in.json"
	OutputFileName = "out.json"

	// Default dut address port
	DefaultDutAddressPort = "22"

	// Root directory for the cros-test artifacts inside docker.
	CrosTestRootDirInsideDocker = "/tmp/test"

	// Root directory for the cros-test-finder artifacts inside docker.
	CrosTestFinderRootDirInsideDocker = "/tmp/test"

	// Root directory for the pre-process artifacts inside docker.
	PreProcessRootDirInsideDocker = "/tmp/test"

	// Root directory for the post-process artifacts inside docker.
	PostProcessRootDirInsideDocker = "/tmp/test"

	// Directories inside root dir
	CrosTestDirInsideDocker        = "/tmp/test/cros-test"
	CrosTestResultsDirInsideDocker = "/tmp/test/results"
)

// CreateDutService pulls and starts cros-dut service.
func CreateDutService(ctx context.Context, image *build_api.ContainerImageInfo, dutName, networkName string, cacheServer *lab_api.CacheServer, dutSshInfo *lab_api.IpEndpoint, dir string, t string) (*docker.Docker, error) {
	p, err := createImagePath(image)
	if err != nil {
		log.Printf("create cros-dut service: %s", err)
	}
	r, err := createRegistryName(image)
	if err != nil {
		log.Printf("create cros-dut service: %s", err)
	}
	return startDutService(ctx, p, r, dutName, networkName, cacheServer, dutSshInfo, 0, dir, t)
}

// startDutService starts cros-dut service.
func startDutService(ctx context.Context, imagePath, registerName, dutName, networkName string, cacheServer *lab_api.CacheServer, dutSshInfo *lab_api.IpEndpoint, port int, dir string, tokenFile string) (*docker.Docker, error) {
	crosDutResultDirName := "/tmp/cros-dut"
	d := &docker.Docker{
		Name:               fmt.Sprintf(crosDutContainerNameTemplate, dutName, os.Getpid(), time.Now().Unix()),
		RequestedImageName: imagePath,
		Registry:           registerName,
		TokenFile:          tokenFile,
		// Fallback version used in case when main image fail to pull.
		// FallbackImageName: "gcr.io/chromeos-bot/cros-dut:fallback",
		// TODO: discuss whether we should have fallback.
		ExecCommand: []string{
			"cros-dut",
			"-dut_address", endPointToString(dutSshInfo),
			"-cache_address", endPointToString(cacheServer.GetAddress()),
			"-port", strconv.Itoa(port),
		},
		Volumes: []string{
			fmt.Sprintf("%s:%s", dir, crosDutResultDirName),
		},
		ServicePort: port,
		Detach:      true,
		Network:     networkName,
		LogFileDir:  dir,
	}
	d, err := startService(ctx, d, false, true, "cros-dut")

	if err != nil {
		log.Println("DUT Service Failed to start, exiting.")
		return d, err
	}
	log.Println("DUT Service started, polling for port.")

	// After starting the DUTService, find the port it binded to.
	var dsPort int

	err = common.Poll(ctx, func(ctx context.Context) error {
		var err error
		var filePath string
		filePath, err = common.FindFile("log.txt", dir)

		if err != nil {
			return errors.Annotate(err, "failed to find file cros-dut log file").Err()
		}
		d.Started = true
		dsPort, err = dutServerPort(filePath)
		if err != nil {
			return errors.Annotate(err, "failed to extract dut server port from %s", filePath).Err()
		}
		return nil
	}, &common.PollOptions{Timeout: 5 * time.Minute, Interval: time.Second})

	stdout := d.Stdoutbuf.String()
	stderr := d.Stderrbuf.String()
	log.Println("cros-dut stdout follows:")
	log.Println(stdout)
	log.Println("cros-dut stderr follows:")
	log.Println(stderr)

	if err != nil {
		log.Printf("DUT Service polling for port err: %s", err)
		return d, err
	}

	log.Println("DUT Service polling for port completed.")
	d.ServicePort = dsPort
	return d, nil
}

type DutServerInfo struct {
	Docker *docker.Docker
	Port   int32
}

// CreateDutServicesForHostNetwork pulls and starts cros-dut services in host network.
func CreateDutServicesForHostNetwork(ctx context.Context, image *build_api.ContainerImageInfo, duts []*lab_api.Dut, dir, t string) ([]*DutServerInfo, error) {
	p, err := createImagePath(image)
	if err != nil {
		return nil, errors.Annotate(err, "create dut services for host network: failed to create image path").Err()
	}
	r, err := createRegistryName(image)
	if err != nil {
		return nil, errors.Annotate(err, "create dut services for host network: failed to create registry name").Err()
	}

	var dockerContainers []*docker.Docker
	var dutServers []*DutServerInfo
	defer func() {
		for _, d := range dockerContainers {
			log.Printf("Removing container %s", d.Name)
			d.Remove(ctx)
		}
	}()

	for _, dut := range duts {
		dutID := dut.Id.GetValue()
		if dut.CacheServer == nil {
			return nil, errors.Annotate(err, "create dut services for host network: cache server must be specified in DUT %s", dutID).Err()
		}
		logDir := path.Join(dir, dutID)
		d, err := startDutService(ctx, p, r, dutID, "host", dut.CacheServer, dutEndPoint(dut), 0, logDir, t)
		if err != nil {
			return nil, errors.Annotate(err, "create dut services: failed to run cros-dut").Err()
		}
		dockerContainers = append(dockerContainers, d)
		dutServers = append(dutServers, &DutServerInfo{Docker: d, Port: int32(d.ServicePort)})

	}
	// There are no errors so don't clean up existing dockers.
	dockerContainers = nil
	return dutServers, nil
}

// RunProvisionCLI pulls and starts cros-provision as CLI.
func RunProvisionCLI(ctx context.Context, image *build_api.ContainerImageInfo, networkName string, req *api.CrosProvisionRequest, dir string, tokenFile string) (*docker.Docker, error) {
	// Create directory to provide input files and collect output files.
	// The directory will also has logs of the provisioning.
	if err := createProvisionInput(req, dir); err != nil {
		return nil, errors.Reason("run provision").Err()
	}
	// Path on the drone where service put the logs by default.
	dockerResultDirName := "/tmp/provisionservice"
	p, err := createImagePath(image)
	if err != nil {
		return nil, errors.Reason("failed to create image for run provision").Err()
	}
	r, err := createRegistryName(image)
	if err != nil {
		return nil, errors.Reason("failed to create registry path run provision").Err()
	}
	dutName := req.Dut.Id.GetValue()
	d := &docker.Docker{
		Name:               fmt.Sprintf(crosProvisionContainerNameTemplate, dutName, os.Getpid(), time.Now().Unix()),
		RequestedImageName: p,
		Registry:           r,
		TokenFile:          tokenFile,
		// Fallback version used in case when main image fail to pull.
		FallbackImageName: "gcr.io/chromeos-bot/cros-provision:fallback",
		ExecCommand: []string{
			"cros-provision",
			"cli",
			"-input", path.Join(dockerResultDirName, InputFileName),
			"-output", path.Join(dockerResultDirName, OutputFileName),
		},
		Volumes: []string{
			fmt.Sprintf("%s:%s", dir, dockerResultDirName),
		},
		Detach:     false,
		Network:    networkName,
		LogFileDir: dir,
	}
	return startService(ctx, d, true, true, "cros-provision")
}

// RunPostProcessCLI pulls and runs cros-test as CLI.
func RunPostProcessCLI(ctx context.Context, image *build_api.ContainerImageInfo, networkName, inputFileName, postProcessDir string, dutServicePort int32, tokenFile string) error {
	p, err := createImagePath(image)
	if err != nil {
		return errors.Annotate(err, "failed to create image for cros-test").Err()
	}
	r, err := createRegistryName(image)
	if err != nil {
		return errors.Annotate(err, "failed to create registry path for cros-test").Err()
	}
	// It is necessary to do sudo here because /tmp/test is owned by root inside docker
	// when docker mount /tmp/test. However, the user that is running cros-test is
	// chromeos-test inside docker. Hence, the user chromeos-test does not have write
	// permission in /tmp/test. Therefore, we need to change the owner of the directory.
	volumes := []string{fmt.Sprintf("%s:%s", postProcessDir, filepath.Join(PostProcessRootDirInsideDocker, "post-process"))}

	dutService := fmt.Sprintf("localhost:%v", dutServicePort)
	// TODO add the dutservice port to the CLI call.
	d := &docker.Docker{
		Name:               fmt.Sprintf(crosTestContainerNameTemplate, os.Getpid(), time.Now().Unix()),
		RequestedImageName: p,
		Registry:           r,
		TokenFile:          tokenFile,
		ExecCommand: []string{
			"post-process",
			"-dutendpoint",
			dutService,
		},
		Volumes: volumes,
		Detach:  false,
		Network: networkName,
	}
	_, err = startService(ctx, d, true, true, "cros-test")
	return err
}

// RunTestCLI pulls and runs cros-test as CLI.
func RunTestCLI(ctx context.Context, image *build_api.ContainerImageInfo, networkName, inputFileName, crosTestDir, resultDir string, tokenFile string) error {
	p, err := createImagePath(image)
	if err != nil {
		return errors.Annotate(err, "failed to create image for cros-test").Err()
	}
	r, err := createRegistryName(image)
	if err != nil {
		return errors.Annotate(err, "failed to create registry path for cros-test").Err()
	}
	// It is necessary to do sudo here because /tmp/test is owned by root inside docker
	// when docker mount /tmp/test. However, the user that is running cros-test is
	// chromeos-test inside docker. Hence, the user chromeos-test does not have write
	// permission in /tmp/test. Therefore, we need to change the owner of the directory.
	cmd := fmt.Sprintf("sudo --non-interactive chown -R chromeos-test:chromeos-test %s && cros-test", CrosTestRootDirInsideDocker)

	volumes := []string{
		fmt.Sprintf("%s:%s", crosTestDir, CrosTestDirInsideDocker),
		fmt.Sprintf("%s:%s", resultDir, CrosTestResultsDirInsideDocker),
	}
	// Mount authorization file for gsutil if exists. See b/239855913
	gsutilAuthFile := "/home/chromeos-test/.boto"
	if _, err := os.Stat(gsutilAuthFile); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:%s", gsutilAuthFile, gsutilAuthFile))
	}
	// Mount autotest results shared folder if exists. See b/239855163
	autotestResultsFolder := "/usr/local/autotest/results/shared"
	if _, err := os.Stat(autotestResultsFolder); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:%s", autotestResultsFolder, autotestResultsFolder))
	}
	// Mount drone credentials folder if exists. See b/291654727
	credsFolder := "/creds/service_accounts"
	if _, err := os.Stat(credsFolder); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:%s", credsFolder, credsFolder))
	}
	// TODO: b/319321943 -- Remove Satlab specific logic after cros-servod completion
	droneName := os.Getenv("DRONE_AGENT_HIVE")
	if strings.Contains(droneName, "satlab") {
		volumes = append(volumes, "/dev:/dev")
	}

	d := &docker.Docker{
		Name:               fmt.Sprintf(crosTestContainerNameTemplate, os.Getpid(), time.Now().Unix()),
		RequestedImageName: p,
		Registry:           r,
		TokenFile:          tokenFile,
		ExecCommand: []string{
			"bash",
			"-c",

			cmd,
		},
		Volumes:    volumes,
		Detach:     false,
		Network:    networkName,
		LogFileDir: crosTestDir,
	}
	_, err = startService(ctx, d, true, true, "cros-test")
	return err
}

// RunTestFinderCLI pulls and runs cros-test-finder as CLI.
func RunTestFinderCLI(ctx context.Context, image *build_api.ContainerImageInfo, networkName, crosTestFinderDir string, tokenFile string) error {
	p, err := createImagePath(image)
	if err != nil {
		return errors.Annotate(err, "failed to create image for cros-test").Err()
	}
	r, err := createRegistryName(image)
	if err != nil {
		return errors.Annotate(err, "failed to create reigstry path for cros-test-finder").Err()
	}
	// The files or directories used by cros-test-finder container is set up this way.
	// File or Directory inside the container   Source
	// ++++++++++++++++++++++++++++++++++++++   +++++++++++++++++++++++++++++++++++++++++++
	// /tmp/test/cros-test-finder               Mount /tmp/test/cros-test-finder during run
	// /tmp/test/cros-test-finder/request.json  Generated before execute cros-test-finder
	// /tmp/test/cros-test-finder/result.json   Generated by cros-test-finder
	// /usr/bin/cros-test-finder                Included in container image
	// /tmp/test/metadata                       Included in container image
	d := &docker.Docker{
		Name:               fmt.Sprintf(crosTestContainerNameTemplate, os.Getpid(), time.Now().Unix()),
		RequestedImageName: p,
		Registry:           r,
		TokenFile:          tokenFile,
		// Fallback version used in case when main image fail to pull.
		FallbackImageName: "gcr.io/chromeos-bot/cros-test-finder:fallback",
		ExecCommand: []string{
			"cros-test-finder",
		},
		Volumes: []string{
			fmt.Sprintf("%s:%s", crosTestFinderDir, filepath.Join(CrosTestFinderRootDirInsideDocker, "cros-test-finder")),
		},
		Detach:  false,
		Network: networkName,
	}
	_, err = startService(ctx, d, true, false, "test-finder")
	return err
}

func dutEndPoint(dut *lab_api.Dut) *lab_api.IpEndpoint {
	if dut == nil {
		return nil
	}
	chromeOS := dut.GetChromeos()
	if chromeOS == nil {
		return nil
	}
	return chromeOS.GetSsh()
}

func dutAddress(dut *lab_api.Dut) string {
	if dut == nil {
		return ""
	}
	chromeOS := dut.GetChromeos()
	if chromeOS == nil {
		return ""
	}
	endPoint := chromeOS.GetSsh()
	return endPointToString(endPoint)
}

func endPointToString(endPoint *lab_api.IpEndpoint) string {
	if endPoint == nil {
		return ""
	}
	if endPoint.GetPort() == 0 {
		return fmt.Sprintf("%s:%v", endPoint.GetAddress(), DefaultDutAddressPort)
	}
	return fmt.Sprintf("%s:%v", endPoint.GetAddress(), endPoint.GetPort())
}

// dutServerPort extracts dut server end point from dut server log file.
// TODO: b/225046577 -- Find a more robust way to get the DUT server port.
func dutServerPort(dutServerLogFileName string) (int, error) {
	file, err := os.Open(dutServerLogFileName)
	if err != nil {
		return 0, errors.Annotate(err, "failed to open cros-dut log file %s", dutServerLogFileName).Err()
	}
	defer file.Close()

	// Example of the line with dutservice port number.
	// "Starting dutservice on port 12300"
	const searchStr = "Started server on address"
	s := bufio.NewScanner(file)
	for s.Scan() {
		line := s.Text()
		// Example Input: "2022/03/15 23:38:47 Started server on address  [::]:39115".
		index := strings.Index(line, searchStr)
		if index < 0 {
			continue
		}
		// Find last ":".
		address := line[index+len(searchStr):]
		index = strings.LastIndex(address, ":")
		if index < 0 {
			return 0, errors.Annotate(err, "fail to get port from line %q in file %s", line, dutServerLogFileName).Err()
		}
		portStr := address[index+1:]
		return strconv.Atoi(portStr)
	}
	return 0, errors.Reason("failed to extract port from %s", dutServerLogFileName).Err()

}

func RunPreProcessCLI(ctx context.Context, image *build_api.ContainerImageInfo, networkName, preProcessDir string, tokenFile string) error {
	p, err := createImagePath(image)
	if err != nil {
		return errors.Annotate(err, "failed to create image for pre-process").Err()
	}
	r, err := createRegistryName(image)
	if err != nil {
		return errors.Annotate(err, "failed to create reigstry path for pre-process").Err()
	}

	// Mount the host creds.
	credsFile := "/creds/"
	volumes := []string{}
	volumes = append(volumes, fmt.Sprintf("%s:%s", credsFile, credsFile))

	// TODO: fix this being test-finder dir.
	volumes = append(volumes, fmt.Sprintf("%s:%s", preProcessDir, filepath.Join(PreProcessRootDirInsideDocker, "pre-process")))

	d := &docker.Docker{
		Name:               fmt.Sprintf(preProcessContainerNameTemplate, os.Getpid(), time.Now().Unix()),
		RequestedImageName: p,
		Registry:           r,
		TokenFile:          tokenFile,
		// Fallback version used in case when main image fail to pull.
		FallbackImageName: "",
		ExecCommand: []string{
			"pre-process",
		},
		Volumes: volumes,
		Detach:  false,
		Network: networkName,
	}
	_, err = startService(ctx, d, true, true, "pre-process")
	return err

}
