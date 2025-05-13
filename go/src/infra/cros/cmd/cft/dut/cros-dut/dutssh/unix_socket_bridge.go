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
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type unixSocketBridge struct {
	ctx        context.Context
	close      context.CancelCauseFunc
	ssh        *ssh.Client
	socketPath string
	target     string
	wg         sync.WaitGroup
}

// CreateUnixSocketServer creates a new Unix Socket using (the connname for the
// path). The socket will forward all new connections to the specified target on
// the ssh host.
func CreateUnixSocketServer(conname string, ssh *ssh.Client, target string) (*unixSocketBridge, error) {
	socketPath := fmt.Sprintf("/tmp/%s.sock", conname)

	ctx, cancel := context.WithCancelCause(context.Background())
	return &unixSocketBridge{
		ctx:        ctx,
		close:      cancel,
		ssh:        ssh,
		socketPath: socketPath,
		target:     target,
	}, nil
}

// Start listen for connections on the local unix socket.
func (s *unixSocketBridge) Start() error {
	err := os.Remove(s.socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to remove %s: %w", s.socketPath, err)
	}

	log.Printf("Starting SSH proxy for %s <-> %s <-> %s", s.socketPath, s.ssh.RemoteAddr().String(), s.target)
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("Error creating proxy on %s: %w", s.socketPath, err)
	}

	// Monitor context and shutdown server.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		select {
		case <-s.ctx.Done():
			if err := listener.Close(); err != nil {
				log.Printf("SSH Proxy: error closing listener: %s", err.Error())
			}

			if err := os.Remove(s.socketPath); err != nil {
				log.Printf("SSH Proxy: failed to cleanup socket %s: %s", s.socketPath, err.Error())
			}
		}
	}()

	// If the ssh connection closes we need to shut down the listener.
	//
	// `ssh` is using `sync.Cond` to signal when the ssh client closes. This
	// doesn't play well with `select` or `context`. We unfortunately can't cancel
	// this goroutine when our `context` is canceled, so this goroutine will leak
	// until the ssh connection is closed.
	go func() {
		var err error
		// Be extra cautious in case `Wait()` panics.
		defer func() {
			log.Printf("SSH Proxy: ssh connection was closed, shutting down.")
			s.close(fmt.Errorf("SSH proxy: ssh connection was closed by %w", err))
		}()
		err = s.ssh.Wait()
	}()

	// Main server loop.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			conn, err := listener.Accept()
			if err != nil {
				// Shut down the server on accept errors.
				s.close(fmt.Errorf("SSH Proxy: Accept error: %w", err))
				return
			}

			log.Printf("SSH Proxy: Accepted connection on %s", s.socketPath)
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.handleConnection(s.ctx, conn)
			}()
		}
	}()

	return nil
}

// Stop the proxy and all connections.
func (s *unixSocketBridge) Stop() error {
	// Shut down the server.
	s.close(nil)

	// Wait for all goroutines to complete.
	s.wg.Wait()

	return context.Cause(s.ctx)
}

// Socket returns the path to the unix socket.
func (s *unixSocketBridge) Socket() string {
	return s.socketPath
}

// dialTarget connects to the remote target and returns a new connection.
func (s *unixSocketBridge) dialTarget(ctx context.Context) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	remoteConn, err := s.ssh.DialContext(ctx, "tcp", s.target)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to %s: %w", s.target, err)
	}

	return remoteConn, nil
}

// handleConnection bridges the local unix socket connection to the remote TCP port.
func (s *unixSocketBridge) handleConnection(ctx context.Context, localConn net.Conn) {
	remoteConn, err := s.dialTarget(ctx)
	if err != nil {
		localConn.Close()
		log.Printf("%s", err.Error())
		return
	}

	var wg sync.WaitGroup

	ctx, closeConnections := context.WithCancel(ctx)

	// Monitor the context and close the connections when done.
	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
			if err := localConn.Close(); err != nil && err != io.EOF {
				log.Printf("Error closing local connection: %s", err.Error())
			}

			if err := remoteConn.Close(); err != nil && err != io.EOF {
				log.Printf("Error closing remote connection: %s", err.Error())
			}
		}
	}()

	copyIo := func(destName string, dest net.Conn, srcName string, src net.Conn) {
		defer wg.Done()
		defer closeConnections()

		// io.Copy returns a nil error when src terminates with io.EOF.
		if _, err := io.Copy(dest, src); err == nil {
			log.Printf("SSH Proxy: %s hung up", srcName)
		} else if errors.Is(err, net.ErrClosed) {
			log.Printf("SSH Proxy: %s was closed", srcName)
		} else {
			log.Printf("SSH Proxy: Reading from %s or writing to %s failed: %s", srcName, destName, err.Error())
		}
	}

	wg.Add(1)
	go copyIo("local", localConn, "remote", remoteConn)

	wg.Add(1)
	go copyIo("remote", remoteConn, "local", localConn)

	wg.Wait()
}
