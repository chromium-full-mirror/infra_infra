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
	"go.chromium.org/infra/cros/recovery/tlw"
)

// GetVersion call doloscmd version and parse the results ( if received ) from the output text proto.
func GetVersion(ctx context.Context, run components.Runner, dolosInfo *tlw.Dolos, timeout time.Duration) (string, error) {

	const dolosSubCmdVersion = "version"

	output, err := runCommand(ctx, run, dolosSubCmdVersion, dolosInfo, timeout)
	if err != nil {
		return "", errors.Annotate(err, "unable to retrieve dolos version").Err()
	}
	var decoded GetVersionResponse
	err = protojson.Unmarshal([]byte(output), &decoded)
	if err != nil {
		return "", errors.Annotate(err, "determine dolos version").Err()
	}

	return decoded.GetVersion(), nil
}
