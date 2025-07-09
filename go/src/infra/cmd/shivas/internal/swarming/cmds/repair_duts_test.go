// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tasks

import (
	"bytes"
	"io"
	"testing"

	"github.com/maruel/subcommands"

	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/libs/skylab/buildbucket"
)

func TestGetBuilderAndTaskName(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                string
		cmd                 *repairDuts
		expectedBuilderName string
		expectedTaskName    string
	}{
		{
			"normal repair job",
			&repairDuts{
				bbBuilder: "repair",
			},
			"repair",
			string(buildbucket.Recovery),
		},
		{
			"verify job",
			&repairDuts{
				bbBuilder:  "repair",
				onlyVerify: true,
			},
			"verify",
			string(buildbucket.Recovery),
		},
		{
			"deep repair job",
			&repairDuts{
				bbBuilder:  "repair",
				deepRepair: true,
			},
			"repair",
			string(buildbucket.DeepRecovery),
		},
		{
			"Verify with deep-repair enabled",
			&repairDuts{
				bbBuilder:  "repair",
				onlyVerify: true,
				deepRepair: true,
			},
			"verify",
			string(buildbucket.DeepRecovery),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualBuilderName, actualTaskName := tc.cmd.getBuilderAndTaskName()
			if actualBuilderName != tc.expectedBuilderName {
				t.Errorf("unexpected buildername %s (expected %s) for cmd %v", actualBuilderName, tc.expectedBuilderName, tc.cmd)
			}

			if actualTaskName != tc.expectedTaskName {
				t.Errorf("unexpected taskname %s (expected %s) for cmd %v", actualTaskName, tc.expectedTaskName, tc.cmd)
			}
		})
	}
}

type mockApplication struct {
	subcommands.Application
	stderr bytes.Buffer
	stdout bytes.Buffer
}

func (m *mockApplication) GetErr() io.Writer {
	return &m.stderr
}
func (m *mockApplication) GetOut() io.Writer {
	return &m.stdout
}
func (m *mockApplication) GetName() string {
	return "shivas"
}

func TestInnerRun(t *testing.T) {
	t.Parallel()

	// Test case: no DUT names provided
	// expecting an error as no DUT names are provided.
	t.Run("no dut names", func(t *testing.T) {
		c := &repairDuts{}
		app := &mockApplication{}
		args := []string{} // no dut names

		err := c.innerRun(app, args, nil)
		assert.ErrIsLike(t, err, "at least one hostname has to be provided")
		assert.That(t, app.stderr.String(), should.Equal(""))
		assert.That(t, app.stdout.String(), should.Equal(""))
	})

	// Note: assertions for stdout and mock expectations are not added
	// as the test environment is not set up to mock various dependencies for auth.
	// Integration will be comprehensively tested manually with local Shivas build
}
