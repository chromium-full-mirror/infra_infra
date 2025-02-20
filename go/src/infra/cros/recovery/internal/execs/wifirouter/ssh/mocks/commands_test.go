// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mocks

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"go.chromium.org/infra/cros/recovery/internal/execs/wifirouter/ssh"
	"go.chromium.org/infra/cros/recovery/tlw"
)

// TestWgetURL tests the ssh.WgetURL function.
//
// Note: This is in the mocks package because it cannot use mocks and be in th
// package the mocks depend on, as that would introduce an import loop.
func TestWgetURL(t *testing.T) {
	const testURL = "https://google.com"
	tests := []struct {
		name                      string
		additionalWgetArgs        []string
		wgetRunResult             *tlw.RunResult
		wantHTTPErrorResponseCode int
		wantErr                   bool
	}{
		{
			"no additional args, success response",
			[]string{},
			&tlw.RunResult{
				ExitCode: 0,
			},
			0,
			false,
		},
		{
			"additional args, success response",
			[]string{"a", "b", "c"},
			&tlw.RunResult{
				ExitCode: 0,
			},
			0,
			false,
		},
		{
			"no additional args, success response, non-empty stdout",
			[]string{},
			&tlw.RunResult{
				Stdout:   "test stdout",
				ExitCode: 0,
			},
			0,
			false,
		},
		{
			"example error response, with http code",
			[]string{},
			&tlw.RunResult{
				Stderr:   "HTTP error 404",
				ExitCode: 8,
			},
			404,
			true,
		},
		{
			"example non-http error response",
			[]string{},
			&tlw.RunResult{
				Stderr:   "Failed to allocate uclient context",
				ExitCode: 1,
			},
			-1,
			true,
		},
		{
			"example non-http error response",
			[]string{},
			&tlw.RunResult{
				Stderr:   "Failed to allocate uclient context",
				ExitCode: 1,
			},
			-1,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockRunner := NewMockRunner(ctrl)
			var runCmdArgs []interface{}
			runCmdArgs = append(runCmdArgs, testURL)
			for _, arg := range tt.additionalWgetArgs {
				runCmdArgs = append(runCmdArgs, arg)
			}
			mockRunner.EXPECT().
				RunForResult(gomock.Any(), gomock.Any(), false, "wget", runCmdArgs...).
				Return(tt.wgetRunResult)
			gotStdout, gotStderr, gotHTTPErrorResponseCode, err := ssh.WgetURL(context.Background(), mockRunner, 0, testURL, tt.additionalWgetArgs...)
			if (err != nil) != tt.wantErr {
				t.Errorf("WgetURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStdout != tt.wgetRunResult.Stdout {
				t.Errorf("WgetURL() gotStdout = %v, want %v", gotStdout, tt.wgetRunResult.Stdout)
			}
			if gotStderr != tt.wgetRunResult.Stderr {
				t.Errorf("WgetURL() gotStderr = %v, want %v", gotStderr, tt.wgetRunResult.Stderr)
			}
			if gotHTTPErrorResponseCode != tt.wantHTTPErrorResponseCode {
				t.Errorf("WgetURL() gotHTTPErrorResponseCode = %v, want %v", gotHTTPErrorResponseCode, tt.wantHTTPErrorResponseCode)
			}
		})
	}
}
