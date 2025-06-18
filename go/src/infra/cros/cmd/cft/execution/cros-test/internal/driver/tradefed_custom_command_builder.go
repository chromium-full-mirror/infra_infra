// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"fmt"
	"log"
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

const (
	commandMetadataFlag = "command"
)

func BuildCustomTestCommand(logger *log.Logger, testType string, tests []*api.TestCaseMetadata,
	serials []string, metadata *api.ExecutionMetadata, board string, args map[string][]string, model string,
	servo *labapi.Servo, lsnexus *labapi.IpEndpoint) []string {

	cmd := []string{}

	command, err := extractMetadataFlag(metadata, commandMetadataFlag)
	if err != nil {
		logger.Println("Missing/invalid command for custom Tradefed test: ", err)
		return cmd
	}
	cmd = append(cmd, command)

	// Appending all arguments to the command.
	values, ok := args["command-params"]
	if ok {
		for _, value := range values {
			cmd = append(cmd, strings.Split(value, ",")...)
		}
	}

	// Add devices
	for _, s := range serials {
		cmd = append(cmd, "-s", s)
	}

	if servo != nil && servo.ServodAddress != nil && servo.ServodAddress.Address != "" && servo.ServodAddress.Port != 0 {
		cmd = append(cmd,
			"--invocation-data", fmt.Sprintf("servo.host=%s", servo.ServodAddress.Address),
			"--invocation-data", fmt.Sprintf("servo.port=%d", servo.ServodAddress.Port),
		)
	}

	if lsnexus != nil && servo != nil && servo.GetState() != labapi.PeripheralState_BROKEN {
		cmd = append(cmd,
			"--invocation-data", fmt.Sprintf("lsnexus_primary.host=%s", lsnexus.GetAddress()),
			"--invocation-data", fmt.Sprintf("lsnexus_primary.port=%d", lsnexus.GetPort()),
		)
	}

	return cmd
}
