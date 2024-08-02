// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package errors

import (
	"errors"
	"os/exec"
)

var (
	NotMatch = errors.New("can't match the value")
)

// HandleExitError extracts and passes downstream stderr from CLI call.
func HandleExitError(err error) error {
	var xerr *exec.ExitError
	if errors.As(err, &xerr) {
		err = errors.New(string(xerr.Stderr))
	}
	return err
}
