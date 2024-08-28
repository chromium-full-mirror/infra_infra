// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package docker provide abstaraction to pull/start/stop/remove docker image.
// Package uses docker-cli from running host.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/tsmon/field"
	"go.chromium.org/luci/common/tsmon/metric"
	"go.chromium.org/luci/common/tsmon/types"

	"infra/cros/cmd/cros-tool-runner/internal/common"
	"infra/cros/internal/env"
)

const (
	// Default fallback docket tag.
	DefaultImageTag  = "stable"
	basePodmanConfig = "/run/containers/0/auth.json"
	baseDockerConfig = "~/.docker/config.json"
	dockerRegistry   = "us-docker.pkg.dev"
	lockFile         = "/var/lock/go-lock.lock"
	RETRYNUM         = 2
)

// Docker holds data to perform the docker manipulations.
type Docker struct {
	// Requested docker image, if not exist then use FallbackImageName.
	RequestedImageName string
	// Registry to auth for docker interactions.
	Registry string
	// token to token
	TokenFile string
	// Fall back docker image name. Used if RequestedImageName is empty or image not found.
	FallbackImageName string
	// ServicePort tells which port need to bing bind from docker to the host.
	// Bind is always to the first free port.
	ServicePort int
	// Run container in detach mode.
	Detach bool
	// Name to be assigned to the container - should be unique.
	Name string
	// ExecCommand tells if we need run special command when we start container.
	ExecCommand []string
	// Attach volumes to the docker image.
	Volumes []string
	// PortMappings is a list of "host port:docker port" or "docker port" to publish.
	PortMappings []string

	// Successful pulled docker image.
	pulledImage string
	// Started container ID.
	containerID string
	// Network used for running container.
	Network string

	// LogFileDir used for the logfile for the service in the container.
	LogFileDir string

	Stdoutbuf    *bytes.Buffer
	Stderrbuf    *bytes.Buffer
	PullExitCode int
	Started      bool
}

// MatchingHostPort returns the port which the given docker port maps to.
func (d *Docker) MatchingHostPort(ctx context.Context, dockerPort string) (string, error) {
	cmd := exec.Command("docker", "port", d.Name, dockerPort)
	stdout, stderr, err := common.RunWithTimeout(ctx, cmd, 2*time.Minute, true)
	if err != nil {
		log.Printf(fmt.Sprintf("Could not find port %v for %v: %v", dockerPort, d.Name, err), stdout, stderr)
		return "", errors.Annotate(err, "find mapped port").Err()
	}

	// Expected stdout is of the form "0.0.0.0:12345\n".
	port := strings.TrimPrefix(stdout, "0.0.0.0:")
	port = strings.TrimSuffix(port, "\n")
	return port, nil
}

// Auth with docker registry so that pulling and stuff works.
func (d *Docker) Auth(ctx context.Context) (err error) {
	if err := configureDockerToGcloudAuth(ctx); err != nil {
		return err
	}
	return nil
}

// configureDockerToGcloudAuth configures to gcloud and hence automates
// the process of configuring Docker to authenticate with Artifact Registry
func configureDockerToGcloudAuth(ctx context.Context) error {
	cmd := exec.Command("gcloud", "auth", "configure-docker", dockerRegistry)
	logStr := fmt.Sprintf("gcloud auth configure-docker %s", dockerRegistry)
	stdout, stderr, err := common.RunWithTimeoutSpecialLog(ctx, cmd, 1*time.Minute, true, logStr)
	common.PrintToLog("configure docker to gcloud auth", stdout, stderr)
	if err != nil {
		return errors.Annotate(err, "failed configuring docker to gcloud auth").Err()
	}
	log.Printf("configured docker to gcloud auth successfully!")
	return nil
}

// Remove removes the containers with matched name.
func (d *Docker) Remove(ctx context.Context) error {
	if d == nil {
		return nil
	}
	// Use force to avoid any un-related issues.
	cmd := exec.Command("docker", "rm", "--force", d.Name)
	stdout, stderr, err := common.RunWithTimeout(ctx, cmd, time.Minute, true)
	common.PrintToLog(fmt.Sprintf("Remove container %q", d.Name), stdout, stderr)
	if err != nil {
		log.Printf("remove container %q failed with error: %s", d.Name, err)
		return errors.Annotate(err, "remove container %q", d.Name).Err()
	}
	log.Printf("remove container %q: done.", d.Name)
	return nil
}

