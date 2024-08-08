// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package docker

// TODO(otabek): Move package to common lib when developing finished.

import (
	"bytes"
	"context"
	"encoding/json"
	base_error "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/log"
)

// TODO(otabek): Add basic unittest for each method.

const (
	// Connection to docker service can be set by socket or by open tcp connection.
	dockerSocketFilePath = "/var/run/docker.sock"

	// Enable more debug logs to triage issue.
	// Will be set to false after stabilize work with container.
	// TODO(otabek): Set false after testing in the prod.
	enablePrintAllContainers = false
)

// Proxy wraps a Servo object and forwards connections to the servod instance
// over SSH if needed.
type dockerClient struct {
	client *client.Client
}

// NewClient creates client to work with docker client.
func NewClient(ctx context.Context) (Client, error) {
	// Disabled by b/292794064.
	useSocketFile := false
	if client, err := createDockerClient(ctx, useSocketFile); err != nil {
		log.Debugf(ctx, "New docker client: failed to create docker client: %s", err)
		if client != nil {
			client.Close()
		}
		return nil, errors.Annotate(err, "new docker client").Err()
	} else {
		d := &dockerClient{
			client: client,
		}
		if enablePrintAllContainers {
			d.PrintAll(ctx)
		}
		return d, nil
	}
}

// Create Docker Client.
func createDockerClient(ctx context.Context, useSocketFile bool) (*client.Client, error) {
	if useSocketFile {
		// Use the dockerd socket if allowed.
		if _, err := os.Lstat(dockerSocketFilePath); err != nil {
			log.Debugf(ctx, "Docker file check fail: %v", err)
			if !base_error.Is(err, os.ErrNotExist) {
				log.Debugf(ctx, "Docker file is exist: %v", err)
				return nil, err
			}
		} else {
			log.Debugf(ctx, "Docker client connecting over docker.sock")
			return client.NewClientWithOpts(client.WithAPIVersionNegotiation())
		}
	}
	log.Debugf(ctx, "Docker client connecting over TCP")
	// For TLS create Docker Client from env variables.
	if path := os.Getenv("DOCKER_CERT_PATH"); path != "" {
		// Use the tcp connection, host IP is defined by DOCKER_HOST env variable.
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}
	// TODO(klimkowicz): remove this legacy Docker Client fallback when
	// Satlab with TLS dockerd is fully rolled out.
	dockerTCPPath := "tcp://192.168.231.1:2375"

	// Default HTTPClient inside the Docker Client object fails to
	// connects to docker daemon. Create the transport with DialContext and use
	// this while initializing new docker client object.
	timeout := time.Duration(1 * time.Second)
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: timeout,
		}).DialContext,
	}
	c := http.Client{Transport: transport}
	return client.NewClientWithOpts(client.WithHost(dockerTCPPath), client.WithHTTPClient(&c), client.WithAPIVersionNegotiation())
}

// Pull is pulling docker image.
//
// The pull is guaranty by docker that it will verify is image is latest version with required tag.
// If image is already latest the process will finished without any errors.
func (d *dockerClient) Pull(ctx context.Context, imageName string, timeout time.Duration) error {
	if imageName == "" {
		return errors.Reason("pull: name is not provided").Err()
	}
	// Timeouts less than 1 second are too short for docker pull to realistically finish.
	if timeout < time.Second {
		return errors.Reason("pull: timeout %v is less than 1 second", timeout).Err()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Only able to pull image from public registry.
	res, err := d.client.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		log.Debugf(ctx, "Run docker pull %q: err: %v", imageName, err)
		return errors.Annotate(err, "pull image").Err()
	}
	defer res.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(res)
	log.Debugf(ctx, "Run docker pull %q: stdout: %v", imageName, buf.String())
	return errors.Annotate(err, "pull image").Err()
}

