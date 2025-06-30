// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command labservice implements the Chrome OS Lab Service.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/unifiedfleet/app/util"
)

func main() {
	// Configure the default Go logger only for handling fatal
	// errors in main and any libraries that are using it.
	// Otherwise, labservice code should use the labservice
	// internal log package.
	log.SetPrefix("labservice: ")
	if err := innerMain(); err != nil {
		log.Fatalf("Fatal error: %s", err)
	}
}

func innerMain() error {
	var (
		addr                     = flag.String("addr", "0.0.0.0:1485", "Address to listen to")
		preferredCachingServices strSlice
		serviceAccountPath       = flag.String("service-account-json", "",
			"Path to service account JSON file")
		ufsService = flag.String("ufs-service", "ufs.api.cr.dev", "UFS service host")
	)
	flag.Var(&preferredCachingServices, "preferred-caching-services", "Comma separated preferred caching services (each in format: [http://]server[:port]) which superseded the ones fetche from UFS")

	flag.Parse()
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	gs := newGRPCServer(&serverConfig{
		preferredCachingServices: preferredCachingServices,
		serviceAccountPath:       *serviceAccountPath,
		ufsService:               *ufsService,
	})
	c := make(chan os.Signal, 1)
	signal.Notify(c, handledSignals...)
	ctx := context.Background()
	// This goroutine exits when the program exits.
	go func() {
		for sig := range c {
			// Handle asynchronously so we can handle
			// cases like getting a SIGINT (graceful stop)
			// followed by a SIGTERM (immediate stop).
			go handleSignal(ctx, gs, sig)
		}
	}()
	return gs.Serve(l)
}

// newGRPCServer creates a new gRPC server for labservice.
func newGRPCServer(c *serverConfig) *grpc.Server {
	ic := interceptor{}
	gs := grpc.NewServer(ic.unaryOption(), grpc.StreamInterceptor(streamNamespaceInterceptor))
	s := newServer(c)
	labapi.RegisterInventoryServiceServer(gs, s)
	return gs
}

// interceptor has gRPC interceptor methods.
// This is the only way to modify the context passed to method handlers.
type interceptor struct{}

func (interceptor) unary(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
	ctx, err := withUFSContext(ctx)
	if err != nil {
		return nil, err
	}
	return h(ctx, req)
}

func (ic interceptor) unaryOption() grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(ic.unary)
}

// withUFSContext returns a context with the gRPC metadata set with either the
// *incoming* gRPC metadata, or a reasonable default of `os` namespace.
func withUFSContext(ctx context.Context) (context.Context, error) {
	ns, err := determineNamespaceFromContext(ctx)
	log.Printf("Setting ns to : %s", ns)
	if err != nil {
		return nil, err
	}

	md := metadata.Pairs("namespace", ns)
	return metadata.NewOutgoingContext(ctx, md), nil
}

// determineNamespaceFromContext decides the namespace in outgoing context for
// UFS requests
//
// Handles three situations:
// - nothing set on incoming call: defaults to `os` namespace
// - valid value in incoming call: uses that value
// - invalid value in incoming call: errors out
func determineNamespaceFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	// No metadata is set- we should default to `os`.
	if !ok {
		return util.OSNamespace, nil
	}

	namespace, ok := md[util.Namespace]
	if ok {
		ns := strings.ToLower(namespace[0])
		datastoreNamespace, ok := util.ClientToDatastoreNamespace[ns]
		if ok {
			return datastoreNamespace, nil
		} else {
			return "", status.Errorf(codes.InvalidArgument, "namespace %s in the context metadata is invalid. Valid namespaces: [%s]", namespace[0], strings.Join(util.ValidClientNamespaceStr(), ", "))
		}
	}

	return util.OSNamespace, nil
}

// serverStream overrides behavior of `grpc.serverStream` by allowing us to
// set and get the context.
type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context implements the Context() method of the serverStream interface.
func (s *serverStream) Context() context.Context {
	return s.ctx
}

// streamNamespaceInterceptor adds the os namespace as *outgoing* context for
// all GRPC stream requests.
func streamNamespaceInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx, err := withUFSContext(ss.Context())
	if err != nil {
		return err
	}
	return handler(srv, &serverStream{ss, ctx})
}

// strSlice implements flag.Value interface for specify multiple value.
type strSlice []string

func (s *strSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *strSlice) Set(value string) error {
	if value == "" {
		return nil
	}
	*s = strings.Split(value, ",")
	return nil
}