// Run docker image.
// The step will create container and start server inside or execution CLI.
func (d *Docker) Run(ctx context.Context, block bool, netbind bool, service string) error {
	out, err := d.runDockerImage(ctx, block, netbind, service)
	if err != nil {
		return errors.Annotate(err, "run docker %q", d.Name).Err()
	}
	if d.Detach {
		d.containerID = strings.TrimSuffix(out, "\n")
		log.Printf("Run docker %q: container Id: %q.", d.Name, d.containerID)
	}

	// Not detached, no err, then the container is started. Detched started must be determined by the caller.
	if !d.Detach {
		d.Started = true
	}
	return nil
}

func pullImage(ctx context.Context, image string, service string) (error, int) {
	startTime := time.Now()
	cmd := exec.Command("docker", "pull", image)
	stdout, stderr, err := common.RunWithTimeout(ctx, cmd, 3*time.Minute, true)
	common.PrintToLog(fmt.Sprintf("Pull image %q", image), stdout, stderr)
	if err != nil {
		log.Printf("pull image %q: failed with error: %s", image, err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()

			return errors.Annotate(err, "Pull image").Err(), exitCode
		}
		return errors.Annotate(err, "Pull image").Err(), 0
	}
	log.Printf("pull image %q: successful pulled.", image)
	logPullTimeProd(ctx, startTime, service)
	return nil, 0
}

func pullWithRetry(ctx context.Context, image string, service string) (error, int) {
	var exitCode int
	var err error
	for range [RETRYNUM]int{} {
		err, exitCode = pullImage(ctx, image, service)
		// do not retry if no err, or a non-critical failure.
		if err == nil || !common.IsCriticalPullCrash(exitCode) {
			break
		}
		log.Printf("Failed to pull with critical failure, retry .")
	}
	return err, exitCode

}
func (d *Docker) runDockerImage(ctx context.Context, block bool, netbind bool, service string) (string, error) {
	d.Started = false
	err, exitCode := pullWithRetry(ctx, d.RequestedImageName, service)
	d.PullExitCode = exitCode
	if err != nil {
		if common.IsCriticalPullCrash(exitCode) {
			return "", errors.Annotate(err, "pull docker image failed with a critical ExitCode %d", exitCode).Err()
		}
		log.Printf("Failed to pull image with non-critical failure, so will try to run anyways.")
	}

	args := []string{"run"}
	args = append(args, envvars()...)
	if d.Detach {
		// PRE-pend the log-level log for detached containers.
		args = append([]string{"--log-level=debug"}, args...)
		args = append(args, "-d")
	}
	args = append(args, "--name", d.Name)
	for _, v := range d.Volumes {
		args = append(args, "-v")
		args = append(args, v)
	}
	// Add cloudbots related args such as env var, volume.
	if env.IsCloudBot() {
		args = append(args, cloudbotsDockerArgs()...)
	}
	// Add Satlab related args such as env var, volume.
	if droneName := os.Getenv("DRONE_AGENT_HIVE"); strings.Contains(droneName, "satlab") {
		args = append(args, satlabTLSDockerArgs()...)
	}
	// Set to automatically remove the container when it exits.
	args = append(args, "--rm")
	if d.Network != "" {
		args = append(args, "--network", d.Network)
	}

	// give access to net_raw so things like `ping` can work in the container.
	if netbind == true {
		args = append(args, "--cap-add=NET_RAW")
	}

	// Publish in-docker ports; any without an explicit mapping will need to be looked up later.
	if len(d.PortMappings) != 0 {
		args = append(args, "-p")
		args = append(args, d.PortMappings...)
	}

	args = append(args, d.RequestedImageName)
	if len(d.ExecCommand) > 0 {
		args = append(args, d.ExecCommand...)
	}

	cmd := exec.Command("docker", args...)
	if d.LogFileDir != "" {
		log.Printf("Attempting to gather metrics")
		go d.logRunTime(ctx, service, d.RequestedImageName)
		log.Printf("\nfinished metrics \n")

	} else {
		log.Printf("Skipping metrics gathering")
	}

	if block {
		log.Println("Runing Blocking Docker Run")
		so, se, err := common.RunWithTimeout(ctx, cmd, time.Hour, block)
		common.PrintToLog(fmt.Sprintf("Run docker image %q", d.Name), so, se)
		return so, errors.Annotate(err, "run docker image %q: %s", d.Name, se).Err()
	} else {
		log.Println("Runing Non-Blocking Docker Run")

		var stdoutbuf, stderrbuf bytes.Buffer
		cmd.Stdout = &stdoutbuf
		cmd.Stderr = &stderrbuf

		log.Printf("Running cmd %s", cmd)
		cmd.Start()
		d.Stdoutbuf = &stdoutbuf
		d.Stderrbuf = &stderrbuf
		return "", errors.Annotate(err, "run docker image %q: %s", d.Name, "").Err()

	}

}

