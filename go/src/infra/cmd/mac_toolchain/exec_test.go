// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

// MockSession mocks a series of command invocations.
type MockSession struct {
	Calls        []*MockCmd // Records command invocations
	ReturnError  []error    // Errors to return by commands (default: nil)
	ReturnOutput []string   // Stdout to return by commands (default: "")
}

// MockCmd mocks a single command invocation.
type MockCmd struct {
	Stdin      io.Reader // Saved Stdin to be read by Run()
	Executable string    // The name of the command
	Args       []string  // Command line arguments of the command
	Env        []string  // Additional environment variables VAR=value.

	ReturnError   error  // Error to be returned by the invocation
	ReturnOutput  string // Stdout to be return by the invocation
	ConsumedStdin string // Result of reading Stdin (for reading in tests)
}

var _ Cmd = &MockCmd{}

// SetStdin implements Cmd.
func (c *MockCmd) SetStdin(r io.Reader) {
	c.Stdin = r
}

// SetStdout implements Cmd.
func (c *MockCmd) SetStdout(f *os.File) {}

// SetStderr implements Cmd.
func (c *MockCmd) SetStderr(f *os.File) {}

// SetEnvVar implements Cmd.
func (c *MockCmd) SetEnvVar(variable, value string) {
	c.Env = append(c.Env, variable+"="+value)
}

// CommandContext implements Session.
func (s *MockSession) CommandContext(_ context.Context, executable string, args ...string) Cmd {
	c := &MockCmd{Executable: executable, Args: args}
	if len(s.ReturnError) > len(s.Calls) {
		c.ReturnError = s.ReturnError[len(s.Calls)]
	}
	if len(s.ReturnOutput) > len(s.Calls) {
		c.ReturnOutput = s.ReturnOutput[len(s.Calls)]
	}
	s.Calls = append(s.Calls, c)
	return c
}

func (c *MockCmd) Run() error {
	if c.Stdin != nil {
		data, err := ioutil.ReadAll(c.Stdin)
		if err != nil {
			return err
		}
		c.ConsumedStdin = string(data)
	}
	return c.ReturnError
}

func (c *MockCmd) Output() ([]byte, error) {
	if err := c.Run(); err != nil {
		return nil, err
	}
	return []byte(c.ReturnOutput), nil
}

func useMockCmd(ctx context.Context, s *MockSession) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func TestExec(t *testing.T) {
	t.Parallel()

	ftt.Run("External command execution works", t, func(t *ftt.Test) {
		var s MockSession
		ctx := useMockCmd(context.Background(), &s)

		t.Run("CommandContext uses RealCmd by default", func(t *ftt.Test) {
			_, ok := CommandContext(context.Background(), "test-exe", "arg1", "arg2").(*RealCmd)
			assert.Loosely(t, ok, should.BeTrue)
		})

		t.Run("CommandContext works", func(t *ftt.Test) {
			cmd, ok := CommandContext(ctx, "test-exe", "arg1", "arg2").(*MockCmd)
			assert.Loosely(t, ok, should.BeTrue)
			assert.Loosely(t, cmd.Executable, should.Equal("test-exe"))
			assert.Loosely(t, cmd.Args, should.Resemble([]string{"arg1", "arg2"}))
		})

		t.Run("RunCommand works for succeeding command", func(t *ftt.Test) {
			err := RunCommand(ctx, "test-run", "arg")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("test-run"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{"arg"}))
		})

		t.Run("RunCommand works for failing command", func(t *ftt.Test) {
			s.ReturnError = []error{errors.Reason("test error").Err()}
			err := RunCommand(ctx, "bad-cmd")
			assert.Loosely(t, err, should.NotBeNil)
		})

		t.Run("RunWithStdin works", func(t *ftt.Test) {
			err := RunWithStdin(ctx, "test input", "test-exec")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, s.Calls[0].ConsumedStdin, should.Equal("test input"))
			assert.Loosely(t, s.Calls[0].Executable, should.Equal("test-exec"))
			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string(nil)))
		})

		t.Run("RunOutput works", func(t *ftt.Test) {
			s.ReturnOutput = []string{"test output 1", "out2"}
			out, err := RunOutput(ctx, "cmd1", "b1", "b2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, out, should.Equal("test output 1"))

			out, err = RunOutput(ctx, "cmd2", "c1", "c2")
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, out, should.Equal("out2"))

			assert.Loosely(t, s.Calls[0].Executable, should.Equal("cmd1"))
			assert.Loosely(t, s.Calls[1].Executable, should.Equal("cmd2"))

			assert.Loosely(t, s.Calls[0].Args, should.Resemble([]string{"b1", "b2"}))
			assert.Loosely(t, s.Calls[1].Args, should.Resemble([]string{"c1", "c2"}))
		})

		t.Run("RunOutput works for failing command", func(t *ftt.Test) {
			s.ReturnError = []error{errors.Reason("test error").Err()}
			s.ReturnOutput = []string{"test output"}
			_, err := RunOutput(ctx, "output-cmd", "b1", "b2")
			assert.Loosely(t, err, should.NotBeNil)
		})
	})
}
