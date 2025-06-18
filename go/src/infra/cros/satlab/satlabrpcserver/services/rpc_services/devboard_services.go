// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package rpc_services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	longrunning "go.chromium.org/chromiumos/config/go/longrunning"
	api "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cmd/shivas/utils"
	"go.chromium.org/infra/cros/recovery/docker"
	"go.chromium.org/infra/cros/satlab/common/services/ufs"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/servo/testing"
	ufsApi "go.chromium.org/infra/unifiedfleet/api/v1/rpc"
	ufsUtil "go.chromium.org/infra/unifiedfleet/app/util"
)

const (
	//TODO(b/401097884): Update to non-deprecated registry.
	DEVBOARD_REGISTRY         = "gcr.io/satlab-images/"
	DEFAULT_DEVBOARD_IMAGE    = DEVBOARD_REGISTRY + "gsc_dev_board:release"
	DEFAULT_DEVBOARD_PORT     = "39999"
	DEVBOARD_LOG_FILE         = "/var/log/devboardsvc_exec.log"
	PULL_CONTAINER            = "compose"
	START_TIMEOUT             = 20 * time.Second
	DEVBOARD_CONTAINER_SUFFIX = "-gscdevboardsvc"
)

var (
	re_SERVER_STARTED = regexp.MustCompile(`: Server started`)
	re_SERVER_EXITED  = regexp.MustCompile(`Exit code:`)

	re_SERVER_PORT = regexp.MustCompile(`: Server locked:.*Port:([0-9]+)`)

	re_IMAGE_LS = regexp.MustCompile(`image::(.*?)::end`)

	devboardServicePools map[string]*api.StartDevboardServiceRequest

	devboardServiceTailCmd  = []string{"tail", "-f", "/dev/null"}
	devboardServiceStartCmd = []string{"bash", "start_devboardsvc.sh"}
)

func init() {
	devboardServicePools = make(map[string]*api.StartDevboardServiceRequest)
	devboardServicePools[""] = &api.StartDevboardServiceRequest{
		TestbedType:    "",
		GscSerial:      "",
		DebuggerSerial: "",
		ServicePort:    DEFAULT_DEVBOARD_PORT,
		ContainerImage: DEFAULT_DEVBOARD_IMAGE,
		Debug:          true,
	}
	//TODO(b/401092644): Remove once devboards become 1st class DUTs.
	devboardServicePools["gscdevboard-dt_shield"] = &api.StartDevboardServiceRequest{
		TestbedType:    "dauntless_andreishield",
		GscSerial:      "1482903f-4c2ac261",
		DebuggerSerial: "205936743452",
		ServicePort:    DEFAULT_DEVBOARD_PORT,
		ContainerImage: DEFAULT_DEVBOARD_IMAGE,
		Debug:          true,
	}
	devboardServicePools["gscdevboard-dt_shield-gscqual"] = &api.StartDevboardServiceRequest{
		TestbedType:    "dauntless_andreishield",
		GscSerial:      "1482104e-4c2ac261",
		DebuggerSerial: "2059356F3236",
		ServicePort:    DEFAULT_DEVBOARD_PORT,
		ContainerImage: DEFAULT_DEVBOARD_IMAGE,
		Debug:          true,
	}
	devboardServicePools["gscdevboard-h1_shield"] = &api.StartDevboardServiceRequest{
		TestbedType:    "haven_shield",
		GscSerial:      "0701A002-91AC3132",
		DebuggerSerial: "2055355D3236",
		ServicePort:    DEFAULT_DEVBOARD_PORT,
		ContainerImage: DEFAULT_DEVBOARD_IMAGE,
		Debug:          true,
	}
	devboardServicePools["gscdevboard-ot_shield"] = &api.StartDevboardServiceRequest{
		TestbedType:    "nt11_shield",
		GscSerial:      "02008115-00052092",
		DebuggerSerial: "205637655853",
		ServicePort:    DEFAULT_DEVBOARD_PORT,
		ContainerImage: DEFAULT_DEVBOARD_IMAGE,
		Debug:          true,
	}
}

