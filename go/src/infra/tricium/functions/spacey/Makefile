# Copyright 2018 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: test clean

spacey:
	go build

test: spacey
	./spacey -input=test -output=out

clean:
	rm -rf out spacey
