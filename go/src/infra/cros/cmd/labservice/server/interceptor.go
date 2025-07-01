// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.chromium.org/infra/unifiedfleet/app/util"
)

// interceptor has gRPC interceptor methods.
// This is the only way to modify the context passed to method handlers.
type interceptor struct{}

// unary is the unary interceptor for the gRPC server.
func (interceptor) unary(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
	ctx, err := withUFSContext(ctx)
	if err != nil {
		return nil, err
	}
	return h(ctx, req)
}

// unaryOption returns the unary server option for the gRPC server.
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
		}
		return "", status.Errorf(codes.InvalidArgument, "namespace %s in the context metadata is invalid. Valid namespaces: [%s]", namespace[0], strings.Join(util.ValidClientNamespaceStr(), ", "))
	}

	return util.OSNamespace, nil
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
