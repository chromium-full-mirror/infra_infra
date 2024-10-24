// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cookflags

import (
	"flag"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestPropertyFlag(t *testing.T) {
	t.Parallel()

	ftt.Run("PropertyFlag", t, func(t *ftt.Test) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.Usage = func() {}
		f := PropertyFlag{}
		fs.Var(&f, "prop", "use the flag")

		t.Run("works", func(t *ftt.Test) {
			assert.Loosely(t, fs.Parse([]string{"-prop", `{"stuff": "yes"}`}), should.BeNil)

			assert.Loosely(t, f, should.Resemble(PropertyFlag{
				"stuff": "yes",
			}))

			assert.Loosely(t, f.String(), should.Match(`{"stuff":"yes"}`))
		})

		t.Run("breaks appropriately", func(t *ftt.Test) {
			assert.Loosely(t, fs.Parse([]string{"-prop", `wat`}), should.ErrLike(
				"invalid character 'w'"))
			assert.Loosely(t, fs.Parse([]string{"-prop", `{"stuff": "yes"} extra crap`}), should.ErrLike(
				"invalid character 'e' after top-level value"))
		})
	})
}
