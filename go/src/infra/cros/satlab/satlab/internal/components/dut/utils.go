// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dut

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/satlab/common/satlabcommands"
	"go.chromium.org/infra/cros/satlab/common/site"
	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// Flagmap is a map from the name of a flag to its value(s).
type flagmap = map[string][]string

// GetDockerHostBoxIdentifier gets the identifier for the satlab DHB, either from the command line, or
// by running a command inside the current container if no flag was given on the command line.
//
// Note that this function always returns the satlab ID in lowercase.
func getDockerHostBoxIdentifier(ctx context.Context, common site.CommonFlags) (string, error) {
	// Use the string provided in the common flags by default.
	if common.SatlabID != "" {
		return strings.ToLower(common.SatlabID), nil
	}

	dockerHostBoxIdentifier, err := satlabcommands.GetDockerHostBoxIdentifier(ctx, &executor.ExecCommander{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to determine -satlab prefix, use %s to pass explicitly\n", common.SatlabID)
		return "", errors.Annotate(err, "get docker host box").Err()
	}

	return dockerHostBoxIdentifier, nil
}

// Pinger allows checking aliveness of DUTs.
type Pinger interface {
	// Ping attempts to contact the device.
	Ping() error
}

// DUTPinger uses the hostname of DUTs to send the pings.
type DUTPinger struct {
	hostname string
	count    int
}

func (p *DUTPinger) Ping() error {
	if p.hostname == "" {
		return errors.Reason("ping: addr is empty").Err()
	}
	cmd := exec.Command("sudo",
		"ping",
		p.hostname,
		"-c",
		strconv.Itoa(p.count), // How many times will ping.
		"-W",
		"1", // How long wait for response.
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.Annotate(err, stderr.String()).Err()
	}
	return nil
}

// DefaultPinger creates a Pinger targeting a hostname.
func DefaultPinger(hostname string) Pinger {
	return &DUTPinger{
		hostname: hostname,
		count:    2, // arbitrary
	}
}
