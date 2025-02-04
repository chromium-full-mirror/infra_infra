// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"go.chromium.org/luci/common/logging"
)

// RunCommand supports running any cli command
func RunCommand(ctx context.Context, cmd *exec.Cmd, cmdName string, input proto.Message, block bool) (stdout string, stderr string, err error) {
	var se, so bytes.Buffer
	cmd.Stderr = &se
	cmd.Stdout = &so
	if input != nil {
		marshalOps := prototext.MarshalOptions{Multiline: true, Indent: "  "}
		printableInput, err := marshalOps.Marshal(input)
		if err != nil {
			logging.Infof(ctx, "error while marshaling input: %s", err.Error())
			return "", "", fmt.Errorf("error while marshaling input for cmd %s: %s", cmdName, err.Error())
		}
		logging.Infof(ctx, "input for cmd %q: %s", cmdName, string(printableInput))
		cmd.Stdin = bytes.NewReader(printableInput)
	}
	defer func() {
		stdout = so.String()
		stderr = se.String()
		logOutputs(ctx, cmdName, stdout, stderr)
	}()

	logging.Infof(ctx, "Run cmd: %q", cmd)
	if block {
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}

	if err != nil {
		logging.Infof(ctx, "error found with cmd: %q: %s", cmd, err)
	}
	return
}

// RunCommandWithCustomWriter runs a command with custom writer
func RunCommandWithCustomWriter(ctx context.Context, cmd *exec.Cmd, cmdName string, writer io.Writer) error {
	cmd.Stdout = writer
	cmd.Stderr = writer

	logging.Infof(ctx, "Run cmd: %q", cmd)
	err := cmd.Run()
	if err != nil {
		logging.Infof(ctx, "error found with cmd: %q: %s", cmd, err)
	}

	return err
}

// logOutputs logs cmd stdout and stderr
func logOutputs(ctx context.Context, cmdName string, stdout string, stderr string) {
	if stdout != "" {
		logging.Infof(ctx, "#### stdout from %q start ####\n", cmdName)
		logging.Infof(ctx, stdout)
		log.Printf("#### stdout from %q end ####\n", cmdName)
	}
	if stderr != "" {
		logging.Infof(ctx, "#### stderr from %q start ####\n", cmdName)
		logging.Infof(ctx, stderr)
		logging.Infof(ctx, "#### stderr from %q end ####\n", cmdName)
	}
}

// FormatSwarmingTaskID does...
// # A swarming task may have multiple attempts ("runs").
// # The swarming task ID always ends in "0", e.g. "123456789abcdef0".
// # The corresponding runs will have IDs ending in "1", "2", etc., e.g. "123456789abcdef1".
// # All attempts should be recorded under same job ending with 0.
func FormatSwarmingTaskID(swarmingTaskID string) string {
	return swarmingTaskID[:len(swarmingTaskID)-1]
}
