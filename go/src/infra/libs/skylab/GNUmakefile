# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: default
default: test

# quick test running
.PHONY: test
test:
	go test ./...
	go test ../fleet/...
	go test ../../unifiedfleet/...
	go test ../../cros/cmd/ctpv2-filters/test-finder/...
