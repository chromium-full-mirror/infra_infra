// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the servo-nexus for starting/stopping servod daemon
// and sending commands to it to control and test DUTs via servo hardware by
// simulating user actions such as power on/off, flashing of firmware/OS,
// screen close, etc.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"infra/cros/cmd/cft/dut/servo-nexus/commandexecutor"
	"infra/cros/cmd/cft/dut/servo-nexus/servodserver"
)

const (
	defaultLogDirectory = "/tmp/servod/"
	defaultServerPort   = 80
	defaultServodPort   = 9999
)

// createLogFile creates a file and its parent directory for logging purpose.
func createLogFile(logPath string) (*os.File, error) {
	t := time.Now()
	fullPath := filepath.Join(logPath, t.Format("20060102-150405"))
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %v: %v", fullPath, err)
	}

	logFullPathName := filepath.Join(fullPath, "log.txt")

	// Log the full output of the command to disk.
	logFile, err := os.Create(logFullPathName)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %v: %v", fullPath, err)
	}
	return logFile, nil
}

// newLogger creates a logger. Using go default logger for now.
func newLogger(logFile *os.File) *log.Logger {
	mw := io.MultiWriter(logFile, os.Stderr)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}

func startServer(ctx context.Context, d []string) int {
	var logPath string
	var serverPortTmp int
	fs := flag.NewFlagSet("Start servod server", flag.ExitOnError)
	fs.StringVar(&logPath, "log_path", defaultLogDirectory, fmt.Sprintf("The path to record execution logs. The default value is %s", defaultLogDirectory))
	fs.IntVar(&serverPortTmp, "server_port", defaultServerPort, fmt.Sprintf("The port for the servod GRPC server. The default value is %d.", defaultServerPort))
	fs.Parse(d)
	serverPort := int32(serverPortTmp)
	logFile, err := createLogFile(logPath)
	if err != nil {
		log.Println("Failed to create log file", err)
		return 2
	}
	defer logFile.Close()
	logger := newLogger(logFile)
	commandexecutor := commandexecutor.NewServodCommandExecutor(logger)
	servodService, destructor, err := servodserver.NewServodService(ctx, logger, commandexecutor)
	defer destructor()
	if err != nil {
		logger.Println("Failed to create servod service: ", err)
		return 2
	}
	if err := servodService.StartServer(serverPort); err != nil {
		logger.Println("Failed to start servod server: ", err)
		return 1
	}
	return 0
}

func mainInternal() int {
	ctx := context.Background()
	log.Printf("Starting Server!")
	return startServer(ctx, os.Args[1:])
}

func main() {
	os.Exit(mainInternal())
}
