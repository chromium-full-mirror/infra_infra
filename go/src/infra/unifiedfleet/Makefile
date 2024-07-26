# Copyright 2020 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
APP_PATH := $(PROJECT_PATH)
GCP_PROJECT_DEV = unified-fleet-system-dev

define help_message
Helpers for managing ufs project in $(PROJECT_PATH):
  gen: regenerate all go bindings
  test: run all unittests
  cover: generate code coverage report
  build: build the main ufs service
  build-dumper: build the dumper service
  dev: run the main ufs service locally
  dev-dumper: run the dumper service locally
endef

.PHONY: default
default: vet test build

.PHONY: help
help:
	$(info $(help_message))

# Generate bindings
.PHONY: gen
gen:
	go generate ./...

# Run go vet
vet:
	go vet -copylocks=false ./...

# Run unittests
.PHONY: test
test:
	go test -fullpath ./... -coverprofile=coverage.out

# Check Code coverage
.PHONY: cover
cover:
	go tool cover -func=coverage.out

.PHONY: build
build:
	go build -o ufs-service $(PROJECT_PATH)/cmd/ufs-service

.PHONY: build-dumper
build-dumper:
	go build -o dumper $(PROJECT_PATH)/cmd/dumper

# Run service locally
.PHONY: dev
dev: build
	./ufs-service --cloud-project="unified-fleet-system-dev" --auth-service-host="chrome-infra-auth-dev.appspot.com"

.PHONY: dev-dumper
dev-dumper: build-dumper
	./dumper --cloud-project="unified-fleet-system-dev" -http-addr="127.0.0.1:8800" -admin-addr="127.0.0.1:8889"