// envvars sets the needed environment variables for the container.
func envvars() []string {
	bbid := common.BuildBucketID()
	if bbid == "" {
		bbid = "none"
	}
	swarmingTaskID := os.Getenv("SWARMING_TASK_ID")
	if swarmingTaskID == "" {
		swarmingTaskID = "none"
	}
	servodContainerLabel := os.Getenv("SERVOD_CONTAINER_LABEL")
	if servodContainerLabel == "" {
		servodContainerLabel = "release"
	}
	return []string{
		"--env", fmt.Sprintf("BUILD_BUCKET_ID=%s", bbid),
		"--env", fmt.Sprintf("SWARMING_TASK_ID=%s", swarmingTaskID),
		"--env", fmt.Sprintf("SERVOD_CONTAINER_LABEL=%s", servodContainerLabel),
	}
}

// cloudbotsDockerArgs returns cloudbots args such as env vars, volumes.
func cloudbotsDockerArgs() []string {
	args := []string{
		"--env", fmt.Sprintf("SWARMING_BOT_ID=%s", os.Getenv("SWARMING_BOT_ID")),
	}
	// cloudbots environment variables
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "CLOUDBOTS_") {
			args = append(args, "--env", env)
		}
	}
	// cloudbots host files
	if v, found := os.LookupEnv("CLOUDBOTS_CA_CERTIFICATE"); found {
		args = append(args, "-v", fmt.Sprintf("%s:%s", v, v))
	}
	hostSSHConfig := "/home/chrome-bot/.ssh/config"
	cntSSHConfig := "/home/chromeos-test/.ssh/config"
	if _, err := os.Stat(hostSSHConfig); err != nil {
		log.Printf("warning: cloudbots .ssh/config file do no exist")
	} else {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostSSHConfig, cntSSHConfig))
	}
	return args
}

// satlabTLSDockerArgs returns Satlab specific args such as env vars and volume.
func satlabTLSDockerArgs() []string {
	var args []string
	// Required for Satlab's Docker TLS daemon. See b/197875817
	if path := os.Getenv("DOCKER_CERT_PATH"); path != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s", path, path))
		var tlsVars = []string{"DOCKER_CERT_PATH", "DOCKER_HOST", "DOCKER_TLS_VERIFY"}
		for _, env := range tlsVars {
			args = append(args, "--env", env)
		}
	}
	return args
}

func (d *Docker) logRunTime(ctx context.Context, service string, imageName string) {
	// The runtime detection will not work for R108.
	r := regexp.MustCompile(".*R108-.*")
	if r.MatchString(imageName) {
		log.Printf("METRICS: Skipping logRunTime for image %s\n", imageName)
		return
	}
	startTime := time.Now()
	err := common.Poll(ctx, func(ctx context.Context) error {
		var err error
		var filePath string
		filePath, err = common.FindFile("log.txt", d.LogFileDir)

		if err != nil {
			return errors.Annotate(err, "failed to find file %s log file; logged timeout as metric", d.LogFileDir).Err()
		}

		// File found? This is enough signal to show the service started.
		logServiceFound(ctx, filePath, startTime, service)
		log.Printf("METRICS: Successful Log for %s\n", service)

		return nil
	}, &common.PollOptions{Timeout: 5 * time.Minute, Interval: time.Second})

	// File not found? Log the timeout duration && fail.
	if err != nil {
		// One last check. Its possible that the file is found, service exits
		// the poll is killed, all in the 1 second loop interval.
		log.Printf("METRICS: Final log check for %s\n", service)

		filePath, err := common.FindFile("log.txt", d.LogFileDir)
		// No err? File is found.
		if err == nil {
			logServiceFound(ctx, filePath, startTime, service)
			return
		}
		// Otherwise, its not found. And I give up trying to fix this race without breaking other flows.
		logRunTimeProd(ctx, startTime, service)
		log.Printf("CRITICAL ERROR: Service: %s unable to start. Likely underlying environmental issues. Task will fail.\n", service)
		logStatusProd(ctx, "fail")
		log.Println("Log file not found, logged timediff anyways..")
		return
	}
}

// CreateImageName creates docker image name from repo-path and tag.
func CreateImageName(repoPath, tag string) string {
	return fmt.Sprintf("%s:%s", repoPath, tag)
}

