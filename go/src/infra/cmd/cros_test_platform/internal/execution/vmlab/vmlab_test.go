// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vmlab

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestConvertBuilderName(t *testing.T) {
	ftt.Run("success", t, func(t *ftt.Test) {
		want := "test_runner_gce"
		got := ConvertBuilderName("test_runner")
		assert.Loosely(t, got, should.Equal(want))
	})

	ftt.Run("success non prod", t, func(t *ftt.Test) {
		want := "test_runner_gce-staging"
		got := ConvertBuilderName("test_runner-staging")
		assert.Loosely(t, got, should.Equal(want))
	})

	ftt.Run("ignore already converted", t, func(t *ftt.Test) {
		want := "test_runner_gce"
		got := ConvertBuilderName("test_runner_gce")
		assert.Loosely(t, got, should.Equal(want))
	})

	ftt.Run("ignore unknown", t, func(t *ftt.Test) {
		want := "trv3"
		got := ConvertBuilderName("trv3")
		assert.Loosely(t, got, should.Equal(want))
	})

	ftt.Run("ignore empty", t, func(t *ftt.Test) {
		want := ""
		got := ConvertBuilderName("")
		assert.Loosely(t, got, should.Equal(want))
	})
}

func TestEligible(t *testing.T) {
	var boards = []string{"betty", "reven-vmtest", "amd64-generic"}
	for _, board := range boards {
		ftt.Run("experiment not enabled", t, func(t *ftt.Test) {
			got := eligible(board, []string{"exp1", "exp2"})
			assert.Loosely(t, got, should.BeFalse)
		})

		ftt.Run("experiment enabled", t, func(t *ftt.Test) {
			got := eligible(board, []string{"exp1", "chromeos.cros_infra_config.vmlab.launch", "exp2"})
			assert.Loosely(t, got, should.BeTrue)
		})
	}
	ftt.Run("unsupported board", t, func(t *ftt.Test) {
		got := eligible("anotherboard", []string{"chromeos.cros_infra_config.vmlab.launch"})
		assert.Loosely(t, got, should.BeFalse)
	})

	ftt.Run("unsupported board and no experiment", t, func(t *ftt.Test) {
		got := eligible("anotherboard", nil)
		assert.Loosely(t, got, should.BeFalse)
	})
}
