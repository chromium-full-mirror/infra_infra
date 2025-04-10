// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commonutils

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// RunCommand supports running any cli command
func RunCommand(ctx context.Context, cmd *exec.Cmd, cmdName string, input proto.Message, block bool) (stdout string, stderr string, err error) {
	var se, so bytes.Buffer
	cmd.Stderr = &se
	cmd.Stdout = &so
	if input != nil {
		marshaler := protojson.MarshalOptions{
			Multiline: true,
		}
		printableInput, err := marshaler.Marshal(input)
		if err != nil {
			log.Printf("error while marshaling input: %s", err.Error())
			return "", "", fmt.Errorf("error while marshaling input for cmd %s: %s", cmdName, err.Error())
		}
		log.Printf("input for cmd %q: %s", cmdName, string(printableInput))
		cmd.Stdin = bytes.NewReader(printableInput)
	}
	defer func() {
		stdout = so.String()
		stderr = se.String()
		LogOutputs(cmdName, stdout, stderr)
	}()

	log.Printf("Run cmd: %q", cmd)
	if block {
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}

	if err != nil {
		log.Printf("error found with cmd: %q: %s", cmd, err)
	}
	return
}

// LogOutputs logs cmd stdout and stderr
func LogOutputs(cmdName string, stdout string, stderr string) {
	if stdout != "" {
		log.Printf("#### stdout from %q start ####\n", cmdName)
		log.Print(stdout)
		log.Printf("#### stdout from %q end ####\n", cmdName)
	}
	if stderr != "" {
		log.Printf("#### stderr from %q start ####\n", cmdName)
		log.Print(stderr)
		log.Printf("#### stderr from %q end ####\n", cmdName)
	}
}
