// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"testing"

	"infra/cros/servo/errors"
)

func TestErrorChecking(t *testing.T) {
	var err error = &TabletModeCmdUnsupportedErr{E: errors.New("Some wrapped error")}

	if _, ok := err.(*TabletModeCmdUnsupportedErr); !ok { //nolint
		t.Error("Type cast didn't work")
	}
}
