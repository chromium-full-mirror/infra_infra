// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build windows
// +build windows

// Package testexec is a wrapper of the standard os/exec package optimized for
// use cases of Tast. Tast tests should always use this package instead of
// os/exec.
//
// This package is designed to be a drop-in replacement of os/exec. Just
// rewriting imports should work. In addition, several methods are available,
// such as Kill and DumpLog.
//
// This package is target for unix application. Therefore, this window
// implementation will not do anything.
package testexec

import (
	"bytes"
	"context"
	"os/exec"
	"sync"

	"go.chromium.org/infra/cros/servo/errors"
	tastexec "go.chromium.org/infra/cros/servo/exec"
)

// Cmd represents an external command being prepared or run.
//
// This struct embeds Cmd in os/exec.
//
// Callers may wish to use shutil.EscapeSlice when logging Args.
type Cmd struct {
	// Cmd is the underlying exec.Cmd object.
	*exec.Cmd
	// log is the buffer uncaptured stdout/stderr is sent to by default.
	log bytes.Buffer
	// ctx is the context given to Command that specifies the timeout of the external command.
	ctx context.Context
	// timedOut indicates if the process hit timeout. This is set in Wait().
	timedOut bool
	// watchdogStop is notified in Wait to ask the watchdog goroutine to stop.
	watchdogStop chan bool

	// doneFlg is set to 1, when the process is terminated but before collecting
	// the process. This can be accessed from various goroutines concurrently,
	// so it should be read/written through done() and setDone().
	doneFlg uint32

	// sigMu is the mutex lock to guard from sending signals to processes
	// which is already collected.
	sigMu sync.RWMutex
}

// RunOption is enum of options which can be passed to Run, Output,
// CombinedOutput and Wait to control precise behavior of them.
type RunOption = tastexec.RunOption

// DumpLogOnError is an option to dump logs if the executed command fails
// (i.e., exited with non-zero status code).
const DumpLogOnError = tastexec.DumpLogOnError

var (
	// ErrNotStarted is the error returned when the process has not yet started.
	ErrNotStarted = errors.New("Start was not yet called")
	// ErrAlreadyWaited is the error returned when the process has been waited
	// and also returned.
	// Please see https://pkg.go.dev/os/exec#Cmd.ProcessState.
	ErrAlreadyWaited = errors.New("Wait was already called and the process has returned")
)

// CommandContext prepares to run an external command.
//
// Timeout set in ctx is honored on running the command.
//
// See os/exec package for details.
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	return nil
}

// Run runs an external command and waits for its completion.
//
// See os/exec package for details.
func (c *Cmd) Run(opts ...RunOption) error {
	if err := c.Start(); err != nil {
		return err
	}

	err := c.Wait(opts...)
	return err
}

// Output runs an external command, waits for its completion and returns
// stdout output of the command.
//
// See os/exec package for details.
func (c *Cmd) Output(opts ...RunOption) ([]byte, error) {
	return nil, errors.New("the function Output is not supported in window")
}

// CombinedOutput runs an external command, waits for its completion and
// returns stdout/stderr output of the command.
//
// See os/exec package for details.
func (c *Cmd) CombinedOutput(opts ...RunOption) ([]byte, error) {
	return nil, errors.New("the function CombinedOutput is not supported in window")
}

// SeparatedOutput runs an external command, waits for its completion and
// returns stdout/stderr output of the command separately.
func (c *Cmd) SeparatedOutput(opts ...RunOption) (stdout, stderr []byte, err error) {
	return nil, nil, errors.New("the function SeparatedOutput is not supported in window")
}

// Start starts an external command.
//
// See os/exec package for details.
func (c *Cmd) Start() error {
	return errors.New("the function Start is not supported in window")
}

// Wait waits for the process to finish and releases all associated resources.
//
// See os/exec package for details.
func (c *Cmd) Wait(opts ...RunOption) error {
	return errors.New("the function Wait is not supported in window")
}

// Kill sends SIGKILL to the process tree.
//
// This is a new method that does not exist in os/exec.
//
// Even after successful completion of this function, you still need to call
// Wait to release all associated resources.
func (c *Cmd) Kill() error {
	return errors.New("the function Kill is not supported in window")
}
