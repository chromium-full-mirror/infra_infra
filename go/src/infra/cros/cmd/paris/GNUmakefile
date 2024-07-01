# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: paris
paris:
	@$(RM) -rf ./paris
	env CGO_ENABLED=0 go build -o ./paris -- .

.PHONY: test
test:
	go test ./...

.PHONY: check
check: test

.PHONY: clean
clean:
	$(RM) ./paris

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	go fmt ./...
