// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Files here concern a service adapter to communicate with CrosDUT

package common_utils

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"go.chromium.org/chromiumos/test/util/common"

	"github.com/pkg/errors"
	"go.chromium.org/chromiumos/config/go/longrunning"
	"go.chromium.org/chromiumos/config/go/test/api"
)

const (
	defaulTimeout = 5 * time.Minute
)

// ServiceAdapters are used to interface with a DUT
// All methods here are proxies to cros-dut (with some additions for simplicity)
type ServiceAdapterInterface interface {
	// RunCmd takes a command and argument and executes it remotely in the DUT,
	// returning the stdout as the string result and any execution error as the error.
	// The args are not quoted in any way, so beware of spaces and shell variables.
	RunCmd(ctx context.Context, cmd string, args []string) (string, error)
	// Restart restarts a DUT (allowing cros-dut to reconnect for connection caching).
	Restart(ctx context.Context) error
	// PathExists is a simple wrapper for RunCmd for the sake of simplicity. If
	// the path exists True is returned, else False. An error implies a
	// a communication failure.
	PathExists(ctx context.Context, path string) (bool, error)
	// PipeData uses the caching infrastructure to bring an image into the lab.
	// Contrary to CopyData, the data here is pipeable to whatever is fed into
	// pipeCommand, rather than directly placed locally.
	PipeData(ctx context.Context, sourceUrl string, pipeCommand string) error
	// CopyData uses the caching infrastructure to copy a remote image to
	// the local path specified by destPath.
	CopyData(ctx context.Context, sourceUrl string, destPath string) error
	// DeleteDirectory is a simple wrapper for RunCmd for the sake of simplicity.
	DeleteDirectory(ctx context.Context, dir string) error
	// CreateDirectory is a simple wrapper for RunCmd for the sake of simplicity.
	// All directories specified in the array will be created.
	// As this uses "-p" option, subdirs are created regardless of whether parents
	// exist or not.
	CreateDirectories(ctx context.Context, dirs []string) error
	// FetchFile downloads a file from the DUT.
	FetchFile(ctx context.Context, path string) (io.ReadCloser, error)
	// ForceReconnectWithBackoff waits for the DUT to come back up/reconnect.
	ForceReconnectWithBackoff(ctx context.Context) error
}

type execCmdResult struct {
	response string
	err      error
}

type ServiceAdapter struct {
	dutClient api.DutServiceClient
	noReboot  bool
}

func NewServiceAdapter(dutClient api.DutServiceClient, noReboot bool) ServiceAdapter {
	return ServiceAdapter{
		dutClient: dutClient,
		noReboot:  noReboot,
	}
}

// RunCmd runs a command in a remote DUT
func (s ServiceAdapter) RunCmd(ctx context.Context, cmd string, args []string) (string, error) {
	log.Printf("<cros-provision> Run cmd: %s, %s\n", cmd, args)
	var timeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	} else {
		timeout = defaulTimeout
	}

	// Channel used to receive the result from ExecCommand function.
	ch := make(chan execCmdResult, 1)

	// Create a context with the specified timeout.
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start the execCmd function.
	go s.execCmd(ctxTimeout, cmd, args, ch)

	select {
	case <-ctxTimeout.Done():
		return "", fmt.Errorf("<cros-provision> Timeout %s reached", timeout)
	case result := <-ch:
		return result.response, result.err
	}
}

