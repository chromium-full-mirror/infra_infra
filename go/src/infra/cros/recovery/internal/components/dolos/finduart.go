// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/recovery/internal/components"
	"go.chromium.org/infra/cros/recovery/internal/log"
	"go.chromium.org/infra/cros/recovery/tlw"
)

const (
	dolosSubCmdFindUart = "find-uartname"
)

// DolosFindUart Call doloscmd find-uartname on the host machine and parse the result text proto if received.
// Knowing the UART allows direct communication with the Dolos, otherwise it is necessary to search for the
// correct dolos with external serial number.
func DolosFindUart(ctx context.Context, run components.Runner, dolosInfo *tlw.Dolos, timeout time.Duration) (string, error) {

	output, err := runDolosCommand(ctx, run, dolosSubCmdFindUart, dolosInfo, timeout)

	if err != nil {
		return "", errors.Annotate(err, "unable to get dolos UART").Err()
	}

	var decoded FindUartNameResponse
	if err := protojson.Unmarshal([]byte(output), &decoded); err != nil {
		return "", errors.Annotate(err, "update dolos UART: fail to parse results").Err()
	}
	log.Infof(ctx, "Found dolos uartname %s.", decoded.GetUartname())
	return decoded.GetUartname(), nil
}
