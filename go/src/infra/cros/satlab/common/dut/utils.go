// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

type Flagmap = map[string][]string

// GetDockerHostBoxIdentifier gets the identifier for the satlab DHB, either from the command line, or
// by running a command inside the current container if no flag was given on the command line.
//
// Note that this function always returns the satlab ID in lowercase.
func getDockerHostBoxIdentifier(ctx context.Context, executor executor.IExecCommander, id string) (string, error) {
	// Use the string provided in the common flags by default.
	if id != "" {
		return strings.ToLower(id), nil
	}

	dockerHostBoxIdentifier, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, executor)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Unable to determine -satlab prefix, use %s to pass explicitly\n",
			id,
		)
		return "", errors.Annotate(err, "get docker host box").Err()
	}

	return dockerHostBoxIdentifier, nil
}
