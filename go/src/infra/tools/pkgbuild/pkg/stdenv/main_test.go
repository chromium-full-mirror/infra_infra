// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package stdenv

import (
	"os"
	"testing"

	"go.chromium.org/luci/common/exec/execmock"
)

func TestMain(m *testing.M) {
	execmock.Intercept(true)
	os.Exit(m.Run())
}
