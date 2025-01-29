// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"infra/cros/cmd/cft/cros-test-finder/test_finder"
	"os"
)

func main() {
	os.Exit(test_finder.TestFinderInternal(context.Background()))
}
