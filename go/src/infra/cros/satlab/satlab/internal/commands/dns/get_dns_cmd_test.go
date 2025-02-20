// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dns

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

func TestRunGetCmdInjected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fakeContents := func(_ context.Context, _ executor.IExecCommander) (string, error) {
		return "content", nil
	}
	out := new(bytes.Buffer)
	expectedOut := "Satlab internal DNS:\ncontent\n"
	cmd := getDNSRun{}

	cmd.runCmdInjected(ctx, out, fakeContents)

	if diff := cmp.Diff(out.String(), expectedOut); diff != "" {
		t.Errorf("unexpected diff in output: %s", diff)
	}
}
