// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package server implement adb-service API.
package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/test/util/portdiscovery"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/cros/cmd/cft/base-adb/internal/version"
)

// Server to run adb service.
type Server interface {
	Run(ctx context.Context) error
}

// adbService holds info for server.
type adbService struct {
	logPath    string
	port       int
	deviceName string

	// Initialized later.
	logger *log.Logger
}

// New creates new server to perform.
func New(ctx context.Context, logPath string, port int, deviceName string) (Server, error) {
	if deviceName == "" {
		return nil, errors.Reason("device name is not specified").Err()
	}
	a := &adbService{
		logPath:    logPath,
		port:       port,
		deviceName: deviceName,
	}

	return a, nil
}

// startServer starts service on requested port.
func (s *adbService) Run(ctx context.Context) error {
	logFile, err := createLogFile(s.logPath)
	if err != nil {
		log.Fatalln("Failed to create log file: ", err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			log.Fatalln("Failed to close log file: ", err)
		}
	}()
	logger := newLogger(logFile)
	s.logger = logger
	logger.Println("Starting base-adb version ", version.Version)
	logger.Println("Starting base-adb on port ", s.port)
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return errors.Annotate(err, "start server: create listener at %d", s.port).Err()
	}
	// Log the port _after_ the conn is established server.
	logger.Println("Started server on address ", l.Addr().String())
	logger.Println("Continue")
	// Write port number to ~/.cftmeta for go/cft-port-discovery
	if err := portdiscovery.WriteServiceMetadata("base-adb", l.Addr().String(), logger); err != nil {
		logger.Println("Warning: error when writing to metadata file: ", err)
	}
	srv := grpc.NewServer()
	api.RegisterADBServiceServer(srv, s)
	logger.Println("service listen to request at ", l.Addr().String())
	if err := srv.Serve(l); err != nil {
		return err
	}
	logger.Println("Server stopped!")
	return err
}

// createLogFile creates a file and its parent directory for logging purpose.
func createLogFile(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	logFilePath := filepath.Join(dir, "log.txt")
	// Log the full output of the command to disk.
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", logFilePath, err)
	}
	return logFile, nil
}

// newLogger creates a logger. Using go default logger for now.
func newLogger(logFile *os.File) *log.Logger {
	mw := io.MultiWriter(logFile, os.Stderr)
	return log.New(mw, "", log.LstdFlags|log.LUTC)
}
