# Copyright 2021 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: default
default: build

.PHONY:
build:
	@go vet ./...
	@go test ./...
	go build ./...

.PHONY: test
test:
	go test ./...
	go test ../cmd/paris/...
	go test ../cmd/labpack/...
	go test ../../cmd/mallet/...


.PHONY: gen trees
gen:
	# Generate proto files
	@find . -name '*.pb.go' -delete
	go generate ./...
trees:
	# Generate config tree files
	RECOVERY_GENERATE_CONFIG_TREE=ON go test -run TestConfigTreeChanges