// StartContainer pull and start container by request.
// More details https://docs.docker.com/engine/reference/run/
func (d *dockerClient) Start(ctx context.Context, containerName string, req *ContainerArgs, timeout time.Duration) (*StartResponse, error) {
	if containerName == "" {
		return nil, errors.Reason("start: containerName is not provided").Err()
	}
	// Timeouts less than 1 second are too short for docker run to realistically finish.
	if timeout < time.Second {
		return nil, errors.Reason("start: timeout %v is less than 1 second", timeout).Err()
	}
	err := d.Pull(ctx, req.ImageName, timeout)
	if err != nil {
		return nil, errors.Reason("start: fail to pull docker image %q", req.ImageName).Err()
	}
	var portsMapping []string
	for _, exposePort := range req.ExposePorts {
		portsMapping = append(portsMapping, fmt.Sprintf("[::]:%s", exposePort))
	}
	exposedPorts, portBindings, err := nat.ParsePortSpecs(portsMapping)
	if err != nil {
		return nil, errors.Annotate(err, "start: fail parse ports %v", portsMapping).Err()
	}
	config := &container.Config{
		Image:        req.ImageName,
		Env:          req.EnvVar,
		Cmd:          req.Exec,
		ExposedPorts: exposedPorts,
	}
	hostConfig := &container.HostConfig{
		Privileged:      true,
		Binds:           req.Volumes,
		NetworkMode:     container.NetworkMode(req.Network),
		AutoRemove:      true,
		PublishAllPorts: true,
		PortBindings:    portBindings,
	}
	c, err := d.client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, errors.Annotate(err, "Fail to pull Docker image: %q", req.ImageName).Err()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	outputDone := make(chan error, 1)

	go func() {
		// Demultiplexing the exec stdout into two buffers
		err = d.client.ContainerStart(ctx, c.ID, container.StartOptions{})
		outputDone <- err
	}()
	select {
	case err := <-outputDone:
		if err != nil {
			log.Debugf(ctx, "Fail to start docker container %q with cmd %+v using image %q\n", containerName, req.Exec, req.ImageName)
			return &StartResponse{ExitCode: 1}, err
		}
		break

	case <-ctx.Done():
		return &StartResponse{ExitCode: 124}, errors.Reason("Start container with timeout %s: exceeded timeout", timeout).Err()
	}
	return &StartResponse{}, err
}

// generateCommandArray takes the raw ContainerArgs we get and convert to an array of strings used to form the docker run command in Start
func generateCommandArray(containerName string, req *ContainerArgs) []string {
	args := []string{"run"}
	if req.Detached {
		args = append(args, "-d")
	}
	args = append(args, "--name", containerName)
	for _, v := range req.PublishPorts {
		args = append(args, "-p", v)
	}
	if len(req.ExposePorts) > 0 {
		for _, v := range req.ExposePorts {
			args = append(args, "--expose", v)
		}
		args = append(args, "-P")
	}
	for _, v := range req.Volumes {
		args = append(args, "-v", v)
	}
	for _, v := range req.EnvVar {
		args = append(args, "--env", v)
	}
	if req.Privileged {
		args = append(args, "--privileged")
	}
	// Always set to remove container when stop it.
	args = append(args, "--rm")
	if req.Network != "" {
		args = append(args, "--network", req.Network)
	}
	args = append(args, req.ImageName)
	if len(req.Exec) > 0 {
		args = append(args, req.Exec...)
	}

	return args
}

// Remove removes existed container.
func (d *dockerClient) Remove(ctx context.Context, containerName string, force bool) error {
	log.Debugf(ctx, "Removing container %q, using force:%v", containerName, force)
	o := container.RemoveOptions{Force: force}
	err := d.client.ContainerRemove(ctx, containerName, o)
	return errors.Annotate(err, "docker remove container  %s", containerName).Err()
}

// Run executes command on running container.
func (d *dockerClient) Exec(ctx context.Context, containerName string, req *ExecRequest) (*ExecResponse, error) {
	if len(req.Cmd) == 0 {
		return &ExecResponse{
			ExitCode: -1,
		}, errors.Reason("exec container: command is not provided").Err()
	}
	if up, err := d.IsUp(ctx, containerName); err != nil {
		return &ExecResponse{
			ExitCode: -1,
		}, errors.Annotate(err, "exec container").Err()
	} else if !up {
		return &ExecResponse{
			ExitCode: -1,
		}, errors.Reason("exec container: container is down").Err()
	}
	return d.execSDK(ctx, containerName, req)
}

// Run executes command on running container using docker SDK.
func (d *dockerClient) execSDK(ctx context.Context, containerName string, req *ExecRequest) (*ExecResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	c, err := json.Marshal(req.Cmd)
	if err != nil {
		log.Debugf(ctx, "Run docker exec using sdk %q: err: %v", containerName, err)
		return nil, err
	}
	log.Debugf(ctx, "Docker exec using sdk cmd %s on container %q\n", c, containerName)
	execConfig := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
		Cmd:          escapeSpecialChars(req.Cmd), // Escaping special characters in the command
	}
	cresp, err := d.client.ContainerExecCreate(ctx, containerName, execConfig)
	if err != nil {
		return nil, errors.Annotate(err, "exec container: Fail to create exec command.").Err()
	}
	execID := cresp.ID

	aresp, err := d.client.ContainerExecAttach(ctx, execID, types.ExecStartCheck{})
	if err != nil {
		log.Debugf(ctx, "Fail to attach to PID %q", execID)
		return &ExecResponse{ExitCode: -1}, errors.Reason("exec container: Fail to attach to exec process").Err()
	}
	defer aresp.Close()

	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error, 1)

	go func() {
		// Demultiplexing the exec stdout into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, aresp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			log.Debugf(ctx, "Fail to get output docker exec cmd %+v on container %q\n", req.Cmd, containerName)
			return &ExecResponse{ExitCode: 1}, err
		}
		break

	case <-ctx.Done():
		return &ExecResponse{ExitCode: 124}, errors.Reason("run with timeout %s: exceeded timeout", req.Timeout).Err()
	}

	// get the exit code
	iresp, err := d.client.ContainerExecInspect(ctx, execID)
	if err != nil {
		return &ExecResponse{ExitCode: 1}, errors.Annotate(err, "docker exec: fail to get exit code").Err()
	}
	res := &ExecResponse{ExitCode: iresp.ExitCode, Stdout: outBuf.String(), Stderr: errBuf.String()}
	log.Debugf(ctx, "Run docker exec using sdk %q: exitcode: %v", containerName, res.ExitCode)
	log.Debugf(ctx, "Run docker exec using sdk %q: stdout: %v", containerName, res.Stdout)
	log.Debugf(ctx, "Run docker exec using sdk %q: stderr: %v", containerName, res.Stderr)
	log.Debugf(ctx, "Run docker exec using sdk %q: err: %v", containerName, err)
	return res, nil
}