func (s ServiceAdapter) execCmd(ctx context.Context, cmd string, args []string, ch chan execCmdResult) {
	req := api.ExecCommandRequest{
		Command: cmd,
		Args:    args,
		Stdout:  api.Output_OUTPUT_PIPE,
		Stderr:  api.Output_OUTPUT_PIPE,
	}
	stream, err := s.dutClient.ExecCommand(ctx, &req)
	if err != nil {
		log.Printf("<cros-provision> Run cmd FAILED: %s\n", err)
		ch <- execCmdResult{response: "", err: fmt.Errorf("execution fail: %w", err)}
		return
	}
	// Expecting single stream result
	execCmdResponse, err := stream.Recv()
	if err != nil {
		ch <- execCmdResult{response: "", err: fmt.Errorf("execution single stream result: %w", err)}
		return
	}
	if execCmdResponse.ExitInfo.Status != 0 {
		err = fmt.Errorf("status: %v message: %v STDOUT: %v STDERR :%v", execCmdResponse.ExitInfo.Status, execCmdResponse.ExitInfo.ErrorMessage, execCmdResponse.Stdout, execCmdResponse.Stderr)
	}
	if string(execCmdResponse.Stderr) != "" {
		log.Printf("<cros-provision> execution finished with stderr: %s\n", string(execCmdResponse.Stderr))
	}
	ch <- execCmdResult{response: string(execCmdResponse.Stdout), err: err}
}

// FetchFile downloads a file from the DUT.
func (s ServiceAdapter) FetchFile(ctx context.Context, path string) (io.ReadCloser, error) {
	log.Printf("Running FetchFile cmd: %s\n", path)
	filename, err := common.GetFile(ctx, path, s.dutClient)
	if err != nil {
		return nil, errors.Wrap(err, "fetch file failed")
	}

	log.Printf("Local filename: %s\n", filename)
	return os.Open(filename)
}

