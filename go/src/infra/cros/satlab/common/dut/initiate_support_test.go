// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.chromium.org/infra/cros/satlab/common/paths"
)

func TestValidateArgs(t *testing.T) {
	testCases := []struct {
		name        string
		projectID   string
		zone        string
		expectedErr error
	}{
		{
			name:        "Valid arguments",
			projectID:   "test-project",
			zone:        "test-zone",
			expectedErr: nil,
		},
		{
			name:        "Missing ProjectID",
			projectID:   "",
			zone:        "test-zone",
			expectedErr: errors.New("Must specify --project-id"),
		},
		{
			name:        "Missing Zone",
			projectID:   "test-project",
			zone:        "",
			expectedErr: errors.New("Must specify --zone"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &InitiateSupportRun{
				ProjectID: tc.projectID,
				Zone:      tc.zone,
			}

			err := c.validateArgs()

			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("Expected error: %v, but got nil", tc.expectedErr)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error message: %q, but got: %q", tc.expectedErr.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestExtractServiceAccountNameFromKeyFile(t *testing.T) {
	// Create a temporary directory for the key file
	tempDir, err := os.MkdirTemp("", "test_key_file")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	const testKeyFilePath = "test.json"

	testCases := []struct {
		name           string
		keyFileContent string
		expectedName   string
		expectedError  string
		keyFilePath    string
	}{
		{
			name:           "Valid key file",
			keyFileContent: `{"client_email": "test-account@test-project.iam.gserviceaccount.com"}`,
			expectedName:   "test-account",
			keyFilePath:    testKeyFilePath,
		},
		{
			name:           "Missing client_email",
			keyFileContent: `{}`,
			expectedError:  "unable to extract client_email from key file",
			keyFilePath:    testKeyFilePath,
		},
		{
			name:           "Invalid client_email format",
			keyFileContent: `{"client_email": "test-account"}`,
			expectedError:  "invalid client_email format",
			keyFilePath:    testKeyFilePath,
		},
		{
			name:           "Invalid JSON",
			keyFileContent: `invalid}`,
			expectedError:  "failed to unmarshal key file",
			keyFilePath:    testKeyFilePath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyFilePath := filepath.Join(tempDir, tc.keyFilePath)
			err := os.WriteFile(keyFilePath, []byte(tc.keyFileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write key file: %v", err)
			}

			name, err := extractServiceAccountNameFromKeyFile(keyFilePath)

			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedName, name)
		})
	}
}

func TestStartDockerContainerForCloudSDK(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
	}

	expected := []string{
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

	actual := c.startDockerContainerForCloudSDK()
	assert.Equal(t, expected, actual)
}

func TestDockerCloudSDKImageName(t *testing.T) {
	testCases := []struct {
		version        string
		expectedOutput string
	}{
		{
			version:        gcloudSDKVersion,
			expectedOutput: fmt.Sprintf("google/cloud-sdk:%s-slim", gcloudSDKVersion),
		}, {
			version:        "latest",
			expectedOutput: "google/cloud-sdk:latest-slim",
		},
	}
	for _, tc := range testCases {
		originalVERSION := os.Getenv("GCLOUD_SDK_VERSION")
		os.Setenv("GCLOUD_SDK_VERSION", tc.version)
		assert.Equal(t, tc.expectedOutput, dockerCloudSDKImageName())
		os.Setenv("GCLOUD_SDK_VERSION", originalVERSION)
	}
}

func TestAuthenticateToGCPWithServiceAccount(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
	}

	expected := []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"auth",
		"activate-service-account",
		fmt.Sprintf("--key-file=%s", paths.PubSubKey),
	}

	actual := c.authenticateToGCPWithServiceAccount()
	assert.Equal(t, expected, actual)
}

func TestSetProject(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
		ProjectID:     "test-project",
	}

	expected := []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"config",
		"set",
		"project",
		c.ProjectID,
	}

	actual := c.setProject()
	assert.Equal(t, expected, actual)
}

func TestSetZone(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
		Zone:          "test-zone",
	}

	expected := []string{
		paths.DockerPath,
		"exec",
		c.containerName,
		"gcloud",
		"config",
		"set",
		"compute/zone",
		c.Zone,
	}

	actual := c.setZone()
	assert.Equal(t, expected, actual)
}

func TestCreateComputeInstance(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
		satlabID:      "satlab-instance-123",
		Network:       "satlab-network",
		Timeout:       "1h",
	}

	expected := []string{
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

	actual := c.createComputeInstance()
	assert.Equal(t, expected, actual)
}

func TestKillDockerContainerForCloudSDK(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
	}

	expected := []string{
		paths.DockerPath,
		"kill",
		c.containerName,
	}

	actual := c.killDockerContainerForCloudSDK()
	assert.Equal(t, expected, actual)
}

func TestDeleteComputeInstance(t *testing.T) {
	c := &InitiateSupportRun{
		containerName: "support-container",
		satlabID:      "satlab-instance-123",
	}

	expected := []string{
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

	actual := c.deleteComputeInstance()
	assert.Equal(t, expected, actual)
}

func TestStartPortForwarding(t *testing.T) {
	c := &InitiateSupportRun{
		serviceAccountName: "test-account",
		satlabID:           "satlab-id-123",
		containerName:      "support-container",
		Port:               8080,
	}

	expected := []string{
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

	actual := c.startPortForwarding()
	assert.Equal(t, expected, actual)
}
