// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package parser parses input data to target Runners.
package parser

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cft/base-adb/internal/cli"
	"go.chromium.org/infra/cros/cmd/cft/base-adb/internal/server"
	"go.chromium.org/infra/cros/cmd/cft/base-adb/internal/version"
)

type Runner interface {
	Run(context.Context) error
}

var (
	// ExitCodeTag provides exit code value.
	ExitCodeTag = errors.NewTagKey("exit_code")
)

var (
	ServiceName     = "base-adb"
	defaultLogDir   = fmt.Sprintf("/tmp/%s/", ServiceName)
	defaultPort     = 80
	helpDescription = fmt.Sprintf(`base-adb tool
The tool is allow to communicate with adb on the host.
Commands:
  cli		Execution RPCs as CLI command. Not implemented yet! 
  server	Starting server and allow work with server by RPC calls. Mostly used for tests.
  		usage: base-adb server -device {device_name} [-log-path /tmp/%s/] [-port %d]
  version	Print version of lib.
  help		Print this help.`, defaultLogDir, defaultPort)
)

// Specify run mode for CLI.
type runMode string

const (
	runCli     runMode = "cli"
	runServer  runMode = "server"
	runVersion runMode = "version"
	runHelp    runMode = "help"
)

// parseServer parses argsuments for the server mode.
func parseServer(ctx context.Context, d []string) (Runner, error) {
	var logPath, deviceName string
	var port int
	fs := flag.NewFlagSet("Start server", flag.ExitOnError)
	fs.StringVar(&logPath, "log-path", defaultLogDir, fmt.Sprintf("Path to record execution logs. Default value is %s", defaultLogDir))
	fs.StringVar(&deviceName, "device", "", "Specify device name for target device.")
	fs.IntVar(&port, "port", defaultPort, fmt.Sprintf("Specify the port for the server. Default value %d.", defaultPort))
	if err := fs.Parse(d); err != nil {
		return nil, errors.Annotate(err, "parse server args").Err()
	}
	return server.New(ctx, logPath, port, deviceName)
}

// getRunMode extract run-mode for CLI.
func getRunMode(args []string) (runMode, error) {
	if len(args) > 1 {
		for _, a := range args {
			if a == "-version" {
				return runVersion, nil
			}
			if a == "-help" {
				return runVersion, nil
			}
		}
		switch strings.ToLower(args[1]) {
		case "cli":
			return runCli, nil
		case "server":
			return runServer, nil
		case "help":
			return runHelp, nil
		case "version":
			return runVersion, nil
		}
	}
	// If we did not find special run mode then just print help for user.
	return runHelp, nil
}

// ParseArgs parses arguments and return a runner to execute CLI.
func ParseArgs(ctx context.Context, args []string) (Runner, error) {
	rm, err := getRunMode(args)
	if err != nil {
		return nil, errors.Annotate(err, "parse args").Err()
	}
	log.Printf("Detected mode: %s\n", rm)
	switch rm {
	case runCli:
		return cli.New(), nil
	case runServer:
		return parseServer(ctx, args[2:])
	case runVersion:
		return version.New(), nil
	case runHelp:
		fallthrough
	default:
		log.Println(helpDescription)
		return nil, nil
	}
}
