// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package box

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

func TestRunUpdateSatlabCmdInjected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name           string
		mockReadFunc   readContentsFunc
		expectedOutput string
		expectedError  bool
		errorMsg       string
	}{
		{
			name: "Update Available",
			mockReadFunc: func(ctx context.Context, commander executor.IExecCommander) (bool, error) {
				return true, nil
			},
			expectedOutput: "true\n",
			expectedError:  false,
		},
		{
			name: "Update Not Available",
			mockReadFunc: func(ctx context.Context, commander executor.IExecCommander) (bool, error) {
				return false, nil
			},
			expectedOutput: "false\n",
			expectedError:  false,
		},
		{
			name: "Error Reading Contents",
			mockReadFunc: func(ctx context.Context, commander executor.IExecCommander) (bool, error) {
				return false, errors.New("failed to check")
			},
			expectedOutput: "",
			expectedError:  true,
			errorMsg:       "read contents: failed to check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := new(bytes.Buffer)
			cmd := updateSatlabRun{}

			err := cmd.runCmdInjected(ctx, out, tt.mockReadFunc)

			if diff := cmp.Diff(tt.expectedOutput, out.String()); diff != "" {
				t.Errorf("runCmdInjected() output mismatch (-want +got):\n%s", diff)
			}

			if (err != nil) != tt.expectedError {
				t.Errorf("runCmdInjected() error = %v, expectedError %v", err, tt.expectedError)
			} else if err != nil && err.Error() != tt.errorMsg {
				t.Errorf("runCmdInjected() error message = %q, want %q", err.Error(), tt.errorMsg)
			}
		})
	}
}
