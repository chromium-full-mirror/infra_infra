# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: default build test

default: build

build:
	@go vet ./...
	@go test ./...
	go build ./...

test:
	CGO_ENABLED=1 go test ./...