// StartDevboardService start Docker servod container.
func (s *SatlabRpcServiceServer) StartDevboardService(ctx context.Context, in *api.StartDevboardServiceRequest) (*longrunning.Operation, error) {

	pool, err := getDutPool(ctx, in.GetContainerName(), s.dev)
	if err != nil {
		logging.Infof(ctx, "StartDevboardService: could not get pool: %w", err)
	}

	svc := devboardService{}

	svc.configDevboardServiceFromRequest(ctx, in, pool)

	if err := svc.StartContainer(ctx); err != nil {
		logging.Infof(ctx, "start devboard service fail:  %s\n", err)
		return nil, err
	}

	startRes := &api.StartDevboardServiceResponse{}
	startResAnypb, err := anypb.New(startRes)

	if err != nil {
		return nil, err
	}

	return &longrunning.Operation{
		Done: true,
		Result: &longrunning.Operation_Response{
			Response: startResAnypb,
		},
	}, nil
}

type devboardService struct {
	containerName  string
	containerImage string
	debug          bool
	testbedType    string
	gscSerial      string
	debuggerSerial string
	port           string
	dockerClient   docker.Client
}

func (s *devboardService) String() string {
	return fmt.Sprintf("%s::%s(%s)=>testbed:%s,debugger:%s,gsc:%s,debug:%v", s.containerName,
		s.port, s.containerImage, s.testbedType, s.debuggerSerial, s.gscSerial, s.debug)
}

func getDutPool(ctx context.Context, containerName string, dev bool) (string, error) {
	p := "devboardService.getDutPool"
	ufsCtx := utils.SetupContext(context.Background(), site.GetNamespace(""))

	if containerName == "" {
		return "", fmt.Errorf("%s: empty ContainerName", p)
	}

	dutName := strings.TrimSuffix(containerName, DEVBOARD_CONTAINER_SUFFIX)
	if dutName == containerName {
		return "", fmt.Errorf("%s: container name does not end with %q: %s", p, DEVBOARD_CONTAINER_SUFFIX, containerName)
	}

	ufsClient, err := ufs.NewUFSClientWithDefaultOptions(ufsCtx, site.GetUFSService(dev))
	if err != nil {
		return "", fmt.Errorf("%s: error connecting to UFS: %w", p, err)
	}

	dut, err := ufsClient.GetMachineLSE(ufsCtx, &ufsApi.GetMachineLSERequest{
		Name: ufsUtil.AddPrefix(ufsUtil.MachineLSECollection, dutName),
	})
	if err != nil {
		return "", fmt.Errorf("%s: error fetching DUT %s from UFS: %w", p, dutName, err)
	}

	pools := dut.GetChromeosMachineLse().GetDeviceLse().GetDut().GetPools()
	if len(pools) == 0 {
		return "", nil
	}

	logging.Infof(ctx, "%s: Picking 1st pool %s from %d pools", p, pools[0], len(pools))
	return pools[0], nil
}

// configDevboardServiceFromRequest sets service params based on request and pool.
func (s *devboardService) configDevboardServiceFromRequest(ctx context.Context, in *api.StartDevboardServiceRequest, pool string) {

	p := "devboardService.configDevboardServiceFromRequest"

	s.containerName = in.GetContainerName()
	s.containerImage = in.GetContainerImage()
	s.testbedType = in.GetTestbedType()
	s.gscSerial = in.GetGscSerial()
	s.debuggerSerial = in.GetDebuggerSerial()
	s.port = in.GetServicePort()
	s.debug = in.GetDebug()

	var defaultReq *api.StartDevboardServiceRequest

	if pool == "" {
		logging.Infof(ctx, "%s: use default service params, pool is blank", p)
		defaultReq = devboardServicePools[""]
	} else if req, ok := devboardServicePools[pool]; !ok {
		logging.Infof(ctx, "%s: use default service params, params for pool %q not found", p, pool)
		defaultReq = devboardServicePools[""]
	} else {
		logging.Infof(ctx, "%s: use service params for pool %q", p, pool)
		defaultReq = req
	}

	if s.containerName == "" {
		s.containerName = defaultReq.GetContainerName()
		logging.Infof(ctx, "%s: Using default containerName", p)
	}
	if s.containerImage == "" {
		s.containerImage = defaultReq.GetContainerImage()
		logging.Infof(ctx, "%s: Using default containerImage", p)
	}
	if s.testbedType == "" {
		s.testbedType = defaultReq.GetTestbedType()
		logging.Infof(ctx, "%s: Using default testbedType", p)
	}
	if s.gscSerial == "" {
		s.gscSerial = defaultReq.GetGscSerial()
		logging.Infof(ctx, "%s: Using default gscSerial", p)
	}
	if s.debuggerSerial == "" {
		s.debuggerSerial = defaultReq.GetDebuggerSerial()
		logging.Infof(ctx, "%s: Using default debuggerSerial", p)
	}
	if s.port == "" {
		s.port = defaultReq.GetServicePort()
		logging.Infof(ctx, "%s: Using default port", p)
	}

	if !s.debug {
		if defaultReq.GetDebug() {
			s.debug = true
			logging.Infof(ctx, "%s: Using default debug", p)
		}
	}

	logging.Infof(ctx, "%s: Configured Devboard Service: %s", p, s.String())
}

