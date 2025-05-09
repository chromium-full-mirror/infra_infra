// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"text/template"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/dns"
	"go.chromium.org/infra/cros/satlab/common/paths"
	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
	"go.chromium.org/infra/cros/satlab/common/utils/misc"
)

const gcloudSDKVersion = "507.0.0"

const satlabConfigTemplate = `Host {{.SatlabID}}
  HostName 127.0.0.1
  user moblab
  Port 2222
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  IdentityFile %d/.ssh/testing_rsa
  ForwardAgent no
  ProxyCommand gcloud compute ssh {{.SatlabID}} --zone {{.Zone}} --project {{.ProjectID}} -- -W %h:%p
`

const sshConfigTemplate = `
Host {{.Name}}
  HostName {{.Name}}
  User root
  ProxyJump {{.Info.SatlabID}}
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  IdentityFile %d/.ssh/testing_rsa
`

const adbConfigTemplate = `
Host {{.Name}}
  HostName 127.0.0.1
  user moblab
  Port 2222
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  IdentityFile %d/.ssh/testing_rsa
  ProxyCommand gcloud compute ssh {{.Info.SatlabID}} --zone {{.Info.Zone}} --project {{.Info.ProjectID}} -- -W %h:%p
  LocalForward 5555 {{.Name}}:5555
  RequestTTY no
`

type InitiateSupportRun struct {
	ProjectID string
	Zone      string
	Network   string
	Timeout   string
	Port      int

	SatlabID           string
	ServiceAccountName string
	ContainerName      string
}

type DUTInfo struct {
	IP   string
	Name string
	Info InitiateSupportRun
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

	if err := c.generateAndUpload(ctx, executor); err != nil {
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

	c.SatlabID, err = getSatlabDockerHostName(ctx, executor)
	if err != nil {
		return errors.New("Failed to get Satlab Docker hostname")
	}

	c.ServiceAccountName, err = extractServiceAccountNameFromKeyFile(paths.PubSubKey)
	if err != nil {
		return errors.New("Failed to extract service account name")
	}

	c.ContainerName = "support"

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

	var keyData map[string]any
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

	return parts[0], nil
}

func (c *InitiateSupportRun) startDockerContainerForCloudSDK() []string {
	return []string{
		paths.DockerPath,
		"run",
		"-dit",
		"--name",
		c.ContainerName,
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
		c.ContainerName,
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
		c.ContainerName,
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
		c.ContainerName,
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
		c.ContainerName,
		"gcloud",
		"compute",
		"instances",
		"create",
		c.SatlabID,
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
		c.ContainerName,
	}
}

func (c *InitiateSupportRun) deleteComputeInstance() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.ContainerName,
		"gcloud",
		"compute",
		"instances",
		"delete",
		c.SatlabID,
		"--delete-disks",
		"all",
		"--quiet",
	}
}

func (c *InitiateSupportRun) startPortForwarding() []string {
	return []string{
		paths.DockerPath,
		"exec",
		c.ContainerName,
		"gcloud",
		"compute",
		"ssh",
		fmt.Sprintf("%s@%s", c.ServiceAccountName, c.SatlabID),
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

func (c *InitiateSupportRun) generateAndUpload(ctx context.Context, executor executor.IExecCommander) error {
	content, err := dns.ReadContents(ctx, executor)
	if err != nil {
		return errors.Annotate(err, "read DNS entries").Err()
	}

	duts := c.parseAndGroupDUTs(content)

	config, err := c.generateConfig(duts, checkDutSshConnectivity)
	if err != nil {
		return errors.Annotate(err, "generate remote support config").Err()
	}
	fmt.Println("Generated remote support config.")

	if err := c.uploadConfigToBucket(config, ctx, executor); err != nil {
		return errors.Annotate(err, "upload remote support config to bucket").Err()
	}
	fmt.Println("Uploaded remote support config to bucket.")

	return nil
}

func (c *InitiateSupportRun) parseAndGroupDUTs(dnsOutput string) []DUTInfo {
	var duts []DUTInfo
	lines := strings.Split(dnsOutput, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)

		if len(parts) != 2 {
			fmt.Printf("Skipping malformed line: '%s'\n", line)
			continue
		}

		dut := DUTInfo{
			IP:   parts[0],
			Name: parts[1],
			Info: *c,
		}
		duts = append(duts, dut)
	}

	return duts
}

func (c *InitiateSupportRun) generateConfig(duts []DUTInfo, checkDutSshConnectivity func(DUTInfo) bool) (string, error) {
	var sb strings.Builder

	config, err := executeTemplate("satlabConfig", satlabConfigTemplate, c)
	if err != nil {
		return "", errors.Annotate(err, "error generating Satlab config").Err()
	}
	sb.WriteString(config)

	for _, dut := range duts {
		if checkDutSshConnectivity(dut) {
			config, err := executeTemplate("sshConfig", sshConfigTemplate, dut)
			if err != nil {
				fmt.Println("Error generating DUT SSH config:", err)
			} else {
				sb.WriteString(config)
			}
		} else {
			config, err := executeTemplate("adbConfig", adbConfigTemplate, dut)
			if err != nil {
				fmt.Println("Error generating DUT ADB config:", err)
			} else {
				sb.WriteString(config)
			}
		}
	}
	return sb.String(), nil
}

func executeTemplate(templateName, templateContent string, data interface{}) (string, error) {
	tmpl, err := template.New(templateName).Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("error parsing template '%s': %w", templateName, err)
	}

	var outputBuffer bytes.Buffer

	if err := tmpl.Execute(&outputBuffer, data); err != nil {
		return "", fmt.Errorf("error executing template '%s': %w", templateName, err)
	}

	return outputBuffer.String(), nil
}

func checkDutSshConnectivity(c DUTInfo) bool {
	args := []string{
		"ssh",
		"-o UserKnownHostsFile=/dev/null",
		"-o StrictHostKeyChecking no",
		"-o IdentityFile=/home/moblab/.ssh/testing_rsa",
		fmt.Sprintf("root@%s", c.Name),
		"echo ok",
	}

	cmd := exec.Command(args[0], args[1:]...)

	var stdoutBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf

	if err := cmd.Run(); err != nil {
		return false
	}

	if stdoutBuf.String() == "ok\n" {
		return true
	}

	return false
}

func (c *InitiateSupportRun) uploadConfigToBucket(config string, ctx context.Context, executor executor.IExecCommander) error {
	bucket := site.GetGCSPartnerBucket()

	client, err := storage.NewClient(ctx, option.WithCredentialsFile(site.GetServiceAccountPath()))
	if err != nil {
		return errors.Annotate(err, "create storage client").Err()
	}

	filePath := fmt.Sprintf("support/%s-config", c.SatlabID)
	writer := client.Bucket(bucket).Object(fmt.Sprintf(filePath)).NewWriter(ctx)

	defer writer.Close()

	if _, err := writer.Write([]byte(config)); err != nil {
		return errors.Annotate(err, "write config to bucket").Err()
	}

	return nil
}
