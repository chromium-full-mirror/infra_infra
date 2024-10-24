// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSkipTFUpload(t *testing.T) {
	req := &api.InternalTestplan{
		SuiteInfo: &api.SuiteInfo{
			SuiteMetadata: &api.SuiteMetadata{
				ExecutionMetadata: &api.ExecutionMetadata{
					Args: []*api.Arg{{Flag: "foo", Value: "test"}},
				},
			},
		},
	}

	apu := &ANTSPublishUpdater{}
	apu.skipTFUpload(req)

	gotArgs := req.GetSuiteInfo().GetSuiteMetadata().GetExecutionMetadata().GetArgs()
	if len(gotArgs) != 2 {
		t.Errorf("Unexpected number of args: got %d want 2", len(gotArgs))
	}

	wantArg := &api.Arg{Flag: skipTFUpload, Value: "true"}
	if diff := cmp.Diff(gotArgs[1], wantArg, protocmp.Transform()); diff != "" {
		t.Errorf("Unexpected diff: %s", diff)
	}
}