// CreateImageNameFromInputInfo creates docker image name from input info.
//
// If info is empty then return empty name.
// If one of the fields empty then use related default value.
func CreateImageNameFromInputInfo(di *api.DutInput_DockerImage, defaultRepoPath, defaultTag string) string {
	if di == nil {
		return ""
	}
	if di.GetRepositoryPath() == "" && di.GetTag() == "" {
		return ""
	}
	repoPath := di.GetRepositoryPath()
	if repoPath == "" {
		repoPath = defaultRepoPath
	}
	tag := di.GetTag()
	if tag == "" {
		tag = defaultTag
	}
	if repoPath == "" || tag == "" {
		panic("Default repository path or tag for docker image was not passed.")
	}
	return CreateImageName(repoPath, tag)
}

// Define metrics. Note: in Go you have to declare metric field types.
var (
	pullTime = metric.NewFloat("chrome/infra/CFT/docker_pull",
		"Duration of the docker pull.",
		&types.MetricMetadata{Units: types.Seconds},
		field.String("service"),
		field.String("drone"),
		field.String("image"))
	runTime = metric.NewFloat("chrome/infra/CFT/docker_runNew",
		"Duration of the docker run.",
		&types.MetricMetadata{Units: types.Seconds},
		field.String("service"),
		field.String("drone"),
		field.String("image"))
	pullTimeExperimental = metric.NewFloat("chrome/infra/CFT/docker_pullExperimental",
		"Duration of the docker pull.",
		&types.MetricMetadata{Units: types.Seconds},
		field.String("service"),
		field.String("drone"),
		field.String("image"))
	runTimeExperimental = metric.NewFloat("chrome/infra/CFT/docker_runNewExperimental",
		"Duration of the docker run.",
		&types.MetricMetadata{Units: types.Seconds},
		field.String("service"),
		field.String("drone"),
		field.String("image"))
)

func getEnvVar(v string) string {
	out := os.Getenv(v)
	if out == "" {
		out = "NOT_FOUND"
	}
	return out
}
func droneName() string {
	dn := getEnvVar("DOCKER_DRONE_SERVER_NAME")
	log.Printf("INFORMATIONAL: Drone name used for metrics: %s", dn)
	return dn
}

func droneImage() string {
	dv := getEnvVar("DOCKER_DRONE_IMAGE")
	log.Printf("INFORMATIONAL: Drone Image used for metrics: %s", dv)
	return dv
}

func logPullTime(ctx context.Context, startTime time.Time, service string) {
	td := float64(time.Since(startTime).Seconds())
	log.Printf("Service: %s logging pulltime (non-prod): %v.\n", service, td)
	pullTimeExperimental.Set(ctx, td, service, droneName(), droneImage())
}

func logRunTime(ctx context.Context, startTime time.Time, service string) {
	td := float64(time.Since(startTime).Seconds())
	log.Printf("Service: %s logging runtime (non-prod): %v.\n", service, td)
	runTimeExperimental.Set(ctx, td, service, droneName(), droneImage())
}

func logPullTimeProd(ctx context.Context, startTime time.Time, service string) {
	td := float64(time.Since(startTime).Seconds())
	log.Printf("Service: %s logging pulltime (prod): %v.\n", service, td)
	pullTime.Set(ctx, td, service, droneName(), droneImage())
}

func logRunTimeProd(ctx context.Context, startTime time.Time, service string) {
	td := float64(time.Since(startTime).Seconds())
	log.Printf("Service: %s logging runtime (prod): %v.\n", service, td)
	runTime.Set(ctx, td, service, droneName(), droneImage())
}

// logServiceFound logs the when the service has started.
func logServiceFound(ctx context.Context, LogFileName string, startTime time.Time, service string) {
	log.Printf("Service: %s started. \n", service)
	logStatusProd(ctx, "pass")
	logRunTimeProd(ctx, startTime, service)
}

// Define metrics. Note: in Go you have to declare metric field types.
var (
	statusMetrics = metric.NewCounter("chrome/infra/CFT/docker_run_passrate",
		"Note of pass or fail.",
		&types.MetricMetadata{},
		field.String("status"))
	statusMetricsExperimental = metric.NewCounter("chrome/infra/CFT/docker_run_passrateExperimental",
		"Note of pass or fail.",
		&types.MetricMetadata{},
		field.String("status"))
)

func logStatus(ctx context.Context, status string) {
	log.Printf("Logging Status (non-prod): %s\n", status)
	statusMetricsExperimental.Set(ctx, 1, status)
}

func logStatusProd(ctx context.Context, status string) {
	log.Printf("Logging Status (prod): %s\n", status)
	statusMetrics.Set(ctx, 1, status)
}

// Export metrics API.
var (
	LogPullTime     = logPullTime
	LogRunTime      = logRunTime
	LogStatus       = logStatus
	LogPullTimeProd = logPullTimeProd
	LogRunTimeProd  = logRunTimeProd
	LogStatusProd   = logStatusProd
)
