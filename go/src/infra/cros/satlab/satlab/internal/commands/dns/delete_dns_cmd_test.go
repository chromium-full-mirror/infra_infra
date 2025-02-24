// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dns

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestDeleteCommandValidates tests various inputs to our runCmdInjected function
func TestDeleteCommandValidates(t *testing.T) {
	t.Parallel()

	type input struct {
		command  *deleteDNSRun
		args     []string
		satlabID string
	}
	type output struct {
		errored bool
		command *deleteDNSRun
	}
	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{{
		"happy path",
		input{&deleteDNSRun{host: "satlab-123-eli"}, make([]string, 0), "123"},
		output{false, &deleteDNSRun{host: "satlab-123-eli"}},
	}, {
		"no host",
		input{&deleteDNSRun{}, make([]string, 0), "eli"},
		output{true, &deleteDNSRun{host: ""}},
	}, {
		"positional args",
		input{&deleteDNSRun{host: "satlab-123-eli"}, []string{"hi"}, "123"},
		output{true, &deleteDNSRun{host: "satlab-123-eli"}},
	}, {
		"prepend host",
		input{&deleteDNSRun{host: "eli"}, make([]string, 0), "123"},
		output{false, &deleteDNSRun{host: "satlab-123-eli"}},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			i, o := tc.input, tc.output
			err := i.command.validate(i.args, i.satlabID)

			if o.errored != (err != nil) {
				t.Errorf("Testing(%+v) failed. Got error: %t, expected error: %t", tc, err, o.errored)
			}
			if diff := cmp.Diff(i.command.host, o.command.host); diff != "" {
				t.Errorf("Testing(%+v) failed with diff in host of command: %s", tc, diff)
			}
		})
	}
}
