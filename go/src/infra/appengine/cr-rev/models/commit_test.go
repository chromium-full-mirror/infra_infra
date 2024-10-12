package models

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestCommit(t *testing.T) {
	ftt.Run("Compare repos", t, func(t *ftt.Test) {
		t.Run("Empty repo", func(t *ftt.Test) {
			c1 := Commit{}
			c2 := Commit{}
			assert.Loosely(t, c1.SameRepoAs(c1), should.BeTrue)
			assert.Loosely(t, c1.SameRepoAs(c2), should.BeTrue)
		})
		t.Run("Identical", func(t *ftt.Test) {
			c1 := Commit{Host: "foo", Repository: "bar"}
			c2 := Commit{Host: "foo", Repository: "bar"}
			assert.Loosely(t, c1.SameRepoAs(c2), should.BeTrue)
		})
		t.Run("Different hosts", func(t *ftt.Test) {
			c1 := Commit{Host: "foo", Repository: "bar"}
			c2 := Commit{Host: "baz", Repository: "bar"}
			assert.Loosely(t, c1.SameRepoAs(c2), should.BeFalse)
		})
	})
}
