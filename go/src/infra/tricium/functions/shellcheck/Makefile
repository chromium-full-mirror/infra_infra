# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

SRCS := $(wildcard *.go runner/*.go)

.PHONY: build-for-deployment test clean shellcheck_subtool

build-for-deployment: $(SRCS) shellcheck_subtool
	GOOS=linux GOARCH=amd64 go build -o shellcheck_tricium

all: shellcheck_tricium shellcheck_subtool

shellcheck_tricium: $(SRCS)
	go build -o shellcheck_tricium

shellcheck_subtool:
  # Note: `export` (not `ensure`) to avoid symlinks into a .cipd subfolder.
	cipd export -ensure-file shellcheck_ensure -root shellcheck_subtool

test: shellcheck_subtool
	go test .

testrun: shellcheck_tricium shellcheck_subtool
	./shellcheck_tricium -input=testdata -output=testout -path_filters="*.sh" -enable=require-variable-braces

clean:
	rm -rf shellcheck_tricium shellcheck_subtool testout
