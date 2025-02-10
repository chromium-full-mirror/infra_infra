// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package satlabrpcserver contains protocol buffers that are exchanged between the client
// and server.
//
// # Generating Protocol Buffer Code
//
// Anytime the Protocol Buffer definitions change, the generated Go code must be
// regenerated. This can be done with "go generate". Just run:
//
//	go generate ./...
//
// Upstream documentation:
// https://developers.google.com/protocol-buffers/docs/reference/go-generated
//
// # Code Generation Dependencies
//
// To generate the Go code, your system must have "protoc" installed. See:
// https://github.com/protocolbuffers/protobuf#protocol-compiler-installation
//
// The "protoc-gen-go" tool must also be installed. To install it, run:
//
//	go install google.golang.org/protobuf/cmd/protoc-gen-go
//	go install google.golang.org/protobuf/cmd/protoc-gen-go-grpc
//
// If you see a 'protoc-gen-go: program not found or is not executable' error
// for the 'go generate' command, run the following:
//
//	echo 'export PATH=$PATH:$GOPATH/bin' >> $HOME/.bashrc
//	source $HOME/.bashrc
package satlabrpcserver

//go:generate-b395127903-disabled sh generate.sh
