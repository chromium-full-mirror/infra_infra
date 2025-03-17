# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: test
test:
	go vet ./...
	go test ./...
	go build -o=consoleadmin ./cmd/consoleadmin
	go build -o=fleetconsoleserver ./cmd/fleetconsoleserver

.PHONY: build
build:
	go build -o=consoleadmin ./cmd/consoleadmin
	go build -o=fleetconsoleserver ./cmd/fleetconsoleserver

.PHONY: run
run:
	./fleetconsoleserver \
		-sqldb-connection-url=pgx://postgres@localhost:"$$(read -p'PG PORT:' port; printf '%s' "$$port")"/postgres \
		-sqldb-password-secret=devsecret-text://"$$(read -s -p'PG PASS:' password; printf '%s' "$$password")"
