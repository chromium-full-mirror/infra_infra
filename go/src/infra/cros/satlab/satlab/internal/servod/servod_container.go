// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servod

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/docker"
	"infra/cros/satlab/common/utils/misc"
	ufspb "infra/unifiedfleet/api/v1/models/chromeos/lab"
)

// DockerClient is an interface fulfilled by the recovery docker lib
// Used here to facilitate testing
type DockerClient interface {
	Start(ctx context.Context, containerName string, req *docker.ContainerArgs, timeout time.Duration) (*docker.StartResponse, error)
	IsUp(ctx context.Context, containerName string) (bool, error)
	Remove(ctx context.Context, containerName string, force bool) error
}

type ServodContainerOptions struct {
	containerName string
	board         string
	model         string
	servoSerial   string
	withServod    bool
	servoSetup    ufspb.ServoSetupType
	subrailConfig string
	useRecMode    bool
	dockerTag     string
}

func (opts *ServodContainerOptions) Validate() error {
	if opts.containerName == "" || opts.board == "" || opts.model == "" || opts.servoSerial == "" {
		return errors.Reason("invalid container options, at least one non-nullable string is nil: %+v", opts).Err()
	}

	return nil
}

// startServodContainer is used to start the docker container for servod
// If there is already a container running with the same name it will not start a new container
func startServodContainer(ctx context.Context, d DockerClient, servoContainerName string, dockerArgs *docker.ContainerArgs) (*docker.StartResponse, error) {
	// check presence of running container already
	if up, err := d.IsUp(ctx, servoContainerName); err != nil {
		return nil, err
	} else if up {
		return nil, errors.Reason("Docker container with name %s is already running", servoContainerName).Err()
	}

	res, err := d.Start(ctx, servoContainerName, dockerArgs, time.Minute)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Launched container. To access, run:\n\tdocker exec -it %s bash\n", servoContainerName)
	return res, nil
}

// buildServodContainerArgs produces ContainerArgs which has the full information needed to spin up a servod container via `docker run ...`
func buildServodContainerArgs(opts ServodContainerOptions) *docker.ContainerArgs {
	exec := []string{"tail", "-f", "/dev/null"}
	if opts.withServod {
		exec = []string{"bash", "/start_servod.sh"}
	}

	return &docker.ContainerArgs{
		Detached:   true,
		ImageName:  dockerServodImageName(opts.dockerTag),
		EnvVar:     generateEnvVars(opts.board, opts.model, opts.servoSerial, opts.servoSetup, opts.subrailConfig, opts.useRecMode),
		Volumes:    generateVols(opts.servoSerial),
		Network:    "default_satlab",
		Privileged: true,
		Exec:       exec,
	}
}

// generateEnvVars builds a string array of env vars needed to launch servod in docker
func generateEnvVars(board string, model string, servoSerial string, servoSetup ufspb.ServoSetupType, subrailConfig string, useRecMode bool) []string {
	port := 9999
	var envVars []string

	envVars = append(envVars, fmt.Sprintf("BOARD=%s", board))
	envVars = append(envVars, fmt.Sprintf("MODEL=%s", model))
	envVars = append(envVars, fmt.Sprintf("SERIAL=%s", servoSerial))
	envVars = append(envVars, fmt.Sprintf("PORT=%d", port))

	if subrailConfig != "" {
		envVars = append(envVars, fmt.Sprintf("CONFIG=%s", subrailConfig))
	}
	if servoSetup == ufspb.ServoSetupType_SERVO_SETUP_DUAL_V4 {
		envVars = append(envVars, "DUAL_V4=1")
	}
	if useRecMode {
		envVars = append(envVars, "REC_MODE=1")
	}

	return envVars
}

// generateVols builds a string array of volumes needed to launch servod in docker
func generateVols(servoContainerName string) []string {
	var vols []string

	vols = append(vols, "/dev:/dev")
	vols = append(vols, fmt.Sprintf("%s_log:/var/log/servod_9999/", servoContainerName))

	return vols
}

// dockerServodImageName builds the appropriate image name for servod based on env vars
// duplicates logic in TLW client
func dockerServodImageName(tag string) string {
	if tag == "" {
		// TODO(elijahtrexler) add these variables to SATLAB_REMOTE_ACCESS
		tag = misc.GetEnv("SERVOD_CONTAINER_LABEL", "release")
	}
	registry := misc.GetEnv("REGISTRY_URI", "us-docker.pkg.dev/chromeos-partner-moblab/common-core")
	return fmt.Sprintf("%s/servod:%s", registry, tag)
}
