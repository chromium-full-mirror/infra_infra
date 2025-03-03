// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package components

import (
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/errors/errtag"
)

var (
	// ErrCodeTag is the key value pair for storing the error code for the linux command.
	ErrCodeTag = errors.NewTagKey("error_code")

	// StdErrTag is the key value pair for storing the error code
	// associated with the standard error
	StdErrTag = errors.NewTagKey("std_error")

	// 127: linux command line error of command not found.
	SSHErrorCLINotFound = errtag.Make("ssh_error_cli_not_found", true)

	// 124: linux command line error of command timeout.
	SSHErrorLinuxTimeout = errtag.Make("linux_timeout", true)

	// other linux error tag.
	GeneralError = errtag.Make("general_error", true)

	// internal error tag.
	SSHErrorInternal = errtag.Make("ssh_error_internal", true)

	// -1: fail to create ssh session.
	FailToCreateSSHErrorInternal = errtag.Make("fail_to_create_ssh_error_internal", true)

	// -2: session is down, but the server sends no confirmation of the exit status.
	NoExitStatusErrorInternal = errtag.Make("no_exit_status_error_internal", true)

	// other internal error tag.
	OtherErrorInternal = errtag.Make("other_error_internal", true)
)
