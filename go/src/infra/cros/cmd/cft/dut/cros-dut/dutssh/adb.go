// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func runCommand(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	log.Printf("Running %+v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed to run %+v: %w", cmd.Args, err)
	}
	return nil
}

type ADBClient struct {
	parentCtx context.Context

	// internalCtx is closed with either the `parentCtx` has closed, or when
	// the `Close()` method is called. This signal is used to disconnect from the
	// target.
	internalCtx context.Context
	// stop is called to begin disconnecting.
	stop context.CancelCauseFunc
	// externalCtx will be closed when ADB has disconnected, and it will contain
	// any errors encountered while disconnecting.
	externalCtx context.Context
	target      string
	adbPath     string
}

// NewADBClient creates a new ADB client. `parentCtx` will be used to
// automatically close the ADB client. This is useful if `target` is provided
// by a different service.
func NewADBClient(parentCtx context.Context, target string) (*ADBClient, error) {
	adbPath := "adb"
	if path, ok := os.LookupEnv("ADB_PATH"); ok {
		adbPath = path
	}

	if err := runCommand(adbPath, "connect", target); err != nil {
		return nil, err
	}

	if err := runCommand(adbPath, "-s", target, "root"); err != nil {
		log.Printf("adb: Failed to switch to root, running user build?")
	}

	internalCtx, stop := context.WithCancelCause(parentCtx)
	externalCtx, signalStopped := context.WithCancelCause(context.Background())
	context.AfterFunc(internalCtx, func() {
		signalStopped(runCommand(adbPath, "disconnect", target))
	})

	return &ADBClient{parentCtx, internalCtx, stop, externalCtx, target, adbPath}, nil
}

// Ctx returns a context that will be closed once ADB has been disconnected.
func (s *ADBClient) Ctx() context.Context {
	return s.externalCtx
}

func (c *ADBClient) Close() error {
	c.stop(nil)

	<-c.externalCtx.Done()

	err := context.Cause(c.externalCtx)
	if errors.Is(err, context.Canceled) {
		// signalStopped was called with a nil error.
		return nil
	}
	return err
}

func (c *ADBClient) Wait() error {
	return runCommand(c.adbPath, "-s", c.target, "wait-for-disconnect")
}

func (c *ADBClient) IsAlive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.adbPath, "-s", c.target, "get-state")
	log.Printf("Running %+v", cmd.Args)

	out, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to run %+v: %s", cmd.Args, err.Error())
		return false
	}
	state := string(out)
	state = strings.TrimSpace(state)
	log.Printf("Got state '%s' for target %s", state, c.target)

	return state == "device"
}

func (c *ADBClient) NewSession(ctx context.Context) (SessionInterface, error) {
	return NewAdbSession(ctx, c)
}

type AdbSession struct {
	client *ADBClient
	cmd    *exec.Cmd
}

func NewAdbSession(ctx context.Context, client *ADBClient) (*AdbSession, error) {
	cmd := exec.CommandContext(ctx, client.adbPath, "-s", client.target, "shell")

	// We want to send a SIGTERM instead of SIGKILL so that adb can clean up
	// the remote process.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}

	return &AdbSession{
		client: client,
		cmd:    cmd,
	}, nil
}

func (s *AdbSession) SetStdout(writer io.Writer) {
	s.cmd.Stdout = writer
}

func (s *AdbSession) SetStderr(writer io.Writer) {
	s.cmd.Stderr = writer
}

func (s *AdbSession) SetStdin(reader io.Reader) {
	s.cmd.Stdin = reader
}

func (s *AdbSession) Run(script string) error {
	if err := s.Start(script); err != nil {
		return err
	}
	return s.Close()
}

func (s *AdbSession) Start(script string) error {
	if s.cmd.Process != nil {
		return fmt.Errorf("Command already executed")
	}

	s.cmd.Args = append(s.cmd.Args, script)

	log.Printf("<adb> running %+v", s.cmd.Args)
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("<adb> Failed to invoke %+v: %w", s.cmd.Args, err)
	}

	return nil
}

func (s *AdbSession) Close() error {
	if err := s.cmd.Wait(); err != nil {
		return fmt.Errorf("<adb> command %+v failed: %w", s.cmd.Args, err)
	}

	return nil
}

func (s *AdbSession) Output(script string) ([]byte, error) {
	s.cmd.Args = append(s.cmd.Args, script)

	return s.cmd.Output()
}

func (s *AdbSession) StdoutPipe() (io.Reader, error) {
	return s.cmd.StdoutPipe()
}

func (s *AdbSession) StderrPipe() (io.Reader, error) {
	return s.cmd.StderrPipe()
}
