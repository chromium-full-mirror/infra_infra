// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the lsnexus_testing for testing functionality of BOLS.
package exec

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.chromium.org/infra/cros/cmd/cft/common/errors"
)

const (
	defaultRootPath = "/tmp/test/lsnexus_testing"
)

// createLogFile creates a file and its parent directory for logging purpose.
func createLogFile(fullPath string) (*os.File, error) {
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create directory %v: %w", fullPath, err))
	}

	logFullPathName := filepath.Join(fullPath, "log.txt")

	// Log the full output of the command to disk.
	logFile, err := os.Create(logFullPathName)
	if err != nil {
		return nil, errors.NewStatusError(errors.IOCreateError,
			fmt.Errorf("failed to create file %v: %w", logFullPathName, err))
	}
	return logFile, nil
}

// newLogger creates a logger. Using go default logger for now.
func newLogger(logFiles ...*os.File) *log.Logger {
	writers := []io.Writer{os.Stderr}
	for _, logFile := range logFiles {
		writers = append(writers, logFile)
	}
	mw := io.MultiWriter(writers...)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}

// Version is the version info of this command. It is filled in during emerge.
var Version = "<unknown>"

type args struct {
	// Common input params.
	workingDir      string
	bolsAddr        string
	lsNexusAddr     string
	servodPort      int
	servodContainer string
	servoSerial     string
	testServod      bool
	board           string
	model           string
}

// runCLI is the entry point for running cros-test (TestFinderService) in CLI mode.
func runCLI(ctx context.Context, d []string) int {
	t := time.Now()
	defaultWorkingDir := filepath.Join(defaultRootPath, t.Format("20060102-150405"))

	a := args{}

	fs := flag.NewFlagSet("Run lsnexus_testing", flag.ExitOnError)
	fs.StringVar(&a.workingDir, "working_dir", defaultWorkingDir, fmt.Sprintf("Working directory. Default value is %s", defaultWorkingDir))
	fs.StringVar(&a.bolsAddr, "bols_addr", "", "The address of BOLS.")
	fs.StringVar(&a.lsNexusAddr, "lsnexus_addr", "", "The address of LSNexus.")
	fs.StringVar(&a.servodContainer, "servod_container", "", "The container of BOLS.")
	fs.IntVar(&a.servodPort, "servod_port", 0, "The servod port.")
	fs.StringVar(&a.servoSerial, "servo_serial", "", "The serial of the servo.")
	fs.BoolVar(&a.testServod, "test_servod", true, "Test servo related APIs.")
	fs.StringVar(&a.board, "board", "", "The board of the DUT")
	fs.StringVar(&a.model, "model", "", "The model of the DUT")
	fs.Parse(d)

	os.MkdirAll(filepath.Join(a.workingDir, "data"), 0755)

	logFile, err := createLogFile(filepath.Join(a.workingDir, "log"))
	if err != nil {
		log.Fatalln("Failed to create log file", err)
	}
	defer logFile.Close()

	logger := newLogger(logFile)
	logger.Println("lsnexus_testing version ", Version)
	if err := verify(ctx, logger, &a); err != nil {
		logger.Fatalln("Failed in verification: ", err)
	}

	return 0
}

// runLSNexusServer is the entry point for running lsnexus_testing in server mode.
func runLSNexusServer(ctx context.Context, d []string) int {
	t := time.Now()
	defaultWorkingDir := filepath.Join(defaultRootPath, t.Format("20060102-150405"))

	a := args{}

	fs := flag.NewFlagSet("Run lsnexus_testing in server mode", flag.ExitOnError)
	fs.StringVar(&a.workingDir, "working_dir", defaultWorkingDir, fmt.Sprintf("Working directory. Default value is %s", defaultWorkingDir))
	fs.StringVar(&a.bolsAddr, "bols_addr", "", "The address of BOLS.")
	fs.StringVar(&a.lsNexusAddr, "lsnexus_addr", "", "The address of LSNexus.")
	fs.StringVar(&a.servodContainer, "servod_container", "", "The container of BOLS.")
	fs.IntVar(&a.servodPort, "servod_port", 0, "The servod port.")
	fs.StringVar(&a.servoSerial, "servo_serial", "", "The serial of the servo.")
	fs.StringVar(&a.board, "board", "", "The board of the DUT")
	fs.StringVar(&a.model, "model", "", "The model of the DUT")
	fs.Parse(d)

	os.MkdirAll(filepath.Join(a.workingDir, "data"), 0755)

	logFile, err := createLogFile(filepath.Join(a.workingDir, "log"))
	if err != nil {
		log.Fatalln("Failed to create log file", err)
	}
	defer logFile.Close()

	logger := newLogger(logFile)
	logger.Println("lsnexus_testing server version ", Version)
	if err := startServer(ctx, logger, &a); err != nil {
		logger.Fatalln("Failed to run server: ", err)
	}

	return 0
}

// Specify run mode for CLI.
type runMode string

const (
	runCli     runMode = "cli"
	runServer  runMode = "server"
	runVersion runMode = "version"
	runHelp    runMode = "help"
)

func getRunMode() (runMode, error) {
	if len(os.Args) > 1 {
		for _, a := range os.Args {
			if a == "-version" {
				return runVersion, nil
			}
		}
		switch strings.ToLower(os.Args[1]) {
		case "cli":
			return runCli, nil

		case "server":
			return runServer, nil
		}
		return runHelp, fmt.Errorf("unknown subcommand %s", os.Args[1])
	}
	return runHelp, nil
}

func LSNexusTestingInternal(ctx context.Context) int {
	runMode, err := getRunMode()
	if err != nil {
		log.Fatalln(err)
	}
	switch runMode {
	case runCli:
		log.Printf("Running CLI mode!")
		return runCLI(ctx, os.Args[2:])
	case runServer:
		log.Printf("Running Server mode!")
		return runLSNexusServer(ctx, os.Args[2:])
	case runVersion:
		log.Printf("lsnexus_testing version: %s", Version)
		return 0
	}
	return 0
}
