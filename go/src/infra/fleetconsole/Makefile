# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

LOCAL_DB_CONNECTION_URL := pgx://postgres@localhost:5432/postgres
LOCAL_DB_PASSWORD_SECRET := devsecret-text://password
DEV_DB_CONNECTION_URL := pgx://postgres@localhost:5432/console_db
DEV_DB_PASSWORD_SECRET := devsecret-text://"$$(gcloud secrets versions access latest --project="fleet-console-dev" --secret=db-password)"

RUN_EXEC_DB_CONNECTION_VALUE := pgx://postgres@localhost:$$(read -p 'PG PORT: ' port; printf '%s' "$$port")/postgres
RUN_EXEC_DB_PASSWORD_VALUE := devsecret-text://$$(read -s -p 'PG PASS: ' password; printf '%s' "$$password")

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
	go run ./cmd/fleetconsoleserver \
		-sqldb-connection-url=$(RUN_EXEC_DB_CONNECTION_VALUE) \
		-sqldb-password-secret=$(RUN_EXEC_DB_PASSWORD_VALUE)

.PHONY: run-local-db
run-local-db:
	go run ./cmd/fleetconsoleserver \
		-sqldb-connection-url=$(LOCAL_DB_CONNECTION_URL) \
		-sqldb-password-secret=$(LOCAL_DB_PASSWORD_SECRET)

.PHONY: run-dev-db
run-dev-db:
	go run ./cmd/fleetconsoleserver \
		-sqldb-connection-url=$(DEV_DB_CONNECTION_URL) \
		-sqldb-password-secret=$(DEV_DB_PASSWORD_SECRET)

.PHONY: exec
exec:
	./fleetconsoleserver \
		-sqldb-connection-url=$(RUN_EXEC_DB_CONNECTION_VALUE) \
		-sqldb-password-secret=$(RUN_EXEC_DB_PASSWORD_VALUE)

.PHONY: exec-local-db
exec-local-db:
	./fleetconsoleserver \
		-sqldb-connection-url=$(LOCAL_DB_CONNECTION_URL) \
		-sqldb-password-secret=$(LOCAL_DB_PASSWORD_SECRET)

.PHONY: exec-dev-db
exec-dev-db:
	./fleetconsoleserver \
		-sqldb-connection-url=$(DEV_DB_CONNECTION_URL) \
		-sqldb-password-secret=$(DEV_DB_PASSWORD_SECRET)
