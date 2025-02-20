// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dirmd

import (
	"fmt"

	"go.chromium.org/luci/common/data/strpair"
	"go.chromium.org/luci/resultdb/pbutil"
	resultpb "go.chromium.org/luci/resultdb/proto/v1"
	sinkpb "go.chromium.org/luci/resultdb/sink/proto/v1"

	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

func createTag(md *dirmdpb.Metadata) []*resultpb.StringPair {
	var tags []*resultpb.StringPair

	if md.GetMonorail().GetComponent() != "" {
		tags = append(tags, pbutil.StringPair("monorail_component", md.Monorail.Component))
	}

	if md.GetOs() != dirmdpb.OS_OS_UNSPECIFIED {
		tags = append(tags, pbutil.StringPair("os", md.Os.String()))
	}

	if md.GetTeamEmail() != "" {
		tags = append(tags, pbutil.StringPair("team_email", md.TeamEmail))
	}

	if len(md.GetResultdb().GetTags()) > 0 {
		tags = append(tags, pbutil.FromStrpairMap(strpair.ParseMap(md.Resultdb.Tags))...)
	}

	if md.GetBuganizerPublic().GetComponentId() != 0 {
		tags = append(tags, pbutil.StringPair("public_buganizer_component", fmt.Sprint(md.BuganizerPublic.ComponentId)))
	}

	return tags
}

func extractBugComponent(md *dirmdpb.Metadata) *resultpb.BugComponent {
	if md.GetBuganizerPublic().GetComponentId() != 0 {
		return &resultpb.BugComponent{
			System: &resultpb.BugComponent_IssueTracker{
				IssueTracker: &resultpb.IssueTrackerComponent{
					ComponentId: md.BuganizerPublic.ComponentId,
				},
			},
		}
	}
	if md.GetBuganizer().GetComponentId() != 0 {
		return &resultpb.BugComponent{
			System: &resultpb.BugComponent_IssueTracker{
				IssueTracker: &resultpb.IssueTrackerComponent{
					ComponentId: md.Buganizer.ComponentId,
				},
			},
		}
	}
	if md.GetMonorail().GetComponent() != "" && md.GetMonorail().GetProject() != "" {
		return &resultpb.BugComponent{
			System: &resultpb.BugComponent_Monorail{
				Monorail: &resultpb.MonorailComponent{
					Project: md.Monorail.Project,
					Value:   md.Monorail.Component,
				},
			},
		}
	}

	return nil
}

// ToLocationTags converts all dir metadata to test location tags.
func ToLocationTags(mapping *Mapping) (*sinkpb.LocationTags_Repo, error) {
	dirs := map[string]*sinkpb.LocationTags_Dir{}
	for k, md := range mapping.Dirs {
		tags := createTag(md)
		component := extractBugComponent(md)

		dirs[k] = &sinkpb.LocationTags_Dir{
			Tags:         tags,
			BugComponent: component,
		}
	}

	files := map[string]*sinkpb.LocationTags_File{}
	for k, md := range mapping.Files {
		tags := createTag(md)
		component := extractBugComponent(md)

		files[k] = &sinkpb.LocationTags_File{
			Tags:         tags,
			BugComponent: component,
		}
	}
	return &sinkpb.LocationTags_Repo{
		Dirs:  dirs,
		Files: files,
	}, nil
}
