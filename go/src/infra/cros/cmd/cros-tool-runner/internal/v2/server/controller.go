// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/system/signals"

	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/templates"
	"go.chromium.org/infra/cros/cmd/cros-tool-runner/internal/v2/tsmon"
)

var serverCleanup = &serverStateManager{}

// NewContainerServer returns a new gRPC server for container services.
func NewContainerServer() (*grpc.Server, func()) {
	containerServer := &ContainerServerImpl{
		executor:          &DefaultCommandExecutor{},
		templateProcessor: &templates.RequestRouter{},
		containerLookuper: &templates.TemplateUtils,
	}
	// Only unary interceptor is needed as CTRv2 has no streaming endpoint.
	s := grpc.NewServer(grpc.UnaryInterceptor(panicInterceptor))
	destructor := serverCleanup.cleanup
	api.RegisterCrosToolRunnerContainerServiceServer(s, containerServer)
	reflection.Register(s)
	return s, destructor
}

// panicInterceptor implements grpc.UnaryServerInterceptor to handle panic
// (caused by bugs) with proper cleanup for CTRv2 container service.
func panicInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	defer serverCleanup.handlePanic()
	return handler(ctx, req)
}

// StartServer starts server on the requested port.
func StartServer(port int, exportTo string) int {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return 1
	}

	grpcServer, destructor := NewContainerServer()

	errChan := make(chan error, 1)
	stopChan := make(chan os.Signal, 1)

	// Bind signal to stopChan
	signal.Notify(stopChan, signals.Interrupts()...)

	// Start server in a goroutine, send errors to errChan
	go func() {
		log.Printf("server listening at %v", lis.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			errChan <- err
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Graceful stop server
	defer func() {
		grpcServer.GracefulStop()
		destructor()
	}()

	if exportTo != "" {
		exportMetadata(lis, exportTo)
	}
	// init metrics
	if err = tsmon.Init(); err != nil {
		log.Printf("warning: metrics init Failed (NON-CRITICAL): %s", err)
	} else {
		defer tsmon.Shutdown()
	}

	// Wait for channel operations
	select {
	case err := <-errChan:
		log.Println("fatal error:", err)
		return 1
	case <-stopChan:
		log.Println("interrupt signal received")
	}
	return 0
}

func exportMetadata(address net.Listener, exportTo string) {
	metaFile := path.Join(exportTo, ".cftmeta")

	f, err := os.OpenFile(metaFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("error: cannot open metadata file %v", err)
		return
	}
	defer f.Close()

	r := regexp.MustCompile(`.*:(\d+)$`)
	match := r.FindStringSubmatch(address.Addr().String())
	if match == nil {
		log.Printf("error: cannot find port from address %v", address)
		return
	}

	port := match[1]
	content := fmt.Sprintf("%s=%s\n%s=%s\n%s=%s\n",
		"SERVICE_PORT", port,
		"SERVICE_NAME", "CTRv2",
		"SERVICE_START_TIME", time.Now().Format(time.RFC3339))
	_, err = f.WriteString(content)
	if err != nil {
		log.Printf("error: cannot write to metadata file %v", err)
		return
	}

	log.Printf("service metadata has been exported to %v", metaFile)
}
