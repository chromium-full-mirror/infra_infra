# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

build:
	go build ./...

deploy-staging:
	gae.py upload -x -f -A chromium-build-stats-staging --app-dir app

deploy-prod:
	gae.py upload -x -f -A chromium-build-stats --app-dir app

test:
	go test ./...
