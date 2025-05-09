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
		ContainerName: "support-container",
	}

	expected := []string{
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
		ContainerName: "support-container",
	}

	expected := []string{
		paths.DockerPath,
		"exec",
		c.ContainerName,
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
		ContainerName: "support-container",
		ProjectID:     "test-project",
	}

	expected := []string{
		paths.DockerPath,
		"exec",
		c.ContainerName,
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
		ContainerName: "support-container",
		Zone:          "test-zone",
	}

	expected := []string{
		paths.DockerPath,
		"exec",
		c.ContainerName,
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
		ContainerName: "support-container",
		SatlabID:      "satlab-instance-123",
		Network:       "satlab-network",
		Timeout:       "1h",
	}

	expected := []string{
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

	actual := c.createComputeInstance()
	assert.Equal(t, expected, actual)
}

func TestKillDockerContainerForCloudSDK(t *testing.T) {
	c := &InitiateSupportRun{
		ContainerName: "support-container",
	}

	expected := []string{
		paths.DockerPath,
		"kill",
		c.ContainerName,
	}

	actual := c.killDockerContainerForCloudSDK()
	assert.Equal(t, expected, actual)
}

func TestDeleteComputeInstance(t *testing.T) {
	c := &InitiateSupportRun{
		ContainerName: "support-container",
		SatlabID:      "satlab-instance-123",
	}

	expected := []string{
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

	actual := c.deleteComputeInstance()
	assert.Equal(t, expected, actual)
}

func TestStartPortForwarding(t *testing.T) {
	c := &InitiateSupportRun{
		ServiceAccountName: "test-account",
		SatlabID:           "satlab-id-123",
		ContainerName:      "support-container",
		Port:               8080,
	}

	expected := []string{
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

	actual := c.startPortForwarding()
	assert.Equal(t, expected, actual)
}

func TestParseAndGroupDUTs(t *testing.T) {
	baseInfo := InitiateSupportRun{
		ProjectID: "test-proj",
		Zone:      "test-zone",
		SatlabID:  "test-satlab",
		Port:      22,
	}

	testCases := []struct {
		name       string
		dnsOutput  string
		expected   []DUTInfo
		expectLogs bool // Flag to indicate if we expect "Skipping malformed line" logs
	}{
		{
			name:      "Empty input",
			dnsOutput: "",
			expected:  nil,
		},
		{
			name:      "Single valid line",
			dnsOutput: "192.168.1.100 dut-1",
			expected: []DUTInfo{
				{IP: "192.168.1.100", Name: "dut-1", Info: baseInfo},
			},
		},
		{
			name: "Multiple valid lines",
			dnsOutput: `
192.168.1.100 dut-1
192.168.1.101 dut-2
10.0.0.5      dut-3
			`,
			expected: []DUTInfo{
				{IP: "192.168.1.100", Name: "dut-1", Info: baseInfo},
				{IP: "192.168.1.101", Name: "dut-2", Info: baseInfo},
				{IP: "10.0.0.5", Name: "dut-3", Info: baseInfo},
			},
		},
		{
			name:      "Line with extra whitespace",
			dnsOutput: "  192.168.1.102   dut-4  ",
			expected: []DUTInfo{
				{IP: "192.168.1.102", Name: "dut-4", Info: baseInfo},
			},
		},
		{
			name:       "Line with only IP",
			dnsOutput:  "192.168.1.103",
			expected:   nil,
			expectLogs: true,
		},
		{
			name:       "Line with only Name",
			dnsOutput:  "dut-5",
			expected:   nil,
			expectLogs: true,
		},
		{
			name:       "Line with more than two fields",
			dnsOutput:  "192.168.1.104 dut-6 extra-field",
			expected:   nil,
			expectLogs: true,
		},
		{
			name: "Mixed valid and invalid lines",
			dnsOutput: `
192.168.1.105 dut-7
malformed_line
192.168.1.106 dut-8 extra
192.168.1.107 dut-9
			`,
			expected: []DUTInfo{
				{IP: "192.168.1.105", Name: "dut-7", Info: baseInfo},
				{IP: "192.168.1.107", Name: "dut-9", Info: baseInfo},
			},
			expectLogs: true,
		},
		{
			name: "Input with empty lines",
			dnsOutput: `
192.168.1.108 dut-10
192.168.1.109 dut-11

			`,
			expected: []DUTInfo{
				{IP: "192.168.1.108", Name: "dut-10", Info: baseInfo},
				{IP: "192.168.1.109", Name: "dut-11", Info: baseInfo},
			},
		},
		{
			name:      "Input with trailing newline",
			dnsOutput: "192.168.1.110 dut-12\n",
			expected: []DUTInfo{
				{IP: "192.168.1.110", Name: "dut-12", Info: baseInfo},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := baseInfo.parseAndGroupDUTs(tc.dnsOutput)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestExecuteTemplate(t *testing.T) {
	type testData struct {
		Name  string
		Value int
		Items []string
	}

	testCases := []struct {
		name            string
		templateName    string
		templateContent string
		data            interface{}
		expectedOutput  string
		expectedError   string // Substring of the expected error message
	}{
		{
			name:            "Simple template with map data",
			templateName:    "simpleMap",
			templateContent: "Hello, {{.Place}}!",
			data:            map[string]string{"Place": "World"},
			expectedOutput:  "Hello, World!",
			expectedError:   "",
		},
		{
			name:            "Template with struct data",
			templateName:    "structData",
			templateContent: "Name: {{.Name}}, Value: {{.Value}}, Items: {{range .Items}}{{.}} {{end}}",
			data:            testData{Name: "Test", Value: 123, Items: []string{"a", "b"}},
			expectedOutput:  "Name: Test, Value: 123, Items: a b ",
			expectedError:   "",
		},
		{
			name:            "Template with nil data",
			templateName:    "nilData",
			templateContent: "Data is {{.}}",
			data:            nil,
			expectedOutput:  "Data is <no value>",
			expectedError:   "",
		},
		{
			name:            "Empty template content",
			templateName:    "emptyContent",
			templateContent: "",
			data:            testData{Name: "Test"},
			expectedOutput:  "",
			expectedError:   "",
		},
		{
			name:            "Invalid template syntax",
			templateName:    "invalidSyntax",
			templateContent: "Hello, {{.Place",
			data:            map[string]string{"Place": "World"},
			expectedOutput:  "",
			expectedError:   "error parsing template 'invalidSyntax'",
		},
		{
			name:            "Template execution error - missing field",
			templateName:    "missingField",
			templateContent: "Value: {{.Missing}}",
			data:            testData{Name: "Test"},
			expectedOutput:  "",
			expectedError:   "error executing template 'missingField'",
		},
		{
			name:            "Template with special characters",
			templateName:    "specialChars",
			templateContent: "Special: \"{{.Text}}\"",
			data:            map[string]string{"Text": "Line1\nLine2\tTab"},
			expectedOutput:  "Special: \"Line1\nLine2\tTab\"",
			expectedError:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeTemplate(tc.templateName, tc.templateContent, tc.data)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Empty(t, output, "Output should be empty on error")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedOutput, output)
			}
		})
	}
}

func TestGenerateConfig(t *testing.T) {
	baseInfo := InitiateSupportRun{
		ProjectID: "test-proj",
		Zone:      "test-zone",
		SatlabID:  "test-satlab",
		Port:      22,
	}

	generateExpectedSatlabConfig := func(info InitiateSupportRun) string {
		cfg, _ := executeTemplate("satlabConfig", satlabConfigTemplate, &info)
		return cfg
	}

	generateExpectedSSHConfig := func(dut DUTInfo) string {
		cfg, _ := executeTemplate("sshConfig", sshConfigTemplate, dut)
		return cfg
	}

	generateExpectedADBConfig := func(dut DUTInfo) string {
		cfg, _ := executeTemplate("adbConfig", adbConfigTemplate, dut)
		return cfg
	}

	testCases := []struct {
		name            string
		duts            []DUTInfo
		sshConnectivity map[string]bool
		expectedConfig  string
		expectError     bool
	}{
		{
			name:            "No DUTs",
			duts:            []DUTInfo{},
			sshConnectivity: map[string]bool{},
			expectedConfig:  generateExpectedSatlabConfig(baseInfo),
			expectError:     false,
		},
		{
			name: "One DUT with SSH",
			duts: []DUTInfo{
				{Name: "dut-ssh-1", IP: "192.168.1.1", Info: baseInfo},
			},
			sshConnectivity: map[string]bool{"dut-ssh-1": true},
			expectedConfig: generateExpectedSatlabConfig(baseInfo) +
				generateExpectedSSHConfig(DUTInfo{Name: "dut-ssh-1", IP: "192.168.1.1", Info: baseInfo}),
			expectError: false,
		},
		{
			name: "One DUT with ADB",
			duts: []DUTInfo{
				{Name: "dut-adb-1", IP: "192.168.1.2", Info: baseInfo},
			},
			sshConnectivity: map[string]bool{"dut-adb-1": false},
			expectedConfig: generateExpectedSatlabConfig(baseInfo) +
				generateExpectedADBConfig(DUTInfo{Name: "dut-adb-1", IP: "192.168.1.2", Info: baseInfo}),
			expectError: false,
		},
		{
			name: "Multiple DUTs mixed connectivity",
			duts: []DUTInfo{
				{Name: "dut-ssh-2", IP: "192.168.1.3", Info: baseInfo},
				{Name: "dut-adb-2", IP: "192.168.1.4", Info: baseInfo},
				{Name: "dut-ssh-3", IP: "192.168.1.5", Info: baseInfo},
			},
			sshConnectivity: map[string]bool{
				"dut-ssh-2": true,
				"dut-adb-2": false,
				"dut-ssh-3": true,
			},
			expectedConfig: generateExpectedSatlabConfig(baseInfo) +
				generateExpectedSSHConfig(DUTInfo{Name: "dut-ssh-2", IP: "192.168.1.3", Info: baseInfo}) +
				generateExpectedADBConfig(DUTInfo{Name: "dut-adb-2", IP: "192.168.1.4", Info: baseInfo}) +
				generateExpectedSSHConfig(DUTInfo{Name: "dut-ssh-3", IP: "192.168.1.5", Info: baseInfo}),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCheckFunc := func(dut DUTInfo) bool {
				return tc.sshConnectivity[dut.Name]
			}

			actualConfig, err := baseInfo.generateConfig(tc.duts, mockCheckFunc)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedConfig, actualConfig)
		})
	}
}
