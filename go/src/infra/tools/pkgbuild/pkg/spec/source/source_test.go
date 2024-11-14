// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package source

import (
	"context"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"infra/tools/pkgbuild/pkg/spec"
)

func TestSource(t *testing.T) {
	ftt.Run("source", t, func(t *ftt.Test) {
		ctx := context.Background()

		dir := t.TempDir() // Empty dir
		creates := []*spec.Spec_Create{{
			PlatformRe: "^somewhere$",
			Source: &spec.Spec_Create_Source{
				Method: &spec.Spec_Create_Source_Url{
					Url: &spec.UrlSource{
						DownloadUrl: "https://zlib.net/fossils/zlib-1.2.12.tar.gz",
						Version:     "1.2.12",
					},
				},
			},
		}}
		expected := &SourceInfo{
			Version: "1.2.12",
			Source: &SourceInfo_Http{
				Http: &SourceInfo_HTTP{
					Url: []string{"https://zlib.net/fossils/zlib-1.2.12.tar.gz"},
					Ext: ".tar.gz",
				},
			},
		}

		s := LoadSource(dir, creates)
		assert.Loosely(t, s.GetInfo("somewhere"), should.BeNil) // not locked && not updated

		err := s.UpdateInfo(ctx, []string{"somewhere", "elsewhere"}, NewResolver())
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, s.GetInfo("somewhere"), should.Match(expected)) // updated
		assert.Loosely(t, s.GetInfo("elsewhere"), should.BeNil)

		assert.Loosely(t, LoadSource(dir, creates).GetInfo("somewhere"), should.BeNil) // load again without persisted

		err = s.PersistInfo()
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, LoadSource(dir, creates).GetInfo("somewhere"), should.Match(expected)) // locked
	})
}
