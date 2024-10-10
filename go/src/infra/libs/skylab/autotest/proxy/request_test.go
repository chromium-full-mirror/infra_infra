// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package proxy_test

import (
	"strings"
	"testing"
	"time"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/libs/skylab/autotest/proxy"
)

func TestRunSuite(t *testing.T) {
	ftt.Run("When creating a request for a set of RunSuite arguments", t, func(t *ftt.Test) {
		args := proxy.RunSuiteArgs{
			Board:           "foo-board",
			Build:           "foo-build",
			FirmwareRWBuild: "foo-rw-build",
			FirmwareROBuild: "foo-ro-build",
			Model:           "foo-model",
			Pool:            "foo-pool",
			Priority:        11,
			SuiteName:       "foo-suite",
			SuiteArgs:       map[string]int{"arg1": 1},
			Timeout:         2 * time.Hour,
		}
		req, err := proxy.NewRunSuite(args)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, req, should.NotBeNil)
		assert.Loosely(t, req.TaskSlices, should.HaveLength(1))
		t.Run("the correct commandline args and slice properties are present.", func(t *ftt.Test) {
			slice := req.TaskSlices[0]
			flatCmd := strings.Join(slice.Properties.Command, " ")
			assert.Loosely(t, flatCmd, should.ContainSubstring("--board foo-board"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--build foo-build"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--firmware_rw_build foo-rw-build"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--firmware_ro_build foo-ro-build"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--model foo-model"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--pool foo-pool"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--priority 11"))
			assert.Loosely(t, flatCmd, should.ContainSubstring("--suite_name foo-suite"))

			assert.Loosely(t, slice.Properties.Command, should.Contain("--suite_args_json"))
			for i, v := range slice.Properties.Command {
				if v == "--suite_args_json" {
					assert.Loosely(t, slice.Properties.Command[i+1], should.Equal("{\"arg1\":1}"))
					break
				}
			}

			assert.Loosely(t, slice.ExpirationSecs, should.Equal(7800))
			assert.Loosely(t, slice.Properties.ExecutionTimeoutSecs, should.Equal(7800))
		})
	})
}
