// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type ADBOverSSHClient struct {
	server *unixSocketBridge
	ssh    ClientInterface
	adb    ClientInterface
}

// NewADBOverSSHClient creates a new ADB over SSH client. The SSH host is
// expected to be the Android DUT. This relies on the sshforwarder process
// running on the DUT.
func NewADBOverSSHClient(conname string, ssh *ssh.Client) (*ADBOverSSHClient, error) {
	// ADB doesn't like having a : in the socket name.
	conname = strings.ReplaceAll(conname, ":", ".")

	// TODO: Switch from localhost:5555 to the ADB unix socket.
	server, err := CreateUnixSocketServer(conname, ssh, "localhost:5555")
	if err != nil {
		return nil, err
	}

	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("Failed to start unix socket server: %w", err)
	}

	target := fmt.Sprintf("localfilesystem:%s", server.Socket())

	adb, err := NewADBClient(target)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("ADB failed to connect to %s: %w", target, err),
			server.Stop(),
		)
	}

	return &ADBOverSSHClient{
		server: server,
		ssh:    &SSHClient{Client: ssh},
		adb:    adb,
	}, nil
}

func (c *ADBOverSSHClient) Close() error {
	var errs []error

	if c.adb != nil {
		if err := c.adb.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Failed to close ADB: %w", err))
		}
		c.adb = nil
	}

	if c.server != nil {
		if err := c.server.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("Failed to stop SSH proxy: %w", err))
		}
		c.server = nil
	}

	if c.ssh != nil {
		if err := c.ssh.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Failed to close SSH: %w", err))
		}
		c.ssh = nil
	}

	return errors.Join(errs...)
}

func (c *ADBOverSSHClient) NewSession(ctx context.Context) (SessionInterface, error) {
	return c.adb.NewSession(ctx)
}

func (c *ADBOverSSHClient) Wait() error {
	if c.adb == nil {
		return errors.New("Client already closed")
	}

	return c.adb.Wait()
}

func (c *ADBOverSSHClient) IsAlive() bool {
	if c.ssh == nil || c.adb == nil {
		return false
	}

	return c.ssh.IsAlive() && c.adb.IsAlive()
}
