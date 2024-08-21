// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dolos is commands related to interacting with dolos, a virtual
// power source for DUT's.
package dolos

import (
	"context"
	"time"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/internal/log"
	"infra/cros/recovery/tlw"
)

const (
	dolosCmd = "/usr/bin/doloscmd "
)

// runDolosCommand build the params to run doloscmd.   In most cases the command can be called with either --uartname ( faster )
// or --serial (slower but works if you do not know uartname)
func runDolosCommand(ctx context.Context, run components.Runner, dolosSubCommand string, dolosInfo *tlw.Dolos, timeout time.Duration) (string, error) {

	command := dolosCmd

	if dolosInfo == nil {
		return "", errors.Reason("dolos ufs data is missing.").Err()
	}

	if dolosInfo.GetSerialUsb() != "" {
		command += dolosSubCommand + " --uartname " + dolosInfo.GetSerialUsb()
	} else {
		command += dolosSubCommand + " --serial " + dolosInfo.GetSerialCable()
	}

	output, err := run(ctx, timeout, command)
	log.Infof(ctx, "Dolos command %s returned %s ", command, output)
	return output, err
}
