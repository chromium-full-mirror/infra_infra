// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cros

import (
	"context"
	_ "embed"
	"os"
	"path"
	"strconv"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/execs"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

//go:embed data/keyboard.hex
var b []byte

// transferKeyboardHexExec transfers the keyboard hex file to the DUT
// for flashing firmware.
func transferKeyboardHexExec(ctx context.Context, info *execs.ExecInfo) error {
	// The path at which the embedded keyboard hex will be created so
	// that it can be copied over to DUT. We are prepending the
	// process ID to the name to avoid clash among files created by
	// multiple repairs that can be handled by the drone.
	keyboardHexSrcPath := path.Join("/tmp", strconv.Itoa(os.Getpid())+"_keyboard.hex")
	// If multiple repairs are done over a period of time, their
	// locally staged keyboard hex will accumulate. Hence, a cleanup
	// is good.
	defer func() {
		if err := os.Remove(keyboardHexSrcPath); err != nil {
			log.Debugf(ctx, "Transfer Keyboard Hex: could not remove the local staging file %s due to %s", keyboardHexSrcPath, err)
		}
	}()
	if err := os.WriteFile(keyboardHexSrcPath, b, 0666); err != nil {
		return errors.Annotate(err, "transfer keyboard hex").Err()
	}
	log.Debugf(ctx, "Transfer Keyboard Hex: copied keyboard.hex to %q locally.", keyboardHexSrcPath)
	// The destination file is a pre-decided location on the DUT that
	// will be used by the dfu-programmer command.
	keyboardHexDestPath := path.Join("/tmp", "keyboard.hex")
	log.Debugf(ctx, "Transfer Keybaord Hex: destination path is :%q", keyboardHexDestPath)
	return info.GetAccess().CopyFileTo(ctx, &tlw.CopyRequest{
		Resource:        info.GetActiveResource(),
		PathSource:      keyboardHexSrcPath,
		PathDestination: keyboardHexDestPath,
	})
}

func init() {
	execs.Register("transfer_keyboard_hex", transferKeyboardHexExec)
}
