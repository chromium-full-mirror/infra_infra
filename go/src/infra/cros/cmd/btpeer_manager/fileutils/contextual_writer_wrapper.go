// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fileutils

import (
	"context"
	"io"
)

// ContextualWriterWrapper is a wrapper around an existing io.Writer that allows
// for the interruption of writes with the cancellation of a context.
type ContextualWriterWrapper struct {
	ctx    context.Context
	writer io.Writer
}

// NewContextualWriterWrapper initializes a new ContextualWriterWrapper.
func NewContextualWriterWrapper(ctx context.Context, writer io.Writer) *ContextualWriterWrapper {
	return &ContextualWriterWrapper{
		ctx:    ctx,
		writer: writer,
	}
}

// Write calls Write on the wrapped writer, allowing for interruption from
// a cancellation of the context.
func (c *ContextualWriterWrapper) Write(p []byte) (n int, err error) {
	writeChan := make(chan error)
	go func() {
		writeChan <- func() error {
			n, err = c.writer.Write(p)
			return err
		}()
	}()
	select {
	case <-c.ctx.Done():
		err = c.ctx.Err()
		break
	case err = <-writeChan:
		break
	}
	return n, err
}
