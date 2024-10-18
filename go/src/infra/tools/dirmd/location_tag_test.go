// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/resultdb/pbutil"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"

	dirmdpb "infra/tools/dirmd/proto"
)

func TestLocationTag(t *testing.T) {
	t.Parallel()

	ftt.Run(`ToLocationTags`, t, func(t *ftt.Test) {
		mapping := &dirmdpb.Mapping{
			Dirs: map[string]*dirmdpb.Metadata{
				".": {
					TeamEmail: "chromium-review@chromium.org",
					Os:        dirmdpb.OS_LINUX,
				},
				"subdir": {
					TeamEmail: "team-email@chromium.org",
					Monorail: &dirmdpb.Monorail{
						Project:   "chromium",
						Component: "Some>Component",
					},
					Resultdb: &dirmdpb.ResultDB{
						Tags: []string{
							"feature:read-later",
							"feature:another-one",
						},
					},
				},
				"subdir_with_owners": {
					TeamEmail: "team-email@chromium.org",
					Monorail: &dirmdpb.Monorail{
						Project:   "chromium",
						Component: "Some>Component",
					},
				},
			},
			Files: map[string]*dirmdpb.Metadata{
				"subdir/test.txt": {
					Monorail: &dirmdpb.Monorail{
						Project:   "chromium",
						Component: "Some>File>Component",
					},
					BuganizerPublic: &dirmdpb.Buganizer{
						ComponentId: 123456,
					},
				},
			},
		}
		tags, err := ToLocationTags((*Mapping)(mapping))
		for _, dir := range tags.Dirs {
			pbutil.SortStringPairs(dir.Tags)
		}

		expected := &sinkpb.LocationTags_Repo{
			Dirs: map[string]*sinkpb.LocationTags_Dir{
				".": {
					Tags: pbutil.StringPairs(
						"os", dirmdpb.OS_LINUX.String(),
						"team_email", "chromium-review@chromium.org"),
				},
				"subdir": {
					Tags: pbutil.StringPairs(
						"feature", "another-one",
						"feature", "read-later",
						"monorail_component", "Some>Component",
						"team_email", "team-email@chromium.org"),
					BugComponent: &resultpb.BugComponent{
						System: &resultpb.BugComponent_Monorail{
							Monorail: &resultpb.MonorailComponent{
								Project: "chromium",
								Value:   "Some>Component",
							},
						},
					},
				},
				"subdir_with_owners": {
					Tags: pbutil.StringPairs(
						"monorail_component", "Some>Component",
						"team_email", "team-email@chromium.org"),
					BugComponent: &resultpb.BugComponent{
						System: &resultpb.BugComponent_Monorail{
							Monorail: &resultpb.MonorailComponent{
								Project: "chromium",
								Value:   "Some>Component",
							},
						},
					},
				},
			},
			Files: map[string]*sinkpb.LocationTags_File{
				"subdir/test.txt": {
					Tags: pbutil.StringPairs(
						"monorail_component", "Some>File>Component",
						"public_buganizer_component", "123456",
					),
					BugComponent: &resultpb.BugComponent{
						System: &resultpb.BugComponent_IssueTracker{
							IssueTracker: &resultpb.IssueTrackerComponent{
								ComponentId: 123456,
							},
						},
					},
				},
			},
		}

		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, tags, should.Match(expected))
	})
}
