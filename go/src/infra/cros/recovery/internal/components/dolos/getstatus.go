// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dolos

import (
	"context"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/common/errors"

	"infra/cros/recovery/internal/components"
	"infra/cros/recovery/tlw"
)

const (
	dolosSubCmdGetStatus = "get-status"
)

// DolosGetStatus call doloscmd get-status and parse the results ( if received ) from the output text proto.
func DolosGetStatus(ctx context.Context, run components.Runner, dolosInfo *tlw.Dolos, timeout time.Duration) (string, error) {

	output, err := runDolosCommand(ctx, run, dolosSubCmdGetStatus, dolosInfo, timeout)
	if err != nil {
		return "", errors.Annotate(err, "unable to get dolos status").Err()
	}
	var decoded GetStatusResponse
	err = protojson.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return "", errors.Annotate(err, "determine dolos state").Err()
	}

	return decoded.GetStatus().String(), nil
}
