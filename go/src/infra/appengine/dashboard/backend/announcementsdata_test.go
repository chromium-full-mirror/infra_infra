// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package backend

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

	"go.chromium.org/luci/common/clock/testclock"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/gae/service/datastore"

	dashpb "infra/appengine/dashboard/api/dashboard"
)

var chickenAnn = &Announcement{Message: "chicken is missing", Creator: "farmer1"}
var cowAnn = &Announcement{Message: "cow is missing", Creator: "farmer2"}
var foxAnn = &Announcement{Message: "fox is missing", Creator: "farmer3"}

var chickBarnPlat = &Platform{Name: "barn"}
var chickHousePlat = &Platform{Name: "house", URLPaths: []string{"kitchen/*"}}
var chickenPlats = []*Platform{chickBarnPlat, chickHousePlat}

var cowBarnPlat = &Platform{Name: "barn"}
var cowFieldPlat = &Platform{Name: "field"}
var cowPlats = []*Platform{cowBarnPlat, cowFieldPlat}

var foxPlats = []*Platform{{Name: "forest"}}

var closer = "closer@test.com"

func retireAnnouncementTesting(ctx context.Context, annProto *dashpb.Announcement) {
	recentTS, _ := ptypes.TimestampProto(testclock.TestRecentTimeUTC.Round(time.Microsecond))
	RetireAnnouncement(ctx, annProto.Id, closer)
	annProto.Retired = true
	annProto.Closer = closer
	annProto.EndTime = recentTS
}

func TestConvertAnnouncement(t *testing.T) {
	startTS := int64(764797594)
	endTS := int64(764883994)
	testCases := []struct {
		ann       Announcement
		platforms []*Platform
		expected  *dashpb.Announcement
	}{
		{
			ann: Announcement{
				ID:        1234,
				Retired:   true,
				Message:   "CQ issues",
				Creator:   "trooper",
				StartTime: time.Unix(startTS, 0),
				EndTime:   time.Unix(endTS, 0),
			},
			platforms: []*Platform{
				{Name: "monorail"},
				{
					Name:            "gerrit",
					URLPaths:        []string{"c/infra/infra/*", "src/*"},
					AnnouncementKey: &datastore.Key{},
				},
			},
			expected: &dashpb.Announcement{
				Id:             1234,
				MessageContent: "CQ issues",
				Creator:        "trooper",
				Retired:        true,
				StartTime:      &timestamp.Timestamp{Seconds: startTS},
				EndTime:        &timestamp.Timestamp{Seconds: endTS},
				Platforms: []*dashpb.Platform{
					{Name: "monorail"},
					{
						Name:     "gerrit",
						UrlPaths: []string{"c/infra/infra/*", "src/*"},
					},
				},
			},
		},
		{
			ann: Announcement{
				ID:        13,
				StartTime: time.Unix(startTS, 0),
				EndTime:   time.Unix(endTS, 0),
			},
			expected: &dashpb.Announcement{
				Id:             13,
				MessageContent: "",
				Creator:        "",
				Retired:        false,
				StartTime:      &timestamp.Timestamp{Seconds: startTS},
				EndTime:        &timestamp.Timestamp{Seconds: endTS},
				Platforms:      []*dashpb.Platform{},
			},
		},
	}
	for i, tc := range testCases {
		actual, err := tc.ann.ToProto(tc.platforms)
		if err != nil {
			t.Errorf("%d: unexpected error - %s", i, err)
		}
		if !reflect.DeepEqual(tc.expected, actual) {
			t.Errorf("%d: expected %+v, found %+v", i, tc.expected, actual)
		}
	}
}

