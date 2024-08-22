// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registry

import (
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"strings"
	"testing"
)

func TestValidateTag(t *testing.T) {
	t.Parallel()

	ftt.Run("Works", t, func(t *ftt.Test) {
		assert.Loosely(t, ValidateTag("good-TAG-.123_456"), should.BeNil)
		assert.Loosely(t, ValidateTag(strings.Repeat("a", 128)), should.BeNil)

		assert.Loosely(t, ValidateTag(""), should.ErrLike("can't be empty"))
		assert.Loosely(t, ValidateTag("notascii\x02"), should.ErrLike("should match"))
		assert.Loosely(t, ValidateTag(":forbiddenchar"), should.ErrLike("should match"))
		assert.Loosely(t, ValidateTag(".noperiodinfront"), should.ErrLike("can't start with '.'"))
		assert.Loosely(t, ValidateTag("-nodashinfront"), should.ErrLike("can't start with '-'"))
		assert.Loosely(t, ValidateTag(strings.Repeat("a", 129)), should.ErrLike("can't have more than 128 characters"))
	})
}
