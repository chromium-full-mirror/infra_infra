// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

import (
	"context"
	"fmt"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/internal/env"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

// SSHProvider provide access to SSH client manager.
//
// Provider gives option to use pool or create new client always.
type SSHProvider interface {
	Get(ctx context.Context, addr string) (SSHClient, error)
	CloseClient(ctx context.Context, c SSHClient) error
	Close() error
	Config() Config
	SetUser(username string)
	UseClientPool() bool
	Clone() SSHProvider
}

// Implementation of SSHProvider.
type sshProviderImpl struct {
	username   string // overrides SSH config username
	config     Config
	clientPool map[string]SSHClient
}

// NewProvider creates new provider for use.
//
//	clientConfig: SSH configuration to configure the new clients.
//	tlsConfig: Optional TLS configuration to establish SSH connections over TLS channel.
func NewProvider(config Config) SSHProvider {
	p := &sshProviderImpl{
		config: config,
	}
	if env.IsCloudBot() {
		p.clientPool = make(map[string]SSHClient)
	}
	return p
}

// UseClientPool return bool for reuse client or not.
func (c *sshProviderImpl) UseClientPool() bool {
	return c.clientPool != nil
}

// Get provides SSH client for requested host.
func (c *sshProviderImpl) Get(ctx context.Context, addr string) (SSHClient, error) {
	if !c.UseClientPool() {
		return NewClient(ctx, addr, c.username, c.config)
	}
	var err error
	if cl, ok := c.clientPool[addr]; !ok {
		log.Debugf(ctx, "Creating a new client for %q", addr)
		cl, err = NewClient(ctx, addr, c.username, c.config)
		if err == nil {
			c.clientPool[addr] = cl
		}
	} else {
		if !cl.IsAlive() {
			log.Debugf(ctx, "client %q not alive, creating a new one", addr)
			delete(c.clientPool, addr)
			if err = cl.Close(); err != nil {
				log.Debugf(ctx, "error closing dead %q client error: %v", addr, err)
			}
			cl, err = NewClient(ctx, addr, c.username, c.config)
			if err == nil {
				c.clientPool[addr] = cl
			}
		}
	}
	return c.clientPool[addr], errors.Annotate(err, "provider get client error:").Err()
}

// CloseClient closes SSH client if it is not using client pool.
func (c *sshProviderImpl) CloseClient(ctx context.Context, cl SSHClient) error {
	if c.UseClientPool() {
		log.Debugf(ctx, "provider uses client pool: connection remains open")
		return nil
	}
	return cl.Close()
}

// Close closing used resource of the provider.
func (c *sshProviderImpl) Close() error {
	for k, v := range c.clientPool {
		if err := v.Close(); err != nil {
			fmt.Printf("provider error closing %q client: %v\n", k, err)
		}
	}
	c.clientPool = nil
	return nil
}

// Config returns SSH provider configuration.
func (c *sshProviderImpl) Config() Config {
	return c.config
}

// SetUser sets an username to override SSH config user.
func (c *sshProviderImpl) SetUser(username string) {
	c.username = username
}

// Clone creates a new SSH provider with its own username property.
// Changes to the username in the clone won't affect the original provider.
func (c *sshProviderImpl) Clone() SSHProvider {
	p := &sshProviderImpl{
		username: c.username,
		config:   c.config,
	}
	if c.clientPool != nil {
		p.clientPool = make(map[string]SSHClient)
	}
	return p
}
