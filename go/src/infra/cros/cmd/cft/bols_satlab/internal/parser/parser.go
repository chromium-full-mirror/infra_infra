// Copyright 2025 The Chromium Authors
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

	"go.chromium.org/infra/cros/cmd/cft/bols_satlab/internal/server"
	"go.chromium.org/infra/cros/cmd/cft/bols_satlab/internal/version"
	"go.chromium.org/infra/cros/lib/bols"
)

type Runner interface {
	Run(context.Context) error
}

var (
	ServiceName     = "bols"
	defaultLogDir   = fmt.Sprintf("/tmp/%s/", ServiceName)
	helpDescription = `bols_satlab tool
The tool is allow to communicate with labstation tools and services on the host.
Commands:
  run		Starting server and allow work with server by RPC calls.
  version	Print version of lib.
  help		Print this help.`
)

// Specify run mode for CLI.
type runMode string

const (
	runServer  runMode = "run"
	runVersion runMode = "version"
	runHelp    runMode = "help"
)

// parseServer parses argsuments for the server mode.
func parseServer(ctx context.Context, d []string) (Runner, error) {
	var logPath string
	var port int
	fs := flag.NewFlagSet("Start server", flag.ExitOnError)
	fs.StringVar(&logPath, "log-path", defaultLogDir, fmt.Sprintf("Path to record execution logs. Default value is %s", defaultLogDir))
	fs.IntVar(&port, "port", bols.DefaultBOLSPort,
		fmt.Sprintf("Specify the port for the server. Default value %d.", bols.DefaultBOLSPort))
	if err := fs.Parse(d); err != nil {
		return nil, errors.Annotate(err, "parse server args").Err()
	}
	return server.New(ctx, logPath, port)
}

// parseRunMode extract run-mode for CLI.
func parseRunMode(args []string) (runMode, error) {
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
	rm, err := parseRunMode(args)
	if err != nil {
		return nil, errors.Annotate(err, "parse args").Err()
	}
	log.Printf("Detected mode: %s\n", rm)
	switch rm {
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
