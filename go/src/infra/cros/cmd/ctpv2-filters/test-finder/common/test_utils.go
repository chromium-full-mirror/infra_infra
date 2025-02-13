// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common is the common package.
package common

import (
	"google.golang.org/protobuf/types/known/durationpb"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// GenerateTestMetadata creates TestCaseMetadata to be used in unittests.
func GenerateTestMetadata() []*api.TestCaseMetadata {
	tcm := []*api.TestCaseMetadata{}
	tags := []*api.TestCase_Tag{}

	tags = append(tags, &api.TestCase_Tag{
		Value: "moblydemo",
	})
	tcm = append(tcm, &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Name: "fakeTest1",
			Id: &api.TestCase_Id{
				Value: "fakeTest1",
			},
			Tags: tags,
		},
	})
	return tcm
}

// GenerateSuiteInfo creates SuiteInfo to be used in unittests.
func GenerateSuiteInfo() *api.SuiteInfo {
	SuiteInfo := &api.SuiteInfo{
		SuiteRequest: &api.SuiteRequest{
			SuiteRequest: &api.SuiteRequest_TestSuite{
				TestSuite: &api.TestSuite{
					Name: "wifi_endtoend",
					Spec: &api.TestSuite_TestCaseTagCriteria_{
						TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
							Tags: []string{"moblydemo"},
						},
					},
				},
			},
			MaximumDuration: durationpb.New(142200),
			AnalyticsName:   "wifi_endtoend_perbuild__wificell_perbuild__wifi_endtoend__tauto__new_build",
			TestArgs:        "",
		},
	}
	return SuiteInfo
}