func (s *devboardService) initDockerClient(ctx context.Context) error {
	if s.dockerClient != nil {
		return nil
	}
	c, err := docker.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("devboardService.initDockerClient: Fail to create docker client: %w", err)
	}
	s.dockerClient = c
	return nil
}

// containerCmd returns the entry point command of the container.
func (s *devboardService) containerCmd() []string {
	if s.debug {
		return devboardServiceTailCmd
	}
	return devboardServiceStartCmd
}

func (s *devboardService) StartContainer(ctx context.Context) error {

	p := "devboardService.StartContainer"
	if err := s.initDockerClient(ctx); err != nil {
		return fmt.Errorf("%s: Failed to init docker client: %w", p, err)
	}

	s.removeContainer(ctx)

	if hasImage, err := s.hasServiceImage(ctx); err != nil {
		logging.Warningf(ctx, "%s: Unable to check if image exists: %w", p, err)
	} else if !hasImage {
		if err := s.pullServiceImage(ctx); err != nil {
			logging.Warningf(ctx, "%s: Unable to pull service image: %w", p, err)
		}
	} else {
		logging.Infof(ctx, "%s: Image exists: %s", p, s.containerImage)
	}

	containerArgs := &docker.ContainerArgs{
		Detached:   true,
		Network:    "default_satlab",
		Privileged: true,
		ImageName:  s.containerImage,
		EnvVar:     s.generateEnvVars(),
		Exec:       s.containerCmd(),
		Volumes:    s.generateVols(),
	}

	resp, err := s.dockerClient.StartOnly(ctx, s.containerName, containerArgs, 20*time.Second)
	if err != nil {
		return err
	}
	if resp.ExitCode != 0 {
		return fmt.Errorf("%s: Exit code %d, Stdout %s, Stderr %s", p, resp.ExitCode, resp.Stdout, resp.Stderr)
	}
	if s.debug {
		eReq := docker.ExecRequest{
			Cmd:     devboardServiceStartCmd,
			Detach:  true,
			Timeout: 20 * time.Second,
		}
		if _, err := s.dockerClient.Exec(ctx, s.containerName, &eReq); err != nil {
			logging.Errorf(ctx, "%s: Exec %s error: %w", p, devboardServiceStartCmd, err)
			return err
		}
	}
	if err := s.waitUntilStarted(ctx); err != nil {
		return err
	}
	return nil
}

func (s *devboardService) removeContainer(ctx context.Context) {

	if err := s.dockerClient.Remove(ctx, s.containerName, true); err != nil {
		logging.Warningf(ctx, "devboardService.removeContainer: Failed to remove `%s`: %w\n", s.containerName, err)
	}
}

func (s *devboardService) imageLs(ctx context.Context) ([]string, error) {
	resp, err := s.execCommand(ctx, PULL_CONTAINER,
		"docker", "image", "ls", "--format=image::{{.Repository}}:{{.Tag}}::end")
	if err != nil {
		return nil, err
	}
	matches := re_IMAGE_LS.FindAllStringSubmatch(resp.Stdout, -1)
	var result []string
	for _, v := range matches {
		result = append(result, v[1])
	}
	return result, nil
}

