// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
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
	target  string
	adbPath string
}

func NewADBClient(target string) (*ADBClient, error) {
	adbPath := "adb"
	if path, ok := os.LookupEnv("ADB_PATH"); ok {
		adbPath = path
	}

	// Close any existing connections since they might be stale.
	// We ignore the return code since the connection might not exist.
	runCommand(adbPath, "disconnect", target)

	if err := runCommand(adbPath, "connect", target); err != nil {
		return nil, err
	}

	if err := runCommand(adbPath, "-s", target, "root"); err != nil {
		log.Printf("adb: Failed to switch to root, running user build?")
	}

	return &ADBClient{target, adbPath}, nil
}

func (c *ADBClient) Close() error {
	return runCommand(c.adbPath, "disconnect", c.target)
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

func (c *ADBClient) NewSession() (SessionInterface, error) {
	return NewAdbSession(c)
}

type AdbSession struct {
	client *ADBClient
	cmd    *exec.Cmd
}

func NewAdbSession(client *ADBClient) (*AdbSession, error) {
	cmd := exec.Command(client.adbPath, "-s", client.target, "shell")

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