func TestCreateLiveAnnouncement(t *testing.T) {
	ftt.Run("CreateLiveAnnouncement", t, func(t *ftt.Test) {
		ctx := newTestContext()
		t.Run("successful Announcement creator", func(t *ftt.Test) {
			platforms := []*Platform{
				{
					Name:     "monorail",
					URLPaths: []string{"p/chromium/*"},
				},
				{
					Name:     "som",
					URLPaths: []string{"c/infra/infra/*"},
				},
			}
			ann, err := CreateLiveAnnouncement(
				ctx, "Cow cow cow", "cowman", platforms)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, ann.Platforms, should.HaveLength(2))
			// Test getting platforms and announcement does not result
			// in error and they were saved correctly in datastore.
			annKey := datastore.NewKey(ctx, "Announcement", "", ann.Id, nil)
			for _, platform := range platforms {
				pKey := datastore.NewKey(ctx, "Platform", platform.Name, 0, annKey)
				existsR, _ := datastore.Exists(ctx, pKey)
				assert.Loosely(t, existsR.All(), should.BeTrue)
			}
			announcement := &Announcement{ID: ann.Id}
			err = datastore.Get(ctx, announcement)
			assert.Loosely(t, err, should.BeNil)
			expected := &Announcement{
				ID:            ann.Id,
				Message:       "Cow cow cow",
				Creator:       "cowman",
				StartTime:     testclock.TestRecentTimeUTC.Round(time.Microsecond),
				PlatformNames: []string{"monorail", "som"},
			}
			assert.Loosely(t, expected, should.Match(announcement))
		})
	})
}

func TestSearchAnnouncements(t *testing.T) {
	ctx := newTestContext()
	foxProto, _ := CreateLiveAnnouncement(ctx, foxAnn.Message, foxAnn.Creator, foxPlats)
	retireAnnouncementTesting(ctx, foxProto)

	cowProto, _ := CreateLiveAnnouncement(ctx, cowAnn.Message, cowAnn.Creator, cowPlats)
	chickenProto, _ := CreateLiveAnnouncement(ctx, chickenAnn.Message, chickenAnn.Creator, chickenPlats)
	ftt.Run("SearchAnnouncements live", t, func(t *ftt.Test) {

		t.Run("get all live announcements", func(t *ftt.Test) {
			anns, err := SearchAnnouncements(ctx, "", false, -1, -1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{cowProto, chickenProto}))
		})
		t.Run("get live announcements for house", func(t *ftt.Test) {
			anns, err := SearchAnnouncements(ctx, "house", false, -1, -1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{chickenProto}))
		})
		t.Run("get live announcements for barn", func(t *ftt.Test) {
			anns, err := SearchAnnouncements(ctx, "barn", false, -1, -1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{cowProto, chickenProto}))
		})
	})
	ftt.Run("SearchAnnouncements retired", t, func(t *ftt.Test) {
		retireAnnouncementTesting(ctx, cowProto)
		t.Run("get all retired announcements", func(t *ftt.Test) {
			anns, err := SearchAnnouncements(ctx, "", true, -1, -1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{foxProto, cowProto}))
		})
		t.Run("get limited and offset retired announcements", func(t *ftt.Test) {
			retireAnnouncementTesting(ctx, chickenProto)
			anns, err := SearchAnnouncements(ctx, "", true, 3, 1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{cowProto, chickenProto}))
			anns, err = SearchAnnouncements(ctx, "", true, 1, -1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{foxProto}))
		})
		t.Run("get retired announcements for field", func(t *ftt.Test) {
			anns, err := SearchAnnouncements(ctx, "field", true, -1, -1)
			assert.Loosely(t, err, should.BeNil)
			assert.Loosely(t, anns, should.Match([]*dashpb.Announcement{cowProto}))
		})
	})
}

func TestRetireAnnouncement(t *testing.T) {
	ctx := newTestContext()
	cowProto, _ := CreateLiveAnnouncement(ctx, cowAnn.Message, cowAnn.Creator, cowPlats)
	recentTime := testclock.TestRecentTimeUTC.Round(time.Microsecond)
	ftt.Run("RetireAnnouncement", t, func(t *ftt.Test) {
		err := RetireAnnouncement(ctx, cowProto.Id, closer)
		assert.Loosely(t, err, should.BeNil)
		announcement := &Announcement{ID: cowProto.Id}
		datastore.Get(ctx, announcement)
		assert.Loosely(t, announcement.Retired, should.BeTrue)
		assert.Loosely(t, announcement.Closer, should.Equal(closer))
		assert.Loosely(t, announcement.EndTime, should.Match(recentTime))
	})
}