// Restart restarts a DUT
func (s ServiceAdapter) Restart(ctx context.Context) error {
	if s.noReboot {
		return nil
	}
	log.Printf("<cros-provision>: ServiceAdaptor: Restart Start")

	req := api.RestartRequest{
		Args: []string{},
		Retry: &api.RestartRequest_ReconnectRetry{
			Times:      50,
			IntervalMs: 10000,
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 500*time.Second)
	defer cancel()

	op, err := s.dutClient.Restart(ctx, &req)
	if err != nil {
		log.Printf("<cros-provision>: ServiceAdaptor: Restart ERROR %s", err)
		return err
	}

	for !op.Done {
		log.Printf("<cros-provision>: ServiceAdaptor: Restart !op.Done")
		time.Sleep(1 * time.Second)
	}

	switch x := op.Result.(type) {
	case *longrunning.Operation_Error:
		log.Printf("<cros-provision>: ServiceAdaptor: Restart LRO ERROR %s", x.Error.Message)
		return fmt.Errorf(x.Error.Message)
	case *longrunning.Operation_Response:
		log.Printf("<cros-provision>: ServiceAdaptor: Restart LRO RESPONSE")

		return nil
	}

	log.Printf("<cros-provision>: ServiceAdaptor: Restart Success")

	return nil
}

// PathExists determines if a path exists in a DUT
func (s ServiceAdapter) PathExists(ctx context.Context, path string) (bool, error) {
	exists, err := s.RunCmd(ctx, "", []string{"[", "-e", path, "]", "&&", "echo", "-n", "1", "||", "echo", "-n", "0"})
	if err != nil {
		return false, fmt.Errorf("path exists: failed to check if %s exists, %s", path, err)
	}
	return exists == "1", nil
}

// PipeData uses the caching infrastructure to bring a file locally,
// allowing a user to pipe the result to any desired application.
func (s ServiceAdapter) PipeData(ctx context.Context, sourceUrl string, pipeCommand string) error {
	log.Printf("Piping %s with command %s\n", sourceUrl, pipeCommand)

	req := api.CacheRequest{
		Source: &api.CacheRequest_GsFile{
			GsFile: &api.CacheRequest_GSFile{
				SourcePath: sourceUrl,
			},
		},
		Destination: &api.CacheRequest_Pipe_{
			Pipe: &api.CacheRequest_Pipe{
				Commands: pipeCommand,
			},
		},
		Retry: &api.CacheRequest_Retry{
			Times:      3,
			IntervalMs: 5000,
		},
	}

	op, err := s.dutClient.Cache(ctx, &req)
	if err != nil {
		return fmt.Errorf("execution failure: %v", err)
	}

	for !op.Done {
		time.Sleep(1 * time.Second)
	}

	switch x := op.Result.(type) {
	case *longrunning.Operation_Error:
		return fmt.Errorf(x.Error.Message)
	case *longrunning.Operation_Response:
		return nil
	}

	return nil
}

// CopyData caches a file for a DUT locally from a GS url.
func (s ServiceAdapter) CopyData(ctx context.Context, sourceUrl string, destPath string) error {
	log.Printf("Copy data from: %s, to: %s\n", sourceUrl, destPath)

	req := api.CacheRequest{
		Source: &api.CacheRequest_GsFile{
			GsFile: &api.CacheRequest_GSFile{
				SourcePath: sourceUrl,
			},
		},
		Destination: &api.CacheRequest_File{
			File: &api.CacheRequest_LocalFile{
				Path: destPath,
			},
		},
		Retry: &api.CacheRequest_Retry{
			Times:      3,
			IntervalMs: 5000,
		},
	}

	op, err := s.dutClient.Cache(ctx, &req)
	if err != nil {
		return fmt.Errorf("execution failure: %v", err)
	}

	for !op.Done {
		time.Sleep(1 * time.Second)
	}

	switch x := op.Result.(type) {
	case *longrunning.Operation_Error:
		return fmt.Errorf(x.Error.Message)
	case *longrunning.Operation_Response:
		return nil
	}

	return nil
}

// DeleteDirectory is a thin wrapper around an rm command. Done here as it is
// expected to be reused often by many services.
func (s ServiceAdapter) DeleteDirectory(ctx context.Context, dir string) error {
	if _, err := s.RunCmd(ctx, "rm", []string{"-rf", fmt.Sprintf("'%s'", dir)}); err != nil {
		return fmt.Errorf("could not delete directory, %w", err)
	}
	return nil
}

// Create directories is a thin wrapper around an mkdir command. Done here as it
// is expected to be reused often by many services.
func (s ServiceAdapter) CreateDirectories(ctx context.Context, dirs []string) error {
	if _, err := s.RunCmd(ctx, "mkdir", append([]string{"-p"}, dirs...)); err != nil {
		return fmt.Errorf("could not create directory, %w", err)
	}
	return nil
}

// ForceReconnectWithBackoff is a thin wrapper around DUT service's ForceReconnectWithBackoff.
func (s ServiceAdapter) ForceReconnectWithBackoff(ctx context.Context) error {
	log.Printf("<cros-provision>: ServiceAdaptor: ForceReconnectWithBackoff Start")

	ctx, cancel := context.WithTimeout(ctx, 500*time.Second)
	defer cancel()

	expDelay := 1
loop:
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("could not force reconnect with backoff")
		default:
			f := func() error {
				op, err := s.dutClient.ForceReconnect(ctx, &api.ForceReconnectRequest{})
				if err != nil {
					log.Printf("<cros-provision>: ServiceAdaptor: ForceReconnectWithBackoff ERROR %s", err)
					return err
				}

				for !op.Done {
					log.Printf("<cros-provision>: ServiceAdaptor: ForceReconnectWithBackoff !op.Done")
					time.Sleep(1 * time.Second)
				}

				switch x := op.Result.(type) {
				case *longrunning.Operation_Error:
					log.Printf("<cros-provision>: ServiceAdaptor: ForceReconnectWithBackoff ignoring LRO ERROR %s", x.Error.Message)
					return fmt.Errorf(x.Error.Message)
				case *longrunning.Operation_Response:
					log.Printf("<cros-provision>: ServiceAdaptor: ForceReconnectWithBackoff LRO RESPONSE")
					return nil
				default:
					return fmt.Errorf("unknown longrunniong operation")
				}
			}
			if err := f(); err == nil {
				// Break out the loop now.
				break loop
			}
		}
		delay := expDelay
		if delay < 16 {
			expDelay = expDelay * 2
		}
		time.Sleep(time.Duration(delay) * time.Second)
	}

	log.Printf("<cros-provision>: ServiceAdaptor: ForceReconnectWithBackoff Success")

	return nil
}
