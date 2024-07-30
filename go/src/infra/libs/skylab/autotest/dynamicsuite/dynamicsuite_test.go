// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dynamicsuite_test

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"infra/libs/skylab/autotest/dynamicsuite"
)

func TestRequest(t *testing.T) {
	ftt.Run("Given a set of arguments", t, func(t *ftt.Test) {
		args := dynamicsuite.Args{
			Board: "board",
			Build: "build",
			Model: "model",
			Pool:  "pool",
			ReimageAndRunArgs: map[string]interface{}{
				"arg_1": 1,
				"arg_2": []string{"v1", "v2"},
			},
		}
		t.Run("a new request has correct properties", func(t *ftt.Test) {
			req, err := dynamicsuite.NewRequest(args)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			assert.Loosely(t, req.TaskSlices, should.HaveLength(1))
			expected := []string{
				"/usr/local/autotest/site_utils/run_suite.py",
				"--json_dump_postfix",
				"--build", "build",
				"--board", "board",
				"--model", "model",
				"--suite_name", "cros_test_platform",
				"--pool", "pool",
				"--suite_args_json", `{"args_dict_json":"{\"arg_1\":1,\"arg_2\":[\"v1\",\"v2\"]}"}`,
			}
			assert.Loosely(t, req.TaskSlices[0].Properties.Command, should.Resemble(expected))
		})
	})
}

func TestLegacyRequest(t *testing.T) {
	ftt.Run("Given a set of arguments with a legacy suite", t, func(t *ftt.Test) {
		args := dynamicsuite.Args{
			Board: "board",
			Build: "build",
			Model: "model",
			Pool:  "pool",
			ReimageAndRunArgs: map[string]interface{}{
				"arg_1": 1,
				"arg_2": []string{"v1", "v2"},
			},
			LegacySuite: "legacy_suite",
		}
		t.Run("a new request has correct properties", func(t *ftt.Test) {
			req, err := dynamicsuite.NewRequest(args)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, req, should.NotBeNil)
			assert.Loosely(t, req.TaskSlices, should.HaveLength(1))
			expected := []string{
				"/usr/local/autotest/site_utils/run_suite.py",
				"--json_dump_postfix",
				"--build", "build",
				"--board", "board",
				"--model", "model",
				"--suite_name", "legacy_suite",
				"--pool", "pool",
				"--suite_args_json", "{}",
			}
			assert.Loosely(t, req.TaskSlices[0].Properties.Command, should.Resemble(expected))
		})
	})
}
