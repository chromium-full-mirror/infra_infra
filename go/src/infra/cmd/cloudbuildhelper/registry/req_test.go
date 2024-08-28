// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package registry

import (
	"testing"

	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	ftt.Run("IsManifestUnknown", t, func(t *ftt.Test) {
		manUnknownErr := &Error{
			Errors: []InnerError{
				{},
				{Code: "MANIFEST_UNKNOWN"},
			},
		}
		assert.Loosely(t, IsManifestUnknown(nil), should.BeFalse)
		assert.Loosely(t, IsManifestUnknown(&Error{}), should.BeFalse)
		assert.Loosely(t, IsManifestUnknown(manUnknownErr), should.BeTrue)
		assert.Loosely(t, IsManifestUnknown(errors.Annotate(manUnknownErr, "blah").Err()), should.BeTrue)
	})
}
