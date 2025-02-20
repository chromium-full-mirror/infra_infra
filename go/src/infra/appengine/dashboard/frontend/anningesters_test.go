// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"reflect"
	"testing"

	dashpb "go.chromium.org/infra/appengine/dashboard/api/dashboard"
	"go.chromium.org/infra/appengine/dashboard/backend"
)

func TestIngestPlatforms(t *testing.T) {
	testCases := []struct {
		platforms []*dashpb.Platform
		expected  []*backend.Platform
	}{
		{
			platforms: []*dashpb.Platform{},
			expected:  []*backend.Platform{},
		},
		{
			platforms: []*dashpb.Platform{
				{
					Name:     "monorail",
					UrlPaths: []string{"p/chromium/*", "p/monorail/*"},
				},
				{Name: "som"},
			},
			expected: []*backend.Platform{
				{
					Name:     "monorail",
					URLPaths: []string{"p/chromium/*", "p/monorail/*"},
				},
				{Name: "som"},
			},
		},
	}
	for i, tc := range testCases {
		actual := IngestPlatforms(tc.platforms)
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("%d: expected %+v, found %+v", i, tc.expected, actual)
		}
	}
}
