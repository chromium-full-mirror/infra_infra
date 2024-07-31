// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ssh

import (
	"bytes"
	"context"
	"fmt"
	"unicode"

	"golang.org/x/crypto/ssh"

	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/internal/rand"
	"infra/cros/recovery/tlw"
)

// Run executes command on the target address by SSH.
func Run(ctx context.Context, provider SSHProvider, addr string, cmd string) (result *tlw.RunResult) {
	return run(ctx, provider, addr, cmd, false)
}

// RunBackground executes command on the target address by SSH in background.
func RunBackground(ctx context.Context, provider SSHProvider, addr string, cmd string) (result *tlw.RunResult) {
	return run(ctx, provider, addr, cmd, true)
}

// run executes commands on a remote host by SSH.
func run(ctx context.Context, provider SSHProvider, addr string, cmd string, background bool) (result *tlw.RunResult) {
	// TODO(b:267504440): Delete session key logs since they are only required for debugging a specific issue.
	sessionLogsKey := rand.String(32)
	result = &tlw.RunResult{
		Command:  cmd,
		ExitCode: -1,
	}
	errorMessage := "run SSH"
	if background {
		errorMessage = "run SSH background"
	}
	if provider == nil {
		result.Stderr = fmt.Sprintf("%s: provider is not initialized", errorMessage)
		return
	} else if addr == "" {
		result.Stderr = fmt.Sprintf("%s: addr is empty", errorMessage)
		return
	}
	// Update message to print running host.
	errorMessage = fmt.Sprintf("run SSH %q", addr)
	if background {
		errorMessage = fmt.Sprintf("run SSH %q in background", addr)
	}
	if cmd == "" {
		result.Stderr = fmt.Sprintf("%s: cmd is empty", errorMessage)
		return
	}
	log.Debugf(ctx, "Getting SSH client: %q for %q", sessionLogsKey, addr)
	sc, err := provider.Get(ctx, addr)
	if err != nil {
		result.Stderr = fmt.Sprintf("%s: fail to get client from pool; %s", errorMessage, err)
		return
	}
	defer func() {
		if err := provider.CloseClient(ctx, sc); err != nil {
			log.Debugf(ctx, "SSH client closed %q with error: %s", sessionLogsKey, err)
		}
	}()
	log.Debugf(ctx, "SSH client received: %q", sessionLogsKey)
	result = createSessionAndExecute(ctx, cmd, sc, background, sessionLogsKey)
	log.Debugf(ctx, "Run SSH %q: Cmd: %q", addr, result.Command)
	log.Debugf(ctx, "Run SSH %q: ExitCode: %d", addr, result.ExitCode)
	log.Debugf(ctx, "Run SSH %q: Stdout(%d): %s", addr, len(result.Stderr), trancateString(result.Stdout, 1000))
	log.Debugf(ctx, "Run SSH %q: Stderr(%d): %s", addr, len(result.Stderr), trancateString(result.Stderr, 1000))
	return result
}

// trancateString trancates string
func trancateString(str string, max int) string {
	if str == "" || len(str) <= max {
		return str
	}
	lastSpaceIx := -1
	for i, r := range str {
		if unicode.IsSpace(r) {
			lastSpaceIx = i
		}
		// We stop when reached max.
		if i+1 >= max {
			break
		}
	}
	// If break found then we cut by last one.
	if lastSpaceIx != -1 {
		return str[:lastSpaceIx] + "..."
	}
	// If there is no breaks then we do just cut by max.
	return str[:max]
}

// createSessionAndExecute creates ssh session and perform execution by ssh.
//
// The function also aborted execution if context canceled.
func createSessionAndExecute(ctx context.Context, cmd string, client SSHClient, background bool, sessionLogsKey string) (result *tlw.RunResult) {
	result = &tlw.RunResult{
		Command:  cmd,
		ExitCode: -1,
	}
	log.Debugf(ctx, "Started SSH session: %q", sessionLogsKey)
	session, err := client.NewSession()
	if err != nil {
		result.Stderr = fmt.Sprintf("internal run ssh: %v", err)
		return
	}
	defer func() {
		log.Debugf(ctx, "Closing SSH session: %q", sessionLogsKey)
		session.Close()
		log.Debugf(ctx, "SSH Session %q closed.", sessionLogsKey)
	}()
	var stdOut, stdErr bytes.Buffer
	session.Stdout = &stdOut
	session.Stderr = &stdErr
	exit := func(err error) *tlw.RunResult {
		result.Stdout = stdOut.String()
		result.Stderr = stdErr.String()
		switch t := err.(type) {
		case nil:
			result.ExitCode = 0
		case *ssh.ExitError:
			result.ExitCode = int32(t.ExitStatus())
		case *ssh.ExitMissingError:
			result.ExitCode = -2
			result.Stderr = t.Error()
		default:
			// Set error 1 as not expected exit.
			result.ExitCode = -3
			result.Stderr = t.Error()
		}
		return result
	}
	if background {
		// No need to run SSH in separate thread and wait for response.
		runErr := session.Start(cmd)
		return exit(runErr)
	} else {
		// Chain to run ssh in separate thread and wait for single response from it.
		// If context will be closed before it will abort the session.
		sw := make(chan bool, 1)
		var runErr error
		go func() {
			runErr = session.Run(cmd)
			sw <- true
		}()
		select {
		case <-sw:
			log.Debugf(ctx, "SSH Session %q: exiting by execution", sessionLogsKey)
			return exit(runErr)
		case <-ctx.Done():
			log.Debugf(ctx, "SSH Session %q: stopping by context", sessionLogsKey)
			// At the end abort session.
			// Session will be closed in defer.
			if err := session.Signal(ssh.SIGABRT); err != nil {
				log.Errorf(ctx, "Fail to abort context by ABORT signal: %s", err)
			}
			log.Debugf(ctx, "SSH Session %q: stopped by context", sessionLogsKey)
			return exit(ctx.Err())
		}
	}
}
