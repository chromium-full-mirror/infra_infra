// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutssh

import (
	"context"
	"io"
)

type ClientInterface interface {
	Close() error
	NewSession(ctx context.Context) (SessionInterface, error)
	Wait() error
	IsAlive() bool
}

type SessionInterface interface {
	Close() error
	SetStdout(writer io.Writer)
	SetStderr(writer io.Writer)
	SetStdin(reader io.Reader)
	Run(cmd string) error
	Start(cmd string) error
	Output(cmd string) ([]byte, error)
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
}