// PrintAllContainers prints all active containers.
func (d *dockerClient) PrintAll(ctx context.Context) error {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return errors.Annotate(err, "docker print all").Err()
	}
	for _, container := range containers {
		log.Debugf(ctx, "docker ps: %s %s\n", container.ID[:10], container.Image)
	}
	return nil
}

// ContainerIsUp checks is container is up.
func (d *dockerClient) IsUp(ctx context.Context, containerName string) (bool, error) {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return false, errors.Annotate(err, "container is up: fail to get a list of containers").Err()
	}
	for _, c := range containers {
		for _, n := range c.Names {
			// Remove first chat as names look like `/some_name` where user mostly use 'some_name'.
			if strings.TrimPrefix(n, "/") == containerName {
				return true, nil
			}
		}
	}
	return false, nil
}

// IPAddress reads assigned Ip address for container.
//
// Execution will use docker CLI:
// $ docker inspect '--format={{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' some_container
// 192.168.27.4
func (d *dockerClient) IPAddress(ctx context.Context, containerName string) (string, error) {
	f := filters.NewArgs()
	f.Add("name", containerName)
	f.Add("status", "running")
	// Get the list of containers based on the filter above.
	containers, err := d.client.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return "", errors.Annotate(err, "ip-address of %q: fail to get a list of containers", containerName).Err()
	}
	// Return error if the container is not found or is not in running state.
	if len(containers) != 1 {
		return "", errors.Reason("ip-address of %q: is not found or not running", containerName).Err()
	}
	if containers[0].NetworkSettings == nil {
		return "", errors.Reason("ip-address of %q: network settings not found", containerName).Err()
	}
	// We expect the container has only one network, so we take the first one.
	for _, sat_net := range containers[0].NetworkSettings.Networks {
		if sat_net != nil {
			return sat_net.IPAddress, nil
		}
	}
	log.Debugf(ctx, "Ip address: %#v", containers[0].NetworkSettings.Networks)
	return "", errors.Reason("ip-address of %q: address not found", containerName).Err()
}

// CopyTo copies a file from the host to the container.
func (d *dockerClient) CopyTo(ctx context.Context, containerName string, sourcePath, destinationPath string) error {
	f, err := os.Open(sourcePath)
	if err != nil {
		return errors.Annotate(err, "copy to %q: could not open local file", containerName).Err()
	}
	defer f.Close()
	return d.client.CopyToContainer(ctx, containerName, destinationPath, f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
}

// CopyFrom copies a file from container to the host.
func (d *dockerClient) CopyFrom(ctx context.Context, containerName string, sourcePath string, destinationPath string) error {
	outFile, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return errors.Annotate(err, "copy from %q: could not create local file", containerName).Err()
	}
	r, _, err := d.client.CopyFromContainer(ctx, containerName, sourcePath)
	if err != nil {
		return errors.Annotate(err, "copy from %q: could not copy remote file", containerName).Err()
	}
	_, err = io.Copy(outFile, r)
	if err != nil {
		return errors.Annotate(err, "copy from %q: could not write to local file", containerName).Err()
	}
	return outFile.Close()
}

// StartCommandString prints the command used in Start to spin up a container
// Uses the same underlying logic as Start to ensure we always print an accurate string
func StartCommandString(containerName string, req *ContainerArgs) string {
	return fmt.Sprintf("docker %v", strings.Trim(fmt.Sprint(generateCommandArray(containerName, req)), "[]"))
}

// Escapes the special characters($, \\, `, \"`) for a command input in the form of a string array
// Returns the command back as a string array after escaping special characters
func escapeSpecialChars(cmd []string) []string {
	var escapedCmd []string
	for _, command := range cmd {
		escapedCmd = append(escapedCmd, strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(
						command, "\\", "\\\\", -1),
					"$", "\\$", -1),
				"\"", "\\\"", -1),
			"`", "\\`", -1))
	}
	return escapedCmd
}
