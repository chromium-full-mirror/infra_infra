// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package connector

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	cssh "golang.org/x/crypto/ssh"

	"go.chromium.org/infra/cros/satlab/satlabrpcserver/fake"
	"go.chromium.org/infra/cros/satlab/satlabrpcserver/utils/constants"
)

func TestSSHConnectionShouldWork(t *testing.T) {
	expectedResponse := "connect success"
	server, err := fake.NewFakeServer(func(session ssh.Session) {
		_, err := io.WriteString(session, expectedResponse)
		if err != nil {
			log.Printf("Can't write the response to ssh client")
			return
		}
	})

	if err != nil {
		t.Errorf("Can't create a fake ssh server")
		return
	}

	go func() {
		err := server.Serve()
		// issue: https://github.com/golang/go/issues/43722
		time.Sleep(time.Second * 5)
		if err != nil {
			// If we start server, and we get an error,
			// we don't need to test other things.
			t.Errorf("Can't listen the addr: %v", server.GetAddr())
			return
		}
	}()

	t.Cleanup(func() {
		if server != nil {
			err := server.Close()
			// We can't do anything here
			// when closing the fake ssh server error.
			// Instead, we can log the error message.
			if err != nil {
				log.Printf("Can't close the fake server")
				return
			}
		}
	})

	// Wait for ssh server bring up
	time.Sleep(time.Second * 2)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	config := cssh.ClientConfig{
		User: "fake_user",
		Auth: []cssh.AuthMethod{
			cssh.Password(fake.Password),
		},
		HostKeyCallback: cssh.InsecureIgnoreHostKey(),
		Timeout:         constants.SSHConnectionTimeout,
	}

	sshConnector := New(0, time.Second)

	_, err = sshConnector.Connect(ctx, server.GetAddr(), &config)
	if err != nil {
		t.Errorf("Can't establish ssh connection")
	}
}

func TestSSHConnectionShouldFailWhenReachContextTimeout(t *testing.T) {
	expectedResponse := "connect success"
	server, err := fake.NewFakeServer(func(session ssh.Session) {
		_, err := io.WriteString(session, expectedResponse)
		if err != nil {
			log.Printf("Can't write the response to ssh client")
			return
		}
	})

	if err != nil {
		t.Errorf("Can't create a fake ssh server")
		return
	}

	go func() {
		err := server.Serve()
		// issue: https://github.com/golang/go/issues/43722
		time.Sleep(time.Second * 5)
		if err != nil {
			// If we start server, and we get an error,
			// we don't need to test other things.
			t.Errorf("Can't listen the addr: %v", server.GetAddr())
			return
		}
	}()

	t.Cleanup(func() {
		if server != nil {
			err := server.Close()
			// We can't do anything here
			// when closing the fake ssh server error.
			// Instead, we can log the error message.
			if err != nil {
				log.Printf("Can't close the fake server")
				return
			}
		}
	})

	// Wait for ssh server bring up
	time.Sleep(time.Second * 2)

	// We use expired context for testing the context deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-500*time.Millisecond))
	defer cancel()

	config := cssh.ClientConfig{
		User: "fake_user",
		Auth: []cssh.AuthMethod{
			cssh.Password(fake.Password),
		},
		HostKeyCallback: cssh.InsecureIgnoreHostKey(),
		Timeout:         constants.SSHConnectionTimeout,
	}

	sshConnector := New(0, time.Second)
	_, err = sshConnector.Connect(ctx, server.GetAddr(), &config)
	if err == nil {
		t.Errorf("Should reach the context timeout")
	}
}