func (s *devboardService) hasServiceImage(ctx context.Context) (bool, error) {
	images, err := s.imageLs(ctx)
	if err != nil {
		return false, err
	}
	for _, v := range images {
		if v == s.containerImage {
			return true, nil
		}
	}
	return false, nil
}

func (s *devboardService) pullServiceImage(ctx context.Context) error {
	logging.Infof(ctx, "Pulling image %s from %s container", s.containerImage, PULL_CONTAINER)
	resp, err := s.execCommand(ctx, PULL_CONTAINER, "docker", "pull", s.containerImage)
	if err != nil {
		return fmt.Errorf("devboardService.pullServiceImage: %w", err)
	}
	if resp.ExitCode != 0 {
		return fmt.Errorf("devboardService.pullServiceImage: exit %d stdout: %s stderr: %s", resp.ExitCode, resp.Stdout, resp.Stderr)
	}
	return nil
}

// generateVols returns the array of mounting volumes for servod container.
func (s *devboardService) generateVols() []string {
	return []string{
		"/dev:/dev",
		fmt.Sprintf("%s_log:/var/log/", s.containerName),
	}
}

// generateEnvVars returns the array of env vars for servod container from StartServodRequest.
func (s *devboardService) generateEnvVars() []string {
	return []string{
		fmt.Sprintf("DEVBOARDSVC_PORT=%s", s.port),
		fmt.Sprintf("GSC_SERIAL=%s", s.gscSerial),
		fmt.Sprintf("DEBUGGER_SERIAL=%s", s.debuggerSerial),
		fmt.Sprintf("TESTBED=%s", s.testbedType),
	}
}

// timeoutFromContext returns timeout from context if it has a deadline, or returns
// a reasonable default for operations used in this file.
func (s *devboardService) timeoutFromContext(ctx context.Context) time.Duration {
	timeout := 2 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	return timeout
}

func (s *devboardService) waitUntilStarted(ctx context.Context) error {
	return testing.Poll(ctx, s.checkStarted,
		&testing.PollOptions{Timeout: START_TIMEOUT, Interval: 8 * time.Second})
}

func (s *devboardService) checkStarted(ctx context.Context) error {

	p := "devboardService.checkStarted"

	resp, err := s.execCommand(ctx, s.containerName, "head", "-n", "20", DEVBOARD_LOG_FILE)
	if err != nil {
		return fmt.Errorf("%s: read log file %w", p, err)
	}
	if re_SERVER_EXITED.MatchString(resp.Stdout) {
		logging.Debugf(ctx, "%s: service exited, log: %s", p, resp.Stdout)
		return testing.PollBreak(fmt.Errorf("%s: service exited: %s", p, resp.Stdout))
	}
	if !re_SERVER_STARTED.MatchString(resp.Stdout) {
		logging.Debugf(ctx, "%s: service not started, log: %s", p, resp.Stdout)
		return fmt.Errorf("%s: service not started: %s", p, resp.Stdout)
	}

	matches := re_SERVER_PORT.FindStringSubmatch(resp.Stdout)

	if matches == nil {
		return nil
	}

	if s.port != "" && s.port != "0" {
		if s.port != matches[1] {
			return testing.PollBreak(fmt.Errorf("%s: unexpected port got %s, want %s", p, matches[1], s.port))
		}
	} else {
		s.port = matches[1]
	}

	logging.Infof(ctx, "%s: service started at port %s", p, s.port)
	return nil
}

func (s *devboardService) execCommand(ctx context.Context, container string, cmd ...string) (*docker.ExecResponse, error) {
	eReq := &docker.ExecRequest{
		Timeout: s.timeoutFromContext(ctx),
		Cmd:     cmd,
	}
	res, err := s.dockerClient.Exec(ctx, container, eReq)
	if err != nil {
		return nil, fmt.Errorf("devboardService: execCommand in %s %v: %w", container, cmd, err)
	}
	return res, nil
}
