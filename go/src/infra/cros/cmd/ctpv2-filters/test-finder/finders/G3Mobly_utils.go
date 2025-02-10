// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package finders

import (
	"encoding/json"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type TestInfo struct {
	Name      string      `json:"name"`
	Tags      []string    `json:"tags"`
	ExtraInfo []ExtraInfo `json:"extra_info"`
}

type ExtraInfo struct {
	ExecutableName string `json:"executable_name"`
}

func translateSrcToMetadata(src []byte) (metaData []*api.TestCaseMetadata) {
	var testInfoList []TestInfo
	err := json.Unmarshal(src, &testInfoList)
	if err != nil {
		return nil
	}
	for _, testInfo := range testInfoList {
		tc := createTestCaseFromTestInfo(testInfo)
		metaData = append(metaData, tc)
	}
	return metaData
}

func createTestCaseFromTestInfo(testInfo TestInfo) *api.TestCaseMetadata {
	return &api.TestCaseMetadata{
		TestCase: &api.TestCase{
			Name: testInfo.Name,
			Tags: formatTags(testInfo.Tags),
		},
		TestCaseInfo: &api.TestCaseInfo{
			ExtraInfo: createExtraInfo(testInfo),
		},
	}
}

func formatTags(tags []string) (tagList []*api.TestCase_Tag) {
	for _, tag := range tags {
		tagList = append(tagList, &api.TestCase_Tag{Value: tag})
	}
	return tagList
}

func createExtraInfo(testInfo TestInfo) map[string]string {
	extraInfo := make(map[string]string)
	extraInfo["executable_name"] = testInfo.ExtraInfo[0].ExecutableName
	return extraInfo
}
