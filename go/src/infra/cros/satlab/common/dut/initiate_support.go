// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

const gcloudSDKVersion = "507.0.0"

type InitiateSupportRun struct {
	ProjectID string
	Zone      string
	Network   string
	Timeout   string
	Port      int

	satlabID           string
	serviceAccountName string
	containerName      string
}

// TriggerRun triggers the Run with the given information
func (c *InitiateSupportRun) TriggerRun(
	ctx context.Context,
	executor executor.IExecCommander,
) error {
	if err := c.validateArgs(); err != nil {
		return err
	}

	if err := c.populateRunParameters(ctx, executor); err != nil {
		return err
	}

	cancelChannel := make(chan os.Signal, 1)
	signal.Notify(cancelChannel, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-cancelChannel
	}()

	if err := c.executeCloudOperations(ctx); err != nil {
		return err
	}

	return nil
}

func (c *InitiateSupportRun) validateArgs() error {
	if c.ProjectID == "" {
		return errors.New("Must specify --project-id")
	}
	if c.Zone == "" {
		return errors.New("Must specify --zone")
	}
	return nil
}

func (c *InitiateSupportRun) populateRunParameters(ctx context.Context, executor executor.IExecCommander) error {
	var err error

	c.satlabID, err = getSatlabDockerHostName(ctx, executor)
	if err != nil {
		return errors.New("Failed to get Satlab Docker hostname")
	}

	c.serviceAccountName, err = extractServiceAccountNameFromKeyFile(paths.PubSubKey)
	if err != nil {
		return errors.New("Failed to extract service account name")
	}

	c.containerName = "support"

	return nil
}

func (c *InitiateSupportRun) executeCloudOperations(ctx context.Context) error {
	if err := execute(ctx, c.startDockerContainerForCloudSDK()); err != nil {
		fmt.Println("Start docker container for Google Cloud SDK:", err)
		return err
	}
	defer func() {
		fmt.Println("Docker container cleanup started...")
		if err := execute(ctx, c.killDockerContainerForCloudSDK()); err != nil {
			fmt.Println("Docker container cleanup:", err)
		}
		fmt.Println("Docker container cleanup finished.")
	}()

	if err := execute(ctx, c.authenticateToGCPWithServiceAccount()); err != nil {
		fmt.Println("Authenticate to Google Cloud Platform:", err)
		return err
	}

	if err := execute(ctx, c.setProject()); err != nil {
		fmt.Println("Set project in Google Cloud Platform:", err)
		return err
	}

	if err := execute(ctx, c.setZone()); err != nil {
		fmt.Println("Set zone in Google Cloud Platform:", err)
		return err
	}

	if err := execute(ctx, c.createComputeInstance()); err != nil {
		fmt.Println("Create compute instance in Google Cloud Platform:", err)
		return err
	}
	defer func() {
		fmt.Println("Instance cleanup on GCP started...")
		if err := execute(ctx, c.deleteComputeInstance()); err != nil {
			fmt.Println("Instance cleanup on GCP:", err)
		}
		fmt.Println("Instance cleanup on GCP finished.")
	}()

	if err := execute(ctx, c.startPortForwarding()); err != nil {
		fmt.Println("Port forwarding to Google Cloud Platform:", err)
		return err
	}

	return nil
}

func dockerCloudSDKImageName() string {
	version := misc.GetEnv("GCLOUD_SDK_VERSION", gcloudSDKVersion)
	return fmt.Sprintf("google/cloud-sdk:%s-slim", version)
}

func getSatlabDockerHostName(ctx context.Context, executor executor.IExecCommander) (string, error) {
	id, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("satlab-%s", id), nil
}

func extractServiceAccountNameFromKeyFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	var keyData map[string]interface{}
	err = json.Unmarshal(data, &keyData)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal key file: %w", err)
	}

	clientEmail, ok := keyData["client_email"].(string)
	if !ok {
		return "", fmt.Errorf("unable to extract client_email from key file")
	}

	parts := strings.Split(clientEmail, "@")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid client_email format")
	}
	serviceAccountName := parts[0]

	return serviceAccountName, nil
}

func (c *InitiateSupportRun) startDockerContainerForCloudSDK() []string {
	return []string{
		paths.DockerPath,
		"run",
		"-dit",
		"--name",
		c.containerName,
		"--net",
		"host",
		"--rm",
		"-v",
		"satlab_keys:/home/satlab/keys",
		dockerCloudSDKImageName(),
	}
}

func (c *InitiateSupportRun) authenticateToGCPWithServiceAccount() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"auth",
		"activate-service-account",
		fmt.Sprintf("--key-file=%s", paths.PubSubKey),
	}
}

func (c *InitiateSupportRun) setProject() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"config",
		"set",
		"project",
		c.ProjectID,
	}
}

func (c *InitiateSupportRun) setZone() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"config",
		"set",
		"compute/zone",
		c.Zone,
	}
}

func (c *InitiateSupportRun) createComputeInstance() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"compute",
		"instances",
		"create",
		c.satlabID,
		"--network",
		c.Network,
		"--max-run-duration",
		c.Timeout,
		"--instance-termination-action",
		"delete",
	}
}

func (c *InitiateSupportRun) killDockerContainerForCloudSDK() []string {
	return []string{
		paths.DockerPath,
		"kill",
		c.containerName,
	}
}

func (c *InitiateSupportRun) deleteComputeInstance() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"compute",
		"instances",
		"delete",
		c.satlabID,
		"--delete-disks",
		"all",
		"--quiet",
	}
}

func (c *InitiateSupportRun) startPortForwarding() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"compute",
		"ssh",
		fmt.Sprintf("%s@%s", c.serviceAccountName, c.satlabID),
		"--",
		"-NR",
		fmt.Sprintf("2222:localhost:%d", c.Port),
		"-v",
	}
}

func execute(
	ctx context.Context,
	args []string,
) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
