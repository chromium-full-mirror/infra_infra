// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package servod

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/satlab/common/services/ufs"
)

// TestStopServodCmd tests that given a command + docker/ufs client behavior we get expected end conditions
// specifically verify whether we report an error thrown, and what containers are expected to be running
func TestStopServodCmd(t *testing.T) {
	t.Parallel()

	type test struct {
		name                   string
		docker                 FakeDockerClient
		ufs                    ufs.UFSClient
		cmd                    stopServodRun
		expectContainerRunning []string
		expectCmdError         bool // whether we expect the commands' execution to result in error
	}

	tests := []test{
		{"TestNoRunningContainer",
			NewFakeDockerClient(false, false), &FakeUFSClient{}, stopServodRun{host: "office"}, []string{}, false},
		{"TestNoRunningContainerUFSErr",
			NewFakeDockerClient(false, false), &FakeUFSClient{badData: true}, stopServodRun{host: "office"}, []string{}, true},
		{"TestRunningContainerStops",
			NewFakeDockerClient(false, false, "ufsDockerContainerName"), &FakeUFSClient{}, stopServodRun{host: "office"}, []string{}, false},
		{"TestRunningContainerUFSErr",
			NewFakeDockerClient(false, false, "ufsDockerContainerName"), &FakeUFSClient{badData: true}, stopServodRun{host: "office"}, []string{"ufsDockerContainerName"}, true},
		{"TestRunningContainerStopsManualName",
			NewFakeDockerClient(false, false, "manualContainerName"), &FakeUFSClient{}, stopServodRun{host: "office", servodContainerName: "manualContainerName"}, []string{}, false},
		{"TestRunningConatinerDifferentName",
			NewFakeDockerClient(false, false, "manualContainerName"), &FakeUFSClient{}, stopServodRun{host: "office"}, []string{"manualContainerName"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cmd.runCmdWithClients(context.Background(), &tc.docker, tc.ufs)

			if err != nil != tc.expectCmdError {
				t.Errorf("Expected error: %t, got error: %t, input: %+v", tc.expectCmdError, err != nil, tc)
			}

			expectedContainers, runningContainers := tc.expectContainerRunning, tc.docker.getContainerNames()
			if diff := cmp.Diff(expectedContainers, runningContainers); diff != "" {
				t.Errorf("Expected containers: %s, got containers: %s\ninput %+v", expectedContainers, runningContainers, tc)
			}
		})
	}
}
