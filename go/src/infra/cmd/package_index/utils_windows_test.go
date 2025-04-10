//go:build windows
// +build windows

package main

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestConvertPathSlashesWindows(t *testing.T) {
	ftt.Run("Convert path with forward slashes", t, func(t *ftt.Test) {
		p := "\\test\\path\\"

		t.Run("On Windows", func(t *ftt.Test) {
			r := convertPathToForwardSlashes(p)

			t.Run("The path should have forward slashes", func(t *ftt.Test) {
				assert.Loosely(t, r, should.Equal("/test/path/"))
			})
		})
	})
}
