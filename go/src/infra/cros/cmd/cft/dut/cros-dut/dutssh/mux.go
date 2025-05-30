// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"log"

	"golang.org/x/crypto/ssh"
)

// NewClientInterface checks the SSH connection to determine the type of device
// the client is connected to. Android devices run an ssh forwarder process that
// can be used to access ADB over SSH.
func NewClientInterface(ctx context.Context, identifier string, ssh *ssh.Client) (ClientInterface, error) {
	serverVersion := string(ssh.ServerVersion())
	// TODO: Remove SSH-2.0-Go once sshforwarder prebuilt has been updated.
	log.Print("SSH Server Version: ", serverVersion)
	if serverVersion == "SSH-2.0-ADB-Proxy" || serverVersion == "SSH-2.0-Go" {
		return NewADBOverSSHClient(identifier, ssh)
	}

	return &SSHClient{Client: ssh}, nil
}
