// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"strings"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

func TestParseOwners(t *testing.T) {
	t.Parallel()

	ftt.Run(`ParseOwners`, t, func(t *ftt.Test) {
		t.Run(`Works`, func(t *ftt.Test) {
			actual, filtered, err := ParseOwners(strings.NewReader(`
# TEAM: team-email@chromium.org
someone@example.com

# Some comments

# OS: iOS
# COMPONENT: Some>Component
# Internal Component: b/components/1234
# WPT-NOTIFY: true`))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, filtered, should.Resemble([]string{
				"",
				"someone@example.com",
				"",
				"# Some comments",
				"",
			}))
			assert.Loosely(t, actual, should.Resemble(&dirmdpb.Metadata{
				TeamEmail: "team-email@chromium.org",
				Os:        dirmdpb.OS_IOS,
				Monorail: &dirmdpb.Monorail{
					Project:   "chromium",
					Component: "Some>Component",
				},
				Wpt: &dirmdpb.WPT{Notify: dirmdpb.Trinary_YES},
				Buganizer: &dirmdpb.Buganizer{
					ComponentId: 1234,
				},
			}))
		})

		t.Run(`ChromeOS`, func(t *ftt.Test) {
			actual, filtered, err := ParseOwners(strings.NewReader(`# OS: ChromeOS`))
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, filtered, should.Match([]string(nil)))
			assert.Loosely(t, actual.Os, should.Equal(dirmdpb.OS_CHROME_OS))
		})

	})
}
