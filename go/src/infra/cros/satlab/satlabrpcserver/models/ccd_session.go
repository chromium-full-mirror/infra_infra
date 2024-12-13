// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package models

import (
	"fmt"
	"io"
)

type CCDSession struct {
	// A writer that is used to send a message
	Writer *io.PipeWriter
}

func (c *CCDSession) Send(message string) error {
	_, err := c.Writer.Write([]byte(fmt.Sprintf("%s\n", message)))
	return err
}
