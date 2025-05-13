// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"errors"
	"io"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	*ssh.Client
}

func (c *SSHClient) Close() error {
	if c.Client == nil {
		return nil
	}
	return c.Client.Close()
}

func (c *SSHClient) NewSession(ctx context.Context) (SessionInterface, error) {
	if c.Client == nil {
		return nil, errors.New("SSH not connected")
	}
	session, err := c.Client.NewSession()
	if err != nil {
		return nil, err
	}

	context.AfterFunc(ctx, func() {
		session.Close()
	})
	return &SSHSession{Session: session}, nil
}

func (c *SSHClient) Wait() error {
	if c.Client == nil {
		return errors.New("SSH not connected")
	}
	return c.Client.Wait()
}

func (c *SSHClient) IsAlive() bool {
	if c.Client == nil {
		return false
	}
	_, _, err := c.Client.SendRequest("keepalive@openssh.org", true, nil)
	return err == nil
}

type SSHSession struct {
	*ssh.Session
}

func (s *SSHSession) Close() error {
	return s.Session.Close()
}
func (s *SSHSession) SetStdout(writer io.Writer) {
	s.Session.Stdout = writer
}

func (s *SSHSession) SetStderr(writer io.Writer) {
	s.Session.Stderr = writer
}

func (s *SSHSession) SetStdin(reader io.Reader) {
	s.Session.Stdin = reader
}

func (s *SSHSession) Run(cmd string) error {
	return s.Session.Run(cmd)
}

func (s *SSHSession) Start(cmd string) error {
	return s.Session.Start(cmd)
}

func (s *SSHSession) Output(cmd string) ([]byte, error) {
	return s.Session.Output(cmd)
}

func (s *SSHSession) StdoutPipe() (io.Reader, error) {
	return s.Session.StdoutPipe()
}

func (s *SSHSession) StderrPipe() (io.Reader, error) {
	return s.Session.StderrPipe()
}
