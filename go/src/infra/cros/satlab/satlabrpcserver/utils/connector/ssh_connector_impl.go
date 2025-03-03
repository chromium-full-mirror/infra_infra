// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package connector

import (
	"context"
	"log"
	"time"

	"golang.org/x/crypto/ssh"

	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/constants"
)

type SSHConnector struct {
	retry      int
	retryDelay time.Duration
}

func New(retry int, retryDelay time.Duration) *SSHConnector {
	if retry < 0 {
		retry = 0
	}

	return &SSHConnector{
		retry:      retry,
		retryDelay: retryDelay,
	}
}

func (s *SSHConnector) Connect(ctx context.Context, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	// use the timeout from constant value because we don't want to wait forever.
	timeout := constants.SSHConnectionTimeout
	// Check if the timeout was set and use it.
	if config.Timeout.Abs().Microseconds() > 0 {
		timeout = config.Timeout
	}
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	clientCh := make(chan *ssh.Client, 1)
	done := make(chan struct{}, 1)
	for range s.retry + 1 {
		go func() {
			client, err := ssh.Dial("tcp", addr, config)
			if err != nil {
				log.Printf("Can't create a ssh client %v", err)
				return
			}

			// As the ssh.Dial is a blocking operation
			// If the context is done or some channel has
			// already returned the client. It shouldn't
			// send the client back again. as the client
			// channel is closed.
			select {
			case <-ctx.Done():
				err := client.Close()
				if err != nil {
					// we can't do anything here. log the err message
					log.Printf("Can't close the ssh connection %v", err)
				}
				return
			case <-done:
				err := client.Close()
				if err != nil {
					// we can't do anything here. log the err message
					log.Printf("Can't close the ssh connection %v", err)
				}
				return
			default:
				// Fire the done event to all goroutines.
				// Let other channel doesn't try to send the
				// back the client as the channel is closed.
				done <- struct{}{}
				clientCh <- client
			}
		}()

		// Create a time ticker
		tick := time.NewTicker(s.retryDelay)
		select {
		case <-tick.C:
			// if the delay is reached, it should start the other connection and try again.
			continue
		case <-ctx.Done():
			// if we reach the context deadline. We should break the loop
			return nil, ctx.Err()
		case cl := <-clientCh:
			// if we receive the client, it means the connection should be established success.
			return cl, nil
		}
	}

	// Do the final check. If we can't get the client back, or reach
	// the context deadline. It should reach the max retry.
	for {
		time.Sleep(time.Second)
		select {
		case cl := <-clientCh:
			return cl, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ctxWithTimeout.Done():
			return nil, utils.ReachMaxRetry
		}

	}
}
