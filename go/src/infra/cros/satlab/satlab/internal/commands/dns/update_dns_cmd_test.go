// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package dns

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"go.chromium.org/infra/cros/satlab/common/utils/executor"
)

// noopUpdateRecord is UpdateRecord with no side effects for testing other functionality
func noopUpdateRecord(_ context.Context, host string, address string) (string, error) {
	return "", nil
}

// fakeDHBGetter emulates fetching SatlabID with a constant value
func fakeDHBGetter(_ context.Context, _ executor.IExecCommander) (string, error) {
	return "123", nil
}

// TestRunCommandValidates tests various inputs to our runCmdInjected function
func TestRunCommandValidates(t *testing.T) {
	t.Parallel()

	type input struct {
		command         *updateDNSRun
		args            []string
		satlabIDFetcher DockerHostBoxIdentifierGetter
	}
	type output struct {
		errored bool
		command *updateDNSRun
	}
	type test struct {
		name   string
		input  input
		output output
	}

	tests := []test{{
		"happy path",
		input{&updateDNSRun{host: "satlab-123-eli", address: "127.0.0.1"}, make([]string, 0), fakeDHBGetter},
		output{false, &updateDNSRun{host: "satlab-123-eli", address: "127.0.0.1"}},
	}, {
		"no host",
		input{&updateDNSRun{address: "127.0.0.1"}, make([]string, 0), fakeDHBGetter},
		output{true, &updateDNSRun{host: "", address: "127.0.0.1"}},
	}, {
		"no address",
		input{&updateDNSRun{host: "satlab-123-eli"}, make([]string, 0), fakeDHBGetter},
		output{true, &updateDNSRun{host: "satlab-123-eli", address: ""}},
	}, {
		"positional args",
		input{&updateDNSRun{host: "satlab-123-eli", address: "127.0.0.1"}, []string{"hi"}, fakeDHBGetter},
		output{true, &updateDNSRun{host: "satlab-123-eli", address: "127.0.0.1"}},
	}, {
		"prepend host",
		input{&updateDNSRun{host: "eli", address: "127.0.0.1"}, make([]string, 0), fakeDHBGetter},
		output{false, &updateDNSRun{host: "satlab-123-eli", address: "127.0.0.1"}},
	}}

	ctx := context.Background()

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			i, o := tc.input, tc.output
			err := i.command.runCmdInjected(ctx, i.args, i.satlabIDFetcher, noopUpdateRecord)

			if o.errored != (err != nil) {
				t.Errorf("Testing(%+v) failed. Got error: %t, expected error: %t", tc, err, o.errored)
			}
			if diff := cmp.Diff(i.command.host, o.command.host); diff != "" {
				t.Errorf("Testing(%+v) failed with diff in host of command: %s", tc, diff)
			}
		})
	}
}

// fakeUpdateRecord produces a function that emulates UpdateRecord but stores the latest results in a records map passed in
func fakeUpdateRecord(records map[string]string) HostfileUpdater {
	return func(_ context.Context, host, address string) (string, error) {
		records[host] = address
		return "", nil
	}
}

// TestRunCmdUpdatesRecords tests that when we call `runCmdInjected` it calls UpdateRecord function with expected args
func TestRunCmdUpdatesRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	callMap := make(map[string]string)
	updateRecord := fakeUpdateRecord(callMap)
	cmd := &updateDNSRun{host: "satlab-123-eli", address: "127.0.0.1"}
	expectedCallMap := map[string]string{"satlab-123-eli": "127.0.0.1"}

	cmd.runCmdInjected(ctx, []string{}, fakeDHBGetter, updateRecord)

	if diff := cmp.Diff(callMap, expectedCallMap); diff != "" {
		t.Errorf("got diff %s for final state of records with input %+v", diff, cmd)
	}
}
