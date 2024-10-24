// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"os/exec"
	"testing"
)

func TestPanicIf(t *testing.T) {
	t.Parallel()

	ftt.Run("PanicIf", t, func(t *ftt.Test) {

		t.Run("does not panic on false", func(t *ftt.Test) {
			f := func() { PanicIf(false, "test message") }

			assert.Loosely(t, f, should.NotPanic)
		})

		t.Run("panics on true", func(t *ftt.Test) {
			f := func() { PanicIf(true, "test message: %s", "foo") }

			assert.Loosely(t, f, should.PanicLike("test message: foo"))
		})

	})
}

func TestPanicOnError(t *testing.T) {
	t.Parallel()

	ftt.Run("PanicOnError", t, func(t *ftt.Test) {

		t.Run("does not panic with no error", func(t *ftt.Test) {
			var err error

			f := func() { PanicOnError(err) }

			assert.Loosely(t, f, should.NotPanic)
		})

		t.Run("panics on error", func(t *ftt.Test) {
			err := errors.New("test error")

			f := func() { PanicOnError(err) }

			assert.Loosely(t, f, should.PanicLike("test error"))
		})

		t.Run("includes stderr for exec.ExitError", func(t *ftt.Test) {
			err := &exec.ExitError{Stderr: []byte("test stderr")}

			f := func() { PanicOnError(err) }

			assert.Loosely(t, f, should.PanicLike("test stderr"))
		})

	})
}
