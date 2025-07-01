// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func TestServer_Lifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	addr := "localhost:0"
	s, err := New(addr, &Config{})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if s == nil {
		t.Fatal("New() returned a nil server")
	}

	errC := make(chan error, 1)
	go func() {
		errC <- s.Start()
	}()

	// Wait for the server to start.
	time.Sleep(100 * time.Millisecond)

	// Try to connect to the server.
	conn, err := grpc.DialContext(ctx, s.lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}
	conn.Close()

	s.Stop(false)

	if err := <-errC; err != nil {
		t.Errorf("Start() returned an error: %v", err)
	}
}

func TestUnaryInterceptor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ic := interceptor{}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			return nil, fmt.Errorf("no outgoing metadata")
		}
		return md, nil
	}
	resp, err := ic.unary(ctx, nil, nil, handler)
	if err != nil {
		t.Fatalf("unary() failed: %v", err)
	}
	md, ok := resp.(metadata.MD)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}
	if diff := cmp.Diff(metadata.Pairs("namespace", "os"), md); diff != "" {
		t.Errorf("unexpected metadata diff (-want +got):\n%s", diff)
	}
}
