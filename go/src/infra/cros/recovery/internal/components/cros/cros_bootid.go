// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	"fmt"
	"time"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
)

const (
	// bootIDFile is the file path to the file that contains the boot id information.
	bootIDFilePath = "/proc/sys/kernel/random/boot_id"
	// noIDMessage is the default boot id file content if the device does not have a boot id.
	noIDMessage = "no boot_id available"
)

// BootID gets a unique ID associated with the current boot.
//
// @returns: A string unique to this boot if there is no error.
func BootID(ctx context.Context, timeout time.Duration, run components.Runner) (string, error) {
	bootId, err := run(ctx, timeout, fmt.Sprintf("cat %s", bootIDFilePath))
	if err != nil {
		return "", errors.Annotate(err, "boot id").Err()
	}
	if bootId == noIDMessage {
		log.Debugf(ctx, "Boot ID: not found, assumed empty.")
		return "", nil
	}
	return bootId, nil
}
