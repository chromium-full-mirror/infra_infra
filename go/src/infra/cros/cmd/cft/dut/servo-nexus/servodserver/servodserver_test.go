// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servodserver

import (
	"bytes"
	"context"
	"io"
	"log"
	"strings"
	"testing"

	"infra/cros/cmd/cft/dut/servo-nexus/mock_commandexecutor"

	"github.com/golang/mock/gomock"
	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"golang.org/x/crypto/ssh"
)

// Tests that servod starts successfully.
func TestServodServer_StartServodSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Any(), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("success!"))
			bErr.Write([]byte("not failed!"))
			return &bOut, &bErr, nil
		},
	)
	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Eq("servodtool instance wait-for-active --timeout 60 -p 0"), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("success ready!"))
			bErr.Write([]byte("not failed!"))
			return &bOut, &bErr, nil
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	op, err := srv.StartServod(ctx, &api.StartServodRequest{
		ServoHostPath: "servoHostPath",
		ServodPort:    0,
		Board:         "board",
		Model:         "model",
		SerialName:    "serialname",
	})
	if err != nil {
		t.Fatalf("Failed at api.StartServod: %v", err)
	}

	switch op.Result.(type) {
	case *longrunning.Operation_Error:
		t.Fatalf("Failed at api.StartServod: %v", err)
	}
}

// Tests that servod start failure is handled successfully.
func TestServodServer_StartServodFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Any(), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("not success!"))
			bErr.Write([]byte("failed!"))
			return &bOut, &bErr, errors.Reason("error message").Err()
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	op, err := srv.StartServod(ctx, &api.StartServodRequest{
		ServoHostPath: "servoHostPath",
		ServodPort:    0,
		Board:         "board",
		Model:         "model",
		SerialName:    "serialname",
	})
	if err == nil {
		t.Fatalf("Should have failed at api.ExecCmd.")
	}

	if !strings.Contains(err.Error(), "error while running command start servod") {
		t.Fatalf("Expecting Error to be \"error while running command start servod\", instead got %v", err.Error())
	}

	switch op.Result.(type) {
	case *longrunning.Operation_Error:
		t.Fatalf("Failed at api.StartServod: %v", err)
	}
}

// Tests that servod stops successfully.
func TestServodServer_StopServodSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Any(), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("success!"))
			bErr.Write([]byte("not failed!"))
			return &bOut, &bErr, nil
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	op, err := srv.StopServod(ctx, &api.StopServodRequest{
		ServoHostPath: "servoHostPath",
		ServodPort:    0,
	})
	if err != nil {
		t.Fatalf("Failed at api.StopServod: %v", err)
	}

	switch op.Result.(type) {
	case *longrunning.Operation_Error:
		t.Fatalf("Failed at api.StopServod: %v", err)
	}
}

// Tests that servod stop failure is handled successfully.
func TestServodServer_StopServodFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Any(), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("not success!"))
			bErr.Write([]byte("failed!"))
			return &bOut, &bErr, errors.Reason("error message").Err()
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	op, err := srv.StopServod(ctx, &api.StopServodRequest{
		ServoHostPath: "servoHostPath",
		ServodPort:    0,
	})
	if err == nil {
		t.Fatalf("Should have failed at api.ExecCmd.")
	}

	if !strings.Contains(err.Error(), "error message") {
		t.Fatalf("Expecting Error to be \"error message\", instead got %v", err.Error())
	}

	switch op.Result.(type) {
	case *longrunning.Operation_Error:
		t.Fatalf("Failed at api.StopServod: %v", err)
	}
}

// Tests that a command executes successfully.
func TestServodServer_ExecCmdSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Eq("command arg1 arg2"), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("success!"))
			bErr.Write([]byte("not failed!"))
			return &bOut, &bErr, nil
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	resp, err := srv.ExecCmd(ctx, &api.ExecCmdRequest{
		ServoHostPath: "servoHostPath",
		Command:       "command arg1 arg2",
	})
	if err != nil {
		t.Fatalf("Failed at api.ExecCmd: %v", err)
	}

	if string(resp.Stderr) != "not failed!" {
		t.Fatalf("Expecting Stderr to be \"not failed!\", instead got %v", string(resp.Stderr))
	}

	if string(resp.Stdout) != "success!" {
		t.Fatalf("Expecting Stdout to be \"success!\", instead got %v", string(resp.Stdout))
	}

	if resp.ExitInfo.Signaled {
		t.Fatalf("ExitInfo.Signaled should not be set!")
	}

	if !resp.ExitInfo.Started {
		t.Fatalf("ExitInfo.Started should be set!")
	}

	if resp.ExitInfo.Status != 0 {
		t.Fatalf("Expecting ExitInfo.Status to be 0, instead got: %v", resp.ExitInfo.Status)
	}
}

