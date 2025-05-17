// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package exec implements the bols_testing for testing functionality of BOLS.
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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.chromium.org/chromiumos/config/go/test/api/bols"

	"go.chromium.org/infra/cros/cmd/cft/common/errors"
)

const (
	defaultRootPath = "/tmp/test/bols_testing"
	DefaultLogPath  = "/tmp/filters"
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
			fmt.Errorf("failed to create file %v: %w", fullPath, err))
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
	WorkingDir      string
	bolsAddr        string
	servodPort      int
	servodContainer string
}

// verify verifies BOLS APIs.
func verify(ctx context.Context, logger *log.Logger, a *args) error {
	conn, err := grpc.NewClient(a.bolsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create BOLS client at %s: %w", a.bolsAddr, err)
	}
	cl := bols.NewBolsServiceClient(conn)
	if err := verifyFileAPIs(ctx, logger, a, cl); err != nil {
		return fmt.Errorf("failed to verify file related APIs in BOLS at %s: %w", a.bolsAddr, err)
	}
	return nil
}

// runCLI is the entry point for running cros-test (TestFinderService) in CLI mode.
func runCLI(ctx context.Context, d []string) int {
	t := time.Now()
	defaultWorkingDir := filepath.Join(defaultRootPath, t.Format("20060102-150405"))

	a := args{}

	fs := flag.NewFlagSet("Run bols_testing", flag.ExitOnError)
	fs.StringVar(&a.WorkingDir, "working_dir", defaultWorkingDir, fmt.Sprintf("Working directory. Default value is %s", defaultWorkingDir))
	fs.StringVar(&a.bolsAddr, "bols_addr", "", "The address of BOLS.")
	fs.StringVar(&a.servodContainer, "servod_container", "", "The container of BOLS.")
	fs.IntVar(&a.servodPort, "servod_port", 0, "The servod port.")
	fs.Parse(d)

	os.MkdirAll(filepath.Join(a.WorkingDir, "data"), 0755)

	logFile, err := createLogFile(filepath.Join(a.WorkingDir, "log"))
	if err != nil {
		log.Fatalln("Failed to create log file", err)
		return 2
	}
	defer logFile.Close()

	logger := newLogger(logFile)
	logger.Println("bols_testing version ", Version)
	if err := verify(ctx, logger, &a); err != nil {
		logger.Fatalln("Failed in verification: ", err)
		return 2
	}

	return 0
}

// Specify run mode for CLI.
type runMode string

const (
	runCli     runMode = "cli"
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
		}
		return runHelp, fmt.Errorf("unknown subcommand %s", os.Args[1])
	}
	return runHelp, nil
}

func BOLSTestingInternal(ctx context.Context) int {
	runMode, err := getRunMode()
	if err != nil {
		log.Fatalln(err)
		return 2
	}
	switch runMode {
	case runCli:
		log.Printf("Running CLI mode!")
		return runCLI(ctx, os.Args[2:])
	case runVersion:
		log.Printf("bols_testing version: %s", Version)
		return 0
	}
	return 0
}
