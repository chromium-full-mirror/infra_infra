// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"go.chromium.org/infra/cros/internal/env"
)

// ConnectionManager will maintain a single open connection.
type ConnectionManager interface {
	// GetConnection will return an open connection. The same connection will be
	// returned as long as it's still open. If the connection is broken, a new one
	// will be established. There will only be one connection open at a time.
	// It is safe to call this method from multiple goroutines.
	//
	// `ctx` is used to terminate the connection attempt.
	GetConnection(ctx context.Context) (ClientInterface, error)

	// ForceReconnect will close the connection and attempt to reopen a new one.
	//
	// `ctx` is used to terminate the reconnect attempt.
	ForceReconnect(ctx context.Context) (ClientInterface, error)

	// Close the current connection if it exists.
	Close() error
}

// DUTConnectionManager manages a single connection to a specific DUT.
type DUTConnectionManager struct {
	logger          *log.Logger
	connectionOrNil ClientInterface
	// connectionMutex must be held when creating a new connection.
	// This prevents multiple connections from being created simultaneously.
	connectionMutex sync.Mutex

	dutName       string
	wiringAddress string
}

// NewDUTConnectionManager creates a new `DUTConnectionManager`.
func NewDUTConnectionManager(logger *log.Logger, dutName, wiringAddress string) *DUTConnectionManager {
	s := &DUTConnectionManager{
		logger:        logger,
		dutName:       dutName,
		wiringAddress: wiringAddress,
	}
	return s
}

func (s *DUTConnectionManager) GetConnection(ctx context.Context) (ClientInterface, error) {
	s.logger.Printf("Checking Connection")

	// This is the fast path. We intentionally don't hold the mutex as to not
	// serialize all the IsAlive checks.
	//
	// Save the pointer to make sure we don't hit a TOCTOU issue.
	conn := s.connectionOrNil
	if conn != nil && conn.IsAlive() {
		return conn, nil
	}

	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()

	// Check the connection again after acquiring the lock since the connection
	// could have been restored while we were blocked waiting for the lock.
	if s.connectionOrNil != nil && s.connectionOrNil.IsAlive() {
		return s.connectionOrNil, nil
	}

	if err := s.reconnect(ctx); err != nil {
		return nil, err
	}

	return s.connectionOrNil, nil
}

// ForceReconnect attempts to reconnect to the DUT.
func (s *DUTConnectionManager) ForceReconnect(ctx context.Context) (ClientInterface, error) {
	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()

	err := s.reconnect(ctx)

	return s.connectionOrNil, err
}

func (s *DUTConnectionManager) Close() error {
	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()

	if s.connectionOrNil == nil {
		return nil
	}

	err := s.connectionOrNil.Close()
	s.connectionOrNil = nil
	return err
}

// reconnect starts a new ssh client connection.
// Callers of this method must hold the connectionMutex to ensure that we don't
// create multiple connections.
func (s *DUTConnectionManager) reconnect(ctx context.Context) error {
	s.logger.Printf("attempting to reconnect to DUT.")

	if s.connectionOrNil != nil {
		if err := s.connectionOrNil.Close(); err != nil {
			s.logger.Printf("Closing previous connection failed: %s", err.Error())
		}
		s.connectionOrNil = nil
	}

	conn, err := openConnection(ctx, s.dutName, s.wiringAddress, s.logger)
	if err != nil {
		s.logger.Printf("Failed to reconnect to DUT.")
		return err
	}
	s.connectionOrNil = conn
	return nil
}

// openConnection connects to a dut server. If wiringAddress is provided,
// it resolves the dut name to ip address; otherwise, uses dutIdentifier as is.
func openConnection(ctx context.Context, dutIdentifier string, wiringAddress string, logger *log.Logger) (ClientInterface, error) {
	logger.Printf("openConnection: Start!")
	if env.IsCloudBot() {
		logger.Printf("CloudBot detected. Will connecting to dut through proxy.")
		if ssh, err := CloudbotsDutProxyClient(ctx, dutIdentifier); err == nil {
			return NewClientInterface(ctx, dutIdentifier, ssh)
		} else {
			return nil, err
		}
	}
	var addr string
	logger.Printf("openConnection: wiringAddress: %s", wiringAddress)

	if wiringAddress != "" {
		var err error
		logger.Printf("openConnection: Calling GetSSHADDR!")

		addr, err = GetSSHAddr(ctx, dutIdentifier, wiringAddress)
		if err != nil {
			logger.Printf("openConnection: FAILED GetSSHADDR!")

			return nil, err
		}
	} else {
		logger.Printf("openConnection: dutIdentifier: %s", dutIdentifier)

		addr = dutIdentifier
	}
	logger.Printf("openConnection: Attempting to Dial!")
	ssh, err := connectWithTimeout(addr, GetSSHConfig(), 5*time.Second)
	if err != nil {
		logger.Printf("openConnection: FAILED Dial! %s\n", err)
		return nil, err
	}

	logger.Printf("openConnection: FINISHED Dial! %s\n", err)
	return NewClientInterface(ctx, dutIdentifier, ssh)
}

func connectWithTimeout(addr string, config *ssh.ClientConfig, timeout time.Duration) (*ssh.Client, error) {
	result := make(chan *ssh.Client)
	errChan := make(chan error)

	go func() {
		client, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			errChan <- err
		} else {
			result <- client
		}
	}()

	select {
	case client := <-result:
		return client, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("SSH timed out")
	}
}
