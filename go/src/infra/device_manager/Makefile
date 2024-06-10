# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
APP_PATH := $(PROJECT_PATH)
GCP_PROJECT := fleet-device-manager-dev
GCP_REGION := us-central1
LUCI_AUTH_SERVICE := chrome-infra-auth-dev.appspot.com

define help_message

Helpers for managing Device Manager project in $(PROJECT_PATH)

For the main Device Lease service:
  gen: regenerate all go bindings
  build: build the Device Lease service image
  dev: run the Device Lease service locally

For running the service using Docker:
  build-docker-compose: build the Device Lease service stack with Docker Compose
  docker-service: build and run the Device Lease service as a Docker container
  docker-notifier: build and run the Notifier service as a Docker container
  docker-db: build and run the Postgres db as a Docker container
  docker-pubsub: build and run the PubSub emulator as a Docker container
endef

.PHONY: help
help:
	$(info $(help_message))

# Run Go tests
.PHONY: test
test:
	go test ./... -coverprofile=coverage.out

# Check code coverage
.PHONY: cover
cover:
	go tool cover -func=coverage.out

# Build service image
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o device-lease-service $(PROJECT_PATH)/cmd/device_lease_service
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o notifier-service $(PROJECT_PATH)/cmd/notifier_service

# Run service locally as a binary
.PHONY: dev
dev: build
	./device-lease-service \
	--auth-service-host=${LUCI_AUTH_SERVICE} \
	--grpc-addr=":50051" \
	--http-addr=":8800" \
	--admin-addr=":8900" \
	--db-host='localhost' \
	--db-port='5432' \
	--db-name='device_manager_db' \
	--db-user='postgres' \
	--db-password-secret="devsecret://cGFzc3dvcmQ"

# Build Docker compose service
.PHONY: build-docker-compose
build-docker-compose: build
	docker compose -f docker-compose.dev.yml build --no-cache


# Run Docker service locally using Docker Compose
.PHONY: docker-service
docker-service: build-docker-compose
	CURRENT_UID="$(id -u):$(id -g)" \
	docker compose -f docker-compose.dev.yml up \
	device_lease_service

# Run Docker notifier service locally using Docker Compose
.PHONY: docker-notifier
docker-notifier: build-docker-compose
	CURRENT_UID="$(id -u):$(id -g)" \
	docker compose -f docker-compose.dev.yml up \
	notifier_service

# Run Docker Postgres db locally using Docker Compose
.PHONY: docker-db
docker-db:
	CURRENT_UID="$(id -u):$(id -g)" \
	docker compose -f docker-compose.dev.yml up \
	device_manager_db

# Run Docker PubSub locally using Docker Compose
.PHONY: docker-pubsub
docker-pubsub:
	CURRENT_UID="$(id -u):$(id -g)" \
	docker compose -f docker-compose.dev.yml up \
	pubsub