// Tests that a command execution failure is handled gracefully.
func TestServodServer_ExecCmdFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Eq("command arg1 arg2"), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("not success!"))
			bErr.Write([]byte("failed!"))
			return &bOut, &bErr, errors.Reason("error message").Err()
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	resp, err := srv.ExecCmd(ctx, &api.ExecCmdRequest{
		ServoHostPath: "servoHostPath",
		Command:       "command arg1 arg2",
	})
	if err == nil {
		t.Fatalf("Should have failed at api.ExecCmd.")
	}

	if string(resp.Stderr) != "failed!" {
		t.Fatalf("Expecting Stderr to be \"failed!\", instead got %v", string(resp.Stderr))
	}

	if string(resp.Stdout) != "not success!" {
		t.Fatalf("Expecting Stdout to be \"not success!\", instead got %v", string(resp.Stdout))
	}

	if resp.ExitInfo.ErrorMessage != "error message" {
		t.Fatalf("Expecting ExitInfo.ErrorMessage to be \"error message\", instead got %v", resp.ExitInfo.ErrorMessage)
	}

	if resp.ExitInfo.Signaled {
		t.Fatalf("ExitInfo.Signaled should not be set!")
	}

	if resp.ExitInfo.Started {
		t.Fatalf("ExitInfo.Started should not be set!")
	}

	if resp.ExitInfo.Status == 0 {
		t.Fatalf("Expecting ExitInfo.Status to be 0, instead got: %v", resp.ExitInfo.Status)
	}
}

// Tests that a command execution with SSH exit failure is handled gracefully.
func TestServodServer_ExecCmdWithSshExitFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mce := mock_commandexecutor.NewMockCommandExecutorInterface(ctrl)

	mce.EXPECT().Run(gomock.Eq("servoHostPath"), gomock.Eq("command arg1 arg2"), gomock.Eq(nil), gomock.Eq(false)).DoAndReturn(
		func(addr string, command string, stdin io.Reader, routeToStd bool) (*bytes.Buffer, *bytes.Buffer, error) {
			var bOut, bErr bytes.Buffer
			bOut.Write([]byte("not success!"))
			bErr.Write([]byte("failed!"))
			wm := ssh.Waitmsg{}
			return &bOut, &bErr, &ssh.ExitError{
				Waitmsg: wm,
			}
		},
	)

	ctx := context.Background()
	var logBuf bytes.Buffer
	srv, destructor, err := NewServodService(ctx, log.New(&logBuf, "", log.LstdFlags|log.LUTC), mce)
	defer destructor()
	if err != nil {
		t.Fatalf("Failed to create new ServodService: %v", err)
	}

	resp, err := srv.ExecCmd(ctx, &api.ExecCmdRequest{
		ServoHostPath: "servoHostPath",
		Command:       "command arg1 arg2",
	})
	if err == nil {
		t.Fatalf("Should have failed at api.ExecCmd.")
	}

	if string(resp.Stderr) != "failed!" {
		t.Fatalf("Expecting Stderr to be \"failed!\", instead got %v", string(resp.Stderr))
	}

	if string(resp.Stdout) != "not success!" {
		t.Fatalf("Expecting Stdout to be \"not success!\", instead got %v", string(resp.Stdout))
	}

	if resp.ExitInfo.ErrorMessage != "" {
		t.Fatalf("Expecting ExitInfo.ErrorMessage to be \"\", instead got %v", resp.ExitInfo.ErrorMessage)
	}

	if !resp.ExitInfo.Signaled {
		t.Fatalf("ExitInfo.Signaled should be set!")
	}

	if !resp.ExitInfo.Started {
		t.Fatalf("ExitInfo.Started should be set!")
	}

	if resp.ExitInfo.Status != 0 {
		t.Fatalf("Expecting ExitInfo.Status to be 0, instead got: %v", resp.ExitInfo.Status)
	}
}
